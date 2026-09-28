package target

import (
	"bytes"
	"encoding/hex"
	"fmt"
	cfg "kamaji/config"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kamaji/obj"
	"kamaji/tools"
)

func ParseBuildFile(buildFileName, targetName string) (obj.ExecTarget, error) {
	build, err := LoadBuildFile(buildFileName)
	if err != nil {
		return obj.ExecTarget{}, err
	}
	for _, entry := range build.Targets {
		if entry.Name == targetName {
			return entry, nil
		}
	}
	names := make([]string, 0, len(build.Targets))
	for _, entry := range build.Targets {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return obj.ExecTarget{}, fmt.Errorf("target %q not found; available targets: %s", targetName, strings.Join(names, ", "))
}

func LoadBuildFile(buildFileName string) (obj.BuildFile, error) {
	data, err := os.ReadFile(buildFileName)
	if err != nil {
		return obj.BuildFile{}, err
	}
	var build obj.BuildFile
	if err := cfg.DecodeYAML(bytes.NewReader(data), &build); err != nil {
		return obj.BuildFile{}, err
	}
	names := make(map[string]bool)
	for i := range build.Targets {
		selected := &build.Targets[i]
		if selected.Name == "" || names[selected.Name] {
			return obj.BuildFile{}, fmt.Errorf("empty or duplicate target %q", selected.Name)
		}
		names[selected.Name] = true
		if strings.TrimSpace(selected.Rule) == "" {
			return obj.BuildFile{}, fmt.Errorf("target %q has no rule", selected.Name)
		}
		if selected.Config == nil {
			selected.Config = make(map[string]any)
		}
	}
	return build, nil
}

func (scope *Manager) ValidateTargetVariables(values map[string]any) error {
	schema, err := scope.Schema()
	if err != nil {
		return fmt.Errorf("load rule variables: %w", err)
	}
	return validateSchema(schema, values)
}

func (scope *Manager) InitThirdPartyUsedInTarget(workspace obj.WorkspaceConfig, selected obj.ExecTarget) error {
	if err := scope.ValidateDependencies(workspace, selected); err != nil {
		return err
	}
	names := []string{}
	seen := map[string]bool{}
	for _, value := range selected.Config {
		if name, ok := value.(string); ok && strings.HasPrefix(name, "@@") {
			if seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, strings.TrimPrefix(name, "@@"))
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if err := scope.downloadThirdParty(workspace, name); err != nil {
			return err
		}
	}
	return nil
}

func (scope *Manager) ValidateDependencies(workspace obj.WorkspaceConfig, selected obj.ExecTarget) error {
	for _, value := range selected.Config {
		if name, ok := value.(string); ok && strings.HasPrefix(name, "@@") {
			entry, err := findThirdPartyConfig(workspace, strings.TrimPrefix(name, "@@"))
			if err != nil {
				return err
			}
			if _, err := scope.cachePath(entry); err != nil {
				return err
			}
			location, err := url.ParseRequestURI(entry.URLs[scope.Runtime.Config.Platform])
			if err != nil || location.Hostname() == "" || (location.Scheme != "https" && location.Scheme != "http") {
				return fmt.Errorf("third party %q requires a valid HTTP(S) URL for %s", entry.Name, scope.Runtime.Config.Platform)
			}
		}
	}
	return nil
}

func findThirdPartyConfig(workspace obj.WorkspaceConfig, name string) (obj.ThirdPartyConfig, error) {
	var selected *obj.ThirdPartyConfig
	for _, entry := range workspace.ThirdParty {
		if entry.Name == name {
			if selected != nil {
				return obj.ThirdPartyConfig{}, fmt.Errorf("duplicate third party %q", name)
			}
			selected = &entry
		}
	}
	if selected != nil {
		return *selected, nil
	}
	return obj.ThirdPartyConfig{}, fmt.Errorf("third party %q not found", name)
}

func (scope *Manager) cachePath(thirdParty obj.ThirdPartyConfig) (string, error) {
	digest := thirdParty.SHA256s[scope.Runtime.Config.Platform]
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != 32 {
		return "", fmt.Errorf("invalid SHA256 for third party %q on %s", thirdParty.Name, scope.Runtime.Config.Platform)
	}
	if !filepath.IsLocal(thirdParty.FilePath) || thirdParty.FilePath == "." {
		return "", fmt.Errorf("invalid third party file path")
	}
	return filepath.Join(scope.Runtime.Config.CacheDir, strings.ToLower(digest)), nil
}

func (scope *Manager) downloadThirdParty(workspace obj.WorkspaceConfig, name string) error {
	entry, err := findThirdPartyConfig(workspace, name)
	if err != nil {
		return err
	}
	if _, err := scope.cachePath(entry); err != nil {
		return err
	}
	if err := scope.validateCachedFile(entry); err == nil {
		return nil
	}
	return scope.downloadAndCacheFile(entry)
}

func (scope *Manager) registerCachedFile(entry obj.ThirdPartyConfig, dir string) {
	if scope.Runtime.Config.ThirdPartyFiles == nil {
		scope.Runtime.Config.ThirdPartyFiles = make(map[string]obj.ThirdPartyFileInfo)
	}
	scope.Runtime.Config.ThirdPartyFiles[entry.Name] = obj.ThirdPartyFileInfo{FileName: dir, FinalName: entry.FilePath}
}

func (scope *Manager) downloadAndCacheFile(entry obj.ThirdPartyConfig) error {
	if scope.Runtime.Config.CacheDir == "" {
		return fmt.Errorf("cache directory is not initialized")
	}
	dir, err := scope.cachePath(entry)
	if err != nil {
		return err
	}
	url := entry.URLs[scope.Runtime.Config.Platform]
	if url == "" {
		return fmt.Errorf("missing download URL for third party %q", entry.Name)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, "download-*")
	if err != nil {
		return err
	}
	path := temp.Name()
	if err := temp.Close(); err != nil {
		return err
	}
	defer os.Remove(path)
	if err := scope.downloadFile(url, path); err != nil {
		return err
	}
	if !tools.IsFileValid(path, entry.SHA256s[scope.Runtime.Config.Platform]) {
		return fmt.Errorf("checksum mismatch for third party %q", entry.Name)
	}
	if err := os.Rename(path, filepath.Join(dir, "file")); err != nil {
		return err
	}
	if err := tools.CreateMetadataFile(dir, entry.FilePath); err != nil {
		return err
	}
	scope.registerCachedFile(entry, dir)
	return nil
}

func (scope *Manager) validateCachedFile(entry obj.ThirdPartyConfig) error {
	limits, err := scope.Runtime.EffectiveLimits()
	if err != nil {
		return err
	}
	dir, err := scope.cachePath(entry)
	if err != nil {
		return err
	}
	info, err := os.Stat(filepath.Join(dir, "file"))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > limits.MaxDownloadBytes {
		return fmt.Errorf("cached artifact exceeds download limit or is not a regular file")
	}
	if !tools.IsFileValid(filepath.Join(dir, "file"), entry.SHA256s[scope.Runtime.Config.Platform]) {
		return fmt.Errorf("invalid cached file for %q", entry.Name)
	}
	// Metadata can be lost independently of the verified payload. Regenerate it
	// atomically, using this invocation's file alias.
	if err := tools.CreateMetadataFile(dir, entry.FilePath); err != nil {
		return err
	}
	scope.registerCachedFile(entry, dir)
	return nil
}

func (scope *Manager) downloadFile(url, filePath string) error {
	limits, err := scope.Runtime.EffectiveLimits()
	if err != nil {
		return err
	}
	client := scope.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	ctx := scope.Runtime.ExecutionContext()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("invalid download request")
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Transport and redirect errors can include full signed URLs, including
		// in nested causes. Never include their text in a user-visible error.
		return fmt.Errorf("download request failed (check connectivity and the configured endpoint)")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > limits.MaxDownloadBytes {
		return fmt.Errorf("download exceeds configured byte limit")
	}
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	_, copyErr := tools.CopyWithLimit(file, response.Body, limits.MaxDownloadBytes)
	closeErr := file.Close()
	if copyErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("download body could not be copied or exceeded the configured byte limit")
	}
	return closeErr
}
