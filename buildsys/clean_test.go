package buildsys

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCorruptBuildHistory(t *testing.T) {
	p := actionFixture(t)
	result, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, ".kamaji/runs", result.ID+".json"), []byte("invalid JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadRun(result.ID); err == nil {
		t.Fatal("corrupt evidence returned as a valid record")
	}
}

func TestBuildCleanup(t *testing.T) {
	p := actionFixture(t)
	if _, err := p.Build(context.Background(), nil, buildOptions()); err != nil {
		t.Fatal(err)
	}
	paths, err := p.Clean(nil, CleanOptions{DryRun: true, Cache: true})
	if err != nil || len(paths) != 2 {
		t.Fatalf("cleanup preview: %v %v", paths, err)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "out/result")); err != nil {
		t.Fatal("preview removed an output")
	}
	if _, err := p.Clean(nil, CleanOptions{Cache: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "out/result")); !os.IsNotExist(err) {
		t.Fatal("declared output survived clean")
	}
	if _, err := os.Stat(filepath.Join(p.Root, "input.txt")); err != nil {
		t.Fatal("clean removed a source")
	}
	entries, err := os.ReadDir(filepath.Join(p.Root, ".kamaji/runs"))
	if err != nil || len(entries) == 0 {
		t.Fatal("clean removed evidence without --history")
	}
}
