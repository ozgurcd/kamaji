package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"kamaji/obj"
	"kamaji/rt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/h2non/filetype"
	"golang.org/x/exp/rand"
	"gopkg.in/yaml.v2"
)

var WorkspaceDir string

func ParseBuildFile(buildFileName string, targetName string) (obj.Target, error) {
	if rt.Config.DebugMode {
		log.Printf("Parsing build file: %s\n", buildFileName)
	}

	data, err := os.ReadFile(buildFileName)
	if err != nil {
		return obj.Target{}, err
	}

	var buildFile obj.BuildFile
	if err := yaml.Unmarshal(data, &buildFile); err != nil {
		return obj.Target{}, err
	}

	for _, target := range buildFile.Targets {
		if target.Name == targetName {
			return target, nil
		}
	}

	return obj.Target{}, fmt.Errorf("target %s not found", targetName)
}

func InitThirdPartyUsedInTarget(workspaceConfig obj.WorkspaceConfig, target obj.Target) error {
	var lastError error
	for _, value := range target.Config {
		if strValue, ok := value.(string); ok && strings.HasPrefix(strValue, "@@") {
			downloadCandidate := strValue[2:]
			if err := downloadThirdParty(workspaceConfig, downloadCandidate); err != nil {
				log.Printf("Error downloading %s: %v", downloadCandidate, err)
				lastError = err
			}
		}
	}
	return lastError
}

func findThirdPartyConfig(workspaceConfig obj.WorkspaceConfig, downloadCandidate string) (obj.ThirdPartyConfig, error) {
	if rt.Config.DebugMode {
		log.Printf("Looking for third party config for %s\n", downloadCandidate)
	}
	for _, thirdParty := range workspaceConfig.ThirdParty {
		if thirdParty.Name == downloadCandidate {
			return thirdParty, nil
		}
	}
	return obj.ThirdPartyConfig{}, fmt.Errorf("third party config not found for %s", downloadCandidate)
}

func downloadThirdParty(workspaceConfig obj.WorkspaceConfig, downloadCandidate string) error {
	if rt.Config.DebugMode {
		log.Printf("Looking for third party config for %s\n", downloadCandidate)
	}
	thirdParty, err := findThirdPartyConfig(workspaceConfig, downloadCandidate)
	if err != nil {
		log.Fatalf("Third party config requested from BUILD.yaml for %s is not present in workspace config.\n", downloadCandidate)
	}

	if doesThirdPartyExist(thirdParty.Name) {
		if rt.Config.DebugMode {
			log.Printf("Third party %s already exists, skipping\n", thirdParty.Name)
		}
		return validateCachedFile(thirdParty)
	}

	err = downloadAndCacheFile(thirdParty)
	if err != nil {
		log.Printf("Failed to download and cache file: %s\n", err.Error())
		return err
	}

	return nil
}

func validateCachedFile(thirdParty obj.ThirdPartyConfig) error {
	if rt.Config.DebugMode {
		log.Printf("Validating cached file for %s\n", thirdParty.Name)
	}
	cacheDir := filepath.Join(rt.Config.CacheDir, thirdParty.SHA256s[rt.Config.Platform])
	filePath := filepath.Join(cacheDir, "file")

	if !isFileValid(filePath, thirdParty.SHA256s[rt.Config.Platform]) {
		log.Printf("Cached file is invalid\n")
		return fmt.Errorf("file is invalid")
	}

	rt.Config.ThirdPartyFiles[thirdParty.Name] = obj.ThirdPartyFileInfo{
		FileName:  cacheDir,
		FinalName: thirdParty.FilePath,
	}

	if rt.Config.DebugMode {
		log.Printf("Cached file is valid\n")
	}

	return nil
}

