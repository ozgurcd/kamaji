package rt

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"kamaji/obj"
)

func TestDefaultTempDir(t *testing.T) {
	dir, err := DefaultTempDir()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		if err == nil {
			t.Fatal("unsupported platform accepted")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	base := "/tmp"
	if runtime.GOOS == "darwin" {
		base = "/var/tmp"
	}
	if want := filepath.Join(base, "_kamaji_"+current.Username); dir != want {
		t.Fatalf("default directory=%q want %q", dir, want)
	}
}

func TestResourceLimits(t *testing.T) {
	runtimeFixture(t)
	limits, err := EffectiveLimits()
	if err != nil {
		t.Fatal(err)
	}
	if limits.MaxDownloadBytes != 512<<20 || limits.MaxExtractBytes != 2<<30 || limits.MaxArchiveEntries != 10000 {
		t.Fatal("incorrect resource defaults")
	}
	for _, field := range []string{"max_download_bytes", "max_extract_bytes", "max_archive_entries"} {
		t.Run(field, func(t *testing.T) {
			dir := runtimeFixture(t)
			writeWorkspace(t, dir, "limits:\n  "+field+": 17\n")
			config, err := readWorkspaceConfig("")
			if err != nil {
				t.Fatal(err)
			}
			rtBefore := testRuntime.Config.WorkspaceConfig
			testRuntime.Config.WorkspaceConfig = config
			t.Cleanup(func() { testRuntime.Config.WorkspaceConfig = rtBefore })
			got, err := EffectiveLimits()
			if err != nil {
				t.Fatal(err)
			}
			switch field {
			case "max_download_bytes":
				if got.MaxDownloadBytes != 17 {
					t.Fatal("download override lost")
				}
			case "max_extract_bytes":
				if got.MaxExtractBytes != 17 {
					t.Fatal("extract override lost")
				}
			case "max_archive_entries":
				if got.MaxArchiveEntries != 17 {
					t.Fatal("entries override lost")
				}
			}
			writeWorkspace(t, dir, "limits:\n  "+field+": -1\n")
			if _, err := readWorkspaceConfig(""); err == nil {
				t.Fatal("negative limit accepted")
			}
		})
	}
}

func TestResourceLimitOverflow(t *testing.T) {
	for _, limits := range []obj.ResourceLimits{
		{MaxDownloadBytes: 1<<63 - 1}, {MaxExtractBytes: 1<<63 - 1}, {MaxArchiveEntries: 1<<63 - 1},
	} {
		if _, err := resolveLimits(limits); err == nil {
			t.Fatal("overflowing limits accepted")
		}
	}
}

func runtimeFixture(t *testing.T) string {
	t.Helper()
	old, marker := testRuntime.Config, testRuntime.Config.WorkspaceFile
	t.Cleanup(func() { testRuntime.Config = old; testRuntime.Config.WorkspaceFile = marker })
	dir := t.TempDir()
	testRuntime.Config = obj.RuntimeConfig{TmpDir: filepath.Join(dir, "tmp"), WorkspaceDir: dir}
	testRuntime.Config.WorkspaceFile = "kamaji-test-workspace.yaml"
	t.Chdir(dir)
	return dir
}

func writeWorkspace(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, testRuntime.Config.WorkspaceFile), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestInitRuntime(t *testing.T) {
	dir := runtimeFixture(t)
	writeWorkspace(t, dir, "rules_directory: //rules\nworkspace_vars:\n- org_domain: example.invalid\n")
	child := filepath.Join(dir, "one/two")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)
	if err := InitRuntime(""); err != nil {
		t.Fatal(err)
	}
	if testRuntime.Config.WorkspaceDir != dir {
		t.Fatalf("workspace = %q; want fixture %q", testRuntime.Config.WorkspaceDir, dir)
	}
	if testRuntime.Config.WorkspaceConfig.RulesDir != filepath.Join(dir, "rules") {
		t.Fatal("workspace-relative rules not resolved")
	}
	if testRuntime.Config.Platform != runtime.GOOS+"_"+runtime.GOARCH {
		t.Fatalf("platform = %q", testRuntime.Config.Platform)
	}
	if testRuntime.Config.ThirdPartyFiles == nil || testRuntime.Config.ThirdPartyFinalPaths == nil {
		t.Fatal("dependency maps not initialized")
	}
	for _, path := range []string{testRuntime.Config.CacheDir, filepath.Join(testRuntime.Config.CacheDir, "sha256")} {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("missing cache directory: %v", err)
		}
	}
}

