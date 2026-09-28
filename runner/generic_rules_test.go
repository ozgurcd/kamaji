package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"
)

func TestGenericRuleProcess(t *testing.T) {
	for _, mode := range []string{"interpreter", "executable"} {
		t.Run(mode, func(t *testing.T) {
			root := fixture(t)
			state := &rt.Runtime{Config: testRuntime.Config}
			state.Config.PythonInterpreter = "missing-python"
			state.Config.Isolated = true
			state.Config.WorkingDir, _ = os.Getwd()
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			rule := filepath.Join(state.Config.WorkspaceConfig.RulesDir, "rule.custom")
			put(t, rule, "unused synthetic script")
			definition := "language: custom-language\nexecution:\n  mode: " + mode + "\n"
			wantArgs := []string{"--value=literal value", "extra"}
			if mode == "interpreter" {
				if err := tools.CopyFile(binary, filepath.Join(root, "runtime executable")); err != nil {
					t.Fatal(err)
				}
				definition += "  command: [" + strconv.Quote("../runtime executable") + ", --fixture-flag, 'fixed value']\n"
				wantArgs = append([]string{"--fixture-flag", "fixed value", rule}, wantArgs...)
			} else if err := tools.CopyFile(binary, rule); err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(filepath.Dir(rule), "rule_definition.yaml"), definition+"variables: {}\n")
			resultPath := filepath.Join(root, "result.json")
			t.Setenv("KAMAJI_EXECUTOR_TEST_CHILD", "1")
			t.Setenv("KAMAJI_EXECUTOR_TEST_RESULT", resultPath)
			t.Setenv("KAMAJI_EXECUTOR_TEST_FAIL", "0")
			t.Setenv("KAMAJI_EXECUTOR_TEST_WAIT", "0")
			executor := Executor{Runtime: state}
			if err := executor.Run(state.Config.WorkspaceConfig, obj.ExecTarget{Name: "custom", Rule: "rule.custom", Config: map[string]any{"value": "literal value"}}, "extra"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(resultPath)
			if err != nil {
				t.Fatal(err)
			}
			var result struct{ Args []string }
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Args, wantArgs) {
				t.Fatal("configured execution arguments changed")
			}
		})
	}
}