func downloadAndCacheFile(thirdParty obj.ThirdPartyConfig) error {
	fmt.Printf("Downloading Third Party: %s\n", thirdParty.Name)
	if rt.Config.DebugMode {
		log.Printf("Downloading Third Party: %s\n", thirdParty.Name)
	}

	sha256, url := thirdParty.SHA256s[rt.Config.Platform], thirdParty.URLs[rt.Config.Platform]
	if sha256 == "" || url == "" {
		return fmt.Errorf("sha256 or url is empty for %s", thirdParty.Name)
	}

	if rt.Config.DebugMode {
		log.Printf("Downloading Third Party: %s from %s\n", thirdParty.Name, url)
	}

	cacheDir := filepath.Join(rt.Config.CacheDir, sha256)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %s", err.Error())
	}

	filePath := filepath.Join(cacheDir, "file")
	if err := downloadFile(url, filePath); err != nil {
		if rt.Config.DebugMode {
			log.Printf("Failed to download file: %s\n", err.Error())
		}
		return err
	}

	if !isFileValid(filePath, sha256) {
		if rt.Config.DebugMode {
			log.Printf("File is invalid\n")
		}
		return fmt.Errorf("file is invalid")
	}

	if err := createMetadataFile(cacheDir, thirdParty.FilePath); err != nil {
		return err
	}

	rt.Config.ThirdPartyFiles[thirdParty.Name] = obj.ThirdPartyFileInfo{
		FileName:  cacheDir,
		FinalName: thirdParty.FilePath,
	}
	return nil
}

func downloadFile(url, filePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s", resp.Status)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err.Error())
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to copy file: %s", err.Error())
	}

	if rt.Config.DebugMode {
		log.Printf("Downloaded file to: %s\n", filePath)
	}
	return nil
}

