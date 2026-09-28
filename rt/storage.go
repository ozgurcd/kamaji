package rt

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type StorageEntry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Bytes    int64     `json:"bytes"`
	Modified time.Time `json:"modified"`
	Status   string    `json:"status"`
}

// StorageEntries is a read-only snapshot. It never follows symlinks.
func (scope *Runtime) StorageEntries(kind string) ([]StorageEntry, error) {
	if kind != "cache" && kind != "execroot" {
		return nil, fmt.Errorf("invalid storage kind")
	}
	root, err := scope.TempPath()
	if err != nil {
		return nil, err
	}
	for _, path := range []string{root, filepath.Join(root, kind)} {
		if err := CheckPrivateDir(path); os.IsNotExist(err) {
			return []StorageEntry{}, nil
		} else if err != nil {
			return nil, err
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, kind))
	if err != nil {
		return nil, err
	}
	result := []StorageEntry{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, kind, entry.Name())
		if err := CheckPrivateDir(path); err != nil {
			return nil, err
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		item := StorageEntry{Name: entry.Name(), Path: path, Modified: info.ModTime(), Status: "incomplete or active"}
		if kind == "cache" {
			item.Status = "unrecognized"
			if digest, err := hex.DecodeString(item.Name); err == nil && len(digest) == 32 {
				item.Status = "cached"
			}
			if file, err := os.Lstat(filepath.Join(path, "file")); err == nil && file.Mode().IsRegular() {
				item.Modified = file.ModTime()
			}
		} else if complete, err := os.Lstat(filepath.Join(path, ".complete")); err == nil && complete.Mode().IsRegular() {
			item.Status = "completed"
			item.Modified = complete.ModTime()
		}
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.Type().IsRegular() {
				return nil
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if info.Size() > math.MaxInt64-item.Bytes {
				return fmt.Errorf("storage size exceeds supported range")
			}
			item.Bytes += info.Size()
			return nil
		})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// CachePrune selects oldest payloads until the remaining cache fits maxBytes.
// Actual deletion holds the exclusive runtime lease through selection and removal.
func (scope *Runtime) CachePrune(maxBytes int64, dryRun bool) ([]StorageEntry, error) {
	if maxBytes < 0 {
		return nil, fmt.Errorf("cache budget cannot be negative")
	}
	if !dryRun {
		lease, err := scope.RuntimeLease(true)
		if err != nil {
			return nil, err
		}
		defer lease.Close()
	}
	entries, err := scope.StorageEntries("cache")
	if err != nil {
		return nil, err
	}
	total := int64(0)
	for _, entry := range entries {
		if entry.Bytes > math.MaxInt64-total {
			return nil, fmt.Errorf("cache size exceeds supported range")
		}
		total += entry.Bytes
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Modified.Equal(entries[j].Modified) {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Modified.Before(entries[j].Modified)
	})
	removed := []StorageEntry{}
	for _, entry := range entries {
		if maxBytes > 0 && total <= maxBytes {
			break
		}
		if entry.Status != "cached" {
			continue
		}
		if !dryRun {
			if err := os.RemoveAll(entry.Path); err != nil {
				return removed, err
			}
		}
		removed = append(removed, entry)
		total -= entry.Bytes
	}
	if total > maxBytes {
		return removed, fmt.Errorf("unrecognized cache entries prevent reaching budget; inspect cache status")
	}
	return removed, nil
}
