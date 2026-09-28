package runner

import (
	"encoding/json"
	"errors"
	"kamaji/obj"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// The test binary stands in for an interpreter, so the real process boundary
// is exercised without Python, extension code, network access, or credentials.
func TestMain(m *testing.M) {
	if os.Getenv("KAMAJI_EXECUTOR_TEST_CHILD") == "1" {
		if os.Getenv("KAMAJI_EXECUTOR_TEST_WAIT") == "1" {
			time.Sleep(time.Hour)
			os.Exit(0)
		}
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(90)
		}
		result := struct {
			Args                               []string
			Cwd, Pwd, Organization, PythonPath string
		}{os.Args[1:], cwd, os.Getenv("PWD"), os.Getenv("KAMAJI_ORGANIZATION_DOMAIN"), os.Getenv("PYTHONPATH")}
		data, err := json.Marshal(result)
		if err != nil {
			os.Exit(91)
		}
		if os.WriteFile(os.Getenv("KAMAJI_EXECUTOR_TEST_RESULT"), data, 0600) != nil {
			os.Exit(92)
		}
		if os.Getenv("KAMAJI_EXECUTOR_TEST_FAIL") == "1" {
			os.Exit(23)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestExecutorRealProcess(t *testing.T) {
	for _, scenario := range []string{"normal", "isolated", "relative interpreter", "child failure"} {
		t.Run(scenario, func(t *testing.T) {
			root := fixture(t)
			testRuntime.Config.KeepExecRoot = true
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			testRuntime.Config.PythonInterpreter = executable
			testRuntime.Config.Isolated = scenario == "isolated" || scenario == "relative interpreter"
			if scenario == "relative interpreter" {
				// The interpreter lives outside the copied working directory.
				path := filepath.Join(root, "interpreter")
				if err := os.Symlink(executable, path); err != nil {
					t.Fatal(err)
				}
				testRuntime.Config.PythonInterpreter = "../interpreter"
			}
			rule := filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "run.py")
			put(t, rule, "unused fixture")
			resultPath := filepath.Join(root, "result.json")
			t.Setenv("KAMAJI_EXECUTOR_TEST_CHILD", "1")
			t.Setenv("KAMAJI_EXECUTOR_TEST_RESULT", resultPath)
			t.Setenv("KAMAJI_EXECUTOR_TEST_FAIL", "0")
			t.Setenv("PWD", "/stale")
			t.Setenv("PYTHONPATH", "/inherited")
			t.Setenv("KAMAJI_ORGANIZATION_DOMAIN", "inherited.invalid")
			if scenario == "child failure" {
				t.Setenv("KAMAJI_EXECUTOR_TEST_FAIL", "1")
			}
			literal := "a b; $(touch should-not-exist) 'quoted'\nnext"
			err = Run(testRuntime.Config.WorkspaceConfig, obj.ExecTarget{Name: "demo", Rule: "run.py", Config: map[string]any{"value": literal}}, "--extra", "", "a b")
			if scenario == "child failure" {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 23 {
					t.Fatalf("child exit status lost: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(resultPath)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Args                               []string
				Cwd, Pwd, Organization, PythonPath string
			}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Args, []string{rule, "--value=" + literal, "--extra", "", "a b"}) {
				t.Fatal("argument vector changed")
			}
			wantCwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if testRuntime.Config.Isolated {
				wantCwd = filepath.Join(testRuntime.Config.ExecRootDir, "origin")
			}
			wantCwd, err = filepath.EvalSymlinks(wantCwd)
			if err != nil {
				t.Fatal(err)
			}
			pwd, err := filepath.EvalSymlinks(result.Pwd)
			if err != nil {
				t.Fatal(err)
			}
			if result.Cwd != wantCwd || pwd != wantCwd {
				t.Fatal("child working directory mismatch")
			}
			if result.Organization != "fixture.invalid" || result.PythonPath != filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "common") {
				t.Fatal("child environment override lost")
			}
			if _, err := os.Stat(filepath.Join(wantCwd, "should-not-exist")); !os.IsNotExist(err) {
				t.Fatal("shell marker created")
			}
		})
	}
}
