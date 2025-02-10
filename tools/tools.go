package tools

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"kamaji/obj"
	"kamaji/rt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/h2non/filetype"
	"golang.org/x/exp/rand"
)

var WorkspaceDir string

func GetRule(target obj.ExecTarget) (string, error) {
	if target.Rule == "" {
		return "", fmt.Errorf("rule not found")
	}

	// if fist two chars of target.Rule are "//" we need to calculate the full path
	// since // symbolizes the root of the workspace
	if strings.HasPrefix(target.Rule, "//") {
		return filepath.Join(rt.Config.WorkspaceConfig.WorkspaceRoot, target.Rule[2:]), nil
	}

	return target.Rule, nil
}

func CreateMetadataFile(cacheDir string, filePath string) error {
	if rt.Config.DebugMode {
		log.Printf("Creating metadata file for: %s\n", filePath)
	}

	metadataFilePath := filepath.Join(cacheDir, "metadata")
	metadataFile, err := os.Create(metadataFilePath)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %s", err.Error())
	}
	defer metadataFile.Close()

	downloadedFilePath := filepath.Join(cacheDir, "file")
	fileType, err := determineFileType(downloadedFilePath)
	if err != nil {
		return fmt.Errorf("failed to determine file type: %s", err.Error())
	}

	switch fileType {
	case "application/zip":
		Unzip(filePath, cacheDir)
	case "x-mach-binary":
		mac_binary(filePath, cacheDir)
		// TODO: add linux_binary
	}

	finalContent := fmt.Sprintf("%s,%s", filePath, fileType)
	if _, err := metadataFile.WriteString(finalContent); err != nil {
		return fmt.Errorf("failed to write metadata file: %s", err.Error())
	}

	if rt.Config.DebugMode {
		log.Printf("Metadata file created: %s\n", metadataFilePath)
	}
	return nil
}

func mac_binary(filePath string, cacheDir string) error {
	// in this case the file is a binary file, we just need to make executable
	// after we copy it in the __TMP__ directory and rename it to the final name

	binaryFilePath := filepath.Join(cacheDir, "__TMP__", "file")
	if err := CopyFile(filePath, binaryFilePath); err != nil {
		return fmt.Errorf("failed to copy file: %s", err.Error())
	}
	binaryFilePath = filepath.Join(cacheDir, "__TMP__", "file")

	return nil
}

func determineFileType(filePath string) (string, error) {
	if rt.Config.DebugMode {
		log.Printf("Determining file type for: %s\n", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read first 261 bytes for detection
	buf := make([]byte, 261)
	_, err = file.Read(buf)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	kind, unknown := filetype.Match(buf)
	if unknown != nil {
		if rt.Config.DebugMode {
			log.Printf("Unknown file type for: %s\n", filePath)
		}
	} else {
		if rt.Config.DebugMode {
			log.Printf("File: %s, Type: %s, MIME: %s\n", filePath, kind.Extension, kind.MIME.Value)
		}
	}
	return kind.MIME.Value, nil
}

func IsFileValid(filePath, expectedSHA256 string) bool {
	calculatedSHA256 := calculateSHA256(filePath)
	return calculatedSHA256 == expectedSHA256
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
		switch v := value.(type) {
		case map[any]any: // Recursively normalize nested maps
			output[strKey] = NormalizeMap(v)
		default:
			output[strKey] = v
		}
	}
	return output
}

func Unzip(src, dest string) error {
	if rt.Config.DebugMode {
		log.Printf("Unzipping file: %s into: %s\n", src, dest)
	}

	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return err
	}

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		relPath, err := filepath.Rel(dest, fpath)
		if err != nil || strings.HasPrefix(relPath, "..") {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := extractFile(f, fpath); err != nil {
			return err
		}
	}

	return nil
}

func extractFile(f *zip.File, fpath string) error {
	if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
		return err
	}

	outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return err
	}

	return nil
}

func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

func GetFullPath(cacheDir string, targetFileName string) string {
	var targetPath string
	filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == targetFileName {
			targetPath = path
			return filepath.SkipDir
		}
		return nil
	})
	return targetPath
}

func RandStringRunes(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	rand.Seed(uint64(time.Now().UnixNano()))
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// MirrorDirectoryWithSymLinks mirrors the contents of the source directory under the target directory by creating symbolic links.
// For files, a symlink is created from target to the original file.
// For directories, a symlink is created from target to the entire source subdirectory, mirroring its entire content recursively.
func MirrorDirectoryWithSymLinks(source string, target string) error {
	// Get the absolute path for the source directory.
	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of source %s: %w", source, err)
	}

	// Ensure the source exists and is a directory.
	sourceInfo, err := os.Stat(absSource)
	if err != nil {
		return fmt.Errorf("failed to stat source %s: %w", absSource, err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source %s is not a directory", absSource)
	}

	// Ensure the target directory exists.
	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", target, err)
	}

	// Read the immediate entries of the source directory.
	entries, err := os.ReadDir(absSource)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", absSource, err)
	}

	// For each entry, create a symlink in the target.
	for _, entry := range entries {
		srcPath := filepath.Join(absSource, entry.Name())
		destPath := filepath.Join(target, entry.Name())

		// Remove any pre-existing entry at the destination.
		if _, err := os.Lstat(destPath); err == nil {
			if err := os.RemoveAll(destPath); err != nil {
				return fmt.Errorf("failed to remove existing entry at %s: %w", destPath, err)
			}
		}

		// Create the symlink. Whether the entry is a file or a directory,
		// a single symlink is created. In the latter case, the entire subdirectory is symlinked.
		if err := os.Symlink(srcPath, destPath); err != nil {
			return fmt.Errorf("failed to create symlink from %s to %s: %w", destPath, srcPath, err)
		}
	}

	return nil
}
