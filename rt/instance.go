package rt

import (
	"context"
	"kamaji/obj"
	"os"
	"path/filepath"
	"time"
)

type Runtime struct {
	Context          context.Context
	DefaultRulesDir  string
	Timeout          time.Duration
	KillGrace        time.Duration
	Config           obj.RuntimeConfig
	PythonCommand    func(string, ...string) error
	RequirementsFile string
}

func (scope *Runtime) WorkingDirectory() (string, error) {
	if scope.Config.WorkingDir != "" {
		return filepath.Abs(scope.Config.WorkingDir)
	}
	return os.Getwd()
}

func (scope *Runtime) pythonCommand(name string, args ...string) error {
	if scope.PythonCommand != nil {
		return scope.PythonCommand(name, args...)
	}
	return runPython(scope.ExecutionContext(), scope.KillGrace, name, args...)
}
