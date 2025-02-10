package target

import (
	"fmt"
	"io"
	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

func ParseBuildFile(buildFileName string, targetName string) (obj.ExecTarget, error) {
	if rt.Config.DebugMode {
		log.Printf("Parsing build file: %s\n", buildFileName)
	}

	data, err := os.ReadFile(buildFileName)
	if err != nil {
		return obj.ExecTarget{}, err
	}

	var buildFile obj.BuildFile
	if err := yaml.Unmarshal(data, &buildFile); err != nil {
		return obj.ExecTarget{}, err
	}

	for _, target := range buildFile.Targets {
		if target.Name == targetName {
			return target, nil
		}
	}

	return obj.ExecTarget{}, fmt.Errorf("target %s not found", targetName)
}

// loadExpectedVariables is a private helper that reads the variables.yaml file
// and returns the contents of its "variables" section as a map.
func loadExpectedVariables(filePath string) (map[string]any, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading variables file: %s\n", err.Error())
		return nil, err
	}

	// Unmarshal the whole file into a generic map
	var yamlContent map[string]any
	err = yaml.Unmarshal(data, &yamlContent)
	if err != nil {
		return nil, err
	}

	// Extract the "variables" section
	variablesSection, ok := yamlContent["variables"]
	if !ok {
		return nil, fmt.Errorf("no 'variables' section found in the YAML file")
	}

	var variablesMap map[string]any
	switch v := variablesSection.(type) {
	case map[string]any:
		variablesMap = v
	case map[any]any:
		variablesMap = make(map[string]any)
		for key, value := range v {
			strKey, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key found in variables section: %v", key)
			}
			variablesMap[strKey] = value
		}
	default:
		return nil, fmt.Errorf("'variables' section is not a valid map")
	}

	return variablesMap, nil
}

func ValidateTargetVariables(target map[string]any) {
	variablesFile := filepath.Join(rt.Config.WorkspaceConfig.RulesDir, filepath.Dir(rt.Config.ExecTarget.Rule), "variables.yaml")
	expectedTypes, err := loadExpectedVariables(variablesFile)
	if err != nil {
		log.Printf("No variables file found for rule %s\n", filepath.Base(rt.Config.ExecTarget.Rule))
		os.Exit(1)
	}

	// Iterate over each expected variable
	for varName, expectedValue := range expectedTypes {
		var expectedType string

		// Determine the expected type depending on the format provided in YAML.
		switch v := expectedValue.(type) {
		case string:
			// The expected type is provided directly as a string.
			expectedType = v
		case map[string]any:
			// If the expected value is a map, look up the "type" key.
			if typeVal, exists := v["type"]; exists {
				if typeStr, ok := typeVal.(string); ok {
					expectedType = typeStr
				} else {
					fmt.Printf("Expected type for variable '%s' should be a string; got %T\n", varName, typeVal)
					continue
				}
			} else {
				fmt.Printf("No 'type' key found for variable '%s'\n", varName)
				continue
			}
		case map[any]any:
			// Convert this map to map[string]any
			conv := make(map[string]any)
			for key, val := range v {
				strKey, ok := key.(string)
				if !ok {
					fmt.Printf("Key is not a string: %v\n", key)
					continue
				}
				conv[strKey] = val
			}
			if typeVal, exists := conv["type"]; exists {
				if typeStr, ok := typeVal.(string); ok {
					expectedType = typeStr
				} else {
					fmt.Printf("Expected type for variable '%s' should be a string; got %T\n", varName, typeVal)
					continue
				}
			} else {
				fmt.Printf("No 'type' key found for variable '%s'\n", varName)
				continue
			}
		default:
			fmt.Printf("Unsupported expected type format for variable '%s': %T\n", varName, expectedValue)
			continue
		}

		// Retrieve the actual variable value from target
		actualValue, exists := target[varName]
		if !exists {
			fmt.Printf("Variable '%s' is missing in the target configuration.\n", varName)
			continue
		}

		// Determine the type of the actual value using our helper function.
		actualType := determineVariableType(actualValue)
		if actualType != expectedType {
			fmt.Printf("Type mismatch for variable '%s': expected '%s', got '%s'\n", varName, expectedType, actualType)
			os.Exit(1)
		}
	}
}

func determineVariableType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case int:
		return "int"
	case bool:
		return "bool"
	// Add more types as needed
	default:
		return "unknown"
	}
}

func InitThirdPartyUsedInTarget(workspaceConfig obj.WorkspaceConfig, target obj.ExecTarget) error {
	var lastError error
	for _, value := range target.Config {
		if strValue, ok := value.(string); ok && strings.HasPrefix(strValue, "@@") {
			downloadCandidate := strValue[2:]
			if err := downloadThirdParty(workspaceConfig, downloadCandidate); err != nil {
				log.Printf("Error downloading %s: %v", downloadCandidate, err)
				lastError = err
			}
		}
	}
	return lastError
}

