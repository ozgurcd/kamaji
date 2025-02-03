package rt

import (
	"fmt"
	"kamaji/obj"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v2"
)

var Config obj.RuntimeConfig

func readWorkspaceConfig() (obj.WorkspaceConfig, error) {
	workspaceConfig := obj.WorkspaceConfig{}

	workspaceFile, err := os.Open(filepath.Join(Config.WorkspaceDir, "kamaji.workspace.yaml"))
	if err != nil {
		return workspaceConfig, err
	}

	err = yaml.NewDecoder(workspaceFile).Decode(&workspaceConfig)
	if err != nil {
		return workspaceConfig, err
	}

	// if the rules dir starts with a //, replace it with the workspace root
	if strings.HasPrefix(workspaceConfig.RulesDir, "//") {
		workspaceConfig.RulesDir = filepath.Join(Config.WorkspaceDir, workspaceConfig.RulesDir[2:])
	}

	return workspaceConfig, nil
}

func detectWorkspaceRoot() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "kamaji.workspace.yaml")); err == nil {
			Config.WorkspaceDir = dir
			return nil
		}

		dir = filepath.Dir(dir)
		if dir == "/" {
			return fmt.Errorf("kamaji.workspace.yaml file not found")
		}
	}
}

func Init() {
	Config.ThirdPartyFiles = make(map[string]obj.ThirdPartyFileInfo)
	Config.ThirdPartyFinalPaths = make(map[string]string)
	err := detectWorkspaceRoot()
	if err != nil {
		fmt.Printf("Error detecting workspace root: %v\n", err)
		os.Exit(1)
	}

	workspaceConfig, err := readWorkspaceConfig()
	if err != nil {
		fmt.Printf("Error reading workspace file: %v\n", err)
		os.Exit(1)
	}

	Config.WorkspaceConfig = workspaceConfig
	Config.CacheDir = initCacheDir()
	Config.Platform = runtime.GOOS + "_" + runtime.GOARCH
}

func initCacheDir() string {
	user, err := user.Current()
	if err != nil {
		fmt.Printf("Cannot determine current user, exiting\n")
		os.Exit(1)
	}

	tmpDir := map[string]string{
		"darwin": fmt.Sprintf("/var/tmp/_kamaji_%s", user.Username),
		"linux":  fmt.Sprintf("/tmp/_kamaji_%s", user.Username),
	}[runtime.GOOS]

	if tmpDir == "" {
		fmt.Printf("Unsupported OS: %s\n", runtime.GOOS)
		os.Exit(1)
	}

	Config.TmpDir = tmpDir

	err = os.MkdirAll(tmpDir, 0755)
	if err != nil {
		fmt.Printf("Cannot create tmp dir: %s\n", err.Error())
		os.Exit(1)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		fmt.Printf("Cannot create cache dir: %s\n", err.Error())
		os.Exit(1)
	}

	shaDir := filepath.Join(cacheDir, "sha256")
	err = os.MkdirAll(shaDir, 0755)
	if err != nil {
		fmt.Printf("Cannot create sha dir: %s\n", err.Error())
		os.Exit(1)
	}

	return cacheDir
}
