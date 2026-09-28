package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyRejectsExistingDestination(t *testing.T) {
	root := t.TempDir()
	src, dst, out := filepath.Join(root, "src"), filepath.Join(root, "dst"), filepath.Join(root, "outside")
	put(t, filepath.Join(src, "nested/file"), "new", 0600)
	put(t, filepath.Join(out, "file"), "old", 0600)
	if err := os.Mkdir(dst, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(out, filepath.Join(dst, "nested")); err != nil {
		t.Fatal(err)
	}
	if err := CopyDirectory(src, dst); err == nil {
		t.Fatal("existing destination accepted")
	}
	data, err := os.ReadFile(filepath.Join(out, "file"))
	if err != nil || string(data) != "old" {
		t.Fatal("destination symlink changed external file")
	}
}
