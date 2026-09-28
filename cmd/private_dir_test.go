package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupRejectsLinkedRuntimeRoot(t *testing.T) {
	root := commandFixture(t)
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, testRuntime.Config.TmpDir); err != nil {
		t.Fatal(err)
	}
	removeAll = func(string) error { t.Fatal("cleanup followed unsafe runtime root"); return nil }
	if _, err := invoke("--cleanup"); err == nil {
		t.Fatal("unsafe cleanup accepted")
	}
}
