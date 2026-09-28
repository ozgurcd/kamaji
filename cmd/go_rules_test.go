package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"
)

func TestGoCommandsWithoutPython(t *testing.T) {
	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	rule := filepath.Join(root, "rules", "go", "rule")
	write(t, filepath.Join(root, "kamaji.workspace.yaml"), "rules_directory: rules\nworkspace_vars:\n  - org_domain: fixture.invalid\n")
	write(t, filepath.Join(root, "rules", "go", "rule_definition.yaml"), "language: golang\nvariables:\n  message:\n    type: string\n    default: hello\n")
	if err := tools.CopyFile(binary, rule); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "BUILD.yaml"), "targets:\n  - name: native\n    rule: go/rule\n    config: {}\n")
	for _, args := range [][]string{{"validate", "native"}, {"validate", "--all"}, {"doctor"}, {"explain", "native", "--json"}, {"explain", "native"}, {"run", "native"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			state := &rt.Runtime{Config: obj.RuntimeConfig{WorkingDir: root, TmpDir: filepath.Join(root, "runtime")}}
			called := false
			command := NewCommand(Options{Runtime: state,
				LoadUserConfig: func() (map[string]string, error) { return nil, errors.New("Python configuration must not be loaded") },
				LookPath:       func(string) (string, error) { return "", errors.New("Python must not be looked up") },
				RunTarget: func(_ obj.WorkspaceConfig, target obj.ExecTarget, _ ...string) error {
					called = true
					if target.Config["message"] != "hello" {
						t.Fatal("schema default missing")
					}
					return nil
				},
			})
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs(args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if (args[0] == "run") != called {
				t.Fatal("incorrect execution dispatch")
			}
			if args[0] == "explain" && len(args) == 3 {
				var report map[string]any
				if err := json.Unmarshal(output.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				if report["language"] != "go" || report["executable"] != rule {
					t.Fatal("missing Go execution details")
				}
				if _, ok := report["python"]; ok {
					t.Fatal("Go explanation claims a Python interpreter")
				}
			}
		})
	}
}
