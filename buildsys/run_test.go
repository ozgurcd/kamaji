package buildsys

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildActionProcess(t *testing.T) {
	if os.Getenv("KAMAJI_BUILD_TEST_PROCESS") != "1" {
		return
	}
	operation := os.Args[len(os.Args)-1]
	if strings.HasPrefix(operation, "barrier-") {
		name := strings.TrimPrefix(operation, "barrier-")
		other := "a"
		if name == "a" {
			other = "b"
		}
		if err := os.WriteFile(name+".ready", nil, 0600); err != nil {
			t.Fatal(err)
		}
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			if _, err := os.Stat(other + ".ready"); err == nil {
				break
			}
			select {
			case <-deadline.C:
				t.Fatal("independent actions did not run concurrently")
			case <-tick.C:
			}
		}
		if err := os.MkdirAll("out", 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("out/"+name, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	counter, err := os.OpenFile("executions.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := counter.WriteString("executed\n"); err != nil {
		t.Fatal(err)
	}
	if err := counter.Close(); err != nil {
		t.Fatal(err)
	}
	switch os.Args[len(os.Args)-1] {
	case "fail":
		os.Exit(7)
	case "wait":
		time.Sleep(time.Minute)
	case "missing":
		return
	case "mutate":
		if err := os.WriteFile("input.txt", []byte("modified"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile("input.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("out", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("out/result", data, 0600); err != nil {
		t.Fatal(err)
	}
}

func actionFixture(t *testing.T) *Project {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := &Project{Root: t.TempDir(), Document: Document{Version: 1, Targets: []Target{{
		Name: "copy", Command: []string{binary, "-test.run=^TestBuildActionProcess$", "--", "copy"},
		Inputs: []string{"input.txt"}, Outputs: []string{"out/result"}, Cache: true,
		Env: map[string]string{"KAMAJI_BUILD_TEST_PROCESS": "1"},
	}}}}
	if err := p.validate(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, "input.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func buildOptions() Options {
	return Options{Jobs: 2, Stdout: io.Discard, Stderr: io.Discard, Grace: 10 * time.Millisecond}
}

func TestBuildCacheAndEvidence(t *testing.T) {
	p := actionFixture(t)
	first, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || first.Targets[0].Status != "executed" {
		t.Fatalf("first build: %+v %v", first, err)
	}
	second, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || second.Targets[0].Status != "cached" {
		t.Fatalf("cached build: %+v %v", second, err)
	}
	count, err := os.ReadFile(filepath.Join(p.Root, "executions.txt"))
	if err != nil || string(count) != "executed\n" {
		t.Fatal("cache hit reran the command")
	}
	if err := os.Remove(filepath.Join(p.Root, "out/result")); err != nil {
		t.Fatal(err)
	}
	restored, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || restored.Targets[0].Status != "cached" {
		t.Fatalf("restore: %+v %v", restored, err)
	}
	data, err := os.ReadFile(filepath.Join(p.Root, "out/result"))
	if err != nil || string(data) != "hello" {
		t.Fatal("cached artifact was not restored")
	}
	if _, err := os.Stat(filepath.Join(p.Root, ".kamaji/runs", restored.ID+".json")); err != nil {
		t.Fatal("run evidence missing")
	}
	if err := os.WriteFile(filepath.Join(p.Root, "input.txt"), []byte("world"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || changed.Targets[0].Status != "executed" || changed.Targets[0].Key == first.Targets[0].Key {
		t.Fatal("changed source did not invalidate cache")
	}
	cacheFile := filepath.Join(p.Root, ".kamaji/cache", changed.Targets[0].Key, "tree/out/result")
	if err := os.WriteFile(cacheFile, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || rebuilt.Targets[0].Status != "executed" {
		t.Fatal("corrupt cache was reused")
	}
}

func TestBuildRejectsSourceChangeAfterPlanning(t *testing.T) {
	p := actionFixture(t)
	options := buildOptions()
	options.OnEvent = func(event Event) {
		if event.State == "started" {
			if err := os.WriteFile(filepath.Join(p.Root, "input.txt"), []byte("changed-after-plan"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := p.Build(context.Background(), nil, options); err == nil {
		t.Fatal("source changed after planning but command still ran")
	}
	if _, err := os.Stat(filepath.Join(p.Root, "executions.txt")); !os.IsNotExist(err) {
		t.Fatal("stale action started its child process")
	}
}

func TestBuildParallelDependencies(t *testing.T) {
	p := actionFixture(t)
	base := p.Targets[0]
	p.Targets = nil
	for _, name := range []string{"a", "b"} {
		target := base
		target.Name, target.Cache = name, false
		target.Command = append([]string(nil), base.Command...)
		target.Command[len(target.Command)-1] = "barrier-" + name
		target.Outputs = []string{"out/" + name}
		p.Targets = append(p.Targets, target)
	}
	p.Targets = append(p.Targets, Target{Name: "all", Deps: []string{"a", "b"}})
	p.Default = []string{"all"}
	if err := p.validate(); err != nil {
		t.Fatal(err)
	}
	result, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || !result.Success || result.Targets[2].Status != "aggregate" {
		t.Fatalf("parallel build: %+v %v", result, err)
	}
}

func TestPlanReportsVerifiedCache(t *testing.T) {
	p := actionFixture(t)
	if _, err := p.Build(context.Background(), nil, buildOptions()); err != nil {
		t.Fatal(err)
	}
	plan, err := p.Plan(nil)
	if err != nil || plan.Targets[0].Status != "cached" {
		t.Fatal("plan did not explain the verified cache hit")
	}
}

func TestBuildPoliciesAndFailures(t *testing.T) {
	for _, scenario := range []string{"external", "stale", "missing", "mutate", "fail", "wait"} {
		t.Run(scenario, func(t *testing.T) {
			p := actionFixture(t)
			options := buildOptions()
			if scenario == "external" {
				p.Targets[0].Cache = false
				p.Targets[0].Effect = "external"
			} else if scenario == "stale" {
				plan, err := p.Plan(nil)
				if err != nil {
					t.Fatal(err)
				}
				options.ExpectPlan = plan.ID
				if err := os.WriteFile(filepath.Join(p.Root, "input.txt"), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				p.Targets[0].Command[len(p.Targets[0].Command)-1] = scenario
				if scenario == "wait" {
					p.Targets[0].Timeout = "30ms"
				}
			}
			result, err := p.Build(context.Background(), nil, options)
			if err == nil {
				t.Fatal("unsafe or failed build passed")
			}
			if scenario == "external" || scenario == "stale" {
				if _, err := os.Stat(filepath.Join(p.Root, ".kamaji")); !os.IsNotExist(err) {
					t.Fatal("preflight failure wrote runtime state")
				}
			} else if result == nil || result.Success {
				t.Fatal("failed action has no failure evidence")
			}
			if scenario == "fail" && result.Targets[0].ExitCode != 7 {
				t.Fatal("child exit code lost")
			}
			if scenario == "wait" && result.Targets[0].ExitCode != 124 {
				t.Fatal("deadline exit code lost")
			}
			if scenario == "external" {
				options.AllowEffects = true
				allowed, err := p.Build(context.Background(), nil, options)
				if err != nil || !allowed.Success {
					t.Fatal("explicit effect permission was ignored")
				}
			}
		})
	}
}
