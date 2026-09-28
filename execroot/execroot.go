package execroot

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"
)

func parseMetadata(metadata string) (string, string) {
	i := strings.LastIndex(metadata, ",")
	if i <= 0 || i == len(metadata)-1 {
		return "", ""
	}
	return metadata[i+1:], metadata[:i]
}

func handleExecutableFile(tfi obj.ThirdPartyFileInfo) error {
	dest := filepath.Join(tfi.FileName, "__TMP__", tfi.FinalName)
	if !filepath.IsLocal(tfi.FinalName) {
		return fmt.Errorf("invalid artifact path")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	if err := tools.CopyFile(filepath.Join(tfi.FileName, "file"), dest); err != nil {
		return err
	}
	return os.Chmod(dest, 0700)
}

func (scope *Preparer) handleTarGzFile(tfi obj.ThirdPartyFileInfo) error {
	dest := filepath.Join(tfi.FileName, "__TMP__")
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	if err := scope.extractTarGz(filepath.Join(tfi.FileName, "file"), dest); err != nil {
		return err
	}
	path := tools.GetFullPath(dest, tfi.FinalName)
	if path == "" {
		return fmt.Errorf("artifact file not found: %s", tfi.FinalName)
	}
	return os.Chmod(path, 0700)
}

func (scope *Preparer) handleFileType(fileType string, tfi obj.ThirdPartyFileInfo) error {
	switch fileType {
	case "application/zip":
		return (&tools.Context{Runtime: scope.Runtime}).Unzip(filepath.Join(tfi.FileName, "file"), filepath.Join(tfi.FileName, "__TMP__"))
	case "application/x-mach-binary", "application/x-executable", "application/x-elf", "application/x-sharedlib":
		return handleExecutableFile(tfi)
	case "application/gzip":
		return scope.handleTarGzFile(tfi)
	default:
		return fmt.Errorf("unsupported file type: %s", fileType)
	}
}

func (scope *Preparer) extractTarGz(src, dest string) error {
	limits, err := scope.Runtime.EffectiveLimits()
	if err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > limits.MaxDownloadBytes {
		return fmt.Errorf("tar archive exceeds download byte limit")
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read gzip: %w", err)
	}
	defer gz.Close()
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	// Limit decoded headers, PAX metadata, padding, and trailing streams too.
	// EffectiveLimits has checked this arithmetic for overflow.
	streamLimit := limits.MaxExtractBytes + limits.MaxArchiveEntries*1024 + (1 << 20)
	decoded := &io.LimitedReader{R: gz, N: streamLimit + 1}
	tr := tar.NewReader(decoded)
	remaining, entries := limits.MaxExtractBytes, int64(0)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			// A tar EOF precedes the gzip trailer. Drain the bounded remainder
			// so corruption or appended expansion is not silently accepted.
			_, err := io.Copy(io.Discard, decoded)
			if decoded.N == 0 {
				return fmt.Errorf("tar decoded stream exceeds byte limit")
			}
			return err
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		entries++
		if entries > limits.MaxArchiveEntries {
			return fmt.Errorf("tar archive exceeds entry limit")
		}
		if h.Size < 0 || h.Size > remaining {
			return fmt.Errorf("tar archive exceeds extracted byte limit")
		}
		remaining -= h.Size
		if !filepath.IsLocal(h.Name) {
			return fmt.Errorf("illegal archive entry: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(h.Name, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := root.MkdirAll(filepath.Dir(h.Name), 0755); err != nil {
				return err
			}
			out, err := root.OpenFile(h.Name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode).Perm())
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported tar entry type: %c", h.Typeflag)
		}
	}
}

func (scope *Preparer) CreateExecRootDir(target obj.ExecTarget) error {
	if target.Name == "" || target.Name == "." || target.Name == ".." || strings.ContainsAny(target.Name, `/\`) {
		return fmt.Errorf("invalid target name")
	}
	if scope.Runtime.Config.TmpDir == "" {
		return fmt.Errorf("temporary directory is not configured")
	}
	if err := scope.Runtime.EnsureTempDir(); err != nil {
		return err
	}
	parent := filepath.Join(scope.Runtime.Config.TmpDir, "execroot")
	if err := rt.EnsurePrivateDir(parent); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(parent, target.Name+"-")
	if err != nil {
		return err
	}
	scope.Runtime.Config.ExecRootDir = dir
	return nil
}

func (scope *Preparer) CopyThirdPartyIntoExecRootDir() error {
	if scope.Runtime.Config.ThirdPartyFinalPaths == nil {
		scope.Runtime.Config.ThirdPartyFinalPaths = make(map[string]string)
	}
	for name, cached := range scope.Runtime.Config.ThirdPartyFiles {
		if !filepath.IsLocal(cached.FinalName) || cached.FinalName == "." {
			return fmt.Errorf("invalid artifact path")
		}
		metadata, err := os.ReadFile(filepath.Join(cached.FileName, "metadata"))
		if err != nil {
			return fmt.Errorf("read metadata: %w", err)
		}
		fileType, filePath := parseMetadata(string(metadata))
		if fileType == "" || filePath == "" {
			return fmt.Errorf("invalid artifact metadata")
		}
		// Extract into a private execution directory, never into the shared cache.
		dir, err := os.MkdirTemp(scope.Runtime.Config.ExecRootDir, "artifact-")
		if err != nil {
			return err
		}
		if err := tools.CopyFile(filepath.Join(cached.FileName, "file"), filepath.Join(dir, "file")); err != nil {
			return err
		}
		local := obj.ThirdPartyFileInfo{FileName: dir, FinalName: cached.FinalName}
		if err := scope.handleFileType(fileType, local); err != nil {
			return err
		}
		path := tools.GetFullPath(filepath.Join(dir, "__TMP__"), cached.FinalName)
		if path == "" {
			return fmt.Errorf("artifact file not found: %s", cached.FinalName)
		}
		if err := os.Chmod(path, 0700); err != nil {
			return err
		}
		link := filepath.Join(scope.Runtime.Config.ExecRootDir, "external", cached.FinalName)
		if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
			return err
		}
		if _, err := os.Lstat(link); err == nil {
			return fmt.Errorf("duplicate external artifact path: %s", cached.FinalName)
		}
		if err := os.Symlink(path, link); err != nil {
			return err
		}
		scope.Runtime.Config.ThirdPartyFinalPaths[name] = path
	}
	return nil
}
