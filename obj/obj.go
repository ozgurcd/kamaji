package obj

type RuntimeConfig struct {
	WorkspaceConfig      WorkspaceConfig
	ExecTarget           ExecTarget
	DebugMode            bool
	WorkspaceRoot        string
	WorkspaceDir         string
	CacheDir             string
	Platform             string
	TmpDir               string
	ExecRootDir          string
	ThirdPartyFiles      map[string]ThirdPartyFileInfo
	ThirdPartyFinalPaths map[string]string
	//RuleDir              string
	RuleFile string
}

type WorkspaceConfig struct {
	WorkspaceRoot  string             `yaml:"workspace_root"`
	RulesDir       string             `yaml:"rules_directory"`
	RulesCommonDir string             `yaml:"rules_common_directory"`
	WorkspaceVars  []WorkspaceVar     `yaml:"workspace_vars"`
	ThirdParty     []ThirdPartyConfig `yaml:"third_party"`
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
	Name   string         `yaml:"name"`
	Rule   string         `yaml:"rule"`
	Config map[string]any `yaml:"config"`
}

type BuildFile struct {
	Targets []ExecTarget `yaml:"targets"`
}

var WorkspaceFile string