func createMetadataFile(cacheDir string, filePath string) error {
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
		unzip(filePath, cacheDir)
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
	if err := copyFile(filePath, binaryFilePath); err != nil {
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

func isFileValid(filePath, expectedSHA256 string) bool {
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

func doesThirdPartyExist(downloadCandidate string) bool {
	var tpConfig obj.ThirdPartyConfig
	for _, tp := range rt.Config.WorkspaceConfig.ThirdParty {
		if tp.Name == downloadCandidate {
			tpConfig = tp
			break
		}
	}

	sha256 := tpConfig.SHA256s[rt.Config.Platform]
	if sha256 == "" {
		return false
	}

	dirToCheck := filepath.Join(rt.Config.CacheDir, sha256)
	if rt.Config.DebugMode {
		log.Printf("Checking if third party exists: %s\n", dirToCheck)
	}

	if _, err := os.Stat(dirToCheck); os.IsNotExist(err) {
		if rt.Config.DebugMode {
			log.Printf("Third party does not exist: %s\n", dirToCheck)
		}
		return false
	}

	if rt.Config.DebugMode {
		log.Printf("Third party exists: %s\n", dirToCheck)
	}
	return true
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

func CreateSandbox(target obj.Target) error {
	sandboxDir := fmt.Sprintf("%s/sandbox/%s-%s", rt.Config.TmpDir, target.Name, randStringRunes(6))
	if rt.Config.DebugMode {
		log.Printf("Creating sandbox dir: %s\n", sandboxDir)
	}

	if err := os.MkdirAll(sandboxDir, 0700); err != nil {
		return fmt.Errorf("failed to create sandbox dir: %s", err.Error())
	}

	rt.Config.SandboxDir = sandboxDir

	return nil
}

func CopyThirdPartyIntoSandbox() error {
	for fileName, tfi := range rt.Config.ThirdPartyFiles {
		metadataFilePath := filepath.Join(tfi.FileName, "metadata")
		metadataContent, err := os.ReadFile(metadataFilePath)
		if err != nil {
			return fmt.Errorf("failed to read metadata file: %s", err.Error())
		}

		metadata := string(metadataContent)
		fileType, targetFileName := parseMetadata(metadata)
		if err := handleFileType(fileType, tfi, fileName); err != nil {
			return err
		}

		thirdPartyFileInSandbox := filepath.Join(rt.Config.SandboxDir, tfi.FinalName)
		targetFullPath := getFullPath(filepath.Join(tfi.FileName, "__TMP__"), targetFileName)
		if targetFullPath == "" {
			return fmt.Errorf("target file not found in %s", filepath.Join(tfi.FileName, "__TMP__"))
		}

		if err := os.Symlink(targetFullPath, thirdPartyFileInSandbox); err != nil {
			return fmt.Errorf("failed to create softlink: %s", err.Error())
		}

		rt.Config.ThirdPartyFinalPaths[fileName] = targetFullPath
	}

	if rt.Config.DebugMode {
		log.Printf("Third party final paths: %v\n", rt.Config.ThirdPartyFinalPaths)
	}

	return nil
}

func parseMetadata(metadata string) (string, string) {
	parts := strings.Split(metadata, ",")
	return parts[1], parts[0]
}

func handleFileType(fileType string, tfi obj.ThirdPartyFileInfo, fileName string) error {
	switch fileType {
	case "application/zip":
		return handleZipFile(tfi, fileName)
	case "application/x-mach-binary":
		return handleMachBinaryFile(tfi)
	case "application/gzip":
		return handleTarGzFile(tfi)
	default:
		return fmt.Errorf("unsupported file type: %s", fileType)
	}
}

func handleMachBinaryFile(tfi obj.ThirdPartyFileInfo) error {
	tmpDir := filepath.Join(tfi.FileName, "__TMP__")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create __TMP__ directory: %s", err.Error())
	}

	sourceFile := filepath.Join(tfi.FileName, "file")
	destFile := filepath.Join(tmpDir, filepath.Base(tfi.FinalName))
	if err := copyFile(sourceFile, destFile); err != nil {
		return fmt.Errorf("failed to copy Mach-O binary: %s", err.Error())
	}

	if err := os.Chmod(destFile, 0700); err != nil {
		return fmt.Errorf("failed to make Mach-O binary executable: %s", err.Error())
	}

	return nil
}

func handleZipFile(tfi obj.ThirdPartyFileInfo, fileName string) error {
	tmpDir := filepath.Join(tfi.FileName, "__TMP__")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create __TMP__ directory: %s", err.Error())
	}

	destFile := filepath.Join(tmpDir, fmt.Sprintf("%s.zip", fileName))
	sourceFile := filepath.Join(tfi.FileName, "file")
	if err := copyFile(sourceFile, destFile); err != nil {
		return fmt.Errorf("failed to copy file: %s", err.Error())
	}

	return unzip(destFile, tmpDir)
}

func handleTarGzFile(tfi obj.ThirdPartyFileInfo) error {
	tmpDir := filepath.Join(tfi.FileName, "__TMP__")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create __TMP__ directory: %s", err.Error())
	}

	sourceFile := filepath.Join(tfi.FileName, "file")
	if err := extractTarGz(sourceFile, tmpDir); err != nil {
		return fmt.Errorf("failed to extract tar.gz file: %s", err.Error())
	}

	targetFilePath := filepath.Join(tmpDir, "darwin-arm64", filepath.Base(tfi.FinalName))
	if err := os.Chmod(targetFilePath, 0755); err != nil {
		return fmt.Errorf("failed to make file executable: %s", err.Error())
	}

	return nil
}

func extractTarGz(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open tar.gz file: %s", err.Error())
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %s", err.Error())
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %s", err.Error())
		}

		target := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory: %s", err.Error())
			}
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("failed to create file: %s", err.Error())
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to copy file: %s", err.Error())
			}
			outFile.Close()
		default:
			return fmt.Errorf("unsupported file type: %c", header.Typeflag)
		}
	}

	return nil
}

func unzip(src, dest string) error {
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

func copyFile(src, dst string) error {
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

func getFullPath(cacheDir string, targetFileName string) string {
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

func randStringRunes(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	rand.Seed(uint64(time.Now().UnixNano()))
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
