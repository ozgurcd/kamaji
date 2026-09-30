//go:build integration

package buildsys

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCompiledArtifactGraph(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"kamaji.toml", "main.go"} {
		data, err := os.ReadFile(filepath.Join("../examples/build-project", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || !first.Success || len(first.Targets) != 3 {
		t.Fatalf("compile and consume graph: %+v %v", first, err)
	}
	for _, name := range []string{"out/greet", "out/greeting.txt"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	second, err := p.Build(context.Background(), nil, buildOptions())
	if err != nil || second.Targets[0].Status != "cached" || second.Targets[1].Status != "cached" || second.Targets[2].Status != "executed" {
		t.Fatalf("restore and verify graph: %+v %v", second, err)
	}
}
