package runner

import (
	"kamaji/obj"
	"kamaji/rt"
	"os/exec"
)

var testRuntime rt.Runtime
var runCommand = func(c *exec.Cmd) error { return c.Run() }

func prepareArgs(selected obj.ExecTarget) ([]string, error) {
	return (&Executor{Runtime: &testRuntime, RunCommand: runCommand}).prepareArgs(selected)
}
func Run(workspace obj.WorkspaceConfig, selected obj.ExecTarget, args ...string) error {
	return (&Executor{Runtime: &testRuntime, RunCommand: runCommand}).Run(workspace, selected, args...)
}
func ValidateRule(selected obj.ExecTarget) error {
	return (&Executor{Runtime: &testRuntime, RunCommand: runCommand}).ValidateRule(selected)
}
