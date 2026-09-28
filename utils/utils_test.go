package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func put(t *testing.T, path, text string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
}
func TestEnsureWriteAccess(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "new")
	if err := EnsureWriteAccess(path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe left files: %v %v", entries, err)
	}
	put(t, filepath.Join(path, ".kamaji_write_test"), "keep", 0600)
	if err := EnsureWriteAccess(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(path, ".kamaji_write_test"))
	if err != nil || string(data) != "keep" {
		t.Fatalf("existing file changed: %q %v", data, err)
	}
	put(t, filepath.Join(root, "blocked"), "file", 0600)
	if err := EnsureWriteAccess(filepath.Join(root, "blocked", "child")); err == nil {
		t.Fatal("expected blocked directory error")
	}
}
func TestCopyFile(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "source")
	dst := filepath.Join(root, "deep", "dest")
	put(t, src, "payload", 0751)
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "payload" {
		t.Fatalf("copy=%q %v", data, err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0751 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	if err := copyFile(filepath.Join(root, "missing"), dst); err == nil {
		t.Fatal("missing source accepted")
	}
	if err := copyFile(src, root); err == nil {
		t.Fatal("directory destination accepted")
	}
}

func TestCopyDirectory(t *testing.T) {
	for _, scenario := range []string{"nested", "inside source", "missing", "file source", "symlink", "blocked destination"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			source, destination := filepath.Join(root, "source"), filepath.Join(root, "destination")
			put(t, filepath.Join(source, "nested", "file"), "payload", 0751)
			switch scenario {
			case "inside source":
				destination = filepath.Join(source, "destination")
			case "missing":
				source = filepath.Join(root, "missing")
			case "file source":
				source = filepath.Join(source, "nested", "file")
			case "symlink":
				if err := os.Symlink("nested/file", filepath.Join(source, "link")); err != nil {
					t.Fatal(err)
				}
			case "blocked destination":
				put(t, destination, "blocked", 0600)
			}
			err := CopyDirectory(source, destination)
			if scenario != "nested" {
				if err == nil {
					t.Fatal("expected copy error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(destination, "nested", "file"))
			if err != nil || string(data) != "payload" {
				t.Fatalf("copy=%q %v", data, err)
			}
		})
	}
}
func TestCopyRulesToGlobalDir(t *testing.T) {
	for _, scenario := range []string{"success", "no requirements", "missing rules", "failed copy preserves installation"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			old := globalShareDir
			globalShareDir = filepath.Join(root, "installed")
			t.Cleanup(func() { globalShareDir = old })
			put(t, filepath.Join(globalShareDir, "rules", "old.py"), "old", 0600)
			put(t, filepath.Join(globalShareDir, "requirements.txt"), "old requirement", 0600)
			if scenario != "missing rules" {
				put(t, "rules/nested/new.py", "new", 0751)
			}
			if scenario != "no requirements" {
				put(t, "requirements.txt", "new requirement", 0600)
			}
			if scenario == "failed copy preserves installation" {
				if err := os.Symlink("missing", filepath.Join(root, "rules", "broken")); err != nil {
					t.Fatal(err)
				}
			}
			err := CopyRulesToGlobalDir()
			wantErr := scenario == "missing rules" || scenario == "failed copy preserves installation"
			if (err != nil) != wantErr {
				t.Fatalf("error=%v wantError=%v", err, wantErr)
			}
			if wantErr {
				data, err := os.ReadFile(filepath.Join(globalShareDir, "rules", "old.py"))
				if err != nil || string(data) != "old" {
					t.Fatalf("installation lost: %q %v", data, err)
				}
				return
			}
			data, err := os.ReadFile(filepath.Join(globalShareDir, "rules", "nested", "new.py"))
			if err != nil || string(data) != "new" {
				t.Fatalf("installed=%q %v", data, err)
			}
			if _, err := os.Stat(filepath.Join(globalShareDir, "rules", "old.py")); !os.IsNotExist(err) {
				t.Fatalf("stale rule remains: %v", err)
			}
			req, err := os.ReadFile(filepath.Join(globalShareDir, "requirements.txt"))
			if scenario == "no requirements" {
				if !os.IsNotExist(err) {
					t.Fatal("stale requirements remain")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := "new requirement"
			if string(req) != want {
				t.Fatalf("requirements=%q want %q", req, want)
			}
		})
	}
}

func TestCopyRulesFromInstalledDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	old := globalShareDir
	globalShareDir = root
	t.Cleanup(func() { globalShareDir = old })
	put(t, filepath.Join(root, "rules", "installed.py"), "existing rule", 0751)
	put(t, filepath.Join(root, "requirements.txt"), "fixture", 0600)
	if err := CopyRulesToGlobalDir(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "rules", "installed.py"))
	if err != nil || string(data) != "existing rule" {
		t.Fatal("reinstall destroyed its source")
	}
}
