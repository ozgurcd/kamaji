package rt

import (
	"fmt"
	"io/fs"
	"kamaji/internal/fsutil"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func (scope *Runtime) RuntimeLease(exclusive bool) (*os.File, error) {
	if err := scope.EnsureTempDir(); err != nil {
		return nil, err
	}
	return fsutil.Lock(filepath.Join(scope.Config.TmpDir, ".runtime.lock"), exclusive)
}

func (scope *Runtime) Cleanup() error {
	lease, err := scope.RuntimeLease(true)
	if err != nil {
		return err
	}
	defer lease.Close()
	for _, name := range []string{"cache", "execroot"} {
		if err := os.RemoveAll(filepath.Join(scope.Config.TmpDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// PruneExecutionRoots only considers completed runs, never active/incomplete
// roots. The caller holds the exclusive retention lease while marking/pruning.
func (scope *Runtime) PruneExecutionRoots() error {
	maxCount, maxBytes := scope.Config.MaxRetainedExecRoots, scope.Config.MaxRetainedBytes
	if maxCount == 0 {
		maxCount = 10
	}
	if maxBytes == 0 {
		maxBytes = 4 << 30
	}
	if maxCount < 1 || maxBytes < 1 || maxBytes == 1<<63-1 {
		return fmt.Errorf("retention limits must be positive")
	}
	parent := filepath.Join(scope.Config.TmpDir, "execroot")
	entries, err := os.ReadDir(parent)
	if err != nil {
		return err
	}
	type retained struct {
		path  string
		size  int64
		stamp time.Time
	}
	var runs []retained
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(parent, entry.Name())
		info, err := os.Lstat(filepath.Join(path, ".complete"))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		item := retained{path: path, stamp: info.ModTime()}
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.Type().IsRegular() {
				return nil
			}
			stat, e := d.Info()
			if e != nil {
				return e
			}
			if stat.Size() > maxBytes-item.size {
				item.size = maxBytes + 1
				return fs.SkipAll
			}
			item.size += stat.Size()
			return nil
		})
		if err != nil {
			return err
		}
		runs = append(runs, item)
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].stamp.Equal(runs[j].stamp) {
			return runs[i].path > runs[j].path
		}
		return runs[i].stamp.After(runs[j].stamp)
	})
	count := 0
	var total int64
	for _, run := range runs {
		if count >= maxCount || run.size > maxBytes-total {
			if err := os.RemoveAll(run.path); err != nil {
				return err
			}
			continue
		}
		count++
		total += run.size
	}
	return nil
}

func (scope *Runtime) FinishExecution(started bool) error {
	if !started || !scope.Config.KeepExecRoot {
		return os.RemoveAll(scope.Config.ExecRootDir)
	}
	lease, err := fsutil.Lock(filepath.Join(scope.Config.TmpDir, ".retention.lock"), true)
	if err != nil { // Never leak unbounded data if concurrent retention is busy.
		_ = os.RemoveAll(scope.Config.ExecRootDir)
		return err
	}
	defer lease.Close()
	if err := os.WriteFile(filepath.Join(scope.Config.ExecRootDir, ".complete"), nil, 0600); err != nil {
		return err
	}
	if err := scope.PruneExecutionRoots(); err != nil {
		return err
	}
	if _, err := os.Stat(scope.Config.ExecRootDir); err != nil {
		return fmt.Errorf("execution finished but its directory exceeded the retention budget")
	}
	return nil
}