func downloadThirdParty(workspaceConfig obj.WorkspaceConfig, downloadCandidate string) error {
	if rt.Config.DebugMode {
		log.Printf("Looking for third party config for %s\n", downloadCandidate)
	}
	thirdParty, err := findThirdPartyConfig(workspaceConfig, downloadCandidate)
	if err != nil {
		log.Fatalf("Third party config requested from BUILD.yaml for %s is not present in workspace config.\n", downloadCandidate)
	}

	if doesThirdPartyExist(thirdParty.Name) {
		if rt.Config.DebugMode {
			log.Printf("Third party %s already exists, skipping\n", thirdParty.Name)
		}
		return validateCachedFile(thirdParty)
	}

	err = downloadAndCacheFile(thirdParty)
	if err != nil {
		log.Printf("Failed to download and cache file: %s\n", err.Error())
		return err
	}

	return nil
}

func findThirdPartyConfig(workspaceConfig obj.WorkspaceConfig, downloadCandidate string) (obj.ThirdPartyConfig, error) {
	for _, thirdParty := range workspaceConfig.ThirdParty {
		if thirdParty.Name == downloadCandidate {
			return thirdParty, nil
		}
	}
	return obj.ThirdPartyConfig{}, fmt.Errorf("third party config not found for %s", downloadCandidate)
}

func doesThirdPartyExist(downloadCandidate string) bool {
	var tpConfig obj.ThirdPartyConfig
	for _, tp := range rt.Config.WorkspaceConfig.ThirdParty {
		if tp.Name == downloadCandidate {
			tpConfig = tp
			break
		}
	}

	sha256 := tpConfig.SHA256s[rt.Config.Platform]
	if sha256 == "" {
		return false
	}

	dirToCheck := filepath.Join(rt.Config.CacheDir, sha256)
	if rt.Config.DebugMode {
		log.Printf("Checking if third party exists: %s\n", dirToCheck)
	}

	if _, err := os.Stat(dirToCheck); os.IsNotExist(err) {
		if rt.Config.DebugMode {
			log.Printf("Third party does not exist: %s\n", dirToCheck)
		}
		return false
	}

	if rt.Config.DebugMode {
		log.Printf("Third party exists: %s\n", dirToCheck)
	}
	return true
}

func downloadAndCacheFile(thirdParty obj.ThirdPartyConfig) error {
	fmt.Printf("Downloading Third Party: %s\n", thirdParty.Name)
	if rt.Config.DebugMode {
		log.Printf("Downloading Third Party: %s\n", thirdParty.Name)
	}

	sha256, url := thirdParty.SHA256s[rt.Config.Platform], thirdParty.URLs[rt.Config.Platform]
	if sha256 == "" || url == "" {
		return fmt.Errorf("sha256 or url is empty for %s", thirdParty.Name)
	}

	if rt.Config.DebugMode {
		log.Printf("Downloading Third Party: %s from %s\n", thirdParty.Name, url)
	}

	cacheDir := filepath.Join(rt.Config.CacheDir, sha256)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %s", err.Error())
	}

	filePath := filepath.Join(cacheDir, "file")
	if err := downloadFile(url, filePath); err != nil {
		if rt.Config.DebugMode {
			log.Printf("Failed to download file: %s\n", err.Error())
		}
		return err
	}

	if !tools.IsFileValid(filePath, sha256) {
		if rt.Config.DebugMode {
			log.Printf("File is invalid\n")
		}
		return fmt.Errorf("file is invalid")
	}

	if err := tools.CreateMetadataFile(cacheDir, thirdParty.FilePath); err != nil {
		return err
	}

	rt.Config.ThirdPartyFiles[thirdParty.Name] = obj.ThirdPartyFileInfo{
		FileName:  cacheDir,
		FinalName: thirdParty.FilePath,
	}
	return nil
}

func validateCachedFile(thirdParty obj.ThirdPartyConfig) error {
	if rt.Config.DebugMode {
		log.Printf("Validating cached file for %s\n", thirdParty.Name)
	}
	cacheDir := filepath.Join(rt.Config.CacheDir, thirdParty.SHA256s[rt.Config.Platform])
	filePath := filepath.Join(cacheDir, "file")

	if !tools.IsFileValid(filePath, thirdParty.SHA256s[rt.Config.Platform]) {
		log.Printf("Cached file is invalid\n")
		return fmt.Errorf("file is invalid")
	}

	rt.Config.ThirdPartyFiles[thirdParty.Name] = obj.ThirdPartyFileInfo{
		FileName:  cacheDir,
		FinalName: thirdParty.FilePath,
	}

	if rt.Config.DebugMode {
		log.Printf("Cached file is valid\n")
	}

	return nil
}

func downloadFile(url, filePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s", resp.Status)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err.Error())
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to copy file: %s", err.Error())
	}

	if rt.Config.DebugMode {
		log.Printf("Downloaded file to: %s\n", filePath)
	}
	return nil
}
