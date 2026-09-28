package rt

import (
	"kamaji/obj"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheRejectsUnsafeDirectories(t *testing.T) {
	for _, location := range []string{"root", "cache", "sha256"} {
		for _, kind := range []string{"symlink", "shared"} {
			t.Run(location+"/"+kind, func(t *testing.T) {
				old := testRuntime.Config
				t.Cleanup(func() { testRuntime.Config = old })
				root := t.TempDir()
				testRuntime.Config = obj.RuntimeConfig{TmpDir: filepath.Join(root, "runtime")}
				path := testRuntime.Config.TmpDir
				if location != "root" {
					path = filepath.Join(path, "cache")
				}
				if location == "sha256" {
					path = filepath.Join(path, "sha256")
				}
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				outside := filepath.Join(root, "outside")
				if err := os.Mkdir(outside, 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "symlink" {
					if err := os.Symlink(outside, path); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(path, 0777); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := initCacheDir(); err == nil {
					t.Error("unsafe directory accepted")
				}
				entries, err := os.ReadDir(outside)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 0 {
					t.Fatal("wrote through runtime symlink")
				}
			})
		}
	}
}

func TestSetupRejectsLinkedEnvironment(t *testing.T) {
	root := runtimeFixture(t)
	oldRun := runPythonCommand
	t.Cleanup(func() { runPythonCommand = oldRun })
	runPythonCommand = func(string, ...string) error { t.Fatal("unsafe environment reached subprocess"); return nil }
	if err := os.Mkdir(testRuntime.Config.TmpDir, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(testRuntime.Config.TmpDir, "venv")); err != nil {
		t.Fatal(err)
	}
	if err := SetupPythonEnv(); err == nil {
		t.Fatal("linked environment accepted")
	}
}
