package rt

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// EnsurePrivateDir accepts only a real directory owned by the current user,
// inaccessible to other users. Call it for each Kamaji-controlled component,
// starting at TmpDir, before creating descendants. TmpDir's ancestors must be
// trusted (the default uses the system's sticky temporary directory).
func EnsurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("create private directory: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("unsafe runtime directory %q: require a real directory owned by the current user with mode 0700", path)
	}
	return nil
}

func (scope *Runtime) EnsureTempDir() error {
	if scope.Config.TmpDir == "" {
		dir, err := DefaultTempDir()
		if err != nil {
			return err
		}
		scope.Config.TmpDir = dir
	}
	// Resolve relative paths before an executor changes its working directory.
	path, err := filepath.Abs(scope.Config.TmpDir)
	if err != nil {
		return err
	}
	if err := EnsurePrivateDir(path); err != nil {
		return err
	}
	scope.Config.TmpDir = path
	return nil
}
