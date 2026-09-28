package tools

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"kamaji/obj"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"
)

func (scope *Context) GetRule(target obj.ExecTarget) (string, error) {
	if target.Rule == "" {
		return "", fmt.Errorf("rule not found")
	}

	// if fist two chars of target.Rule are "//" we need to calculate the full path
	// since // symbolizes the root of the workspace
	if strings.HasPrefix(target.Rule, "//") {
		return filepath.Join(scope.Runtime.Config.WorkspaceConfig.WorkspaceRoot, target.Rule[2:]), nil
	}

	return target.Rule, nil
}

func CreateMetadataFile(cacheDir string, filePath string) error {

	metadataFilePath := filepath.Join(cacheDir, "metadata")
	downloadedFilePath := filepath.Join(cacheDir, "file")
	fileType, err := determineFileType(downloadedFilePath)
	if err != nil {
		return fmt.Errorf("failed to determine file type: %s", err.Error())
	}

	finalContent := fmt.Sprintf("%s,%s", filePath, fileType)
	if current, err := os.ReadFile(metadataFilePath); err == nil && string(current) == finalContent {
		return nil
	}
	metadataFile, err := os.CreateTemp(cacheDir, "metadata-*")
	if err != nil {
		return fmt.Errorf("create metadata: %w", err)
	}
	defer metadataFile.Close()
	defer os.Remove(metadataFile.Name())
	if _, err := metadataFile.WriteString(finalContent); err != nil {
		return fmt.Errorf("failed to write metadata file: %s", err.Error())
	}

	if err := metadataFile.Close(); err != nil {
		return err
	}
	return os.Rename(metadataFile.Name(), metadataFilePath)
}

func determineFileType(filePath string) (string, error) {

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read first 261 bytes for detection
	buf := make([]byte, 261)
	n, err := file.Read(buf)
	if err != nil {
		return "", fmt.Errorf("read file header: %w", err)
	}

	kind, err := filetype.Match(buf[:n])
	if err != nil {
		return "", fmt.Errorf("detect file type: %w", err)
	}
	return kind.MIME.Value, nil
}

func IsFileValid(filePath, expectedSHA256 string) bool {
	decoded, err := hex.DecodeString(expectedSHA256)
	if err != nil || len(decoded) != sha256.Size {
		return false
	}
	calculatedSHA256 := calculateSHA256(filePath)
	return calculatedSHA256 != "" && strings.EqualFold(calculatedSHA256, expectedSHA256)
}

func calculateSHA256(filePath string) string {
	hash := sha256.New()

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %s\n", err.Error())
		return ""
	}
	defer file.Close()

	_, err = io.Copy(hash, file)
	if err != nil {
		fmt.Printf("Error copying file: %s\n", err.Error())
		return ""
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

func NormalizeMap(input map[any]any) map[string]any {
	output := make(map[string]any)
	for key, value := range input {
		strKey := fmt.Sprintf("%v", key) // Convert key to string
		output[strKey] = normalizeValue(value)
	}
	return output
}

func normalizeValue(value any) any {
	switch v := value.(type) {
	case map[any]any:
		return NormalizeMap(v)
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = normalizeValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeValue(item)
		}
		return out
	default:
		return value
	}
}

func (scope *Context) Unzip(src, dest string) error {
	limits, err := scope.Runtime.EffectiveLimits()
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.Size() > limits.MaxDownloadBytes {
		return fmt.Errorf("ZIP archive exceeds download byte limit")
	}

	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	if int64(len(r.File)) > limits.MaxArchiveEntries {
		return fmt.Errorf("ZIP archive exceeds entry limit")
	}
	remaining := limits.MaxExtractBytes
	for _, file := range r.File {
		if file.UncompressedSize64 > uint64(remaining) {
			return fmt.Errorf("ZIP archive exceeds extracted byte limit")
		}
		remaining -= int64(file.UncompressedSize64)
	}

	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()

	for _, f := range r.File {
		if !filepath.IsLocal(f.Name) || f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("illegal archive entry: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := root.MkdirAll(f.Name, 0755); err != nil {
				return err
			}
			continue
		}

		if err := extractFile(root, f); err != nil {
			return err
		}
	}

	return nil
}

func extractFile(root *os.Root, f *zip.File) error {
	if err := root.MkdirAll(filepath.Dir(f.Name), 0755); err != nil {
		return err
	}

	outFile, err := root.OpenFile(f.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode().Perm())
	if err != nil {
		return err
	}
	defer outFile.Close()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if _, err := CopyWithLimit(outFile, rc, int64(f.UncompressedSize64)); err != nil {
		return err
	}

	return outFile.Close()
}

// CopyWithLimit never writes more than limit bytes and checks one extra byte
// from the input to distinguish an exact-size stream from an oversized one.
func CopyWithLimit(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	if limit < 0 {
		return 0, fmt.Errorf("invalid byte limit")
	}
	written, err := io.Copy(dst, io.LimitReader(src, limit))
	if err != nil || written < limit {
		return written, err
	}
	var probe [1]byte
	n, err := io.ReadFull(src, probe[:])
	if n != 0 {
		return written, fmt.Errorf("stream exceeds configured byte limit")
	}
	if err != io.EOF {
		return written, err
	}
	return written, nil
}

func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("source must be a regular file")
	}
	if dstInfo, err := os.Stat(dst); err == nil && os.SameFile(srcInfo, dstInfo) {
		return fmt.Errorf("source and destination are the same file")
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	if err := dstFile.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

func GetFullPath(cacheDir string, targetFileName string) string {
	if !filepath.IsLocal(targetFileName) {
		return ""
	}
	if strings.ContainsAny(targetFileName, `/\`) {
		path := filepath.Join(cacheDir, targetFileName)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return ""
		}
		resolved, err := filepath.EvalSymlinks(path)
		base, baseErr := filepath.EvalSymlinks(cacheDir)
		rel, relErr := filepath.Rel(base, resolved)
		if err != nil || baseErr != nil || relErr != nil || !filepath.IsLocal(rel) {
			return ""
		}
		return path
	}
	var matches []string
	err := filepath.WalkDir(cacheDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() && entry.Name() == targetFileName {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil || len(matches) != 1 {
		return ""
	}
	return matches[0]
}
