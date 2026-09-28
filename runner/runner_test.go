package runner

import (
	"bytes"
	"errors"
	"kamaji/obj"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	old := testRuntime.Config
	t.Cleanup(func() { testRuntime.Config = old })
	root := t.TempDir()
	cwd := filepath.Join(root, "source directory")
	if err := os.MkdirAll(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	testRuntime.Config = obj.RuntimeConfig{TmpDir: filepath.Join(root, "tmp"), PythonInterpreter: "/usr/bin/true", WorkspaceConfig: obj.WorkspaceConfig{WorkspaceRoot: cwd, RulesDir: filepath.Join(root, "rules"), RulesCommonDir: "common", WorkspaceVars: []obj.WorkspaceVar{{Org_Domain: "fixture.invalid"}}}, ThirdPartyFiles: map[string]obj.ThirdPartyFileInfo{}, ThirdPartyFinalPaths: map[string]string{}}
	return root
}
func put(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareArgs(t *testing.T) {
	fixture(t)
	testRuntime.Config.ThirdPartyFinalPaths["tool"] = "/cache/tool with spaces"
	args, err := prepareArgs(obj.ExecTarget{Rule: "nested/run.py", Config: map[string]any{"str": "a b; $(touch sentinel) 'quoted'", "num": 3, "bool": true, "map": map[any]any{"list": []any{map[any]any{"value": "a'b"}}}, "bin": "@@tool"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "nested/run.py"), "--bin=/cache/tool with spaces", "--bool=true", "--map={\"list\":[{\"value\":\"a'b\"}]}", "--num=3", "--str=a b; $(touch sentinel) 'quoted'"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("arguments = %#v; want %#v", args, want)
	}
	for _, tc := range []struct {
		name   string
		target obj.ExecTarget
	}{
		{"missing rule", obj.ExecTarget{}},
		{"missing dependency", obj.ExecTarget{Rule: "run.py", Config: map[string]any{"bin": "@@missing"}}},
		{"unsupported value", obj.ExecTarget{Rule: "run.py", Config: map[string]any{"bad": make(chan int)}}},
		{"invalid option", obj.ExecTarget{Rule: "run.py", Config: map[string]any{"": true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := prepareArgs(tc.target); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRun(t *testing.T) {
	for _, rule := range []string{"flat.py", "nested/deep/run.py", "//local.py", "absolute"} {
		t.Run(rule, func(t *testing.T) {
			root := fixture(t)
			expected := filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, rule)
			if strings.HasPrefix(rule, "//") {
				expected = filepath.Join(testRuntime.Config.WorkspaceConfig.WorkspaceRoot, rule[2:])
			}
			if rule == "absolute" {
				rule = filepath.Join(root, "absolute.py")
				expected = rule
			}
			put(t, expected, "rule fixture")
			old := runCommand
			t.Cleanup(func() { runCommand = old })
			calls := 0
			runCommand = func(c *exec.Cmd) error {
				calls++
				if !reflect.DeepEqual(c.Args, []string{testRuntime.Config.PythonInterpreter, expected, "--value=literal ; ' $(x)", "extra arg"}) {
					t.Fatalf("args=%#v", c.Args)
				}
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				if c.Dir != cwd {
					t.Fatalf("cwd=%q want %q", c.Dir, cwd)
				}
				if !contains(c.Env, "KAMAJI_ORGANIZATION_DOMAIN=fixture.invalid") || !contains(c.Env, "PYTHONPATH="+filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "common")) {
					t.Fatalf("required environment missing")
				}
				return nil
			}
			t.Setenv("PWD", "/not/the/working/directory")
			if err := Run(testRuntime.Config.WorkspaceConfig, obj.ExecTarget{Name: "demo", Rule: rule, Config: map[string]any{"value": "literal ; ' $(x)"}}, "extra arg"); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}
func contains(values []string, value string) bool {
	for _, s := range values {
		if s == value {
			return true
		}
	}
	return false
}

func TestRunErrors(t *testing.T) {
	for _, scenario := range []string{"missing workspace variables", "missing rule", "child failure", "invalid target"} {
		t.Run(scenario, func(t *testing.T) {
			fixture(t)
			put(t, filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "run.py"), "fixture")
			target := obj.ExecTarget{Name: "demo", Rule: "run.py"}
			old := runCommand
			t.Cleanup(func() { runCommand = old })
			runCommand = func(*exec.Cmd) error { return errors.New("child failed") }
			switch scenario {
			case "missing workspace variables":
				testRuntime.Config.WorkspaceConfig.WorkspaceVars = nil
			case "missing rule":
				target.Rule = "missing.py"
			case "invalid target":
				target.Name = "../escape"
			}
			if err := Run(testRuntime.Config.WorkspaceConfig, target); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRunIsolationDoesNotModifySource(t *testing.T) {
	fixture(t)
	testRuntime.Config.Isolated = true
	put(t, "input.txt", "original")
	put(t, filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "run.py"), "fixture")
	old := runCommand
	t.Cleanup(func() { runCommand = old })
	runCommand = func(c *exec.Cmd) error {
		path := filepath.Join(c.Dir, "input.txt")
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "original" {
			t.Fatalf("copied input=%q err=%v", data, err)
		}
		return os.WriteFile(path, []byte("changed"), 0600)
	}
	if err := Run(testRuntime.Config.WorkspaceConfig, obj.ExecTarget{Name: "demo", Rule: "run.py"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("input.txt")
	if err != nil || string(got) != "original" {
		t.Fatalf("source changed: %q %v", got, err)
	}
}

func TestRunMissingInterpreterAndDirectoryRule(t *testing.T) {
	fixture(t)
	old := runCommand
	t.Cleanup(func() { runCommand = old })
	runCommand = func(*exec.Cmd) error { t.Fatal("invalid command was executed"); return nil }
	testRuntime.Config.PythonInterpreter = ""
	t.Setenv("PATH", t.TempDir())
	if err := Run(testRuntime.Config.WorkspaceConfig, obj.ExecTarget{Name: "demo", Rule: "run.py"}); err == nil {
		t.Fatal("missing interpreter accepted")
	}
	testRuntime.Config.PythonInterpreter = "/fixture/python"
	if err := os.MkdirAll(filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "directory.py"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := Run(testRuntime.Config.WorkspaceConfig, obj.ExecTarget{Name: "demo", Rule: "directory.py"}); err == nil {
		t.Fatal("directory accepted as rule")
	}
}

func TestDebugDoesNotLogArgumentValues(t *testing.T) {
	fixture(t)
	testRuntime.Config.DebugMode = true
	put(t, filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "run.py"), "fixture")
	var output bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previousWriter) })
	old := runCommand
	runCommand = func(*exec.Cmd) error { return nil }
	t.Cleanup(func() { runCommand = old })
	const canary = "synthetic-log-canary-not-a-credential"
	if err := Run(testRuntime.Config.WorkspaceConfig, obj.ExecTarget{Name: "demo", Rule: "run.py", Config: map[string]any{"value": canary}}, canary); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), canary) {
		t.Fatal("debug log exposed argument values")
	}
}
