package execroot

import (
	"kamaji/obj"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectLinkedExecutionParent(t *testing.T) {
	root := fixture(t)
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "execroot")); err != nil {
		t.Fatal(err)
	}
	if err := CreateExecRootDir(obj.ExecTarget{Name: "demo"}); err == nil {
		t.Fatal("linked execution parent accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("created execution directory outside runtime root")
	}
}
