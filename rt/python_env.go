package rt

import (
	"context"
	"fmt"
	"kamaji/internal/fsutil"
	"kamaji/internal/process"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SetupPythonEnv sets up a virtual environment under the tmp directory and installs requirements if available.

func runPython(ctx context.Context, grace time.Duration, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_, err := process.Run(ctx, cmd, grace)
	return err
}

func (scope *Runtime) SetupPythonEnv() error {
	lease, err := scope.RuntimeLease(true)
	if err != nil {
		return err
	}
	defer lease.Close()
	// Reject a linked legacy environment even when migrating to managed versions.
	if info, err := os.Lstat(filepath.Join(scope.Config.TmpDir, "venv")); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("legacy environment must not be a symlink")
	}
	parent := filepath.Join(scope.Config.TmpDir, "venvs")
	if err := EnsurePrivateDir(parent); err != nil {
		return err
	}
	venvDir, err := os.MkdirTemp(parent, "env-")
	if err != nil {
		return err
	}
	promoted := false
	defer func() {
		if !promoted {
			_ = os.RemoveAll(venvDir)
		}
	}()
	python := scope.Config.PythonInterpreter
	if python == "" {
		python = "python3"
	}

	fmt.Printf("Creating virtual environment at %s...\n", venvDir)
	if err := scope.pythonCommand(python, "-m", "venv", venvDir); err != nil {
		return fmt.Errorf("failed to create virtualenv: %w", err)
	}

	reqFile := scope.RequirementsFile
	if reqFile == "" {
		reqFile = "/usr/local/share/kamaji/requirements.txt"
	}
	if _, err := os.Stat(reqFile); err == nil {
		// Install requirements
		pipPath := filepath.Join(venvDir, "bin", "pip")

		fmt.Printf("Installing requirements from %s...\n", reqFile)
		if err := scope.pythonCommand(pipPath, "install", "-r", reqFile); err != nil {
			return fmt.Errorf("failed to install requirements: %w", err)
		}
	} else if os.IsNotExist(err) {
		fmt.Printf("No requirements.txt found at %s, skipping package installation.\n", reqFile)
	} else {
		return fmt.Errorf("error checking for requirements.txt: %w", err)
	}

	info, err := os.Stat(filepath.Join(venvDir, "bin", "python"))
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("created environment has no executable Python")
	}
	previous, _ := os.ReadFile(filepath.Join(scope.Config.TmpDir, "python-current"))
	if err := fsutil.AtomicWrite(filepath.Join(scope.Config.TmpDir, "python-current"), []byte(filepath.Base(venvDir))); err != nil {
		return err
	}
	promoted = true
	// Absolute shebangs remain valid: the environment itself is never moved.
	old := string(previous)
	if strings.HasPrefix(old, "env-") && filepath.Base(old) == old && old != filepath.Base(venvDir) {
		_ = os.RemoveAll(filepath.Join(parent, old))
	}
	return nil
}

func (scope *Runtime) CurrentPython() (string, error) {
	data, err := os.ReadFile(filepath.Join(scope.Config.TmpDir, "python-current"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	name := string(data)
	if !strings.HasPrefix(name, "env-") || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid managed Python selection")
	}
	dir := filepath.Join(scope.Config.TmpDir, "venvs", name)
	if err := CheckPrivateDir(filepath.Dir(dir)); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("managed Python environment missing")
	}
	if err := CheckPrivateDir(dir); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "bin", "python")
	info, err = os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("managed Python interpreter missing")
	}
	return path, nil
}
