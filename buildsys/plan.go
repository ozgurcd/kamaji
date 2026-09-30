package buildsys

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Artifact struct {
	Path   string `json:"path"`
	Digest string `json:"sha256,omitempty"`
	Mode   uint32 `json:"mode"`
	Dir    bool   `json:"directory,omitempty"`
}

type PlannedTarget struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Deps        []string `json:"deps,omitempty"`
	Inputs      []string `json:"inputs,omitempty"`
	Outputs     []string `json:"outputs,omitempty"`
	Program     string   `json:"program,omitempty"`
	Arguments   int      `json:"argument_count"`
	Environment []string `json:"environment_names,omitempty"`
	Effect      string   `json:"effect"`
	Cache       bool     `json:"cache"`
	Slots       int      `json:"slots"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason"`
	Fingerprint string   `json:"fingerprint"`
	state       actionState
}

type Plan struct {
	Schema  string          `json:"schema"`
	ID      string          `json:"id"`
	Targets []PlannedTarget `json:"targets"`
}

// actionState includes only digests of values. Neither plans nor run records
// serialize command arguments or environment values.
type actionState struct {
	Definition string            `json:"definition"`
	Platform   string            `json:"platform"`
	Inputs     []Artifact        `json:"inputs"`
	Tools      map[string]string `json:"tools"`
	Generated  map[string]string `json:"generated,omitempty"`
}

func digestJSON(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func actionKey(state actionState, dependencies map[string]string) string {
	return digestJSON(struct {
		Schema string
		State  actionState
		Deps   map[string]string
	}{"kamaji.action.v1", state, dependencies})
}

func fileDigest(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("input is not a regular file: %s", filename)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func safePath(root, relative string) (string, error) {
	if err := validPath(relative, false); err != nil {
		return "", err
	}
	current := root
	for _, part := range strings.Split(relative, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
			return "", fmt.Errorf("symlink or special file in project path %s", relative)
		}
	}
	return current, nil
}

func produced(name string, generated map[string]string) bool {
	for output := range generated {
		if within(output, name) {
			return true
		}
	}
	return false
}

// snapshot expands file/directory inputs and ** globs. Generated inputs are
// represented by producer fingerprints during planning, then hashed after the
// producer completes. Wildcards must match something or name a producer output.
func (p *Project) snapshot(patterns []string, generated map[string]string) ([]Artifact, error) {
	entries := map[string]Artifact{}
	for _, pattern := range patterns {
		base := pattern
		if index := strings.IndexAny(base, "*?["); index >= 0 {
			base = filepath.ToSlash(filepath.Dir(base[:index] + "x"))
		}
		matched := false
		for output := range generated {
			if intersectsTree(pattern, output) {
				matched = true
			}
		}
		if produced(base, generated) {
			continue
		}
		start := p.Root
		var err error
		if base != "." {
			start, err = safePath(p.Root, base)
			if err != nil {
				return nil, err
			}
		}
		err = filepath.WalkDir(start, func(filename string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) && matched {
					return nil
				}
				return walkErr
			}
			rel, err := filepath.Rel(p.Root, filename)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if reserved(strings.Split(rel, "/")[0]) || produced(rel, generated) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink in input tree: %s", rel)
			}
			if !covers(pattern, rel) {
				return nil
			}
			matched = true
			info, err := entry.Info()
			if err != nil {
				return err
			}
			// Producers may create parent directories for generated files. Their
			// metadata is not source state, but non-generated siblings still are.
			if info.IsDir() {
				for output := range generated {
					if within(rel, output) {
						return nil
					}
				}
			}
			a := Artifact{Path: rel, Mode: uint32(info.Mode().Perm()), Dir: info.IsDir()}
			if !a.Dir {
				a.Digest, err = fileDigest(filename)
				if err != nil {
					return err
				}
			}
			entries[rel] = a
			return nil
		})
		if err != nil {
			return nil, err
		}
		if !matched {
			return nil, fmt.Errorf("input pattern %q matches no files or directories", pattern)
		}
	}
	result := make([]Artifact, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

func environment(t Target, root string) map[string]string {
	// A small deterministic baseline keeps native tools practical. Additional
	// environment must be declared; all effective values participate in the key.
	values := map[string]string{"PWD": root, "LANG": "C", "LC_ALL": "C"}
	for _, name := range append([]string{"PATH", "HOME", "TMPDIR"}, t.PassEnv...) {
		if value, ok := os.LookupEnv(name); ok {
			values[name] = value
		}
	}
	for name, value := range t.Env {
		values[name] = value
	}
	values["PWD"] = root
	return values
}

// localProgramPath identifies explicit project paths without requiring the tool
// to exist: generated and deleted executables must use the same path identity.
// Bare program names are PATH lookups, not implicit project-relative paths.
func localProgramPath(root, name string) (string, bool) {
	if !strings.Contains(name, "/") {
		return "", false
	}
	relative := filepath.Clean(name)
	if filepath.IsAbs(relative) {
		if root == "" {
			return "", false
		}
		var err error
		relative, err = filepath.Rel(root, relative)
		if err != nil {
			return "", false
		}
	}
	relative = filepath.ToSlash(relative)
	return relative, validPath(relative, false) == nil
}

func resolveProgram(root, name, searchPath string) (string, error) {
	filename := name
	if relative, local := localProgramPath(root, name); local {
		var err error
		filename, err = safePath(root, relative)
		if err != nil {
			return "", err
		}
	} else if !filepath.IsAbs(name) {
		if strings.Contains(name, "/") {
			var err error
			filename, err = safePath(root, filepath.ToSlash(filepath.Clean(name)))
			if err != nil {
				return "", err
			}
		} else {
			filename = ""
			for _, dir := range filepath.SplitList(searchPath) {
				if !filepath.IsAbs(dir) {
					continue
				}
				candidate := filepath.Join(dir, name)
				if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
					filename = candidate
					break
				}
			}
			if filename == "" {
				return "", fmt.Errorf("program %q not found on absolute PATH entries", name)
			}
		}
	}
	info, err := os.Stat(filename)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("program %q is not an executable regular file", name)
	}
	return filename, nil
}

