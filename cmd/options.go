package cmd

import (
	"kamaji/obj"
	"kamaji/rt"
)

// Options binds dependencies to one command instance; zero values select the
// normal local implementation. Tests and embedders can supply their own I/O.
type Options struct {
	LookPath          func(string) (string, error)
	UserHomeDir       func() (string, error)
	Runtime           *rt.Runtime
	LoadUserConfig    func() (map[string]string, error)
	RunTarget         func(obj.WorkspaceConfig, obj.ExecTarget, ...string) error
	SetupPython       func() error
	EnsureWriteAccess func(string) error
	CopyRules         func() error
	DeleteRules       func() error
	RemoveAll         func(string) error
	InstallRoot       string
}
