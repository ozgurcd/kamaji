package utils

import (
	"kamaji/internal/fsutil"
	"os"
	"path/filepath"
)

type Installer struct {
	Directory string
	SourceDir string
}

func (scope *Installer) Remove() error {
	lease, err := fsutil.Lock(filepath.Join(scope.Directory, ".install.lock"), true)
	if err != nil {
		return err
	}
	defer lease.Close()
	for _, name := range []string{"rules", "requirements.txt"} {
		if err := os.RemoveAll(filepath.Join(scope.Directory, name)); err != nil {
			return err
		}
	}
	return nil
}
