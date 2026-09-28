package runner

import (
	"fmt"
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
	if _, err := scope.Interpreter(); err != nil {
		return err
	}
	return scope.ValidateRule(selected)
}

func (scope *Executor) Interpreter() (string, error) {
	python := scope.Runtime.Config.PythonInterpreter
	if python == "" {
		python = "python3"
	}
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
	path, err := lookup(python)
	if err != nil {
		return "", fmt.Errorf("interpreter unavailable: %w", err)
	}
	return path, nil
}
