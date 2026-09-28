package target

import (
	"kamaji/obj"
	"kamaji/rt"
	"net/http"
	"time"
)

var testRuntime rt.Runtime
var downloadClient = &http.Client{Timeout: 2 * time.Minute}

func ValidateTargetVariables(values map[string]any) error {
	return (&Manager{Runtime: &testRuntime, Client: downloadClient}).ValidateTargetVariables(values)
}
func InitThirdPartyUsedInTarget(workspace obj.WorkspaceConfig, selected obj.ExecTarget) error {
	return (&Manager{Runtime: &testRuntime, Client: downloadClient}).InitThirdPartyUsedInTarget(workspace, selected)
}
func ValidateDependencies(workspace obj.WorkspaceConfig, selected obj.ExecTarget) error {
	return (&Manager{Runtime: &testRuntime, Client: downloadClient}).ValidateDependencies(workspace, selected)
}
func cachePath(entry obj.ThirdPartyConfig) (string, error) {
	return (&Manager{Runtime: &testRuntime, Client: downloadClient}).cachePath(entry)
}
func downloadThirdParty(workspace obj.WorkspaceConfig, name string) error {
	return (&Manager{Runtime: &testRuntime, Client: downloadClient}).downloadThirdParty(workspace, name)
}

func downloadAndCacheFile(entry obj.ThirdPartyConfig) error {
	return (&Manager{Runtime: &testRuntime, Client: downloadClient}).downloadAndCacheFile(entry)
}
func validateCachedFile(entry obj.ThirdPartyConfig) error {
	return (&Manager{Runtime: &testRuntime, Client: downloadClient}).validateCachedFile(entry)
}
func downloadFile(url, path string) error {
	return (&Manager{Runtime: &testRuntime, Client: downloadClient}).downloadFile(url, path)
}
