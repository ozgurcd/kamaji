package execroot

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"kamaji/obj"
	"kamaji/rt"
	"kamaji/tools"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func parseMetadata(metadata string) (string, string) {
	parts := strings.Split(metadata, ",")
	return parts[1], parts[0]
}

func handleMacBinaryFile(tfi obj.ThirdPartyFileInfo) error {
	tmpDir := filepath.Join(tfi.FileName, "__TMP__")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create __TMP__ directory: %s", err.Error())
	}

	sourceFile := filepath.Join(tfi.FileName, "file")
	destFile := filepath.Join(tmpDir, filepath.Base(tfi.FinalName))
	if err := tools.CopyFile(sourceFile, destFile); err != nil {
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
	if err := tools.CopyFile(sourceFile, destFile); err != nil {
		return fmt.Errorf("failed to copy file: %s", err.Error())
	}

	return tools.Unzip(destFile, tmpDir)
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

func handleFileType(fileType string, tfi obj.ThirdPartyFileInfo, fileName string) error {
	switch fileType {
	case "application/zip":
		return handleZipFile(tfi, fileName)
	case "application/x-mach-binary":
		return handleMacBinaryFile(tfi)
	case "application/gzip":
		return handleTarGzFile(tfi)
	default:
		return fmt.Errorf("unsupported file type: %s", fileType)
	}
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

func CreateExecRootDir(target obj.ExecTarget) error {
	execRootDir := fmt.Sprintf(
		"%s/execroot/%s-%s",
		rt.Config.TmpDir,
		target.Name,
		tools.RandStringRunes(6),
	)
	if rt.Config.DebugMode {
		log.Printf("Creating execroot dir: %s\n", execRootDir)
	}

	if err := os.MkdirAll(execRootDir, 0700); err != nil {
		return fmt.Errorf("failed to create execroot dir: %s", err.Error())
	}

	rt.Config.ExecRootDir = execRootDir

	return nil
}

func CopyThirdPartyIntoExecRootDir() error {
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

		// create external dir in execroot
		externalDir := filepath.Join(rt.Config.ExecRootDir, "external")
		if err := os.MkdirAll(externalDir, 0700); err != nil {
			return fmt.Errorf("failed to create external dir: %s", err.Error())
		}

		thirdPartyFileInExecRootDir := filepath.Join(rt.Config.ExecRootDir, "external", tfi.FinalName)
		targetFullPath := tools.GetFullPath(filepath.Join(tfi.FileName, "__TMP__"), targetFileName)
		if targetFullPath == "" {
			return fmt.Errorf("target file not found in %s", filepath.Join(tfi.FileName, "__TMP__"))
		}

		if err := os.Symlink(targetFullPath, thirdPartyFileInExecRootDir); err != nil {
			return fmt.Errorf("failed to create softlink: %s", err.Error())
		}

		rt.Config.ThirdPartyFinalPaths[fileName] = targetFullPath
	}

	return nil
}
