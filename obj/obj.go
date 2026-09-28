package obj

type RuntimeConfig struct {
	WorkingDir           string
	WorkspaceFile        string
	KeepExecRoot         bool
	MaxRetainedExecRoots int
	MaxRetainedBytes     int64
	WorkspaceConfig      WorkspaceConfig
	ExecTarget           ExecTarget
	DebugMode            bool
	WorkspaceDir         string
	CacheDir             string
	Platform             string
	TmpDir               string
	Isolated             bool
	ExecRootDir          string
	ThirdPartyFiles      map[string]ThirdPartyFileInfo
	ThirdPartyFinalPaths map[string]string
	PythonInterpreter    string
}

type WorkspaceConfig struct {
	Limits         ResourceLimits     `yaml:"limits"`
	WorkspaceRoot  string             `yaml:"workspace_root"`
	RulesDir       string             `yaml:"rules_directory"`
	RulesCommonDir string             `yaml:"rules_common_directory"`
	WorkspaceVars  []WorkspaceVar     `yaml:"workspace_vars"`
	ThirdParty     []ThirdPartyConfig `yaml:"third_party"`
}

// ResourceLimits bounds one download or archive. Zero selects the default;
// negative values are invalid, so limits cannot accidentally be disabled.
type ResourceLimits struct {
	MaxDownloadBytes  int64 `yaml:"max_download_bytes"`
	MaxExtractBytes   int64 `yaml:"max_extract_bytes"`
	MaxArchiveEntries int64 `yaml:"max_archive_entries"`
}

type WorkspaceVar struct {
	Org_Domain string `yaml:"org_domain"`
	Base_Dir   string `yaml:"base_dir"`
}

type ThirdPartyFileInfo struct {
	FileName  string
	FinalName string
}

type ThirdPartyConfig struct {
	Name     string            `yaml:"name"`
	FilePath string            `yaml:"file_path"`
	URLs     map[string]string `yaml:"url"`
	SHA256s  map[string]string `yaml:"sha256"`
}

type ExecTarget struct {
	Description string         `yaml:"description"`
	Name        string         `yaml:"name"`
	Rule        string         `yaml:"rule"`
	Config      map[string]any `yaml:"config"`
}

type BuildFile struct {
	Targets []ExecTarget `yaml:"targets"`
}