func (p *Project) state(t Target, generated map[string]string) (actionState, error) {
	state := actionState{Platform: runtime.GOOS + "/" + runtime.GOARCH, Tools: map[string]string{}, Generated: generated}
	env := environment(t, p.Root)
	state.Definition = digestJSON(struct {
		Target Target
		Env    map[string]string
	}{t, env})
	var err error
	state.Inputs, err = p.snapshot(t.Inputs, generated)
	if err != nil {
		return state, fmt.Errorf("target %s: %w", t.Name, err)
	}
	for _, output := range t.Outputs {
		if _, err := safePath(p.Root, output); err != nil {
			return state, err
		}
	}
	programs := append([]string(nil), t.Tools...)
	if len(t.Command) > 0 {
		programs = append(programs, t.Command[0])
	}
	for _, program := range programs {
		if relative, local := localProgramPath(p.Root, program); local && produced(relative, generated) {
			continue
		}
		filename, err := resolveProgram(p.Root, program, env["PATH"])
		if err != nil {
			return state, fmt.Errorf("target %s: %w", t.Name, err)
		}
		digest, err := fileDigest(filename)
		if err != nil {
			return state, err
		}
		state.Tools[filename] = digest
	}
	return state, nil
}

func (p *Project) Plan(names []string) (*Plan, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	order, err := p.Select(names)
	if err != nil {
		return nil, err
	}
	index := p.Index()
	plan := &Plan{Schema: "kamaji.plan.v1"}
	fingerprints := map[string]string{}
	for _, name := range order {
		t := index[name]
		closure, _ := p.Select([]string{name})
		generated := map[string]string{}
		dependencies := map[string]string{}
		for _, dep := range closure {
			if dep == name {
				continue
			}
			dependencies[dep] = fingerprints[dep]
			for _, output := range index[dep].Outputs {
				generated[output] = fingerprints[dep]
			}
		}
		state, err := p.state(t, generated)
		if err != nil {
			return nil, err
		}
		fingerprint := digestJSON(struct {
			State actionState
			Deps  map[string]string
		}{state, dependencies})
		fingerprints[name] = fingerprint
		item := PlannedTarget{Name: name, Description: t.Description, Deps: t.Deps, Inputs: t.Inputs, Outputs: t.Outputs, Effect: t.Effect, Cache: t.Cache, Slots: t.Slots, Fingerprint: fingerprint, Status: "run", Reason: "action is not cacheable", state: state}
		if t.Cache {
			item.Reason = "no verified cache entry for current inputs, command, environment, and tools"
			if len(t.Deps) == 0 {
				if _, hit, _ := p.cached(t, actionKey(state, map[string]string{}), false); hit {
					item.Status, item.Reason = "cached", "verified local artifacts available; missing outputs will be restored"
				}
			}
		}
		if len(t.Deps) > 0 {
			item.Status, item.Reason = "pending", "wait for dependencies before checking complete inputs"
		}
		if len(t.Command) > 0 {
			item.Program, item.Arguments = t.Command[0], len(t.Command)-1
		} else {
			item.Status, item.Reason = "aggregate", "dependency-only target"
		}
		for name := range environment(t, p.Root) {
			item.Environment = append(item.Environment, name)
		}
		sort.Strings(item.Environment)
		plan.Targets = append(plan.Targets, item)
	}
	// Cache availability does not change what a plan authorizes. Bind identity
	// to selected actions and their content, not transient cache status.
	plan.ID = digestJSON(fingerprints)
	return plan, nil
}

// Affected handles deleted files too; callers supply project-relative paths.
// It reports the reverse dependency closure, in topological order.
func (p *Project) Affected(paths []string) ([]string, error) {
	for _, name := range paths {
		if err := validPath(name, false); err != nil {
			return nil, err
		}
	}
	all := make([]string, 0, len(p.Targets))
	for _, t := range p.Targets {
		all = append(all, t.Name)
	}
	order, err := p.Select(all)
	if err != nil {
		return nil, err
	}
	index := p.Index()
	marked := map[string]bool{}
	result := []string{}
	for _, name := range order {
		t := index[name]
		for _, changed := range paths {
			info, statErr := os.Lstat(filepath.Join(p.Root, filepath.FromSlash(changed)))
			mayBeTree := statErr != nil || info.IsDir()
			if changed == filepath.Base(p.File) || changed == "kamaji.toml" || changed == "kamaji.yaml" || changed == "kamaji.yml" {
				marked[name] = true
			}
			for _, input := range append(append([]string(nil), t.Inputs...), t.Outputs...) {
				if covers(input, changed) || mayBeTree && intersectsTree(input, changed) {
					marked[name] = true
				}
			}
			programs := append([]string(nil), t.Tools...)
			if len(t.Command) > 0 {
				programs = append(programs, t.Command[0])
			}
			for _, program := range programs {
				if relative, local := localProgramPath(p.Root, program); local {
					marked[name] = marked[name] || relative == changed || mayBeTree && within(changed, relative)
				}
			}
		}
		for _, dep := range t.Deps {
			marked[name] = marked[name] || marked[dep]
		}
		if marked[name] {
			result = append(result, name)
		}
	}
	return result, nil
}
