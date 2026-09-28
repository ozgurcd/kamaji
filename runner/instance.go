package runner

import (
	"kamaji/rt"
	"os/exec"
)

type Executor struct {
	LookPath   func(string) (string, error)
	Runtime    *rt.Runtime
	RunCommand func(*exec.Cmd) error
}
