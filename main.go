package main

import (
	"fmt"
	"kamaji/obj"
	"kamaji/rt"
	"kamaji/runner"
	"kamaji/target"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

func main() {
	obj.WorkspaceFile = "kamaji.workspace.yaml"
	rt.Init()

	buildFileName := pflag.StringP("build", "b", "BUILD.yaml", "name of the build file")
	debugModeFlag := pflag.BoolP("debug", "d", false, "debug mode")
	cleanupFlag := pflag.BoolP("cleanup", "c", false, "cleanup mode")
	isolatedFlag := pflag.BoolP("isolated", "i", false, "isolated mode")
	pythonInterpreterFlag := pflag.StringP("python", "p", "", "Path to python interpreter")
	setupPythonEnvFlag := pflag.Bool("setup-python-env", false, "Set up Python virtual environment for extensions")

	pflag.Parse()

	// Python venv setup mode (standalone)
	if *setupPythonEnvFlag {
		if err := setupPythonEnv(); err != nil {
			log.Fatalf("Failed to set up Python environment: %v\n", err)
		}
		fmt.Println("Python environment setup complete.")
		os.Exit(0)
	}

	rt.Config.PythonInterpreter = *pythonInterpreterFlag

	if *isolatedFlag {
		rt.Config.Isolated = true
	}

	if *cleanupFlag {
		for _, dirName := range []string{"cache", "execroot"} {
			dir := filepath.Join(rt.Config.TmpDir, dirName)
			if _, err := os.Stat(dir); err == nil {
				os.RemoveAll(dir)
			}
		}
		os.Exit(0)
	}

	if *debugModeFlag {
		rt.Config.DebugMode = true
	} else {
		rt.Config.DebugMode = false
	}

	targetName := strings.TrimSpace(pflag.Arg(0))
	if targetName == "" {
		fmt.Printf("Target name is required\n")
		os.Exit(1)
	}
	if rt.Config.DebugMode {
		log.Printf("Target name is %s\n", targetName)
	}

	var restOfTheArgs []string
	for i, arg := range os.Args {
		if arg == "--" {
			restOfTheArgs = os.Args[i+1:]
			break
		}
	}

	execTarget, err := target.ParseBuildFile(*buildFileName, targetName)
	if err != nil {
		log.Fatalf("Error parsing build file: %s\n", err.Error())
	}

	if execTarget.Name == "" {
		log.Fatalf("Target not found in build file\n")
	}

	rt.Config.ExecTarget = execTarget

	err = target.InitThirdPartyUsedInTarget(rt.Config.WorkspaceConfig, execTarget)
	if err != nil {
		log.Fatalf("Error initializing third party used in target: %s\n", err.Error())
	}

	err = runner.Run(rt.Config.WorkspaceConfig, execTarget, restOfTheArgs...)
	if err != nil {
		log.Fatalf("Error running target: %s\n", err.Error())
	}

	if rt.Config.DebugMode {
		log.Printf("Cleaning up execroot directory: %s\n", rt.Config.ExecRootDir)
	}
	err = os.RemoveAll(rt.Config.ExecRootDir)
	if err != nil {
		log.Fatalf("Error removing execroot directory: %s\n", err.Error())
	}
}

func setupPythonEnv() error {
	venvDir := ".venv"
	python := "python3"

	fmt.Println("Setting up Python virtual environment...")

	if _, err := os.Stat(filepath.Join(venvDir, "bin", "activate")); os.IsNotExist(err) {
		cmd := exec.Command(python, "-m", "venv", venvDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Creating virtualenv in %s\n", venvDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create virtualenv: %w", err)
		}
	} else {
		fmt.Println("Virtual environment already exists.")
	}

	pythonPath := filepath.Join(venvDir, "bin", "python")
	if _, err := os.Stat(pythonPath); err != nil {
		return fmt.Errorf("python not found in virtualenv: %w", err)
	}

	var reqFiles []string
	if _, err := os.Stat("requirements.txt"); err == nil {
		reqFiles = append(reqFiles, "requirements.txt")
	}

	_ = filepath.Walk("rules", func(path string, info os.FileInfo, err error) error {
		if info != nil && info.Name() == "requirements.txt" {
			reqFiles = append(reqFiles, path)
		}
		return nil
	})

	if len(reqFiles) == 0 {
		fmt.Println("No Python dependencies found.")
		return nil
	}

	for _, file := range reqFiles {
		fmt.Printf("Installing requirements from %s\n", file)
		cmd := exec.Command(pythonPath, "-m", "pip", "install", "-r", file)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pip install failed for %s: %w", file, err)
		}
	}

	return nil
}
