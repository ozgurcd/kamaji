package rt

import (
	"fmt"
	cfg "kamaji/config"
	"kamaji/obj"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

func (scope *Runtime) readWorkspaceConfig(rulesDirOverride string) (obj.WorkspaceConfig, error) {
	var config obj.WorkspaceConfig
	f, err := os.Open(filepath.Join(scope.Config.WorkspaceDir, scope.Config.WorkspaceFile))
	if err != nil {
		return config, err
	}
	defer f.Close()
	if err := cfg.DecodeYAML(f, &config); err != nil {
		return config, err
	}
	names := make(map[string]bool)
	for _, entry := range config.ThirdParty {
		if entry.Name == "" || names[entry.Name] {
			return config, fmt.Errorf("empty or duplicate third-party name %q", entry.Name)
		}
		names[entry.Name] = true
	}
	if _, err := resolveLimits(config.Limits); err != nil {
		return config, err
	}
	config.WorkspaceRoot = scope.Config.WorkspaceDir
	if rulesDirOverride != "" {
		config.RulesDir = rulesDirOverride
	}
	switch {
	case strings.HasPrefix(config.RulesDir, "//"):
		config.RulesDir = filepath.Join(scope.Config.WorkspaceDir, config.RulesDir[2:])
	case strings.TrimSpace(config.RulesDir) == "":
		config.RulesDir = "/usr/local/share/kamaji/rules"
		if scope.DefaultRulesDir != "" {
			config.RulesDir = scope.DefaultRulesDir
		}
	case !filepath.IsAbs(config.RulesDir):
		config.RulesDir = filepath.Join(scope.Config.WorkspaceDir, config.RulesDir)
	}
	if config.RulesCommonDir == "" {
		config.RulesCommonDir = "common"
	}
	return config, nil
}

// EffectiveLimits also validates direct package callers that do not load YAML.
func (scope *Runtime) EffectiveLimits() (obj.ResourceLimits, error) {
	return resolveLimits(scope.Config.WorkspaceConfig.Limits)
}

func resolveLimits(limits obj.ResourceLimits) (obj.ResourceLimits, error) {
	if limits.MaxDownloadBytes == 0 {
		limits.MaxDownloadBytes = 512 << 20
	}
	if limits.MaxExtractBytes == 0 {
		limits.MaxExtractBytes = 2 << 30
	}
	if limits.MaxArchiveEntries == 0 {
		limits.MaxArchiveEntries = 10000
	}
	const maxInt64 = int64(1<<63 - 1)
	if limits.MaxDownloadBytes < 0 || limits.MaxDownloadBytes == maxInt64 || limits.MaxExtractBytes < 0 || limits.MaxArchiveEntries < 0 {
		return limits, fmt.Errorf("resource limits must be positive integers below the signed 64-bit maximum")
	}
	// Reserve bounded tar header/PAX overhead without overflowing the decoder's
	// byte budget. File contents still share exactly MaxExtractBytes.
	if limits.MaxExtractBytes > maxInt64-(1<<20)-1 || limits.MaxArchiveEntries > (maxInt64-limits.MaxExtractBytes-(1<<20)-1)/1024 {
		return limits, fmt.Errorf("archive resource limits are too large")
	}
	return limits, nil
}

func (scope *Runtime) detectWorkspaceRoot() error {
	dir, err := scope.WorkingDirectory()
	if err != nil {
		return err
	}
	start := dir
	for {
		if scope.isWorkspaceRoot(dir) {
			scope.Config.WorkspaceDir = dir
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return fmt.Errorf("%s file not found starting from %s", scope.Config.WorkspaceFile, start)
}

func (scope *Runtime) isWorkspaceRoot(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, scope.Config.WorkspaceFile))
	return err == nil && info.Mode().IsRegular()
}

func (scope *Runtime) InitRuntime(rulesDirOverride string) error {
	if err := scope.LoadRuntime(rulesDirOverride); err != nil {
		return err
	}
	cache, err := scope.initCacheDir()
	if err != nil {
		return err
	}
	scope.Config.CacheDir = cache
	return nil
}

func (scope *Runtime) LoadRuntime(rulesDirOverride string) error {
	if scope.Config.WorkspaceFile == "" {
		scope.Config.WorkspaceFile = "kamaji.workspace.yaml"
	}
	if err := scope.detectWorkspaceRoot(); err != nil {
		return err
	}
	config, err := scope.readWorkspaceConfig(rulesDirOverride)
	if err != nil {
		return fmt.Errorf("read workspace: %w", err)
	}
	scope.Config.WorkspaceConfig = config
	scope.Config.Platform = runtime.GOOS + "_" + runtime.GOARCH
	scope.Config.ThirdPartyFiles = make(map[string]obj.ThirdPartyFileInfo)
	scope.Config.ThirdPartyFinalPaths = make(map[string]string)

	return nil
}

// DefaultTempDir computes the CLI's per-user cache location without creating it.
func DefaultTempDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("determine current user: %w", err)
	}
	var base string
	switch runtime.GOOS {
	case "darwin":
		base = "/var/tmp"
	case "linux":
		base = "/tmp"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return filepath.Join(base, "_kamaji_"+u.Username), nil
}

func (scope *Runtime) initCacheDir() (string, error) {
	if err := scope.EnsureTempDir(); err != nil {
		return "", err
	}
	cache := filepath.Join(scope.Config.TmpDir, "cache")
	for _, path := range []string{cache, filepath.Join(cache, "sha256")} {
		if err := EnsurePrivateDir(path); err != nil {
			return "", fmt.Errorf("create cache: %w", err)
		}
	}
	return cache, nil
}
