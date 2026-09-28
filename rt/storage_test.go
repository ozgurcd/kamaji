package rt

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kamaji/obj"
)

func TestCachePruneBudgetPreviewAndLease(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime")
	state := &Runtime{Config: obj.RuntimeConfig{TmpDir: root}}
	for i := 1; i <= 3; i++ {
		path := filepath.Join(root, "cache", fmt.Sprintf("%064x", i))
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		payload := filepath.Join(path, "file")
		if err := os.WriteFile(payload, []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(int64(i), 0)
		if err := os.Chtimes(payload, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := state.CachePrune(4, true)
	if err != nil || len(removed) != 2 || removed[0].Name != fmt.Sprintf("%064x", 1) {
		t.Fatalf("preview: %v %v", removed, err)
	}
	entries, err := state.StorageEntries("cache")
	if err != nil || len(entries) != 3 {
		t.Fatal("preview modified cache")
	}
	lease, err := state.RuntimeLease(false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.CachePrune(4, false); err == nil {
		t.Fatal("prune allowed during active run")
	}
	lease.Close()
	if _, err := state.CachePrune(4, false); err != nil {
		t.Fatal(err)
	}
	entries, err = state.StorageEntries("cache")
	if err != nil || len(entries) != 1 || entries[0].Name != fmt.Sprintf("%064x", 3) {
		t.Fatal("prune did not retain newest within budget")
	}
}

func TestStorageInspectionRejectsSymlinkAndDoesNotCreate(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "missing")
	state := &Runtime{Config: obj.RuntimeConfig{TmpDir: missing}}
	if entries, err := state.StorageEntries("execroot"); err != nil || len(entries) != 0 {
		t.Fatal("missing runtime should be empty")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("inspection created storage")
	}
	state.Config.TmpDir = root
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "cache")); err != nil {
		t.Fatal(err)
	}
	if _, err := state.StorageEntries("cache"); err == nil {
		t.Fatal("followed linked cache")
	}
}

func TestZeroCacheBudgetRemovesEmptyEntries(t *testing.T) {
	state := &Runtime{Config: obj.RuntimeConfig{TmpDir: filepath.Join(t.TempDir(), "runtime")}}
	path := filepath.Join(state.Config.TmpDir, "cache", fmt.Sprintf("%064x", 1))
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	removed, err := state.CachePrune(0, false)
	if err != nil || len(removed) != 1 {
		t.Fatalf("empty cache entry survived zero budget: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("zero budget left empty entry")
	}
}
