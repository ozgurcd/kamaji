package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var Version = "dev"

func NewCommand(options Options) *cobra.Command {
	app := newApplication(options)
	root := &cobra.Command{Use: "kamaji <command> [flags]", Short: "Build dependency graphs and run language-independent rules.", Version: Version, SilenceUsage: true, SilenceErrors: true}
	root.Args = app.runArgs
	root.RunE = app.run
	app.targetFlags(root)
	app.runFlags(root)
	root.Flags().BoolVarP(&app.cleanup, "cleanup", "c", false, "Remove cached downloads and execution directories (compatibility alias for cache clean --runs)")
	run := &cobra.Command{Use: "run <target> [-- rule arguments]", Short: "Run a target, including a name matching a built-in command", Args: cobra.MinimumNArgs(1), RunE: app.run}
	app.targetFlags(run)
	app.runFlags(run)
	targets := &cobra.Command{Use: "targets", Short: "List build targets and descriptions", Args: cobra.NoArgs, RunE: app.listTargets}
	app.buildFlags(targets)
	validate := &cobra.Command{Use: "validate [target]", Short: "Check execution prerequisites without downloads, execution, or runtime writes", Args: app.validationArgs, RunE: app.validate}
	app.targetFlags(validate)
	validate.Flags().BoolVar(&app.all, "all", false, "Validate every target and report all target failures")
	doctor := &cobra.Command{Use: "doctor [target]", Short: "Diagnose execution readiness; checks every target when omitted", Args: cobra.MaximumNArgs(1), RunE: app.doctor}
	app.targetFlags(doctor)
	explain := &cobra.Command{Use: "explain <target>", Short: "Show resolved paths, configuration sources and redacted options", Args: cobra.ExactArgs(1), RunE: app.explain}
	app.targetFlags(explain)
	explain.Flags().BoolVar(&app.jsonOutput, "json", false, "Emit machine-readable JSON")
	explain.Flags().BoolVarP(&app.isolated, "isolated", "i", false, "Explain execution with an independent working-directory copy")
	for _, command := range []*cobra.Command{root, run, validate, doctor, explain} {
		command.ValidArgsFunction = app.completeTargets
	}
	setup := &cobra.Command{Use: "setup-python-env", Short: "Create and select a completed Python environment", Args: cobra.NoArgs, RunE: app.setup}
	app.pythonFlags(setup)
	app.installFlags(setup)
	setup.Flags().StringVar(&app.requirements, "requirements", "", "Requirements file to install instead of the installed requirements")
	create := &cobra.Command{Use: "rules-directory-create", Short: "Install rules from the current directory", Args: cobra.NoArgs, RunE: app.install}
	remove := &cobra.Command{Use: "rules-directory-delete", Short: "Remove installed rules and requirements", Args: cobra.NoArgs, RunE: app.uninstall}
	app.installFlags(create)
	app.installFlags(remove)
	initCommand := &cobra.Command{Use: "init [directory]", Short: "Create a minimal workspace without overwriting files", Args: cobra.MaximumNArgs(1), RunE: app.initWorkspace}
	initCommand.Flags().StringVar(&app.template, "template", "build", "Workspace template (build: TOML; minimal: legacy Python/YAML)")
	root.AddCommand(run, targets, validate, doctor, explain, setup, create, remove, initCommand, app.cacheCommand(), app.runsCommand(),
		&cobra.Command{Use: "version", Short: "Print the Kamaji version", Args: cobra.NoArgs, Run: func(command *cobra.Command, _ []string) { command.Println(Version) }})
	root.AddCommand(app.graphCommands()...)
	return root
}

// Execute constructs a fresh command so flags do not leak between invocations.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return NewCommand(Options{}).ExecuteContext(ctx)
}
