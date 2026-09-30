package buildsys

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"kamaji/internal/fsutil"
	"kamaji/internal/process"
)

type Options struct {
	Jobs         int
	AllowEffects bool
	ExpectPlan   string
	NoCache      bool
	Grace        time.Duration
	Stdout       io.Writer
	Stderr       io.Writer
	OnEvent      func(Event)
}

type Event struct {
	Schema string `json:"schema"`
	Target string `json:"target"`
	State  string `json:"state"`
}

type TargetResult struct {
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	Key        string            `json:"key,omitempty"`
	ExitCode   int               `json:"exit_code"`
	Outputs    []Artifact        `json:"outputs,omitempty"`
	Inputs     []Artifact        `json:"inputs,omitempty"`
	Tools      map[string]string `json:"tools,omitempty"`
	Definition string            `json:"definition_sha256,omitempty"`
	Duration   int64             `json:"duration_ms"`
	Error      string            `json:"error,omitempty"`
	signature  string
	err        error
}

type Result struct {
	Schema   string         `json:"schema"`
	ID       string         `json:"id"`
	Plan     string         `json:"plan"`
	Started  time.Time      `json:"started"`
	Finished time.Time      `json:"finished"`
	Success  bool           `json:"success"`
	Targets  []TargetResult `json:"targets"`
}

type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (w lockedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(data)
}

func (p *Project) execute(ctx context.Context, t Target, dependencies map[string]string, options Options, nonce string, planned actionState) TargetResult {
	started := time.Now()
	r := TargetResult{Name: t.Name, Status: "failed"}
	finish := func(err error) TargetResult {
		r.Duration = time.Since(started).Milliseconds()
		if err != nil {
			r.ExitCode = process.ExitCode(err)
			r.Error = err.Error()
			r.err = err
		}
		r.signature = digestJSON(struct {
			Key     string
			Outputs []Artifact
			Nonce   string
		}{r.Key, r.Outputs, func() string {
			if len(t.Command) > 0 && len(t.Outputs) == 0 {
				return nonce
			}
			return ""
		}()})
		return r
	}
	projected, err := p.state(t, planned.Generated)
	if err != nil {
		return finish(err)
	}
	if digestJSON(projected) != digestJSON(planned) {
		return finish(fmt.Errorf("target %s inputs changed after planning; plan again", t.Name))
	}
	state, err := p.state(t, nil)
	if err != nil {
		return finish(err)
	}
	r.Key = actionKey(state, dependencies)
	r.Inputs, r.Tools, r.Definition = state.Inputs, state.Tools, state.Definition
	if len(t.Command) == 0 {
		r.Status = "aggregate"
		return finish(nil)
	}
	if t.Cache && !options.NoCache {
		artifacts, hit, err := p.cached(t, r.Key, true)
		if err != nil {
			return finish(err)
		}
		if hit {
			after, err := p.state(t, nil)
			if err != nil || digestJSON(after) != digestJSON(state) {
				return finish(fmt.Errorf("target %s inputs changed during cache restoration", t.Name))
			}
			r.Status, r.Outputs = "cached", artifacts
			return finish(nil)
		}
	}
	if t.Timeout != "" {
		duration, _ := time.ParseDuration(t.Timeout)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}
	env := environment(t, p.Root)
	program, err := resolveProgram(p.Root, t.Command[0], env["PATH"])
	if err != nil {
		return finish(err)
	}
	command := exec.Command(program, t.Command[1:]...)
	command.Dir = p.Root
	command.Stdout, command.Stderr = options.Stdout, options.Stderr
	// Builds are non-interactive; parallel actions must never compete for stdin.
	for key, value := range env {
		command.Env = append(command.Env, key+"="+value)
	}
	sort.Strings(command.Env)
	if _, err := process.Run(ctx, command, options.Grace); err != nil {
		return finish(err)
	}
	after, err := p.state(t, nil)
	if err != nil {
		return finish(err)
	}
	if digestJSON(state) != digestJSON(after) {
		return finish(fmt.Errorf("target %s inputs or tools changed during execution", t.Name))
	}
	r.Outputs, err = p.snapshot(t.Outputs, nil)
	if err != nil {
		return finish(fmt.Errorf("target %s did not produce its declared outputs: %w", t.Name, err))
	}
	if t.Cache && !options.NoCache {
		if err := p.saveCache(t, r.Key, r.Outputs); err != nil {
			return finish(err)
		}
	}
	r.Status = "executed"
	return finish(nil)
}

