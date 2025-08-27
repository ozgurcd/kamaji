package rt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// SetupPythonEnv sets up a virtual environment under the tmp directory and installs requirements if available.
func SetupPythonEnv() error {
	venvDir := filepath.Join(Config.TmpDir, "venv")

	// Create virtual environment
	cmd := exec.Command("python3", "-m", "venv", venvDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Creating virtual environment at %s...\n", venvDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create virtualenv: %w", err)
	}

	// Check if requirements.txt exists
	reqFile := "/usr/local/share/kamaji/requirements.txt"
	if _, err := os.Stat(reqFile); err == nil {
		// Install requirements
		pipPath := filepath.Join(venvDir, "bin", "pip")
		installCmd := exec.Command(pipPath, "install", "-r", reqFile)
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr

		fmt.Printf("Installing requirements from %s...\n", reqFile)
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install requirements: %w", err)
		}
	} else if os.IsNotExist(err) {
		fmt.Printf("No requirements.txt found at %s, skipping package installation.\n", reqFile)
	} else {
		return fmt.Errorf("error checking for requirements.txt: %w", err)
	}

	return nil
}
