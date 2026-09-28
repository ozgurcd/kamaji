package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"
)

func TestGoRuleProcess(t *testing.T) {
	for _, scenario := range []string{"normal", "isolated", "failure", "deadline"} {
		t.Run(scenario, func(t *testing.T) {
			root := fixture(t)
			state := &rt.Runtime{Config: testRuntime.Config, KillGrace: 20 * time.Millisecond}
			state.Config.PythonInterpreter = filepath.Join(root, "missing-python")
			state.Config.Isolated = scenario == "isolated"
			rule := filepath.Join(state.Config.WorkspaceConfig.RulesDir, "go rule")
			put(t, filepath.Join(filepath.Dir(rule), "rule_definition.yaml"), "language: go\nvariables: {}\n")
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if err := tools.CopyFile(binary, rule); err != nil {
				t.Fatal(err)
			}
			resultPath := filepath.Join(root, "result.json")
			t.Setenv("KAMAJI_EXECUTOR_TEST_CHILD", "1")
			t.Setenv("KAMAJI_EXECUTOR_TEST_RESULT", resultPath)
			t.Setenv("KAMAJI_EXECUTOR_TEST_FAIL", "0")
			t.Setenv("KAMAJI_EXECUTOR_TEST_WAIT", "0")
			t.Setenv("PYTHONPATH", "inherited-value")
			if scenario == "failure" {
				t.Setenv("KAMAJI_EXECUTOR_TEST_FAIL", "1")
			}
			if scenario == "deadline" {
				state.Timeout = 100 * time.Millisecond
				t.Setenv("KAMAJI_EXECUTOR_TEST_WAIT", "1")
			}
			executor := Executor{Runtime: state}
			literal := "a b; $(literal) 'quoted'"
			err = executor.Run(state.Config.WorkspaceConfig, obj.ExecTarget{Name: "native", Rule: "go rule", Config: map[string]any{"value": literal, "enabled": true, "items": []string{"a", "b"}}}, "extra value", "")
			switch scenario {
			case "failure":
				var child *exec.ExitError
				if !errors.As(err, &child) || child.ExitCode() != 23 {
					t.Fatalf("child status lost: %v", err)
				}
			case "deadline":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("deadline lost: %v", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
			}
			if state.Config.ExecRootDir == "" {
				t.Fatal("execution was not prepared")
			}
			if _, err := os.Stat(state.Config.ExecRootDir); !os.IsNotExist(err) {
				t.Fatal("execution files retained")
			}
			if scenario == "deadline" {
				return
			}
			data, err := os.ReadFile(resultPath)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Args                          []string
				Cwd, Organization, PythonPath string
			}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			want := []string{"--enabled=true", "--items=[\"a\",\"b\"]", "--value=" + literal, "extra value", ""}
			if !reflect.DeepEqual(result.Args, want) {
				t.Fatal("native arguments changed or rule path was passed as an argument")
			}
			if result.Organization != "fixture.invalid" || result.PythonPath != "inherited-value" {
				t.Fatal("native environment changed")
			}
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "isolated" {
				cwd = filepath.Join(state.Config.ExecRootDir, "origin")
			}
			// The temporary root can have a platform alias such as /var -> /private/var.
			rootDir, err := filepath.EvalSymlinks(filepath.Dir(state.Config.ExecRootDir))
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "isolated" {
				cwd = filepath.Join(rootDir, filepath.Base(state.Config.ExecRootDir), "origin")
			}
			if result.Cwd != cwd {
				t.Fatal("native working directory changed")
			}
		})
	}
}
