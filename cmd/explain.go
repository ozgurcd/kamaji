package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"

	"github.com/spf13/cobra"
)

type explanation struct {
	ExecutionMode    string                       `json:"execution_mode"`
	Language         string                       `json:"language"`
	Executable       string                       `json:"executable"`
	RuleDescription  string                       `json:"rule_description,omitempty"`
	Target           string                       `json:"target"`
	Description      string                       `json:"description,omitempty"`
	WorkingDirectory string                       `json:"working_directory"`
	Workspace        string                       `json:"workspace"`
	BuildFile        string                       `json:"build_file"`
	Rule             string                       `json:"rule"`
	Python           string                       `json:"python,omitempty"`
	PythonSource     string                       `json:"python_source,omitempty"`
	Isolation        string                       `json:"isolation"`
	RuntimeDirectory string                       `json:"runtime_directory"`
	Limits           obj.ResourceLimits           `json:"limits"`
	Options          map[string]optionExplanation `json:"options"`
	Dependencies     []dependencyExplanation      `json:"dependencies"`
}
type optionExplanation struct {
	Value       string `json:"value"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
}
type dependencyExplanation struct {
	Name  string `json:"name"`
	Cache string `json:"cache"`
}

func (a *application) explain(c *cobra.Command, args []string) error {
	if handled, err := a.inspectGraph(c, args); handled {
		return err
	}
	build, err := a.loadBuild()
	if err != nil {
		return err
	}
	selected, err := selectTarget(build, args[0])
	if err != nil {
		return err
	}
	given := map[string]bool{}
	for key := range selected.Config {
		given[key] = true
	}
	if err := a.check(selected); err != nil {
		return err
	}
	schema, err := a.manager.Schema()
	if err != nil {
		return err
	}
	rule, err := (&tools.Context{Runtime: a.runtime}).GetRule(selected)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(rule) {
		rule = filepath.Join(a.runtime.Config.WorkspaceConfig.RulesDir, rule)
	}
	cwd, err := a.runtime.WorkingDirectory()
	if err != nil {
		return err
	}
	buildPath, err := a.buildPath()
	if err != nil {
		return err
	}
	program, err := a.executor.Program(selected)
	if err != nil {
		return err
	}
	limits, err := a.runtime.EffectiveLimits()
	if err != nil {
		return err
	}
	report := explanation{ExecutionMode: program.Mode, Language: program.Language, Executable: program.Executable, Target: selected.Name, Description: selected.Description, WorkingDirectory: cwd, Workspace: a.runtime.Config.WorkspaceDir, BuildFile: buildPath, Rule: rule, Isolation: "off (run default; --isolated enables a working copy)", RuntimeDirectory: a.runtime.Config.TmpDir, Limits: limits, Options: map[string]optionExplanation{}, Dependencies: []dependencyExplanation{}}
	if program.ManagedPython {
		report.Python, report.PythonSource = program.Executable, a.pythonSource
	}
	report.RuleDescription = schema.Description
	if a.isolated {
		report.Isolation = "on (independent working copy; not a sandbox)"
	}
	names := []string{}
	for key := range selected.Config {
		source := "schema default"
		if given[key] {
			source = "build configuration"
		}
		description := ""
		switch d := schema.Variables[key].(type) {
		case map[string]any:
			description, _ = d["description"].(string)
		case map[any]any:
			description, _ = d["description"].(string)
		}
		report.Options[key] = optionExplanation{Value: "<redacted>", Source: source, Description: description}
		names = append(names, key)
	}
	seen := map[string]bool{}
	for _, value := range selected.Config {
		ref, ok := value.(string)
		if !ok || !strings.HasPrefix(ref, "@@") || seen[ref] {
			continue
		}
		seen[ref] = true
		name := strings.TrimPrefix(ref, "@@")
		status := "missing"
		for _, dep := range a.runtime.Config.WorkspaceConfig.ThirdParty {
			if dep.Name != name {
				continue
			}
			digest := dep.SHA256s[a.runtime.Config.Platform]
			path := filepath.Join(a.runtime.Config.TmpDir, "cache", strings.ToLower(digest), "file")
			parentsSafe := true
			for _, parent := range []string{a.runtime.Config.TmpDir, filepath.Join(a.runtime.Config.TmpDir, "cache"), filepath.Dir(path)} {
				if err := rt.CheckPrivateDir(parent); err != nil {
					parentsSafe = false
					break
				}
			}
			if info, err := os.Lstat(path); err == nil && parentsSafe {
				status = "invalid"
				if info.Mode().IsRegular() && info.Size() <= limits.MaxDownloadBytes && tools.IsFileValid(path, digest) {
					status = "verified"
				}
			}
		}
		report.Dependencies = append(report.Dependencies, dependencyExplanation{Name: name, Cache: status})
	}
	sort.Slice(report.Dependencies, func(i, j int) bool { return report.Dependencies[i].Name < report.Dependencies[j].Name })
	if a.jsonOutput {
		encoder := json.NewEncoder(c.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	c.Printf("Target: %s\nDescription: %s\nWorkspace: %s\nWorking directory: %s\nBuild file: %s\nRule: %s\nLanguage: %s\nExecution mode: %s\nExecutable: %s\nIsolation: %s\nRuntime directory: %s\nLimits: download=%d bytes, extraction=%d bytes, archive entries=%d\n", report.Target, report.Description, report.Workspace, cwd, buildPath, rule, program.Language, program.Mode, program.Executable, report.Isolation, report.RuntimeDirectory, limits.MaxDownloadBytes, limits.MaxExtractBytes, limits.MaxArchiveEntries)
	if program.ManagedPython {
		c.Printf("Python: %s (%s)\n", report.Python, report.PythonSource)
	}
	c.Printf("Rule description: %s\n", report.RuleDescription)
	sort.Strings(names)
	for _, name := range names {
		option := report.Options[name]
		c.Printf("Option %s: %s (%s) %s\n", name, option.Value, option.Source, option.Description)
	}
	for _, dep := range report.Dependencies {
		c.Printf("Dependency %s: cache %s\n", dep.Name, dep.Cache)
	}
	return nil
}
