package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kamaji/obj"
	"kamaji/rt"
)

func workflowOptions(t *testing.T) (string, Options) {
	t.Helper()
	root := t.TempDir()
	python, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	state := &rt.Runtime{Config: obj.RuntimeConfig{WorkingDir: root, TmpDir: filepath.Join(root, "runtime")}}
	t.Setenv("KAMAJI_PYTHON", "")
	return root, Options{Runtime: state, UserHomeDir: func() (string, error) { return root, nil }, LoadUserConfig: func() (map[string]string, error) { return map[string]string{"python": python}, nil }}
}
func workflowInvoke(options Options, args ...string) (string, error) {
	command := NewCommand(options)
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs(args)
	err := command.Execute()
	return output.String(), err
}

func TestScaffoldValidateExplainAndCompletion(t *testing.T) {
	root, options := workflowOptions(t)
	if _, err := workflowInvoke(options, "init", "--template", "minimal"); err != nil {
		t.Fatal(err)
	}
	build := filepath.Join(root, "BUILD.yaml")
	original, err := os.ReadFile(build)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workflowInvoke(options, "init", "--template", "minimal"); err == nil {
		t.Fatal("scaffolding overwrote existing files")
	}
	after, err := os.ReadFile(build)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatal("existing build changed")
	}
	write(t, build, strings.ReplaceAll(string(original), "Hello from Kamaji", "PUBLIC_VALUE_MUST_BE_REDACTED"))
	for _, args := range [][]string{{"validate", "--all"}, {"doctor"}, {"--build", build, "validate", "hello"}} {
		out, err := workflowInvoke(options, args...)
		if err != nil || !strings.Contains(out, "hello: valid") {
			t.Fatalf("validation: %q %v", out, err)
		}
	}
	out, err := workflowInvoke(options, "explain", "hello", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "PUBLIC_VALUE_MUST_BE_REDACTED") {
		t.Fatal("explain disclosed option value")
	}
	var explanation explanation
	if err := json.Unmarshal([]byte(out), &explanation); err != nil {
		t.Fatal(err)
	}
	if explanation.Options["greeting"].Source != "build configuration" || explanation.PythonSource != "user configuration" || explanation.Workspace != root {
		t.Fatal("incorrect explanation provenance")
	}
	command := NewCommand(options)
	run, _, err := command.Find([]string{"run"})
	if err != nil {
		t.Fatal(err)
	}
	completions, _ := run.ValidArgsFunction(run, nil, "he")
	if len(completions) != 1 || completions[0] != "hello\tPrint a greeting" {
		t.Fatalf("completion: %v", completions)
	}
	if _, err := os.Stat(options.Runtime.Config.TmpDir); !os.IsNotExist(err) {
		t.Fatal("read-only workflow created runtime storage")
	}
}

func TestValidationMissingInterpreterAndAllFailures(t *testing.T) {
	root, options := workflowOptions(t)
	if _, err := workflowInvoke(options, "init", "--template", "minimal"); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"run", "validate", "doctor"} {
		if _, err := workflowInvoke(options, command, "hello", "--python", filepath.Join(root, "absent")); err == nil || !strings.Contains(err.Error(), "interpreter unavailable") {
			t.Fatalf("%s missed interpreter: %v", command, err)
		}
	}
	if _, err := os.Stat(options.Runtime.Config.TmpDir); !os.IsNotExist(err) {
		t.Fatal("failed preflight wrote runtime storage")
	}
	write(t, filepath.Join(root, "BUILD.yaml"), "targets:\n  - name: good\n    rule: hello/rule.py\n  - name: broken\n    rule: missing.py\n")
	out, err := workflowInvoke(options, "validate", "--all")
	if err == nil || !strings.Contains(out, "good: valid") || !strings.Contains(out, "broken: invalid") {
		t.Fatalf("all-target results: %q %v", out, err)
	}
}

func TestUserInstallationAndExplicitRequirements(t *testing.T) {
	root, options := workflowOptions(t)
	if _, err := workflowInvoke(options, "init", "--template", "minimal"); err != nil {
		t.Fatal(err)
	}
	if _, err := workflowInvoke(options, "rules-directory-create", "--user"); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(root, ".local/share/kamaji/rules/hello/rule.py")
	if _, err := os.Stat(installed); err != nil {
		t.Fatal("user installation missing")
	}
	requirements := filepath.Join(root, "custom-requirements.txt")
	write(t, requirements, "")
	called := false
	options.SetupPython = func() error {
		called = true
		if options.Runtime.RequirementsFile != requirements {
			t.Fatal("explicit requirements ignored")
		}
		return nil
	}
	if _, err := workflowInvoke(options, "setup-python-env", "--user", "--requirements", requirements); err != nil || !called {
		t.Fatalf("setup callback: %v", err)
	}
	if _, err := workflowInvoke(options, "rules-directory-delete", "--user"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Fatal("user installation not removed")
	}
	if _, err := workflowInvoke(options, "rules-directory-create", "--user", "--install-root", root); err == nil {
		t.Fatal("conflicting installation scopes accepted")
	}
}

