package rt

import (
	"context"
	"kamaji/obj"
	"os"
	"time"
)

var testRuntime Runtime
var runPythonCommand = func(name string, args ...string) error {
	return runPython(context.Background(), 2*time.Second, name, args...)
}
var pythonRequirementsFile = "/usr/local/share/kamaji/requirements.txt"

func testScope() *Runtime {
	testRuntime.PythonCommand = runPythonCommand
	testRuntime.RequirementsFile = pythonRequirementsFile
	return &testRuntime
}
func readWorkspaceConfig(rulesDirOverride string) (obj.WorkspaceConfig, error) {
	return testScope().readWorkspaceConfig(rulesDirOverride)
}
func EffectiveLimits() (obj.ResourceLimits, error) { return testScope().EffectiveLimits() }

func InitRuntime(rulesDirOverride string) error     { return testScope().InitRuntime(rulesDirOverride) }
func LoadRuntime(rulesDirOverride string) error     { return testScope().LoadRuntime(rulesDirOverride) }
func initCacheDir() (string, error)                 { return testScope().initCacheDir() }
func EnsureTempDir() error                          { return testScope().EnsureTempDir() }
func SetupPythonEnv() error                         { return testScope().SetupPythonEnv() }
func CurrentPython() (string, error)                { return testScope().CurrentPython() }
func RuntimeLease(exclusive bool) (*os.File, error) { return testScope().RuntimeLease(exclusive) }
func Cleanup() error                                { return testScope().Cleanup() }
func PruneExecutionRoots() error                    { return testScope().PruneExecutionRoots() }
func FinishExecution(started bool) error            { return testScope().FinishExecution(started) }
func isWorkspaceRoot(dir string) bool               { return testScope().isWorkspaceRoot(dir) }
