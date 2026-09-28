package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kamaji/obj"
	"kamaji/rt"
)

func TestExecutorDeadlineCleansExecutionFiles(t *testing.T) {
	fixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	state := &rt.Runtime{Config: testRuntime.Config, Timeout: 100 * time.Millisecond, KillGrace: 20 * time.Millisecond}
	state.Config.PythonInterpreter = executable
	put(t, filepath.Join(state.Config.WorkspaceConfig.RulesDir, "run.py"), "unused fixture")
	t.Setenv("KAMAJI_EXECUTOR_TEST_CHILD", "1")
	t.Setenv("KAMAJI_EXECUTOR_TEST_WAIT", "1")
	executor := Executor{Runtime: state}
	err = executor.Run(state.Config.WorkspaceConfig, obj.ExecTarget{Name: "deadline", Rule: "run.py", Config: map[string]any{}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error lost: %v", err)
	}
	if state.Config.ExecRootDir == "" {
		t.Fatal("did not exercise execution-root cleanup")
	}
	if _, err := os.Stat(state.Config.ExecRootDir); !os.IsNotExist(err) {
		t.Fatal("cancellation left execution files")
	}
}
