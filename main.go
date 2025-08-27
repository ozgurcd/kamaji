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

	cfg "kamaji/config"

	"github.com/spf13/pflag"
)

func main() {
	obj.WorkspaceFile = "kamaji.workspace.yaml"

	// Global flags
	buildFileName := pflag.StringP("build", "b", "BUILD.yaml", "name of the build file")
	debugModeFlag := pflag.BoolP("debug", "d", false, "debug mode")
	cleanupFlag := pflag.BoolP("cleanup", "c", false, "cleanup mode")
	isolatedFlag := pflag.BoolP("isolated", "i", false, "isolated mode")
	pythonInterpreterFlag := pflag.StringP("python", "p", "", "Path to python interpreter")

	// Special operations
	initRulesFlag := pflag.Bool("rules-directory-create", false, "Set up a new rules directory under /usr/local/share/kamaji/rules")
	deleteRulesFlag := pflag.Bool("rules-directory-delete", false, "Delete global rules directory under /usr/local/share/kamaji/rules")
	rulesDirOverride := pflag.String("rules-directory", "", "Override path to rules directory")
	setupPythonEnvFlag := pflag.Bool("setup-python-env", false, "Set up Python virtual environment for Kamaji extensions")

	pflag.Parse()

	// --- Handle special commands before full init ---
	if *initRulesFlag {
		if err := ensureWriteAccess("/usr/local/share/kamaji"); err != nil {
			log.Fatalf("Permission error: %v\n", err)
		}

		if err := copyRulesToGlobalDir(); err != nil {
			log.Fatalf("Failed to initialize rules: %v\n", err)
		}
		fmt.Println("Rules copied to /usr/local/share/kamaji/rules successfully.")
		os.Exit(0)
	}

	if *deleteRulesFlag {
		if err := ensureWriteAccess("/usr/local/share/kamaji"); err != nil {
			log.Fatalf("Permission error: %v\n", err)
		}

		if err := os.RemoveAll("/usr/local/share/kamaji/rules"); err != nil {
			log.Fatalf("Failed to delete rules directory: %v\n", err)
		}
		fmt.Println("Rules directory deleted: /usr/local/share/kamaji/rules")
		os.Exit(0)
	}

	if *setupPythonEnvFlag {
		if err := ensureWriteAccess("/usr/local/share/kamaji"); err != nil {
			log.Fatalf("Permission error: %v\n", err)
		}
		err := rt.SetupPythonEnv()
		if err != nil {
			log.Fatalf("Failed to set up Python environment: %v\n", err)
		}
		os.Exit(0)
	}

	// --- Load user config ---
	userConfig := cfg.LoadUserConfig()
	rt.Config.PythonInterpreter = cfg.ResolveConfigValue(
		*pythonInterpreterFlag,
		"KAMAJI_PYTHON",
		userConfig["python"],
		"python3",
	)

	// --- Initialize runtime ---
	rt.InitRuntime(*rulesDirOverride)

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

	// --- Target execution ---
	targetName := strings.TrimSpace(pflag.Arg(0))
	if targetName == "" {
		fmt.Printf("Target name is required\n")
		os.Exit(1)
	}
	if rt.Config.DebugMode {
		log.Printf("Target name is %s\n", targetName)
	}

	// Get any trailing arguments after "--"
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
		log.Fatalf("Target %q not found in build file %q\n", targetName, *buildFileName)
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

	// TODO: check this.
	// Only cleanup execroot if it was set by the runner
	if rt.Config.ExecRootDir != "" {
		if rt.Config.DebugMode {
			log.Printf("Cleaning up execroot directory: %s\n", rt.Config.ExecRootDir)
		}
		err = os.RemoveAll(rt.Config.ExecRootDir)
		if err != nil {
			log.Fatalf("Error removing execroot directory: %s\n", err.Error())
		}
	}
}

func copyRulesToGlobalDir() error {
	src := filepath.Join("rules")
	dst := "/usr/local/share/kamaji/rules"
	reqSrc := "requirements.txt"
	reqDst := "/usr/local/share/kamaji/requirements.txt"

	// Ensure rules directory exists
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("rules directory not found at %s", src)
	}

	// Copy rules directory
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("failed to clear destination: %w", err)
	}
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		return copyFile(path, destPath)
	})
	if err != nil {
		return fmt.Errorf("failed to copy rules: %w", err)
	}

	// Copy requirements.txt if it exists
	if _, err := os.Stat(reqSrc); err == nil {
		if err := copyFile(reqSrc, reqDst); err != nil {
			return fmt.Errorf("failed to copy requirements.txt: %w", err)
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func ensureWriteAccess(path string) error {
	testFile := filepath.Join(path, ".kamaji_write_test")
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", path, err)
	}
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("write permission denied for %s (try running with sudo): %w", path, err)
	}
	f.Close()
	os.Remove(testFile)
	return nil
}
