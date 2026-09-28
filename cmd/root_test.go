package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kamaji/obj"
)

func commandFixture(t *testing.T) string {
	t.Helper()
	oldConfig, oldWorkspace := testRuntime.Config, testRuntime.Config.WorkspaceFile
	oldLoad, oldRun, oldSetup := loadUserConfig, runTarget, setupPython
	oldEnsure, oldCopy, oldRemove := ensureWriteAccess, copyRules, removeAll
	t.Cleanup(func() {
		testRuntime.Config = oldConfig
		testRuntime.Config.WorkspaceFile = oldWorkspace
		loadUserConfig = oldLoad
		runTarget = oldRun
		setupPython = oldSetup
		ensureWriteAccess = oldEnsure
		copyRules = oldCopy
		removeAll = oldRemove
	})
	root := t.TempDir()
	t.Chdir(root)
	testRuntime.Config = obj.RuntimeConfig{TmpDir: filepath.Join(root, "tmp")}
	testRuntime.Config.WorkspaceFile = "kamaji.workspace.yaml"
	loadUserConfig = func() (map[string]string, error) { return map[string]string{}, nil }
	t.Setenv("KAMAJI_PYTHON", "")
	write(t, "kamaji.workspace.yaml", "rules_directory: rules\nworkspace_vars:\n  - org_domain: fixture.invalid\n")
	write(t, "rules/demo/run.py", "fixture")
	write(t, "rules/demo/rule_definition.yaml", "variables:\n  message:\n    type: string\n    mandatory: true\n  log:\n    type: string\n    default: INFO\n")
	write(t, "BUILD.yaml", "targets:\n  - name: demo\n    rule: demo/run.py\n    config:\n      message: hello\n")
	return root
}
func write(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}
func invoke(args ...string) (string, error) {
	command := newRootCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs(args)
	err := command.Execute()
	return out.String(), err
}

func TestExecuteTarget(t *testing.T) {
	root := commandFixture(t)
	write(t, "custom.yaml", "targets:\n  - name: custom\n    rule: demo/run.py\n    config:\n      message: with spaces\n")
	calls := 0
	runTarget = func(workspace obj.WorkspaceConfig, target obj.ExecTarget, args ...string) error {
		calls++
		if target.Name != "custom" || target.Config["message"] != "with spaces" || target.Config["log"] != "INFO" {
			t.Fatalf("target configuration did not reach runner")
		}
		if !reflect.DeepEqual(args, []string{"extra arg"}) {
			t.Fatalf("extra args=%q", args)
		}
		if testRuntime.Config.PythonInterpreter != "/fixture/python" || !testRuntime.Config.Isolated || !testRuntime.Config.DebugMode {
			t.Fatal("flags were not applied")
		}
		if workspace.WorkspaceRoot != root || workspace.RulesDir != filepath.Join(root, "rules") {
			t.Fatalf("workspace paths=%q %q", workspace.WorkspaceRoot, workspace.RulesDir)
		}
		return nil
	}
	if _, err := invoke("custom", "--build", "custom.yaml", "--python", "/fixture/python", "--isolated", "--debug", "--", "extra arg"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("runner calls=%d", calls)
	}
}

func TestCoreTargetEndToEnd(t *testing.T) {
	commandFixture(t)
	// A harmless standard executable accepts the same argument vector as a
	// Python rule. This traverses dispatch, parsing, validation, preparation,
	// and real process execution without running an extension or interpreter.
	pythonStandIn, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := invoke("demo", "--python", pythonStandIn); err != nil {
		t.Fatal(err)
	}
	if testRuntime.Config.ExecRootDir == "" {
		t.Fatal("execution directory not initialized")
	}
	if _, err := os.Stat(testRuntime.Config.ExecRootDir); !os.IsNotExist(err) {
		t.Fatal("execution directory not cleaned")
	}
}