func TestInitRuntimeErrors(t *testing.T) {
	t.Run("missing workspace", func(t *testing.T) {
		runtimeFixture(t)
		if err := InitRuntime(""); err == nil {
			t.Fatal("missing marker accepted")
		}
	})
	t.Run("malformed workspace", func(t *testing.T) {
		dir := runtimeFixture(t)
		writeWorkspace(t, dir, "rules_directory: [")
		if err := InitRuntime(""); err == nil {
			t.Fatal("invalid YAML accepted")
		}
	})
	t.Run("blocked cache", func(t *testing.T) {
		dir := runtimeFixture(t)
		writeWorkspace(t, dir, "{}\n")
		p := filepath.Join(dir, "blocker")
		if err := os.WriteFile(p, []byte("file"), 0600); err != nil {
			t.Fatal(err)
		}
		testRuntime.Config.TmpDir = p
		if err := InitRuntime(""); err == nil {
			t.Fatal("file accepted as cache root")
		}
	})
}

func TestReadWorkspaceConfig(t *testing.T) {
	for _, tc := range []struct{ name, value, override, want string }{
		{"workspace-relative", "//rules", "", "relative"},
		{"absolute", "/fixture/rules", "", "/fixture/rules"},
		{"default", "", "", "/usr/local/share/kamaji/rules"},
		{"override", "//rules", "/override/rules", "/override/rules"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := runtimeFixture(t)
			writeWorkspace(t, dir, "rules_directory: '"+tc.value+"'\n")
			got, err := readWorkspaceConfig(tc.override)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == "relative" {
				want = filepath.Join(dir, "rules")
			}
			if got.RulesDir != want {
				t.Fatalf("rules = %q; want %q", got.RulesDir, want)
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		runtimeFixture(t)
		if _, err := readWorkspaceConfig(""); err == nil {
			t.Fatal("missing config accepted")
		}
	})
	t.Run("directory marker", func(t *testing.T) {
		dir := runtimeFixture(t)
		if err := os.Mkdir(filepath.Join(dir, testRuntime.Config.WorkspaceFile), 0700); err != nil {
			t.Fatal(err)
		}
		if isWorkspaceRoot(dir) {
			t.Fatal("directory accepted as marker")
		}
	})
}

func TestSetupPythonEnv(t *testing.T) {
	for _, tc := range []struct {
		name         string
		requirements bool
		failAt       int
	}{
		{"without requirements", false, 0}, {"with requirements", true, 0}, {"venv failure", true, 1}, {"pip failure", true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := runtimeFixture(t)
			oldRun, oldRequirements := runPythonCommand, pythonRequirementsFile
			t.Cleanup(func() { runPythonCommand = oldRun; pythonRequirementsFile = oldRequirements })
			pythonRequirementsFile = filepath.Join(dir, "requirements.txt")
			if tc.requirements {
				if err := os.WriteFile(pythonRequirementsFile, []byte("fixture-only\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var calls [][]string
			runPythonCommand = func(name string, args ...string) error {
				calls = append(calls, append([]string{name}, args...))
				if len(calls) == 1 {
					bin := filepath.Join(args[len(args)-1], "bin")
					if err := os.MkdirAll(bin, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(bin, "python"), nil, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if len(calls) == tc.failAt {
					return errors.New("fixture command failed")
				}
				return nil
			}
			err := SetupPythonEnv()
			if (err != nil) != (tc.failAt != 0) {
				t.Fatalf("error = %v", err)
			}
			venv := calls[0][3]
			if !strings.HasPrefix(venv, filepath.Join(testRuntime.Config.TmpDir, "venvs", "env-")) {
				t.Fatal("environment not staged under managed root")
			}
			want := [][]string{{"python3", "-m", "venv", venv}}
			if tc.requirements && tc.failAt != 1 {
				want = append(want, []string{filepath.Join(venv, "bin/pip"), "install", "-r", pythonRequirementsFile})
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("commands = %#v; want %#v", calls, want)
			}
			if tc.failAt == 1 && !strings.Contains(err.Error(), "virtualenv") {
				t.Fatal("missing venv error context")
			}
		})
	}
}
