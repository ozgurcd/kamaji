package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kamaji/buildsys"
)

func TestBuildCLI(t *testing.T) {
	root, options := workflowOptions(t)
	invoke := func(args ...string) (string, error) {
		command := NewCommand(options)
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(io.Discard)
		command.SetArgs(args)
		err := command.Execute()
		return output.String(), err
	}
	if _, err := workflowInvoke(options, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "kamaji.toml")); err != nil {
		t.Fatal("default scaffold must create a single TOML build document")
	}
	if _, err := workflowInvoke(options, "init"); err == nil {
		t.Fatal("existing TOML scaffold overwritten")
	}
	for _, args := range [][]string{{"targets"}, {"validate", "--all"}, {"doctor"}, {"explain", "hello", "--json"}} {
		out, err := invoke(args...)
		if err != nil || !strings.Contains(out, "hello") {
			t.Fatalf("graph inspection %v: %s %v", args, out, err)
		}
	}
	if _, err := invoke("validate", "--all", "--python", "unused"); err == nil {
		t.Fatal("legacy interpreter flag silently ignored on graph project")
	}
	out, err := workflowInvoke(options, "plan", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var plan buildsys.Plan
	if err := json.Unmarshal([]byte(out), &plan); err != nil || plan.Schema != "kamaji.plan.v1" {
		t.Fatalf("plan protocol: %s %v", out, err)
	}
	out, err = invoke("build", "--json", "--expect-plan", plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	var result buildsys.Result
	if err := json.Unmarshal([]byte(out), &result); err != nil || !result.Success {
		t.Fatalf("build protocol: %s %v", out, err)
	}
	out, err = workflowInvoke(options, "history", result.ID)
	if err != nil || !strings.Contains(out, result.ID) {
		t.Fatalf("history: %s %v", out, err)
	}
	out, err = workflowInvoke(options, "affected", "kamaji.toml", "--json")
	if err != nil || !strings.Contains(out, "hello") {
		t.Fatalf("affected: %s %v", out, err)
	}
}
