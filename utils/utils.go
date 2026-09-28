package utils

import (
	"fmt"
	"io/fs"
	"kamaji/internal/fsutil"
	"kamaji/tools"
	"os"
	"path/filepath"
	"strings"
)

func EnsureWriteAccess(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(path, ".kamaji-write-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", path, err)
	}
	closeErr := file.Close()
	removeErr := os.Remove(file.Name())
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

// CopyDirectory creates an independent copy. Symlinks and special files are
// rejected so writes in an isolated run cannot follow a link back to its source.
func CopyDirectory(src, dst string) error {
	source, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	destination, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	initial, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !initial.IsDir() {
		return fmt.Errorf("source must be a real directory")
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(destination))
	if err != nil {
		return err
	}
	destination = filepath.Join(parent, filepath.Base(destination))
	rel, err := filepath.Rel(source, destination)
	if err != nil {
		return err
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("copy destination must be outside source")
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source must be a directory")
	}
	if err := os.Mkdir(destination, 0700); err != nil {
		return fmt.Errorf("copy requires a fresh destination: %w", err)
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("cannot copy non-regular file %s", path)
		}
		return copyFile(path, target)
	})
}

func (scope *Installer) Install() error {
	source := scope.SourceDir
	if source == "" {
		source = "."
	}
	// Prepare the complete replacement before moving any existing installation.
	if err := os.MkdirAll(scope.Directory, 0755); err != nil {
		return err
	}
	lease, err := fsutil.Lock(filepath.Join(scope.Directory, ".install.lock"), true)
	if err != nil {
		return err
	}
	defer lease.Close()
	stage, err := os.MkdirTemp(scope.Directory, ".install-*")
	if err != nil {
		return err
	}
	preserveBackup := false
	defer func() {
		if !preserveBackup {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := CopyDirectory(filepath.Join(source, "rules"), filepath.Join(stage, "rules")); err != nil {
		return fmt.Errorf("copy rules: %w", err)
	}
	names := []string{"rules", "requirements.txt"}
	if info, err := os.Lstat(filepath.Join(source, "requirements.txt")); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("requirements.txt must be a regular file")
		}
		if err := copyFile(filepath.Join(source, "requirements.txt"), filepath.Join(stage, "requirements.txt")); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	backup := filepath.Join(stage, "backup")
	if err := os.Mkdir(backup, 0700); err != nil {
		return err
	}
	moved := []string{}
	installed := []string{}
	rollback := func(cause error) error {
		for _, name := range installed {
			if err := os.RemoveAll(filepath.Join(scope.Directory, name)); err != nil {
				cause = fmt.Errorf("%w; rollback remove: %v", cause, err)
			}
		}
		for _, name := range moved {
			if err := os.Rename(filepath.Join(backup, name), filepath.Join(scope.Directory, name)); err != nil {
				preserveBackup = true
				cause = fmt.Errorf("%w; rollback restore failed (backup retained at %s): %v", cause, backup, err)
			}
		}
		return cause
	}
	for _, name := range names {
		dst := filepath.Join(scope.Directory, name)
		if _, err := os.Lstat(dst); err == nil {
			if err := os.Rename(dst, filepath.Join(backup, name)); err != nil {
				return rollback(err)
			}
			moved = append(moved, name)
		} else if !os.IsNotExist(err) {
			return rollback(err)
		}
		if _, err := os.Lstat(filepath.Join(stage, name)); os.IsNotExist(err) {
			continue
		}
		if err := os.Rename(filepath.Join(stage, name), dst); err != nil {
			return rollback(err)
		}
		installed = append(installed, name)
	}
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return tools.CopyFile(src, dst)
}
