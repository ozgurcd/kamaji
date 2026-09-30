package buildsys

import (
	"os"
	"path/filepath"
	"sort"

	"kamaji/internal/fsutil"
)

type CleanOptions struct {
	DryRun  bool
	Cache   bool
	History bool
}

// Clean removes only declared outputs and explicitly selected build storage.
// Preview is read-only. Real cleanup shares the build lease with execution.
func (p *Project) Clean(names []string, options CleanOptions) ([]string, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	order, err := p.Select(names)
	if err != nil {
		return nil, err
	}
	index := p.Index()
	paths := []string{}
	for _, name := range order {
		for _, output := range index[name].Outputs {
			if _, err := safePath(p.Root, output); err != nil {
				return nil, err
			}
			paths = append(paths, output)
		}
	}
	if options.Cache {
		paths = append(paths, ".kamaji/cache")
	}
	if options.History {
		paths = append(paths, ".kamaji/runs")
	}
	sort.Strings(paths)
	if options.DryRun {
		return paths, nil
	}
	root, err := p.storage()
	if err != nil {
		return nil, err
	}
	lease, err := fsutil.Lock(filepath.Join(root, "build.lock"), true)
	if err != nil {
		return nil, err
	}
	defer lease.Close()
	for _, relative := range paths {
		if relative != ".kamaji/cache" && relative != ".kamaji/runs" {
			if _, err := safePath(p.Root, relative); err != nil {
				return nil, err
			}
		}
		if err := os.RemoveAll(filepath.Join(p.Root, filepath.FromSlash(relative))); err != nil {
			return nil, err
		}
	}
	return paths, nil
}
