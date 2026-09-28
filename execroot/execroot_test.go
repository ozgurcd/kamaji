package execroot

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kamaji/obj"
)

func fixture(t *testing.T) string {
	t.Helper()
	old := testRuntime.Config
	t.Cleanup(func() { testRuntime.Config = old })
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	testRuntime.Config = obj.RuntimeConfig{TmpDir: dir, ExecRootDir: filepath.Join(dir, "exec"), ThirdPartyFiles: map[string]obj.ThirdPartyFileInfo{}, ThirdPartyFinalPaths: map[string]string{}}
	if err := os.Mkdir(testRuntime.Config.ExecRootDir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func put(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func tarBytes(t *testing.T, name string, kind byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	w := tar.NewWriter(gz)
	h := &tar.Header{Name: name, Typeflag: kind, Mode: 0755, Linkname: "elsewhere"}
	if kind == tar.TypeReg {
		h.Size = 7
	}
	if err := w.WriteHeader(h); err != nil {
		t.Fatal(err)
	}
	if kind == tar.TypeReg {
		if _, err := w.Write([]byte("payload")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func zipBytes(t *testing.T, name string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	h := &zip.FileHeader{Name: name}
	h.SetMode(0755)
	f, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestCreateExecRootDir(t *testing.T) {
	base := fixture(t)
	if err := CreateExecRootDir(obj.ExecTarget{Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	first := testRuntime.Config.ExecRootDir
	if !strings.HasPrefix(first, filepath.Join(base, "execroot")+string(os.PathSeparator)) {
		t.Fatal("execroot escaped configured temporary directory")
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("execroot permissions = %o", info.Mode().Perm())
	}
	if err := CreateExecRootDir(obj.ExecTarget{Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	if testRuntime.Config.ExecRootDir == first {
		t.Fatal("execution roots must be unique")
	}
	for _, name := range []string{"../escape", "nested/escape", ""} {
		if err := CreateExecRootDir(obj.ExecTarget{Name: name}); err == nil {
			t.Errorf("unsafe target name %q accepted", name)
		}
	}
}

func TestExtractTarGz(t *testing.T) {
	for _, name := range []string{"tool", "nested/tool"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			src := filepath.Join(base, "archive")
			out := filepath.Join(base, "out")
			put(t, src, tarBytes(t, name, tar.TypeReg))
			if err := os.Mkdir(out, 0700); err != nil {
				t.Fatal(err)
			}
			if err := extractTarGz(src, out); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(out, name))
			if err != nil || string(got) != "payload" {
				t.Fatalf("extracted payload = %q, %v", got, err)
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		if err := extractTarGz(filepath.Join(t.TempDir(), "missing"), t.TempDir()); err == nil {
			t.Fatal("missing archive accepted")
		}
	})
	t.Run("invalid gzip", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "bad")
		put(t, p, []byte("not gzip"))
		if err := extractTarGz(p, t.TempDir()); err == nil {
			t.Fatal("invalid gzip accepted")
		}
	})
	t.Run("link entry", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "archive")
		put(t, p, tarBytes(t, "link", tar.TypeSymlink))
		if err := extractTarGz(p, t.TempDir()); err == nil {
			t.Fatal("symlink archive entry accepted")
		}
	})
}

func TestExtractTarRejectsTraversalAndSymlinkEscape(t *testing.T) {
	for _, name := range []string{"../escape", "link/escape"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			src := filepath.Join(base, "archive")
			out := filepath.Join(base, "out")
			outside := filepath.Join(base, "outside")
			put(t, src, tarBytes(t, name, tar.TypeReg))
			if err := os.Mkdir(out, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(outside, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(out, "link")); err != nil {
				t.Fatal(err)
			}
			if err := extractTarGz(src, out); err == nil {
				t.Error("escaping archive accepted")
			}
			for _, p := range []string{filepath.Join(base, "escape"), filepath.Join(outside, "escape")} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Error("archive wrote outside extraction directory")
				}
			}
		})
	}
}

func TestCopyThirdPartyIntoExecRootDir(t *testing.T) {
	for _, tc := range []struct {
		name, mime, path string
		data             func(*testing.T, string) []byte
	}{
		{"zip", "application/zip", "bin/tool", zipBytes},
		{"tar root", "application/gzip", "tool", func(t *testing.T, name string) []byte { return tarBytes(t, name, tar.TypeReg) }},
		{"tar nested", "application/gzip", "linux-amd64/tool", func(t *testing.T, name string) []byte { return tarBytes(t, name, tar.TypeReg) }},
		{"mach", "application/x-mach-binary", "tool", func(*testing.T, string) []byte { return []byte("payload") }},
		{"elf", "application/x-executable", "tool", func(*testing.T, string) []byte { return []byte("payload") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := fixture(t)
			cache := filepath.Join(base, "cache")
			put(t, filepath.Join(cache, "file"), tc.data(t, tc.path))
			put(t, filepath.Join(cache, "metadata"), []byte(tc.path+","+tc.mime))
			testRuntime.Config.ThirdPartyFiles["tool"] = obj.ThirdPartyFileInfo{FileName: cache, FinalName: tc.path}
			if err := CopyThirdPartyIntoExecRootDir(); err != nil {
				t.Fatal(err)
			}
			path := testRuntime.Config.ThirdPartyFinalPaths["tool"]
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "payload" {
				t.Fatalf("resolved artifact = %q, %v", got, err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm()&0100 == 0 {
				t.Fatalf("artifact is not executable: %v", err)
			}
			if _, err := os.Stat(filepath.Join(testRuntime.Config.ExecRootDir, "external", tc.path)); err != nil {
				t.Fatalf("external artifact link missing: %v", err)
			}
		})
	}
}

func TestCopyThirdPartyErrors(t *testing.T) {
	for _, metadata := range []string{"", "truncated", "tool,unknown/type"} {
		t.Run(metadata, func(t *testing.T) {
			base := fixture(t)
			cache := filepath.Join(base, "cache")
			put(t, filepath.Join(cache, "metadata"), []byte(metadata))
			testRuntime.Config.ThirdPartyFiles["tool"] = obj.ThirdPartyFileInfo{FileName: cache, FinalName: "tool"}
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("malformed metadata must return an error, not panic: %v", r)
				}
			}()
			if err := CopyThirdPartyIntoExecRootDir(); err == nil {
				t.Error("malformed or unsupported metadata accepted")
			}
		})
	}
	t.Run("missing metadata", func(t *testing.T) {
		base := fixture(t)
		testRuntime.Config.ThirdPartyFiles["tool"] = obj.ThirdPartyFileInfo{FileName: base, FinalName: "tool"}
		if err := CopyThirdPartyIntoExecRootDir(); err == nil {
			t.Fatal("missing metadata accepted")
		}
	})
	t.Run("empty set", func(t *testing.T) {
		fixture(t)
		if err := CopyThirdPartyIntoExecRootDir(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCopyThirdPartyAliasesAndErrors(t *testing.T) {
	for _, scenario := range []string{"alias", "unsafe file", "missing payload", "missing archive entry", "duplicate external path"} {
		t.Run(scenario, func(t *testing.T) {
			base := fixture(t)
			cache := filepath.Join(base, "cache")
			put(t, filepath.Join(cache, "metadata"), []byte("old-name,application/zip"))
			if scenario != "missing payload" {
				put(t, filepath.Join(cache, "file"), zipBytes(t, "bin/tool"))
			}
			file := "bin/tool"
			if scenario == "unsafe file" {
				file = "../escape"
			}
			if scenario == "missing archive entry" {
				file = "absent"
			}
			if scenario == "duplicate external path" {
				put(t, filepath.Join(testRuntime.Config.ExecRootDir, "external", file), []byte("existing"))
			}
			testRuntime.Config.ThirdPartyFiles["alias"] = obj.ThirdPartyFileInfo{FileName: cache, FinalName: file}
			testRuntime.Config.ThirdPartyFinalPaths = nil
			err := CopyThirdPartyIntoExecRootDir()
			if scenario != "alias" {
				if err == nil {
					t.Fatal("invalid artifact accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if testRuntime.Config.ThirdPartyFinalPaths["alias"] == "" {
				t.Fatal("alias not registered")
			}
			if _, err := os.Stat(filepath.Join(cache, "__TMP__")); !os.IsNotExist(err) {
				t.Fatalf("shared cache used for extraction: %v", err)
			}
		})
	}
}

func TestTarResourceLimits(t *testing.T) {
	for _, scenario := range []string{"entry count", "total bytes", "exact boundary", "gzip checksum", "trailing expansion", "compressed bytes"} {
		t.Run(scenario, func(t *testing.T) {
			fixture(t)
			testRuntime.Config.WorkspaceConfig.Limits.MaxArchiveEntries = 2
			testRuntime.Config.WorkspaceConfig.Limits.MaxExtractBytes = 4
			var data bytes.Buffer
			gz := gzip.NewWriter(&data)
			tw := tar.NewWriter(gz)
			entries := []string{"12", "34"}
			if scenario == "entry count" {
				entries = append(entries, "")
			}
			if scenario == "total bytes" {
				entries[1] = "345"
			}
			for i, value := range entries {
				if err := tw.WriteHeader(&tar.Header{Name: string(rune('a' + i)), Mode: 0600, Size: int64(len(value))}); err != nil {
					t.Fatal(err)
				}
				if _, err := tw.Write([]byte(value)); err != nil {
					t.Fatal(err)
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if scenario == "trailing expansion" {
				if _, err := gz.Write(make([]byte, (1<<20)+4096)); err != nil {
					t.Fatal(err)
				}
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			payload := data.Bytes()
			if scenario == "compressed bytes" {
				testRuntime.Config.WorkspaceConfig.Limits.MaxDownloadBytes = int64(len(payload) - 1)
			}
			if scenario == "gzip checksum" {
				payload[len(payload)-8] ^= 0xff
			}
			archive := filepath.Join(t.TempDir(), "archive.tar.gz")
			put(t, archive, payload)
			err := extractTarGz(archive, t.TempDir())
			if (err != nil) != (scenario != "exact boundary") {
				t.Fatalf("error=%v for %s", err, scenario)
			}
		})
	}
}

func TestMetadataWithCommaInFilename(t *testing.T) {
	mime, name := parseMetadata("bin/tool,alternate,application/zip")
	if mime != "application/zip" || name != "bin/tool,alternate" {
		t.Fatal("metadata split corrupted comma-bearing filename")
	}
}
