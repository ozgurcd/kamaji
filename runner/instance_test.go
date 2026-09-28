package runner

import (
	"fmt"
	"kamaji/obj"
	"kamaji/rt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIndependentExecutors(t *testing.T) {
	for i := 0; i < 4; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			rule := filepath.Join(root, "run.py")
			put(t, rule, "fixture")
			python, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if i == 1 {
				if err := os.Symlink(python, filepath.Join(root, "python")); err != nil {
					t.Fatal(err)
				}
				python = "./python"
			}
			state := &rt.Runtime{Config: obj.RuntimeConfig{WorkingDir: root, TmpDir: filepath.Join(root, "tmp"), PythonInterpreter: python, WorkspaceConfig: obj.WorkspaceConfig{RulesDir: root, WorkspaceVars: []obj.WorkspaceVar{{Org_Domain: fmt.Sprintf("run-%d.invalid", i)}}}}}
			executor := Executor{Runtime: state, RunCommand: func(command *exec.Cmd) error {
				if command.Dir != root || command.Args[1] != rule || !contains(command.Env, fmt.Sprintf("KAMAJI_ORGANIZATION_DOMAIN=run-%d.invalid", i)) {
					t.Fatal("executor state crossed instances")
				}
				return nil
			}}
			if err := executor.Run(state.Config.WorkspaceConfig, obj.ExecTarget{Name: "fixture", Rule: "run.py"}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(state.Config.ExecRootDir); !os.IsNotExist(err) {
				t.Fatal("executor retained default scratch directory")
			}
		})
	}
}

func TestFailedPreparationRemovesRoot(t *testing.T) {
	fixture(t)
	testRuntime.Config.KeepExecRoot = true
	put(t, filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "run.py"), "fixture")
	testRuntime.Config.ThirdPartyFiles["missing"] = obj.ThirdPartyFileInfo{FinalName: "tool", FileName: filepath.Join(testRuntime.Config.TmpDir, "absent")}
	if err := Run(testRuntime.Config.WorkspaceConfig, obj.ExecTarget{Name: "demo", Rule: "run.py"}); err == nil {
		t.Fatal("invalid preparation accepted")
	}
	if testRuntime.Config.ExecRootDir == "" {
		t.Fatal("test did not reach execution preparation")
	}
	if _, err := os.Stat(testRuntime.Config.ExecRootDir); !os.IsNotExist(err) {
		t.Fatal("failed preparation retained scratch data")
	}
}
