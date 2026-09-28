package rt

import (
	"errors"
	"kamaji/obj"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupRejectsActiveRun(t *testing.T) {
	runtimeFixture(t)
	shared, err := RuntimeLease(false)
	if err != nil {
		t.Fatal(err)
	}
	defer shared.Close()
	path := filepath.Join(testRuntime.Config.TmpDir, "execroot", "active")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Cleanup(); err == nil {
		t.Fatal("cleanup accepted active execution")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("cleanup removed active execution")
	}
	shared.Close()
	if err := Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestPythonFailurePreservesCurrent(t *testing.T) {
	root := runtimeFixture(t)
	if err := EnsureTempDir(); err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(testRuntime.Config.TmpDir, "venvs", "env-old")
	if err := os.MkdirAll(filepath.Join(oldDir, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "bin", "python"), []byte("old"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testRuntime.Config.TmpDir, "python-current"), []byte("env-old"), 0600); err != nil {
		t.Fatal(err)
	}
	oldRun, oldReq := runPythonCommand, pythonRequirementsFile
	t.Cleanup(func() { runPythonCommand = oldRun; pythonRequirementsFile = oldReq })
	pythonRequirementsFile = filepath.Join(root, "requirements.txt")
	if err := os.WriteFile(pythonRequirementsFile, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	runPythonCommand = func(string, ...string) error {
		calls++
		if calls == 2 {
			return errors.New("fixture pip failure")
		}
		return nil
	}
	if err := SetupPythonEnv(); err == nil {
		t.Fatal("failed install accepted")
	}
	selected, err := CurrentPython()
	if err != nil || selected != filepath.Join(oldDir, "bin", "python") {
		t.Fatal("failed install changed current environment")
	}
	entries, err := os.ReadDir(filepath.Dir(oldDir))
	if err != nil || len(entries) != 1 {
		t.Fatal("failed staging directory retained")
	}
}

func TestPythonPromotionKeepsStablePaths(t *testing.T) {
	root := t.TempDir()
	state := Runtime{Config: obj.RuntimeConfig{TmpDir: filepath.Join(root, "runtime")}, RequirementsFile: filepath.Join(root, "requirements.txt")}
	if err := os.WriteFile(state.RequirementsFile, []byte("fixture-only"), 0600); err != nil {
		t.Fatal(err)
	}
	var created string
	state.PythonCommand = func(name string, args ...string) error {
		if len(args) > 1 && args[1] == "venv" {
			created = args[2]
			bin := filepath.Join(created, "bin")
			if err := os.MkdirAll(bin, 0700); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(bin, "python"), nil, 0700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(bin, "pip"), []byte("#!"+filepath.Join(bin, "python")+"\n"), 0700)
		}
		if name != filepath.Join(created, "bin", "pip") {
			t.Fatal("pip did not use staged environment")
		}
		return nil
	}
	if err := state.SetupPythonEnv(); err != nil {
		t.Fatal(err)
	}
	first := created
	if err := state.SetupPythonEnv(); err != nil {
		t.Fatal(err)
	}
	selected, err := state.CurrentPython()
	if err != nil || selected != filepath.Join(created, "bin", "python") {
		t.Fatal("completed environment was not selected")
	}
	data, err := os.ReadFile(filepath.Join(created, "bin", "pip"))
	if err != nil || string(data) != "#!"+selected+"\n" {
		t.Fatal("promotion broke absolute interpreter path")
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatal("replaced managed environment retained")
	}
}

func TestRetentionBounds(t *testing.T) {
	runtimeFixture(t)
	testRuntime.Config.KeepExecRoot = true
	testRuntime.Config.MaxRetainedExecRoots = 2
	testRuntime.Config.MaxRetainedBytes = 20
	if err := EnsureTempDir(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b", "c"} {
		testRuntime.Config.ExecRootDir = filepath.Join(testRuntime.Config.TmpDir, "execroot", name)
		if err := os.MkdirAll(testRuntime.Config.ExecRootDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(testRuntime.Config.ExecRootDir, "file"), []byte("12345678"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := FinishExecution(true); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(testRuntime.Config.TmpDir, "execroot"))
	if err != nil || len(entries) != 2 {
		t.Fatal("count retention limit not enforced")
	}
	testRuntime.Config.MaxRetainedBytes = 8
	if err := PruneExecutionRoots(); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(filepath.Join(testRuntime.Config.TmpDir, "execroot"))
	if err != nil || len(entries) != 1 {
		t.Fatal("byte retention limit not enforced")
	}
}

func TestPruningPreservesIncompleteRoots(t *testing.T) {
	runtimeFixture(t)
	testRuntime.Config.MaxRetainedExecRoots = 1
	testRuntime.Config.MaxRetainedBytes = 1
	active := filepath.Join(testRuntime.Config.TmpDir, "execroot", "active")
	if err := os.MkdirAll(active, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, "file"), []byte("larger than retention budget"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := PruneExecutionRoots(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatal("incomplete execution was pruned")
	}
}
