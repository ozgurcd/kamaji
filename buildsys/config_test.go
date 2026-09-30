package buildsys

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGraphConfiguration(t *testing.T) {
	data := `version = 1
default = ["test"]
[[targets]]
name = "compile"
command = ["go", "build", "-o", "out/app", "."]
inputs = ["src", "go.mod"]
outputs = ["out/app"]
cache = true
[[targets]]
name = "test"
deps = ["compile"]
command = ["out/app", "--self-test"]
`
	p, err := Parse("kamaji.toml", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	order, err := p.Select(nil)
	if err != nil || !reflect.DeepEqual(order, []string{"compile", "test"}) {
		t.Fatalf("order %v, error %v", order, err)
	}
	if p.Targets[0].Slots != 1 || p.Targets[0].Effect != "build" {
		t.Fatal("defaults not applied")
	}
	if _, err := p.Select([]string{"typo"}); err == nil {
		t.Fatal("unknown target accepted")
	}
	for name, invalid := range map[string]string{
		"generated command dependency": strings.Replace(data, `deps = ["compile"]`, `deps = []`, 1),
		"unknown":                      data + "misspelled = true\n",
		"duplicate":                    data + "[[targets]]\nname = 'test'\n",
		"cycle":                        strings.Replace(data, `name = "compile"`, "name = 'compile'\ndeps = ['test']", 1),
		"missing dependency":           strings.Replace(data, `deps = ["compile"]`, `deps = ["absent"]`, 1),
		"overlapping output":           data + "[[targets]]\nname = 'other'\ncommand = ['true']\noutputs = ['out']\n",
		"escape":                       strings.Replace(data, `outputs = ["out/app"]`, `outputs = ["../app"]`, 1),
		"reserved output":              strings.Replace(data, `outputs = ["out/app"]`, `outputs = [".kamaji/app"]`, 1),
		"cache without outputs":        strings.Replace(data, `outputs = ["out/app"]`, "", 1),
		"external cache":               strings.Replace(data, "cache = true", "cache = true\neffect = 'external'", 1),
		"bad timeout":                  data + "timeout = 'oops'\n",
		"empty argv":                   strings.Replace(data, `command = ["out/app", "--self-test"]`, `command = [""]`, 1),
		"source overwrite":             strings.Replace(data, `outputs = ["out/app"]`, `outputs = ["src"]`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse("kamaji.toml", []byte(invalid)); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestGraphYAMLAndDiscovery(t *testing.T) {
	p, err := Parse("kamaji.yaml", []byte("version: 1\ntargets:\n  - name: check\n    command: [go, test, ./...]\n"))
	if err != nil || len(p.Targets) != 1 {
		t.Fatalf("YAML graph config: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kamaji.toml"), []byte("version = 1\n[[targets]]\nname = 'check'\ncommand = ['true']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(child, "")
	if err != nil || loaded.Root != root {
		t.Fatalf("discovery: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "kamaji.yaml"), []byte("version: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(child, ""); err == nil {
		t.Fatal("ambiguous configuration accepted")
	}
}
