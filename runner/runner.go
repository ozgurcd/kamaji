package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"
	"log"
	"os"
	"os/exec"
	"strings"
)

func prepareCmdline(target obj.Target) (string, error) {
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

func Run(workspaceConfig obj.WorkspaceConfig, target obj.Target, pythonArgs ...string) error {
	err := tools.CreateSandbox(target)
	if err != nil {
		return err
	}

	err = tools.CopyThirdPartyIntoSandbox()
	if err != nil {
		return err
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

	cmd := exec.Command("bash", "-c", cmdline)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "BUILD_WORKSPACE_DIRECTORY="+workspaceConfig.WorkspaceRoot)
	cmd.Env = append(cmd.Env, "KAMAJI_ORGANIZATION_DOMAIN="+workspaceConfig.WorkspaceVars[0].Org_Domain)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		log.Fatalf("Command execution failed: %v", err)
	}

	return nil
}
