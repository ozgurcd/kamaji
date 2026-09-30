package buildsys

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOutputTreeGlobDependencies(t *testing.T) {
	for _, pattern := range []string{"**/*.txt", "gen*/*.txt", "g[ae]nerated/**/value.txt", "**/**/value.txt"} {
		t.Run(pattern, func(t *testing.T) {
			p := &Project{Document: Document{Version: 1, Targets: []Target{
				{Name: "producer", Command: []string{"true"}, Outputs: []string{"generated"}},
				{Name: "consumer", Command: []string{"true"}, Inputs: []string{pattern}},
			}}}
			if err := p.validate(); err == nil || !strings.Contains(err.Error(), "without a dependency") {
				t.Fatalf("undeclared generated input accepted: %v", err)
			}
			p.Targets[1].Deps = []string{"producer"}
			if err := p.validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, pattern := range []string{"src/**/*.txt", "other/*.txt", "*.txt"} {
		p := &Project{Document: Document{Version: 1, Targets: []Target{
			{Name: "producer", Command: []string{"true"}, Outputs: []string{"generated"}},
			{Name: "consumer", Command: []string{"true"}, Inputs: []string{pattern}},
		}}}
		if err := p.validate(); err != nil {
			t.Fatalf("unrelated pattern %q requires dependency: %v", pattern, err)
		}
	}
	p := &Project{Document: Document{Version: 1, Targets: []Target{
		{Name: "self", Command: []string{"true"}, Inputs: []string{"**/*.txt"}, Outputs: []string{"generated"}},
	}}}
	if err := p.validate(); err == nil {
		t.Fatal("self-overlapping directory output accepted")
	}
}

func TestGeneratedDirectoryGlobBuild(t *testing.T) {
	p := actionFixture(t)
	p.Targets[0].Outputs = []string{"out"}
	p.Targets = append(p.Targets, Target{Name: "verify", Deps: []string{"copy"},
		Command: []string{p.Targets[0].Command[0], "-test.run=^$"}, Inputs: []string{"**/result"}})
	if _, err := p.Plan(nil); err != nil {
		t.Fatalf("clean-workspace plan: %v", err)
	}
	first, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || !first.Success {
		t.Fatalf("generated directory build: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(p.Root, "out")); err != nil {
		t.Fatal(err)
	}
	restored, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || !restored.Success || restored.Targets[0].Status != "cached" {
		t.Fatalf("generated directory restore: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(p.Root, "out/result"))
	if err != nil || string(data) != "hello" {
		t.Fatal("restored output content differs")
	}
}

func TestAffectedLocalProgramPaths(t *testing.T) {
	root := t.TempDir()
	for _, spelling := range []string{"./tools/compiler", "tools/compiler", filepath.Join(root, "tools/compiler")} {
		for _, toolOnly := range []bool{false, true} {
			p := &Project{Root: root, Document: Document{Version: 1, Targets: []Target{
				{Name: "compile", Command: []string{spelling}},
				{Name: "check", Deps: []string{"compile"}},
			}}}
			if toolOnly {
				p.Targets[0].Command = []string{"true"}
				p.Targets[0].Tools = []string{spelling}
			}
			for _, changed := range []string{"tools/compiler", "tools"} {
				affected, err := p.Affected([]string{changed})
				if err != nil || !reflect.DeepEqual(affected, []string{"compile", "check"}) {
					t.Errorf("program %q tool=%v changed=%q: %v %v", spelling, toolOnly, changed, affected, err)
				}
			}
			unrelated, err := p.Affected([]string{"elsewhere"})
			if err != nil || len(unrelated) != 0 {
				t.Errorf("unrelated path affected targets: %v %v", unrelated, err)
			}
		}
	}
	p := &Project{Root: root, Document: Document{Version: 1, Targets: []Target{
		{Name: "read", Command: []string{"true"}, Inputs: []string{"**/*.txt"}},
	}}}
	got, err := p.Affected([]string{"deleted-directory"})
	if err != nil || !reflect.DeepEqual(got, []string{"read"}) {
		t.Fatalf("deleted directory should affect recursive glob: %v %v", got, err)
	}
}

func TestGeneratedFileParentInputs(t *testing.T) {
	for _, pattern := range []string{"out", "out/**"} {
		t.Run(pattern, func(t *testing.T) {
			p := actionFixture(t)
			p.Targets = append(p.Targets, Target{Name: "verify", Deps: []string{"copy"},
				Command: []string{p.Targets[0].Command[0], "-test.run=^$"}, Inputs: []string{pattern}})
			if err := p.validate(); err != nil {
				t.Fatal(err)
			}
			plan, err := p.Plan(nil)
			if err != nil {
				t.Fatal(err)
			}
			options := buildOptions()
			options.ExpectPlan = plan.ID
			result, err := p.Build(context.Background(), nil, options)
			if err != nil || !result.Success {
				t.Fatalf("producer-created parent treated as source mutation: %v", err)
			}
			warm, err := p.Plan(nil)
			if err != nil || warm.ID != plan.ID {
				t.Fatal("generated parent changed plan identity")
			}
			if err := os.WriteFile(filepath.Join(p.Root, "out/source.txt"), []byte("source sibling"), 0600); err != nil {
				t.Fatal(err)
			}
			changed, err := p.Plan(nil)
			if err != nil || changed.ID == plan.ID {
				t.Fatal("non-generated sibling must still change plan identity")
			}
		})
	}
}

func TestAbsoluteGeneratedToolDependencies(t *testing.T) {
	root := t.TempDir()
	program := filepath.Join(root, "out/tool")
	quoted, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	text := fmt.Sprintf("version = 1\n[[targets]]\nname = 'producer'\ncommand = ['true']\noutputs = ['out']\n[[targets]]\nname = 'consumer'\ncommand = [%s]\n", quoted)
	filename := filepath.Join(root, "kamaji.toml")
	if err := os.WriteFile(filename, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, ""); err == nil || !strings.Contains(err.Error(), "without a dependency") {
		t.Fatalf("absolute generated executable bypassed validation: %v", err)
	}
	if err := os.WriteFile(filename, []byte(text+"deps = ['producer']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Plan(nil); err != nil {
		t.Fatalf("generated absolute executable must plan before it exists: %v", err)
	}
}

func TestPlanAppliesValidationDefaults(t *testing.T) {
	p := actionFixture(t)
	p.Targets[0].Slots = 0
	p.Targets[0].Effect = ""
	plan, err := p.Plan(nil)
	if err != nil {
		t.Fatal(err)
	}
	options := buildOptions()
	options.ExpectPlan = plan.ID
	if _, err := p.Build(context.Background(), nil, options); err != nil {
		t.Fatalf("fresh plan for programmatic project cannot build: %v", err)
	}
}
