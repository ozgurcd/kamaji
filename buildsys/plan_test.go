package buildsys

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func planFixture(t *testing.T) *Project {
	t.Helper()
	p, err := Parse("kamaji.toml", []byte(`version = 1
default = ["check"]
[[targets]]
name = "compile"
command = ["go", "version"]
inputs = ["src/**/*.txt"]
outputs = ["out/app"]
cache = true
env = {DEMO_VALUE = "synthetic-private-value"}
[[targets]]
name = "check"
deps = ["compile"]
inputs = ["out/app"]
command = ["out/app"]
`))
	if err != nil {
		t.Fatal(err)
	}
	p.Root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(p.Root, "src/nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, "src/nested/input.txt"), []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuildPlanAndAffected(t *testing.T) {
	p := planFixture(t)
	p.Targets[1].Command[0] = "./out/app"
	first, err := p.Plan(nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := p.Plan(nil)
	if err != nil || first.ID != again.ID {
		t.Fatal("unchanged inputs produced a different plan")
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "synthetic-private-value") {
		t.Fatal("plan exposed environment values")
	}
	if len(first.Targets) != 2 || first.Targets[1].Status != "pending" {
		t.Fatal("generated executable should be planned before it exists")
	}
	if _, err := os.Stat(filepath.Join(p.Root, ".kamaji")); !os.IsNotExist(err) {
		t.Fatal("planning wrote runtime storage")
	}
	affected, err := p.Affected([]string{"src/nested/input.txt"})
	if err != nil || !reflect.DeepEqual(affected, []string{"compile", "check"}) {
		t.Fatalf("affected %v: %v", affected, err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, "src/nested/input.txt"), []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := p.Plan(nil)
	if err != nil || changed.ID == first.ID {
		t.Fatal("content change did not invalidate plan")
	}
	p.Targets[0].Env["DEMO_VALUE"] = "changed-private-value"
	envChanged, err := p.Plan(nil)
	if err != nil || envChanged.ID == changed.ID {
		t.Fatal("environment change did not invalidate plan")
	}
}

func TestBuildPlanRejectsUnsafeInputs(t *testing.T) {
	p := planFixture(t)
	p.Targets[0].Inputs = []string{"missing.txt"}
	if _, err := p.Plan(nil); err == nil {
		t.Fatal("missing source accepted")
	}
	p.Targets[0].Inputs = []string{"linked.txt"}
	if err := os.Symlink("src/nested/input.txt", filepath.Join(p.Root, "linked.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Plan(nil); err == nil {
		t.Fatal("symlink source accepted")
	}
	p.Targets[0].Inputs = []string{"src"}
	if err := os.Symlink("src", filepath.Join(p.Root, "out")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Plan(nil); err == nil {
		t.Fatal("symlink output parent accepted")
	}
}
