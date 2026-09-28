package utils

import (
	"kamaji/internal/fsutil"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationOperationsShareLock(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "installed")
	put(t, filepath.Join(destination, "rules/old.py"), "old", 0600)
	put(t, filepath.Join(root, "source/rules/new.py"), "new", 0600)
	lease, err := fsutil.Lock(filepath.Join(destination, ".install.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	installer := Installer{Directory: destination, SourceDir: filepath.Join(root, "source")}
	if err := installer.Install(); err == nil {
		t.Fatal("concurrent installation accepted")
	}
	if err := installer.Remove(); err == nil {
		t.Fatal("concurrent removal accepted")
	}
	data, err := os.ReadFile(filepath.Join(destination, "rules/old.py"))
	if err != nil || string(data) != "old" {
		t.Fatal("busy installation changed")
	}
	lease.Close()
	if err := installer.Install(); err != nil {
		t.Fatal(err)
	}
	if err := installer.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "rules")); !os.IsNotExist(err) {
		t.Fatal("rules were not removed")
	}
}
