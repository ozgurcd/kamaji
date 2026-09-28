package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cfg "kamaji/config"
	"kamaji/obj"
	"kamaji/rt"
	"kamaji/runner"
	"kamaji/target"
	"kamaji/utils"

	"github.com/spf13/cobra"
)

type application struct {
	options                                                                        Options
	runtime                                                                        *rt.Runtime
	executor                                                                       *runner.Executor
	manager                                                                        *target.Manager
	buildFile, python, rulesDir, pythonSource, installRoot, requirements, template string
	cleanup, debug, isolated, keep, all, jsonOutput, user                          bool
	retained                                                                       int
	retainedBytes                                                                  int64
	timeout, grace                                                                 time.Duration
}

func newApplication(options Options) *application {
	runtime := options.Runtime
	if runtime == nil {
		runtime = &rt.Runtime{}
	}
	return &application{options: options, runtime: runtime, executor: &runner.Executor{Runtime: runtime, LookPath: options.LookPath}, manager: &target.Manager{Runtime: runtime}}
}

func (a *application) buildFlags(c *cobra.Command) {
	c.Flags().StringVarP(&a.buildFile, "build", "b", "BUILD.yaml", "Build file path")
}
func (a *application) pythonFlags(c *cobra.Command) {
	c.Flags().StringVarP(&a.python, "python", "p", "", "Python interpreter path")
}
func (a *application) installFlags(c *cobra.Command) {
	c.Flags().BoolVar(&a.user, "user", false, "Use ~/.local/share/kamaji for rules and installed requirements")
	c.Flags().StringVar(&a.installRoot, "install-root", "", "Rules installation root (default /usr/local/share/kamaji)")
	c.MarkFlagsMutuallyExclusive("user", "install-root")
}
func (a *application) targetFlags(c *cobra.Command) {
	a.buildFlags(c)
	a.pythonFlags(c)
	a.installFlags(c)
	c.Flags().StringVar(&a.rulesDir, "rules-directory", "", "Override the workspace rules directory")
}
func (a *application) runFlags(c *cobra.Command) {
	c.Flags().BoolVarP(&a.debug, "debug", "d", false, "Enable safe core diagnostics")
	c.Flags().BoolVarP(&a.isolated, "isolated", "i", false, "Run in an independent working-directory copy")
	c.Flags().BoolVar(&a.keep, "keep-execroot", false, "Retain completed execution files within retention limits")
	c.Flags().IntVar(&a.retained, "max-retained-execroots", 10, "Maximum retained execution directories")
	c.Flags().Int64Var(&a.retainedBytes, "max-retained-bytes", 4<<30, "Maximum bytes in retained execution directories")
	c.Flags().DurationVar(&a.timeout, "timeout", 0, "Execution deadline including dependency preparation (0 disables)")
	c.Flags().DurationVar(&a.grace, "kill-after", 2*time.Second, "Grace period after cancellation before killing the process group")
}
func (a *application) buildPath() (string, error) {
	if filepath.IsAbs(a.buildFile) {
		return a.buildFile, nil
	}
	cwd, err := a.runtime.WorkingDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, a.buildFile), nil
}
func (a *application) installationRoot() (string, error) {
	root := a.installRoot
	if a.user {
		home := a.options.UserHomeDir
		if home == nil {
			home = os.UserHomeDir
		}
		dir, err := home()
		if err != nil {
			return "", err
		}
		root = filepath.Join(dir, ".local", "share", "kamaji")
	}
	if root == "" {
		root = a.options.InstallRoot
	}
	if root == "" {
		root = "/usr/local/share/kamaji"
	}
	if !filepath.IsAbs(root) {
		cwd, err := a.runtime.WorkingDirectory()
		if err != nil {
			return "", err
		}
		root = filepath.Join(cwd, root)
	}
	return root, nil
}
func (a *application) configurePython(managed bool) error {
	load := a.options.LoadUserConfig
	if load == nil {
		load = cfg.LoadUserConfig
	}
	values, err := load()
	if err != nil {
		return err
	}
	root, err := a.installationRoot()
	if err != nil {
		return err
	}
	a.runtime.DefaultRulesDir = filepath.Join(root, "rules")
	if a.requirements != "" {
		path := a.requirements
		if !filepath.IsAbs(path) {
			cwd, err := a.runtime.WorkingDirectory()
			if err != nil {
				return err
			}
			path = filepath.Join(cwd, path)
		}
		a.runtime.RequirementsFile = path
	} else if a.runtime.RequirementsFile == "" {
		a.runtime.RequirementsFile = filepath.Join(root, "requirements.txt")
	}
	path, err := a.runtime.TempPath()
	if err != nil {
		return err
	}
	a.runtime.Config.TmpDir = path
	python := a.python
	a.pythonSource = "flag"
	if python == "" {
		python = os.Getenv("KAMAJI_PYTHON")
		a.pythonSource = "environment"
	}
	if python == "" {
		python = values["python"]
		a.pythonSource = "user configuration"
	}
	if python == "" && managed {
		// Absence is normal for inspection; unsafe existing roots are never followed.
		if err := rt.CheckPrivateDir(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		selected, err := a.runtime.CurrentPython()
		if err != nil {
			return err
		}
		if selected != "" {
			python = selected
			a.pythonSource = "managed environment"
		} else {
			candidate := filepath.Join(path, "venv", "bin", "python")
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
				python = candidate
				a.pythonSource = "legacy environment"
			}
		}
	}
	if python == "" {
		python = "python3"
		a.pythonSource = "default"
	}
	a.runtime.Config.PythonInterpreter = python
	return nil
}
func (a *application) loadBuild() (obj.BuildFile, error) {
	if err := a.configurePython(true); err != nil {
		return obj.BuildFile{}, err
	}
	if err := a.runtime.LoadRuntime(a.rulesDir); err != nil {
		return obj.BuildFile{}, err
	}
	path, err := a.buildPath()
	if err != nil {
		return obj.BuildFile{}, err
	}
	return target.LoadBuildFile(path)
}
func (a *application) check(selected obj.ExecTarget) error {
	a.runtime.Config.ExecTarget = selected
	if err := a.manager.ValidateTargetVariables(selected.Config); err != nil {
		return err
	}
	if err := a.executor.Preflight(a.runtime.Config.WorkspaceConfig, selected); err != nil {
		return err
	}
	return a.manager.ValidateDependencies(a.runtime.Config.WorkspaceConfig, selected)
}
func selectTarget(build obj.BuildFile, name string) (obj.ExecTarget, error) {
	names := []string{}
	for _, entry := range build.Targets {
		if entry.Name == name {
			return entry, nil
		}
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return obj.ExecTarget{}, fmt.Errorf("target %q not found; available targets: %s", name, strings.Join(names, ", "))
}
func (a *application) runArgs(_ *cobra.Command, args []string) error {
	if a.cleanup {
		if len(args) != 0 {
			return fmt.Errorf("--cleanup does not accept a target")
		}
		return nil
	}
	if len(args) == 0 {
		return fmt.Errorf("provide a build target or a command")
	}
	return nil
}
func (a *application) run(c *cobra.Command, args []string) error {
	if a.cleanup {
		for _, name := range []string{"python", "rules-directory", "build", "isolated", "keep-execroot", "timeout", "kill-after", "user", "install-root", "debug", "max-retained-execroots", "max-retained-bytes"} {
			if c.Flags().Changed(name) {
				return fmt.Errorf("--cleanup cannot be combined with --%s", name)
			}
		}
		return a.clean(false, true, c)
	}
	if a.retained < 1 || a.retainedBytes < 1 || a.retainedBytes == 1<<63-1 {
		return fmt.Errorf("retention limits must be positive and below the signed 64-bit maximum")
	}
	if a.timeout < 0 || a.grace <= 0 {
		return fmt.Errorf("timeout must be nonnegative and kill-after must be positive")
	}
	build, err := a.loadBuild()
	if err != nil {
		return err
	}
	selected, err := selectTarget(build, args[0])
	if err != nil {
		return err
	}
	if err := a.check(selected); err != nil {
		return err
	}
	state := &a.runtime.Config
	state.DebugMode = a.debug
	state.Isolated = a.isolated
	state.KeepExecRoot = a.keep
	state.MaxRetainedExecRoots = a.retained
	state.MaxRetainedBytes = a.retainedBytes
	a.runtime.Context = c.Context()
	a.runtime.Timeout = a.timeout
	a.runtime.KillGrace = a.grace
	lease, err := a.runtime.RuntimeLease(false)
	if err != nil {
		return err
	}
	defer lease.Close()
	if err := a.runtime.InitRuntime(a.rulesDir); err != nil {
		return err
	}
	run := a.options.RunTarget
	if run == nil {
		run = a.executor.Run
	}
	result := run(state.WorkspaceConfig, selected, args[1:]...)
	if a.keep && state.ExecRootDir != "" {
		if _, err := os.Stat(filepath.Join(state.ExecRootDir, ".complete")); err == nil {
			c.PrintErrf("Retained execution files: %s\n", state.ExecRootDir)
		}
	}
	return result
}
func (a *application) validationArgs(_ *cobra.Command, args []string) error {
	if a.all && len(args) == 0 || !a.all && len(args) == 1 {
		return nil
	}
	return fmt.Errorf("provide one target or --all")
}
func (a *application) validate(c *cobra.Command, args []string) error {
	build, err := a.loadBuild()
	if err != nil {
		return err
	}
	selected := build.Targets
	if !a.all {
		entry, err := selectTarget(build, args[0])
		if err != nil {
			return err
		}
		selected = []obj.ExecTarget{entry}
	}
	if len(selected) == 0 {
		return fmt.Errorf("build file has no targets")
	}
	var failures []error
	for _, entry := range selected {
		if err := a.check(entry); err != nil {
			c.Printf("%s: invalid: %v\n", entry.Name, err)
			failures = append(failures, fmt.Errorf("target %q: %w", entry.Name, err))
		} else {
			c.Printf("%s: valid\n", entry.Name)
		}
	}
	return errors.Join(failures...)
}
func (a *application) doctor(c *cobra.Command, args []string) error {
	a.all = len(args) == 0
	return a.validate(c, args)
}
func (a *application) listTargets(c *cobra.Command, _ []string) error {
	path, err := a.buildPath()
	if err != nil {
		return err
	}
	build, err := target.LoadBuildFile(path)
	if err != nil {
		return err
	}
	sort.Slice(build.Targets, func(i, j int) bool { return build.Targets[i].Name < build.Targets[j].Name })
	for _, entry := range build.Targets {
		if entry.Description == "" {
			c.Println(entry.Name)
		} else {
			c.Printf("%s\t%s\n", entry.Name, entry.Description)
		}
	}
	return nil
}
func (a *application) completeTargets(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	path, err := a.buildPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	build, err := target.LoadBuildFile(path)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	result := []string{}
	for _, entry := range build.Targets {
		if strings.HasPrefix(entry.Name, prefix) {
			name := entry.Name
			if entry.Description != "" {
				name += "\t" + strings.ReplaceAll(strings.ReplaceAll(entry.Description, "\n", " "), "\t", " ")
			}
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result, cobra.ShellCompDirectiveNoFileComp
}
func (a *application) setup(c *cobra.Command, _ []string) error {
	if err := a.configurePython(false); err != nil {
		return err
	}
	if a.requirements != "" {
		info, err := os.Stat(a.runtime.RequirementsFile)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("requirements must be a regular file")
		}
	}
	a.runtime.Context = c.Context()
	if err := a.runtime.EnsureTempDir(); err != nil {
		return err
	}
	setup := a.options.SetupPython
	if setup == nil {
		setup = a.runtime.SetupPythonEnv
	}
	return setup()
}
func (a *application) mutateInstallation(remove bool) error {
	root, err := a.installationRoot()
	if err != nil {
		return err
	}
	ensure := a.options.EnsureWriteAccess
	if ensure == nil {
		ensure = utils.EnsureWriteAccess
	}
	if err := ensure(root); err != nil {
		return err
	}
	cwd, err := a.runtime.WorkingDirectory()
	if err != nil {
		return err
	}
	installer := utils.Installer{Directory: root, SourceDir: cwd}
	if remove {
		if a.options.DeleteRules != nil {
			return a.options.DeleteRules()
		}
		return installer.Remove()
	}
	if a.options.CopyRules != nil {
		return a.options.CopyRules()
	}
	return installer.Install()
}
func (a *application) install(c *cobra.Command, _ []string) error {
	if err := a.mutateInstallation(false); err != nil {
		return err
	}
	root, _ := a.installationRoot()
	c.Printf("Rules installed in %s.\n", filepath.Join(root, "rules"))
	return nil
}
func (a *application) uninstall(c *cobra.Command, _ []string) error {
	if err := a.mutateInstallation(true); err != nil {
		return err
	}
	c.Println("Installed rules and requirements removed.")
	return nil
}
