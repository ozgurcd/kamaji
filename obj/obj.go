package obj

type RuntimeConfig struct {
	DebugMode            bool
	WorkspaceRoot        string
	WorkspaceDir         string
	WorkspaceConfig      WorkspaceConfig
	CacheDir             string
	Platform             string
	TmpDir               string
	SandboxDir           string
	ThirdPartyFiles      map[string]ThirdPartyFileInfo
	ThirdPartyFinalPaths map[string]string
}

type WorkspaceConfig struct {
	WorkspaceRoot string             `yaml:"workspace_root"`
	RulesDir      string             `yaml:"rules_directory"`
	WorkspaceVars []WorkspaceVar     `yaml:"workspace_vars"`
	ThirdParty    []ThirdPartyConfig `yaml:"third_party"`
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

type Target struct {
	Name   string         `yaml:"name"`
	Rule   string         `yaml:"rule"`
	Config map[string]any `yaml:"config"`
}

type BuildFile struct {
	Targets []Target `yaml:"targets"`
}
