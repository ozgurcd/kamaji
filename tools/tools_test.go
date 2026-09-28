package tools

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kamaji/obj"
)

func writeFixture(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func resetConfig(t *testing.T) {
	t.Helper()
	old := testRuntime.Config
	t.Cleanup(func() { testRuntime.Config = old })
	testRuntime.Config = obj.RuntimeConfig{}
}

func zipFixture(t *testing.T, entries map[string]string) string {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, content := range entries {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0755)
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.zip")
	writeFixture(t, path, b.Bytes(), 0600)
	return path
}

func TestGetRule(t *testing.T) {
	resetConfig(t)
	testRuntime.Config.WorkspaceConfig.WorkspaceRoot = t.TempDir()
	for _, tc := range []struct {
		name, rule, want string
		wantErr          bool
	}{
		{"missing", "", "", true},
		{"relative", "demo/run.py", "demo/run.py", false},
		{"workspace", "//rules/demo.py", filepath.Join(testRuntime.Config.WorkspaceConfig.WorkspaceRoot, "rules/demo.py"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GetRule(obj.ExecTarget{Rule: tc.rule})
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("GetRule = %q, %v; want %q, error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source")
	dst := filepath.Join(dir, "destination")
	writeFixture(t, src, []byte("complete payload"), 0750)
	writeFixture(t, dst, []byte("previous payload that must be truncated"), 0600)
	if err := CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readFixture(t, src), readFixture(t, dst)) {
		t.Fatal("copy changed payload")
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0750 {
		t.Fatalf("mode = %o; want 750", info.Mode().Perm())
	}
	for _, tc := range []struct{ name, src, dst string }{
		{"missing source", filepath.Join(dir, "missing"), dst},
		{"directory source", dir, dst},
		{"directory destination", src, dir},
		{"missing destination parent", src, filepath.Join(dir, "absent", "child")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CopyFile(tc.src, tc.dst); err == nil {
				t.Fatal("expected copy error")
			}
		})
	}
}

func TestCopyFileRejectsSameFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeFixture(t, source, []byte("keep source"), 0600)
	alias := filepath.Join(root, "alias")
	if err := os.Link(source, alias); err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{source, alias} {
		if err := CopyFile(source, destination); err == nil {
			t.Fatal("copy onto the same file accepted")
		}
		if got := string(readFixture(t, source)); got != "keep source" {
			t.Fatalf("source changed: %q", got)
		}
	}
}

func TestZipResourceLimits(t *testing.T) {
	for _, scenario := range []string{"entry count", "total bytes", "exact boundary", "compressed bytes"} {
		t.Run(scenario, func(t *testing.T) {
			resetConfig(t)
			testRuntime.Config.WorkspaceConfig.Limits.MaxArchiveEntries = 2
			testRuntime.Config.WorkspaceConfig.Limits.MaxExtractBytes = 4
			entries := map[string]string{"a": "12", "b": "34"}
			switch scenario {
			case "entry count":
				entries["c"] = ""
			case "total bytes":
				entries["b"] = "345"
			}
			archive := zipFixture(t, entries)
			if scenario == "compressed bytes" {
				testRuntime.Config.WorkspaceConfig.Limits.MaxDownloadBytes = 1
			}
			err := Unzip(archive, t.TempDir())
			if (err != nil) != (scenario != "exact boundary") {
				t.Fatalf("error=%v for %s", err, scenario)
			}
		})
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestCopyWithLimit(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		limit       int64
		wantErr     bool
	}{
		{"empty", "", 0, false}, {"zero limit", "x", 0, true}, {"exact", "abcd", 4, false}, {"short", "abc", 4, false}, {"long", "abcde", 4, true}, {"invalid", "abc", -1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			n, err := CopyWithLimit(&output, strings.NewReader(tc.value), tc.limit)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v expectedError=%v", err, tc.wantErr)
			}
			if n != int64(output.Len()) || (tc.limit >= 0 && n > tc.limit) {
				t.Fatal("copy exceeded limit or reported incorrect count")
			}
			if !tc.wantErr && output.String() != tc.value {
				t.Fatal("copy changed content")
			}
		})
	}
	if _, err := CopyWithLimit(io.Discard, failReader{}, 10); err != io.ErrUnexpectedEOF {
		t.Fatalf("reader error not preserved: %v", err)
	}
}

func TestChecksum(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "input")
	writeFixture(t, file, []byte("abc"), 0600)
	want := fmt.Sprintf("%x", sha256.Sum256([]byte("abc")))
	if got := calculateSHA256(file); got != want {
		t.Fatalf("digest = %q; want %q", got, want)
	}
	if !IsFileValid(file, want) {
		t.Fatal("valid digest rejected")
	}
	if IsFileValid(file, strings.Repeat("0", 64)) {
		t.Fatal("mismatched digest accepted")
	}
	if IsFileValid(filepath.Join(dir, "missing"), want) {
		t.Fatal("missing file accepted")
	}
	if got := calculateSHA256(dir); got != "" {
		t.Fatalf("directory produced digest %q", got)
	}
	writeFixture(t, file, nil, 0600)
	if !IsFileValid(file, fmt.Sprintf("%x", sha256.Sum256(nil))) {
		t.Fatal("empty file digest rejected")
	}
}

func TestIsFileValidRejectsMissingFileAndEmptyDigest(t *testing.T) {
	if IsFileValid(filepath.Join(t.TempDir(), "missing"), "") {
		t.Fatal("missing input must not validate against an empty digest")
	}
}

