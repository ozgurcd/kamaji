package cmd

import (
	"kamaji/config"
	"kamaji/obj"
	"kamaji/rt"
	"kamaji/runner"
	"kamaji/utils"
	"os"

	"github.com/spf13/cobra"
)

var testRuntime rt.Runtime
var loadUserConfig = config.LoadUserConfig
var runTarget = func(w obj.WorkspaceConfig, t obj.ExecTarget, args ...string) error {
	return (&runner.Executor{Runtime: &testRuntime}).Run(w, t, args...)
}
var setupPython = func() error { return testRuntime.SetupPythonEnv() }
var ensureWriteAccess = utils.EnsureWriteAccess
var copyRules = func() error { return (&utils.Installer{Directory: "/usr/local/share/kamaji"}).Install() }
var removeAll = os.RemoveAll

func newRootCommand() *cobra.Command {
	return NewCommand(Options{LookPath: func(name string) (string, error) { return name, nil }, Runtime: &testRuntime, LoadUserConfig: loadUserConfig, RunTarget: runTarget, SetupPython: setupPython, EnsureWriteAccess: ensureWriteAccess, CopyRules: copyRules, RemoveAll: removeAll, DeleteRules: func() error { return removeAll("/usr/local/share/kamaji/rules") }})
}
