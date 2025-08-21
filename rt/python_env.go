package rt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func SetupPythonEnv() error {
	venvPath := filepath.Join(Config.TmpDir, "venv")

	// Check if venv already exists
	if _, err := os.Stat(venvPath); err == nil {
		fmt.Println("Virtual environment already exists.")
		return nil
	}

	fmt.Println("Setting up Python virtual environment...")

	// Create venv
	cmd := exec.Command("python3", "-m", "venv", venvPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create virtual environment: %w", err)
	}

	// Find requirements.txt
	localReqPath := "requirements.txt"
	globalReqPath := "/usr/local/share/kamaji/requirements.txt"
	var reqPath string

	if _, err := os.Stat(localReqPath); err == nil {
		reqPath = localReqPath
	} else if _, err := os.Stat(globalReqPath); err == nil {
		reqPath = globalReqPath
	}

	// Install if a requirements.txt was found
	if reqPath != "" {
		fmt.Println("Installing requirements from", reqPath)
		pipCmd := exec.Command(filepath.Join(venvPath, "bin", "pip"), "install", "-r", reqPath)
		pipCmd.Stdout = os.Stdout
		pipCmd.Stderr = os.Stderr
		if err := pipCmd.Run(); err != nil {
			return fmt.Errorf("failed to install Python requirements: %w", err)
		}
	} else {
		fmt.Println("No requirements.txt found, skipping Python dependency installation.")
	}

	fmt.Println("Python environment setup complete.")
	return nil
}
