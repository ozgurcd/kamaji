package runner

import (
	"fmt"
	"kamaji/target"
	"kamaji/tools"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"kamaji/obj"
)

// Preflight checks execution prerequisites without writes, downloads, or children.
func (scope *Executor) Preflight(workspace obj.WorkspaceConfig, selected obj.ExecTarget) error {
	for key := range selected.Config {
		if key == "" || strings.ContainsAny(key, "=\x00") {
			return fmt.Errorf("invalid option name")
		}
	}
	if len(workspace.WorkspaceVars) == 0 {
		return fmt.Errorf("workspace_vars must contain an organization")
	}
	cwd, err := scope.Runtime.WorkingDirectory()
	if err != nil {
		return err
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return fmt.Errorf("working directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("working directory must be a directory")
	}
	_, err = scope.Program(selected)
	return err
}

// RuleProgram describes execution without invoking an interpreter or compiler.
type RuleProgram struct {
	Language, Mode, Executable string
	PrefixArgs                 []string
	ManagedPython              bool
}

// Program resolves either a configured interpreter or a direct rule executable.
func (scope *Executor) Program(selected obj.ExecTarget) (RuleProgram, error) {
	if err := scope.ValidateRule(selected); err != nil {
		return RuleProgram{}, err
	}
	schema, err := (&target.Manager{Runtime: scope.Runtime}).SchemaFor(selected)
	if err != nil {
		return RuleProgram{}, fmt.Errorf("load rule definition: %w", err)
	}
	program := RuleProgram{Language: schema.Language, Mode: "executable"}
	if schema.Execution == nil && schema.Language == "python" {
		path, err := scope.Interpreter()
		program.Mode, program.Executable, program.ManagedPython = "interpreter", path, true
		return program, err
	}
	if schema.Execution != nil && schema.Execution.Mode == "interpreter" {
		path, err := scope.lookupExecutable(schema.Execution.Command[0])
		if err != nil {
			return RuleProgram{}, fmt.Errorf("rule interpreter unavailable: %w", err)
		}
		program.Mode, program.Executable = "interpreter", path
		program.PrefixArgs = append([]string(nil), schema.Execution.Command[1:]...)
		return program, nil
	}
	rule, err := (&tools.Context{Runtime: scope.Runtime}).GetRule(selected)
	if err != nil {
		return RuleProgram{}, err
	}
	if !filepath.IsAbs(rule) {
		rule = filepath.Join(scope.Runtime.Config.WorkspaceConfig.RulesDir, rule)
	}
	rule, err = filepath.Abs(rule)
	if err != nil {
		return RuleProgram{}, err
	}
	info, err := os.Stat(rule)
	if err != nil {
		return RuleProgram{}, fmt.Errorf("rule executable: %w", err)
	}
	if info.Mode().Perm()&0111 == 0 {
		return RuleProgram{}, fmt.Errorf("rule is not executable; build it for the current platform and grant execute permission")
	}
	program.Executable = rule
	return program, nil
}

func (scope *Executor) Interpreter() (string, error) {
	python := scope.Runtime.Config.PythonInterpreter
	if python == "" {
		python = "python3"
	}
	path, err := scope.lookupExecutable(python)
	if err != nil {
		return "", fmt.Errorf("interpreter unavailable: %w", err)
	}
	return path, nil
}

func (scope *Executor) lookupExecutable(python string) (string, error) {
	if strings.ContainsRune(python, filepath.Separator) && !filepath.IsAbs(python) {
		cwd, err := scope.Runtime.WorkingDirectory()
		if err != nil {
			return "", err
		}
		python = filepath.Join(cwd, python)
	}
	lookup := scope.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	return lookup(python)
}
