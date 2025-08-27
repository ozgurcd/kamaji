package rt

import (
	"fmt"
	"kamaji/obj"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v2"
)

var Config obj.RuntimeConfig

func readWorkspaceConfig(rulesDirOverride string) (obj.WorkspaceConfig, error) {
	workspaceConfig := obj.WorkspaceConfig{}

	workspaceFilePath := filepath.Join(Config.WorkspaceDir, obj.WorkspaceFile)
	if _, err := os.Stat(workspaceFilePath); err == nil {
		f, err := os.Open(workspaceFilePath)
		if err != nil {
			return workspaceConfig, err
		}
		defer f.Close()

		err = yaml.NewDecoder(f).Decode(&workspaceConfig)
		if err != nil {
			return workspaceConfig, err
		}
	}

	// Determine RulesDir with proper priority
	if rulesDirOverride != "" {
		workspaceConfig.RulesDir = rulesDirOverride
	} else if strings.HasPrefix(workspaceConfig.RulesDir, "//") {
		workspaceConfig.RulesDir = filepath.Join(Config.WorkspaceDir, workspaceConfig.RulesDir[2:])
	} else if strings.TrimSpace(workspaceConfig.RulesDir) == "" {
		workspaceConfig.RulesDir = "/usr/local/share/kamaji/rules"
	}

	if Config.DebugMode {
		log.Printf("Resolved rules directory: %s\n", workspaceConfig.RulesDir)
	}
	return workspaceConfig, nil
}

// func detectWorkspaceRoot() error {
// 	dir, err := os.Getwd()
// 	if err != nil {
// 		return err
// 	}

// 	for {
// 		if _, err := os.Stat(filepath.Join(dir, obj.WorkspaceFile)); err == nil {
// 			Config.WorkspaceDir = dir
// 			return nil
// 		}

// 		dir = filepath.Dir(dir)
// 		if dir == "/" {
// 			return fmt.Errorf("%s file not found", obj.WorkspaceFile)
// 		}
// 	}
// }

func detectWorkspaceRoot() error {
	startDir, err := os.Getwd()
	if err != nil {
		return err
	}

	dir := startDir
	for {
		if isWorkspaceRoot(dir) {
			Config.WorkspaceDir = dir
			return nil
		}

		parent := filepath.Dir(dir)
		if parent == dir { // reached root
			break
		}
		dir = parent
	}

	return fmt.Errorf("%s file not found starting from %s", obj.WorkspaceFile, startDir)
}

func isWorkspaceRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, obj.WorkspaceFile))
	return err == nil
}

func InitRuntime(rulesDirOverride string) {
	Config.ThirdPartyFiles = make(map[string]obj.ThirdPartyFileInfo)
	Config.ThirdPartyFinalPaths = make(map[string]string)

	err := detectWorkspaceRoot()
	if err != nil {
		fmt.Printf("Error detecting workspace root: %v\n", err)
		os.Exit(1)
	}

	workspaceConfig, err := readWorkspaceConfig(rulesDirOverride)
	if err != nil {
		fmt.Printf("Error reading workspace file: %v\n", err)
		os.Exit(1)
	}

	Config.WorkspaceConfig = workspaceConfig
	Config.CacheDir = initCacheDir()
	Config.Platform = runtime.GOOS + "_" + runtime.GOARCH

	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func initCacheDir() string {
	var tmpDir string

	// Use the test's temporary directory if set
	if Config.TmpDir != "" {
		tmpDir = Config.TmpDir
	} else {
		// Determine the system-wide temporary directory
		user, err := user.Current()
		if err != nil {
			fmt.Printf("Cannot determine current user, exiting\n")
			os.Exit(1)
		}

		tmpDir = map[string]string{
			"darwin": fmt.Sprintf("/var/tmp/_kamaji_%s", user.Username),
			"linux":  fmt.Sprintf("/tmp/_kamaji_%s", user.Username),
		}[runtime.GOOS]

		if tmpDir == "" {
			fmt.Printf("Unsupported OS: %s\n", runtime.GOOS)
			os.Exit(1)
		}

		Config.TmpDir = tmpDir
	}

	fmt.Printf("Attempting to create tmp dir: %s\n", tmpDir)

	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		fmt.Printf("Error creating tmp dir: %s\n", err.Error())
		os.Exit(1)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	fmt.Printf("Attempting to create cache dir: %s\n", cacheDir)

	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		fmt.Printf("Error creating cache dir: %s\n", err.Error())
		os.Exit(1)
	}

	shaDir := filepath.Join(cacheDir, "sha256")
	fmt.Printf("Attempting to create sha dir: %s\n", shaDir)

	err = os.MkdirAll(shaDir, 0755)
	if err != nil {
		fmt.Printf("Error creating sha dir: %s\n", err.Error())
		os.Exit(1)
	}

	return cacheDir
}
