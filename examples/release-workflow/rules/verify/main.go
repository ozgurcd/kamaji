// The verifier is a standalone process: Kamaji supplies flags, not a Go plugin ABI.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

type inventory struct {
	Format   int               `json:"format"`
	Metadata map[string]string `json:"metadata"`
	Files    []fileRecord      `json:"files"`
}

type fileRecord struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func main() {
	manifest := flag.String("manifest", "out/inventory.json", "Inventory JSON path")
	expected := flag.String("expected_metadata", "{}", "JSON object of required string metadata")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "verify: unexpected positional arguments")
		os.Exit(2)
	}
	if err := verify(*manifest, *expected); err != nil {
		fmt.Fprintln(os.Stderr, "verify:", err)
		os.Exit(1)
	}
}

func verify(manifest, expectedJSON string) error {
	var expected map[string]string
	if err := json.Unmarshal([]byte(expectedJSON), &expected); err != nil || expected == nil {
		return fmt.Errorf("expected_metadata must be a JSON object of strings")
	}
	source, err := os.Open(manifest)
	if err != nil {
		return err
	}
	defer source.Close()
	decoder := json.NewDecoder(source)
	decoder.DisallowUnknownFields()
	var data inventory
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("decode inventory: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("inventory must contain exactly one JSON document")
	}
	if data.Format != 1 || len(data.Files) == 0 {
		return fmt.Errorf("inventory requires format 1 and at least one file")
	}
	for key, value := range expected {
		if actual, ok := data.Metadata[key]; !ok || actual != value {
			return fmt.Errorf("metadata mismatch for %q", key)
		}
	}
	seen := make(map[string]bool)
	for _, record := range data.Files {
		if !fs.ValidPath(record.Path) || record.Path == "." || strings.Contains(record.Path, "\\") || seen[record.Path] {
			return fmt.Errorf("invalid or duplicate relative file path %q", record.Path)
		}
		seen[record.Path] = true
		file, err := os.Open(record.Path)
		if err != nil {
			return err
		}
		digest := sha256.New()
		size, readErr := io.Copy(digest, file)
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if size != record.Size || hex.EncodeToString(digest.Sum(nil)) != record.SHA256 {
			return fmt.Errorf("size or SHA256 mismatch for %q", record.Path)
		}
	}
	fmt.Printf("Verified %d files and expected metadata\n", len(data.Files))
	return nil
}
