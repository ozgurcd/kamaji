//go:build integration

package workflow_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kamaji/cmd"
	"kamaji/internal/process"
	"kamaji/obj"
	"kamaji/rt"
)

// TestReleaseWorkflow runs the checked-in examples through the real CLI dispatcher.
func TestReleaseWorkflow(t *testing.T) {
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"BUILD.yaml", "BUILD.strict.yaml", "kamaji.workspace.yaml", "rules/verify/main.go"} {
		if _, err := os.Stat(filepath.Join(source, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, program := range []string{"go", "python3", "node", "ruby"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Skipf("example integration requires %s", program)
		}
	}
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "out" && entry.IsDir() {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(workspace, rel), 0700)
		}
		if rel == filepath.Join("rules", "verify", "verify") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(workspace, rel), data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(workspace, "rules/verify/verify"), "./examples/release-workflow/rules/verify")
	build.Dir = filepath.Clean(filepath.Join(source, "../.."))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build verifier: %v\n%s", err, output)
	}
	run := func(args ...string) (string, error) {
		state := &rt.Runtime{Config: obj.RuntimeConfig{WorkingDir: workspace, TmpDir: filepath.Join(root, "runtime")}}
		command := cmd.NewCommand(cmd.Options{Runtime: state, LoadUserConfig: func() (map[string]string, error) {
			return nil, errors.New("explicit example runtimes must not load user Python configuration")
		}})
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs(args)
		err := command.Execute()
		return output.String(), err
	}
	if _, err := run("validate", "--all"); err != nil {
		t.Fatal(err)
	}
	explanation, err := run("explain", "verify-go", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(explanation), &details); err != nil {
		t.Fatal(err)
	}
	if details["language"] != "go" || details["execution_mode"] != "executable" {
		t.Fatal("Go example must use native execution")
	}
	for _, target := range []string{"inventory", "verify-go", "policy", "report"} {
		if _, err := run("run", target, "--timeout", "30s"); err != nil {
			t.Fatalf("%s: %v", target, err)
		}
	}
	reportPath := filepath.Join(workspace, "out/report.md")
	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "# Release readiness") || !strings.Contains(string(report), "inputs/app.txt") || !strings.Contains(string(report), "| sha256 |") {
		t.Fatal("report is missing its configured title, file or digest column")
	}
	if _, err := run("run", "policy", "--build", "BUILD.strict.yaml"); process.ExitCode(err) != 3 {
		t.Fatalf("strict policy must exit 3, got %v", err)
	}
	if err := os.WriteFile(reportPath, []byte("original report\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := run("run", "report", "--isolated", "--keep-execroot"); err != nil {
		t.Fatal(err)
	}
	unchanged, err := os.ReadFile(reportPath)
	if err != nil || string(unchanged) != "original report\n" {
		t.Fatal("isolated reporting changed the original workspace")
	}
	found := false
	err = filepath.WalkDir(filepath.Join(root, "runtime"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "report.md" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			found = found || bytes.Equal(data, report)
		}
		return nil
	})
	if err != nil || !found {
		t.Fatal("isolated output was not retained")
	}
	inputPath := filepath.Join(workspace, "inputs/app.txt")
	input, err := os.ReadFile(inputPath)
	if err != nil || len(input) == 0 {
		t.Fatal("missing tamper fixture")
	}
	input[0] ^= 1 // Preserve length so the digest check, not just size, must catch it.
	if err := os.WriteFile(inputPath, input, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := run("run", "verify-go"); process.ExitCode(err) != 1 {
		t.Fatalf("tampered input must fail Go verification with exit 1, got %v", err)
	}
}
