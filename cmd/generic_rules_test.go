package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"
)

func TestMixedLanguageValidationAndExplain(t *testing.T) {
	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "kamaji.workspace.yaml"), "rules_directory: rules\nworkspace_vars:\n  - org_domain: fixture.invalid\n")
	for _, language := range []string{"python", "go", "custom"} {
		definition := "language: " + language + "\nvariables: {}\n"
		if language == "custom" {
			definition += "execution:\n  mode: interpreter\n  command: [custom-runtime, --fixed-argument]\n"
		}
		write(t, filepath.Join(root, "rules", language, "rule_definition.yaml"), definition)
		if err := tools.CopyFile(binary, filepath.Join(root, "rules", language, "rule")); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(root, "BUILD.yaml"), "targets:\n  - name: python\n    rule: python/rule\n  - name: go\n    rule: go/rule\n  - name: custom\n    rule: custom/rule\n")
	for _, args := range [][]string{{"validate", "--all"}, {"explain", "custom", "--json"}, {"explain", "python", "--json"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			loads := 0
			lookups := []string{}
			state := &rt.Runtime{Config: obj.RuntimeConfig{WorkingDir: root, TmpDir: filepath.Join(root, "runtime")}}
			command := NewCommand(Options{Runtime: state,
				LoadUserConfig: func() (map[string]string, error) { loads++; return map[string]string{"python": "selected-python"}, nil },
				LookPath:       func(name string) (string, error) { lookups = append(lookups, name); return binary, nil },
			})
			t.Setenv("KAMAJI_PYTHON", "")
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs(args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			wantLoads := 1
			if args[1] == "custom" {
				wantLoads = 0
			}
			if loads != wantLoads {
				t.Fatal("Python configuration loaded for the wrong rule")
			}
			if args[0] == "validate" {
				if strings.Join(lookups, ",") != "selected-python,custom-runtime" {
					t.Fatal("mixed-language interpreter selection changed")
				}
			} else {
				var report map[string]any
				if err := json.Unmarshal(output.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				if report["language"] != args[1] || report["execution_mode"] != "interpreter" || report["executable"] != binary {
					t.Fatal("incorrect execution description")
				}
				_, hasPython := report["python"]
				if hasPython != (args[1] == "python") {
					t.Fatal("incorrect Python diagnostic fields")
				}
				if strings.Contains(output.String(), "--fixed-argument") {
					t.Fatal("interpreter argument value exposed")
				}
			}
			if _, err := os.Stat(state.Config.TmpDir); !os.IsNotExist(err) {
				t.Fatal("read-only command created runtime storage")
			}
		})
	}
}

func TestInvalidRuleExecutionFailsBeforeRuntimeWrites(t *testing.T) {
	for _, scenario := range []string{"missing", "permission", "directory", "missing-schema", "missing-interpreter"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, "kamaji.workspace.yaml"), "rules_directory: rules\nworkspace_vars:\n  - org_domain: fixture.invalid\n")
			definition := "language: go\nvariables: {}\n"
			if scenario == "missing-interpreter" {
				definition = "language: custom\nexecution:\n  mode: interpreter\n  command: [./missing-runtime]\nvariables: {}\n"
			}
			write(t, filepath.Join(root, "rules", "rule_definition.yaml"), definition)
			rule := filepath.Join(root, "rules", "rule")
			write(t, rule, "not an executable")
			switch scenario {
			case "missing":
				if err := os.Remove(rule); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Remove(rule); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(rule, 0700); err != nil {
					t.Fatal(err)
				}
			case "missing-schema":
				if err := os.Remove(filepath.Join(root, "rules", "rule_definition.yaml")); err != nil {
					t.Fatal(err)
				}
			}
			write(t, filepath.Join(root, "BUILD.yaml"), "targets:\n  - name: bad\n    rule: rule\n")
			for _, verb := range []string{"validate", "run"} {
				state := &rt.Runtime{Config: obj.RuntimeConfig{WorkingDir: root, TmpDir: filepath.Join(root, "runtime")}}
				command := NewCommand(Options{Runtime: state, LoadUserConfig: func() (map[string]string, error) { t.Fatal("unexpected Python configuration"); return nil, nil }})
				command.SetArgs([]string{verb, "bad"})
				var output bytes.Buffer
				command.SetOut(&output)
				command.SetErr(&output)
				if err := command.Execute(); err == nil {
					t.Fatal("invalid executable accepted")
				}
				if _, err := os.Stat(state.Config.TmpDir); !os.IsNotExist(err) {
					t.Fatal("invalid rule created runtime storage")
				}
			}
		})
	}
}
