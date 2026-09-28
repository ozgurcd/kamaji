package cmd

import (
	"bytes"
	"errors"
	"kamaji/obj"
	"kamaji/rt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyCommands(t *testing.T) {
	for _, args := range [][]string{{"targets"}, {"validate", "demo"}, {"version"}, {"--version"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			root := commandFixture(t)
			runTarget = func(obj.WorkspaceConfig, obj.ExecTarget, ...string) error {
				t.Fatal("read-only command executed target")
				return nil
			}
			output, err := invoke(args...)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(output) == "" {
				t.Fatal("command produced no result")
			}
			if _, err := os.Stat(filepath.Join(root, "tmp")); !os.IsNotExist(err) {
				t.Fatal("read-only command initialized runtime storage")
			}
		})
	}
}

func TestCLIConfigurationErrorsStopExecution(t *testing.T) {
	commandFixture(t)
	loadUserConfig = func() (map[string]string, error) { return nil, errors.New("invalid user configuration") }
	runTarget = func(obj.WorkspaceConfig, obj.ExecTarget, ...string) error {
		t.Fatal("invalid configuration executed target")
		return nil
	}
	if _, err := invoke("demo"); err == nil {
		t.Fatal("configuration error swallowed")
	}
}

func TestExplicitPythonOverridesInvalidManagedSelection(t *testing.T) {
	root := commandFixture(t)
	write(t, filepath.Join(root, "tmp", "python-current"), "invalid selection")
	runTarget = func(obj.WorkspaceConfig, obj.ExecTarget, ...string) error { return nil }
	if _, err := invoke("demo", "--python", "/usr/bin/true"); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupHoldsExclusiveLease(t *testing.T) {
	commandFixture(t)
	removeAll = func(path string) error {
		lease, err := testRuntime.RuntimeLease(false)
		if err == nil {
			lease.Close()
			t.Fatal("cleanup released lease before deletion")
		}
		return os.RemoveAll(path)
	}
	if _, err := invoke("--cleanup"); err != nil {
		t.Fatal(err)
	}
}

func TestIndependentCommandInstances(t *testing.T) {
	for _, name := range []string{"alpha", "beta"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, filepath.Join(root, "kamaji.workspace.yaml"), "rules_directory: rules\nworkspace_vars:\n  - org_domain: fixture.invalid\n")
			write(t, filepath.Join(root, "rules/demo/run.py"), "fixture")
			write(t, filepath.Join(root, "rules/demo/rule_definition.yaml"), "variables: {}\n")
			write(t, filepath.Join(root, "BUILD.yaml"), "targets:\n  - name: "+name+"\n    rule: demo/run.py\n")
			state := &rt.Runtime{Config: obj.RuntimeConfig{WorkingDir: root, TmpDir: filepath.Join(root, "runtime")}}
			called := false
			command := NewCommand(Options{Runtime: state, LoadUserConfig: func() (map[string]string, error) { return nil, nil }, RunTarget: func(w obj.WorkspaceConfig, selected obj.ExecTarget, _ ...string) error {
				called = true
				if selected.Name != name || w.WorkspaceRoot != root {
					t.Fatal("command state crossed instances")
				}
				return nil
			}})
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs([]string{"run", name, "--python", "/usr/bin/true"})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("command did not invoke owned executor")
			}
		})
	}
}