func TestPythonPrecedence(t *testing.T) {
	for _, source := range []string{"flag", "environment", "user configuration", "venv", "fallback"} {
		t.Run(source, func(t *testing.T) {
			root := commandFixture(t)
			args := []string{"demo"}
			want := "python3"
			venv := filepath.Join(root, "tmp", "venv", "bin", "python")
			if source != "fallback" {
				write(t, venv, "fixture")
				if err := os.Chmod(venv, 0700); err != nil {
					t.Fatal(err)
				}
				want = venv
			}
			if source == "user configuration" || source == "environment" || source == "flag" {
				loadUserConfig = func() (map[string]string, error) { return map[string]string{"python": "yaml-python"}, nil }
				want = "yaml-python"
			}
			if source == "environment" || source == "flag" {
				t.Setenv("KAMAJI_PYTHON", "env-python")
				want = "env-python"
			}
			if source == "flag" {
				args = append(args, "--python", "flag-python")
				want = "flag-python"
			}
			runTarget = func(obj.WorkspaceConfig, obj.ExecTarget, ...string) error {
				if testRuntime.Config.PythonInterpreter != want {
					t.Fatalf("python=%q want %q", testRuntime.Config.PythonInterpreter, want)
				}
				return nil
			}
			if _, err := invoke(args...); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCommandFailures(t *testing.T) {
	for _, scenario := range []string{"no target", "missing target", "missing build", "invalid variables", "missing dependency", "runner failure", "invalid workspace", "unknown flag"} {
		t.Run(scenario, func(t *testing.T) {
			commandFixture(t)
			args := []string{"demo"}
			calls := 0
			runTarget = func(obj.WorkspaceConfig, obj.ExecTarget, ...string) error {
				calls++
				return errors.New("runner failure")
			}
			switch scenario {
			case "no target":
				args = nil
			case "missing target":
				args = []string{"absent"}
			case "missing build":
				args = append(args, "--build", "missing.yaml")
			case "invalid variables":
				write(t, "BUILD.yaml", "targets:\n  - name: demo\n    rule: demo/run.py\n")
			case "missing dependency":
				write(t, "BUILD.yaml", "targets:\n  - name: demo\n    rule: demo/run.py\n    config:\n      message: '@@missing'\n")
			case "invalid workspace":
				write(t, "kamaji.workspace.yaml", "invalid: [")
			case "unknown flag":
				args = append(args, "--not-a-flag")
			}
			if _, err := invoke(args...); err == nil {
				t.Fatal("expected error")
			}
			if scenario != "runner failure" && calls != 0 {
				t.Fatal("runner called despite invalid input")
			}
		})
	}
}

func TestHelpHasNoRuntimeSideEffects(t *testing.T) {
	root := commandFixture(t)
	if err := os.Remove("kamaji.workspace.yaml"); err != nil {
		t.Fatal(err)
	}
	loadUserConfig = func() (map[string]string, error) { t.Fatal("help loaded configuration"); return nil, nil }
	out, err := invoke("--help")
	if err != nil || !strings.Contains(out, "Usage:") {
		t.Fatalf("help=%q %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(root, "tmp")); !os.IsNotExist(err) {
		t.Fatalf("help created runtime state: %v", err)
	}
}

func TestCleanupWithoutWorkspace(t *testing.T) {
	root := commandFixture(t)
	if err := os.Remove("kamaji.workspace.yaml"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cache", "execroot", "venv"} {
		write(t, filepath.Join(root, "tmp", name, "keep"), "fixture")
	}
	if _, err := invoke("--cleanup"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cache", "execroot"} {
		if _, err := os.Stat(filepath.Join(root, "tmp", name)); !os.IsNotExist(err) {
			t.Fatalf("cleanup left %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "tmp", "venv", "keep")); err != nil {
		t.Fatal("cleanup removed environment")
	}
	removeAll = func(string) error { return errors.New("remove failed") }
	if _, err := invoke("--cleanup"); err == nil {
		t.Fatal("cleanup swallowed removal error")
	}
}

func TestAdministrativeCommands(t *testing.T) {
	for _, name := range []string{"rules-directory-create", "rules-directory-delete", "setup-python-env"} {
		for _, fail := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/success", true: "/failure"}[fail], func(t *testing.T) {
				root := commandFixture(t)
				if err := os.Remove("kamaji.workspace.yaml"); err != nil {
					t.Fatal(err)
				}
				called := false
				action := func() error {
					called = true
					if fail {
						return errors.New("operation failed")
					}
					return nil
				}
				ensureWriteAccess = func(path string) error {
					if path != "/usr/local/share/kamaji" {
						t.Fatalf("unexpected destination %q", path)
					}
					return nil
				}
				copyRules = action
				removeAll = func(path string) error {
					if path != "/usr/local/share/kamaji/rules" {
						t.Fatalf("unexpected removal %q", path)
					}
					return action()
				}
				setupPython = func() error {
					if testRuntime.Config.TmpDir != filepath.Join(root, "tmp") || testRuntime.Config.PythonInterpreter != "test-python" {
						t.Fatal("setup configuration missing")
					}
					return action()
				}
				args := []string{name}
				if name == "setup-python-env" {
					args = append(args, "--python", "test-python")
				}
				_, err := invoke(args...)
				if (err != nil) != fail || !called {
					t.Fatalf("called=%v err=%v wantError=%v", called, err, fail)
				}
			})
		}
	}
}

func TestRulesOverride(t *testing.T) {
	root := commandFixture(t)
	write(t, "alternate/demo/rule_definition.yaml", "variables: {}\n")
	write(t, "alternate/demo/run.py", "fixture")
	runTarget = func(workspace obj.WorkspaceConfig, _ obj.ExecTarget, _ ...string) error {
		if workspace.RulesDir != filepath.Join(root, "alternate") {
			t.Fatalf("rules directory=%q", workspace.RulesDir)
		}
		return nil
	}
	if _, err := invoke("demo", "--rules-directory", "alternate"); err != nil {
		t.Fatal(err)
	}
}

func TestExecutePublicEntry(t *testing.T) {
	commandFixture(t)
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"kamaji", "--help"}
	if err := Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestAdministrativePermissionErrors(t *testing.T) {
	for _, name := range []string{"rules-directory-create", "rules-directory-delete"} {
		t.Run(name, func(t *testing.T) {
			commandFixture(t)
			ensureWriteAccess = func(string) error { return errors.New("permission denied") }
			copyRules = func() error { t.Fatal("installation attempted after permission error"); return nil }
			removeAll = func(string) error { t.Fatal("removal attempted after permission error"); return nil }
			if _, err := invoke(name); err == nil {
				t.Fatal("permission error swallowed")
			}
		})
	}
}