func TestStorageCommandsAndRetainedPath(t *testing.T) {
	root, options := workflowOptions(t)
	if _, err := workflowInvoke(options, "init", "--template", "minimal"); err != nil {
		t.Fatal(err)
	}
	options.RunTarget = func(obj.WorkspaceConfig, obj.ExecTarget, ...string) error {
		options.Runtime.Config.ExecRootDir = filepath.Join(options.Runtime.Config.TmpDir, "execroot", "fixture-run")
		write(t, filepath.Join(options.Runtime.Config.ExecRootDir, ".complete"), "")
		return errors.New("synthetic child failure")
	}
	out, err := workflowInvoke(options, "run", "hello", "--keep-execroot")
	if err == nil || !strings.Contains(out, "Retained execution files:") {
		t.Fatalf("retained path missing: %q %v", out, err)
	}
	for _, args := range [][]string{{"runs", "list"}, {"runs", "show", "fixture-run"}, {"cache", "clean", "--runs", "--dry-run"}} {
		out, err := workflowInvoke(options, args...)
		if err != nil || !strings.Contains(out, "fixture-run") {
			t.Fatalf("storage inspection: %q %v", out, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "runtime/execroot/fixture-run/.complete")); err != nil {
		t.Fatal("dry-run removed execution files")
	}
	if _, err := workflowInvoke(options, "runs", "show", "../fixture-run"); err == nil {
		t.Fatal("traversing execution name accepted")
	}
	if _, err := workflowInvoke(options, "cache", "clean", "--runs"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "runtime/execroot")); !os.IsNotExist(err) {
		t.Fatal("explicit cleanup left execution files")
	}
}

func TestScaffoldRefusesExistingAndLinkedDestinations(t *testing.T) {
	root, options := workflowOptions(t)
	write(t, filepath.Join(root, "rules/hello/rule.py"), "preserve")
	if _, err := workflowInvoke(options, "init", "--template", "minimal"); err == nil {
		t.Fatal("existing rule overwritten")
	}
	if _, err := os.Stat(filepath.Join(root, "BUILD.yaml")); !os.IsNotExist(err) {
		t.Fatal("preflight failure partially scaffolded workspace")
	}
	destination := t.TempDir()
	if err := os.Symlink(destination, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := workflowInvoke(options, "init", "linked"); err == nil {
		t.Fatal("linked destination accepted")
	}
}

func TestCacheCommandBudgetAndJSON(t *testing.T) {
	_, options := workflowOptions(t)
	for i := 1; i <= 2; i++ {
		write(t, filepath.Join(options.Runtime.Config.TmpDir, "cache", fmt.Sprintf("%064x", i), "file"), "data")
	}
	out, err := workflowInvoke(options, "cache", "status", "--json")
	var entries []rt.StorageEntry
	if err != nil || json.Unmarshal([]byte(out), &entries) != nil || len(entries) != 2 {
		t.Fatalf("cache inventory: %q %v", out, err)
	}
	out, err = workflowInvoke(options, "cache", "prune", "--max-bytes", "0", "--dry-run")
	if err != nil || strings.Count(out, "Would remove") != 2 {
		t.Fatalf("cache preview: %q %v", out, err)
	}
	out, err = workflowInvoke(options, "cache", "prune", "--max-bytes", "0")
	if err != nil || strings.Count(out, "Removed") != 2 {
		t.Fatalf("cache prune: %q %v", out, err)
	}
	entries, err = options.Runtime.StorageEntries("cache")
	if err != nil || len(entries) != 0 {
		t.Fatal("cache prune did not apply")
	}
}

func TestExplainDefaultsIsolationAndVerifiedCache(t *testing.T) {
	root, options := workflowOptions(t)
	if _, err := workflowInvoke(options, "init", "--template", "minimal"); err != nil {
		t.Fatal(err)
	}
	payload := []byte("public artifact")
	sum := sha256.Sum256(payload)
	digest := fmt.Sprintf("%x", sum)
	platform := runtime.GOOS + "_" + runtime.GOARCH
	write(t, filepath.Join(root, "kamaji.workspace.yaml"), "rules_directory: rules\nworkspace_vars:\n  - org_domain: fixture.invalid\nthird_party:\n  - name: tool\n    file_path: tool\n    url: {"+platform+": 'https://fixture.invalid/tool?query=PUBLIC_QUERY'}\n    sha256: {"+platform+": '"+digest+"'}\n")
	write(t, filepath.Join(root, "BUILD.yaml"), "targets:\n  - name: hello\n    rule: hello/rule.py\n    config: {tool: '@@tool'}\n")
	write(t, filepath.Join(root, "rules/hello/rule_definition.yaml"), "description: Fixture rule\nvariables:\n  greeting: {type: string, default: PUBLIC_DEFAULT}\n  tool: string\n")
	write(t, filepath.Join(options.Runtime.Config.TmpDir, "cache", digest, "file"), string(payload))
	out, err := workflowInvoke(options, "explain", "hello", "--isolated", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var report explanation
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Options["greeting"].Source != "schema default" || len(report.Dependencies) != 1 || report.Dependencies[0].Cache != "verified" || !strings.HasPrefix(report.Isolation, "on") || report.RuleDescription != "Fixture rule" {
		t.Fatal("explanation is incomplete")
	}
	if strings.Contains(out, "PUBLIC_DEFAULT") || strings.Contains(out, "PUBLIC_QUERY") {
		t.Fatal("explain disclosed values")
	}
	out, err = workflowInvoke(options, "explain", "hello")
	if err != nil || !strings.Contains(out, "cache verified") || strings.Contains(out, "PUBLIC_DEFAULT") {
		t.Fatal("human explanation failed")
	}
}
