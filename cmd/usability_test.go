package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSharesRunPreconditions(t *testing.T) {
	root := commandFixture(t)
	write(t, filepath.Join(root, "kamaji.workspace.yaml"), "rules_directory: rules\n")
	if _, err := invoke("validate", "demo"); err == nil || !strings.Contains(err.Error(), "workspace_vars") {
		t.Fatalf("missing execution prerequisite accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tmp")); !os.IsNotExist(err) {
		t.Fatal("validation wrote runtime storage")
	}
}

func TestValidateAllAndScopedFlags(t *testing.T) {
	root := commandFixture(t)
	write(t, filepath.Join(root, "BUILD.yaml"), "targets:\n  - name: demo\n    rule: demo/run.py\n  - name: invalid\n    rule: missing.py\n")
	out, err := invoke("validate", "--all")
	if err == nil || !strings.Contains(out, "demo") || !strings.Contains(out, "invalid") {
		t.Fatalf("expected per-target results and failure: %q, %v", out, err)
	}
	out, err = invoke("validate", "--help")
	if err != nil || strings.Contains(out, "--cleanup") || strings.Contains(out, "--keep-execroot") {
		t.Fatal("validation advertises execution-only flags")
	}
	if _, err := invoke("validate", "demo", "--cleanup"); err == nil {
		t.Fatal("ignored flag accepted")
	}
}
