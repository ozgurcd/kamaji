package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"kamaji/execroot"
	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func prepareCmdline(target obj.ExecTarget) (string, error) {
	convertToJSON := func(value any) (string, error) {
		switch v := value.(type) {
		case map[any]any:
			normalizedMap := tools.NormalizeMap(v)
			jsonBytes, err := json.Marshal(normalizedMap)
			if err != nil {
				return "", errors.New("error marshaling map to JSON")
			}
			return fmt.Sprintf("'%s'", string(jsonBytes)), nil
		case string:
			if strings.HasPrefix(v, "@@") {
				if rt.Config.DebugMode {
					log.Printf("Resolving third party file: %s\n", v)
					log.Printf("Third party final paths: %+v\n", rt.Config.ThirdPartyFinalPaths)
				}
				resolvedKey := v[2:]
				if resolvedPath, exists := rt.Config.ThirdPartyFinalPaths[resolvedKey]; exists {
					return resolvedPath, nil
				} else {
					return "", fmt.Errorf("third party file not found: %s", resolvedKey)
				}
			}
			return v, nil
		case bool:
			return fmt.Sprintf("%t", v), nil
		default:
			return fmt.Sprintf("%v", v), nil
		}
	}

	cmdline := fmt.Sprintf("python3 %s/%s", rt.Config.WorkspaceConfig.RulesDir, target.Rule)
	for k, v := range target.Config {
		jsonValue, err := convertToJSON(v)
		if err != nil {
			return "", err
		}
		cmdline += fmt.Sprintf(" --%s=%s", k, jsonValue)
	}

	return cmdline, nil
}

func Run(workspaceConfig obj.WorkspaceConfig, target obj.ExecTarget, pythonArgs ...string) error {
	if rt.Config.DebugMode {
		log.Printf("Creating execroot dir: %s\n", rt.Config.ExecRootDir)
	}
	err := execroot.CreateExecRootDir(target)
	if err != nil {
		return err
	}

	if rt.Config.DebugMode {
		log.Printf("Copying third party into execroot dir: %s\n", rt.Config.ExecRootDir)
	}
	err = execroot.CopyThirdPartyIntoExecRootDir()
	if err != nil {
		return err
	}

	if rt.Config.DebugMode {
		log.Printf("Creating rules dir in execroot dir: %s\n", rt.Config.ExecRootDir)
	}
	rulesDirInExecRoot := filepath.Join(rt.Config.ExecRootDir, "rules")
	err = os.MkdirAll(rulesDirInExecRoot, 0700)
	if err != nil {
		return err
	}

	commonDirInExecRoot := filepath.Join(rt.Config.ExecRootDir, "common")

	if rt.Config.DebugMode {
		log.Printf("Creating rule dir in execroot dir: %s\n", rt.Config.ExecRootDir)
	}
	ruleDir := filepath.Dir(target.Rule)
	linkSource := filepath.Join(rt.Config.WorkspaceConfig.RulesDir, ruleDir)
	linkTarget := filepath.Join(rulesDirInExecRoot, ruleDir)

	err = os.Symlink(linkSource, linkTarget)
	if err != nil {
		return err
	}

	if rt.Config.DebugMode {
		log.Printf("Creating common dir in execroot dir: %s\n", rt.Config.ExecRootDir)
	}
	linkSource = filepath.Join(rt.Config.WorkspaceConfig.RulesDir, "common")
	err = os.Symlink(linkSource, commonDirInExecRoot)
	if err != nil {
		if rt.Config.DebugMode {
			log.Printf("Error creating symlink for common dir: %s to %s\n", linkSource, commonDirInExecRoot)
		}
		return err
	}

	if rt.Config.Isolated {
		if rt.Config.DebugMode {
			log.Printf("Mirroring directory with sym links: %s\n", rt.Config.ExecRootDir)
		}
		targetDir := filepath.Join(rt.Config.ExecRootDir, "origin")
		sourceDir := os.Getenv("PWD")
		err = tools.MirrorDirectoryWithSymLinks(sourceDir, targetDir)
		if err != nil {
			return err
		}
	}

	if rt.Config.DebugMode {
		log.Printf("Preparing cmdline for target: %s\n", target.Name)
	}
	cmdline, err := prepareCmdline(target)
	if err != nil {
		return err
	}

	if cmdline == "" {
		return errors.New("cmdline could not be prepared")
	}

	if len(pythonArgs) > 0 {
		cmdline += " " + strings.Join(pythonArgs, " ")
	}

	if rt.Config.DebugMode {
		log.Printf("Running command:\n%s\n", cmdline)
	}

	pythonPath := workspaceConfig.RulesDir + "/" + rt.Config.WorkspaceConfig.RulesCommonDir
	cmd := exec.Command("bash", "-c", cmdline)

	if rt.Config.Isolated {
		cmd.Dir = rt.Config.ExecRootDir + "/" + "origin"
	} else {
		cmd.Dir = os.Getenv("PWD")
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "KAMAJI_ORGANIZATION_DOMAIN="+workspaceConfig.WorkspaceVars[0].Org_Domain)
	cmd.Env = append(cmd.Env, "PYTHONPATH="+pythonPath)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		log.Fatalf("Command execution failed: %v", err)
	}

	return nil
}