// Build validates policy before creating storage, serializes workspace writers,
// then schedules independent ready actions up to the requested slot budget.
func (p *Project) Build(ctx context.Context, names []string, options Options) (*Result, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if options.Jobs <= 0 {
		return nil, fmt.Errorf("jobs must be positive")
	}
	plan, err := p.Plan(names)
	if err != nil {
		return nil, err
	}
	if options.ExpectPlan != "" && options.ExpectPlan != plan.ID {
		return nil, fmt.Errorf("plan changed; inspect a new plan before execution")
	}
	index := p.Index()
	for _, item := range plan.Targets {
		if item.Effect == "external" && !options.AllowEffects {
			return nil, fmt.Errorf("target %s has external effects; review the plan and use --allow-effects", item.Name)
		}
		if item.Slots > options.Jobs {
			return nil, fmt.Errorf("target %s needs %d slots; increase --jobs", item.Name, item.Slots)
		}
	}
	root, err := p.storage()
	if err != nil {
		return nil, err
	}
	lease, err := fsutil.Lock(filepath.Join(root, "build.lock"), true)
	if err != nil {
		return nil, err
	}
	defer lease.Close()
	// Recheck after acquiring the lease so an earlier writer cannot stale the plan.
	lockedPlan, err := p.Plan(names)
	if err != nil || lockedPlan.ID != plan.ID {
		return nil, fmt.Errorf("project changed while acquiring build lease; plan again")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	result := &Result{Schema: "kamaji.result.v1", ID: hex.EncodeToString(random[:]), Plan: plan.ID, Started: time.Now().UTC()}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Stderr == nil {
		options.Stderr = io.Discard
	}
	var outputMu sync.Mutex
	options.Stdout = lockedWriter{&outputMu, options.Stdout}
	options.Stderr = lockedWriter{&outputMu, options.Stderr}
	emit := func(name, state string) {
		if options.OnEvent != nil {
			options.OnEvent(Event{Schema: "kamaji.event.v1", Target: name, State: state})
		}
	}
	finished := make(map[string]TargetResult)
	running := map[string]bool{}
	completed := make(chan TargetResult, len(plan.Targets))
	used := 0
	var failure error
	for len(finished) < len(plan.Targets) {
		if failure == nil && ctx.Err() != nil {
			failure = ctx.Err()
		}
		for _, item := range plan.Targets {
			if _, ok := finished[item.Name]; ok || running[item.Name] {
				continue
			}
			if failure != nil {
				finished[item.Name] = TargetResult{Name: item.Name, Status: "blocked", ExitCode: 1, Error: "dependency failure or build cancellation"}
				emit(item.Name, "blocked")
				continue
			}
			t := index[item.Name]
			if used+t.Slots > options.Jobs {
				continue
			}
			ready := true
			dependencies := map[string]string{}
			for _, dep := range t.Deps {
				value, ok := finished[dep]
				if !ok {
					ready = false
					break
				}
				dependencies[dep] = value.signature
			}
			if !ready {
				continue
			}
			running[t.Name] = true
			used += t.Slots
			emit(t.Name, "started")
			go func() { completed <- p.execute(ctx, t, dependencies, options, result.ID, item.state) }()
		}
		if len(running) == 0 {
			break
		}
		value := <-completed
		delete(running, value.Name)
		used -= index[value.Name].Slots
		finished[value.Name] = value
		emit(value.Name, value.Status)
		if value.err != nil && failure == nil {
			failure = fmt.Errorf("target %s: %w", value.Name, value.err)
			cancel()
		}
	}
	for _, item := range plan.Targets {
		result.Targets = append(result.Targets, finished[item.Name])
	}
	result.Success = failure == nil
	result.Finished = time.Now().UTC()
	if err := writeJSON(filepath.Join(root, "runs", result.ID+".json"), result); err != nil {
		return result, errors.Join(failure, err)
	}
	return result, failure
}