func TestNormalizeMap(t *testing.T) {
	got := NormalizeMap(map[any]any{"outer": map[any]any{7: "value"}, "flag": true})
	want := map[string]any{"outer": map[string]any{"7": "value"}, "flag": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized map = %#v; want %#v", got, want)
	}
	if len(NormalizeMap(nil)) != 0 {
		t.Fatal("nil input should normalize to empty map")
	}
}

func TestNormalizeMapHandlesMapsInsideLists(t *testing.T) {
	got := NormalizeMap(map[any]any{"items": []any{map[any]any{"name": "value"}}})
	if _, err := json.Marshal(got); err != nil {
		t.Fatalf("normalized result is not JSON-compatible: %v", err)
	}
}

func TestUnzip(t *testing.T) {
	resetConfig(t)
	src := zipFixture(t, map[string]string{"bin/tool": "payload", "empty/": ""})
	dest := filepath.Join(t.TempDir(), "expanded")
	if err := Unzip(src, dest); err != nil {
		t.Fatal(err)
	}
	if got := string(readFixture(t, filepath.Join(dest, "bin/tool"))); got != "payload" {
		t.Fatalf("payload = %q", got)
	}
	info, err := os.Stat(filepath.Join(dest, "bin/tool"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("executable mode not preserved: %o", info.Mode().Perm())
	}
	for _, name := range []string{"../escape", "nested/../../escape"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			out := filepath.Join(base, "out")
			if err := Unzip(zipFixture(t, map[string]string{name: "blocked"}), out); err == nil {
				t.Fatal("traversal archive accepted")
			}
			if _, err := os.Stat(filepath.Join(base, "escape")); !os.IsNotExist(err) {
				t.Fatal("archive escaped destination")
			}
		})
	}
	t.Run("missing archive", func(t *testing.T) {
		if err := Unzip(filepath.Join(t.TempDir(), "missing"), t.TempDir()); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("corrupt archive", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "invalid.zip")
		writeFixture(t, p, []byte("invalid"), 0600)
		if err := Unzip(p, t.TempDir()); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("blocked destination", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "file")
		writeFixture(t, p, []byte("existing"), 0600)
		if err := Unzip(src, p); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUnzipRejectsDestinationSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	dest := filepath.Join(base, "out")
	external := filepath.Join(base, "outside")
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(external, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(dest, "link")); err != nil {
		t.Fatal(err)
	}
	err := Unzip(zipFixture(t, map[string]string{"link/escaped": "payload"}), dest)
	if err == nil {
		t.Error("archive followed a symlink outside its destination")
	}
	if _, err := os.Stat(filepath.Join(external, "escaped")); !os.IsNotExist(err) {
		t.Error("archive wrote outside its destination")
	}
}

func TestFileTypeAndMetadata(t *testing.T) {
	resetConfig(t)
	src := zipFixture(t, map[string]string{"tool": "payload"})
	cache := t.TempDir()
	writeFixture(t, filepath.Join(cache, "file"), readFixture(t, src), 0600)
	if got, err := determineFileType(filepath.Join(cache, "file")); err != nil || got != "application/zip" {
		t.Fatalf("file type = %q, %v", got, err)
	}
	if err := CreateMetadataFile(cache, "tool"); err != nil {
		t.Fatal(err)
	}
	if got := string(readFixture(t, filepath.Join(cache, "metadata"))); got != "tool,application/zip" {
		t.Fatalf("metadata = %q", got)
	}
	if _, err := determineFileType(filepath.Join(cache, "missing")); err == nil {
		t.Fatal("missing file accepted")
	}
	if err := CreateMetadataFile(filepath.Join(cache, "missing"), "tool"); err == nil {
		t.Fatal("missing cache accepted")
	}
	if err := CreateMetadataFile(t.TempDir(), "tool"); err == nil {
		t.Fatal("missing cached payload accepted")
	}
}

func TestGetFullPath(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "nested", "tool")
	writeFixture(t, want, []byte("payload"), 0600)
	if got := GetFullPath(dir, "tool"); got != want {
		t.Fatalf("path = %q; want %q", got, want)
	}
	if got := GetFullPath(dir, "absent"); got != "" {
		t.Fatalf("unexpected path %q", got)
	}
	if got := GetFullPath(filepath.Join(dir, "absent"), "tool"); got != "" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestGetFullPathResolvesRelativeFilePath(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "bin/tool")
	writeFixture(t, want, []byte("payload"), 0600)
	if got := GetFullPath(dir, "bin/tool"); got != want {
		t.Fatalf("relative file path = %q; want %q", got, want)
	}
}

func TestGetFullPathDoesNotReturnDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "tool"), 0700); err != nil {
		t.Fatal(err)
	}
	if got := GetFullPath(dir, "tool"); got != "" {
		t.Fatalf("directory returned as executable: %q", got)
	}
}

func TestGetFullPathRejectsAmbiguousBasenames(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "a", "tool"), []byte("first"), 0600)
	writeFixture(t, filepath.Join(dir, "b", "tool"), []byte("second"), 0600)
	if GetFullPath(dir, "tool") != "" {
		t.Fatal("ambiguous basename selected an arbitrary executable")
	}
	if GetFullPath(dir, "b/tool") != filepath.Join(dir, "b", "tool") {
		t.Fatal("explicit relative path failed to disambiguate")
	}
}
