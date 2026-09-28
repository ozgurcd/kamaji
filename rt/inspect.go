package rt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// TempPath resolves the runtime location without creating it.
func (scope *Runtime) TempPath() (string, error) {
	path := scope.Config.TmpDir
	if path == "" {
		var err error
		path, err = DefaultTempDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Abs(path)
}

func (scope *Runtime) ExecutionContext() context.Context {
	if scope.Context != nil {
		return scope.Context
	}
	return context.Background()
}

// CheckPrivateDir checks an existing directory without creating or changing it.
func CheckPrivateDir(path string) error {
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
