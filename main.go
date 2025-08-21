package main

import (
	"fmt"
	"io"
	"kamaji/obj"
	"kamaji/rt"
	"kamaji/runner"
	"kamaji/target"
	"log"
	"os"
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
	initRulesFlag := pflag.Bool("init-rules", false, "Copy built-in rules to /usr/local/share/kamaji/rules")
	setupPythonEnvFlag := pflag.Bool("setup-python-env", false, "Set up Python virtual environment for Kamaji extensions")

	pflag.Parse()

	if *setupPythonEnvFlag {
		err := rt.SetupPythonEnv()
		if err != nil {
			log.Fatalf("Failed to set up Python environment: %v\n", err)
		}
		os.Exit(0)
	}

	if *initRulesFlag {
		err := copyRulesToGlobalDir()
		if err != nil {
			log.Fatalf("Failed to initialize rules: %v\n", err)
		}
		fmt.Println("Rules copied to /usr/local/share/kamaji/rules successfully.")
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

	rt.Config.DebugMode = *debugModeFlag

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

func copyRulesToGlobalDir() error {
	srcRulesDir := "rules"
	dstBaseDir := "/usr/local/share/kamaji"
	dstRulesDir := filepath.Join(dstBaseDir, "rules")
	reqFileSrc := "requirements.txt"
	reqFileDst := filepath.Join(dstBaseDir, "requirements.txt")

	// Ensure destination base directory exists
	if err := os.MkdirAll(dstBaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination base directory: %w", err)
	}

	// Copy rules directory
	if _, err := os.Stat(srcRulesDir); os.IsNotExist(err) {
		return fmt.Errorf("rules directory not found at %s", srcRulesDir)
	}

	// Clear existing rules directory
	if err := os.RemoveAll(dstRulesDir); err != nil {
		return fmt.Errorf("failed to clear existing rules directory: %w", err)
	}

	err := filepath.Walk(srcRulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcRulesDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dstRulesDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to copy rules: %w", err)
	}

	// Optionally copy requirements.txt
	if _, err := os.Stat(reqFileSrc); err == nil {
		srcFile, err := os.Open(reqFileSrc)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", reqFileSrc, err)
		}
		defer srcFile.Close()

		dstFile, err := os.Create(reqFileDst)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", reqFileDst, err)
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return fmt.Errorf("failed to copy requirements.txt: %w", err)
		}
	}

	return nil
}
