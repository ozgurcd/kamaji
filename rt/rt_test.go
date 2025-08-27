package rt

import (
	"fmt"
	"kamaji/obj"
	"os"
	"path/filepath"
	"testing"
)

func TestInitRuntime(t *testing.T) {
	// Setup a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "kamaji_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary workspace file
	workspaceFilePath := filepath.Join(tmpDir, "kamaji.workspace.yaml")
	workspaceContent := `rules_directory: //rules
workspace_vars:
  - org_domain: "example.com"
    base_dir: "/projects/infra"
third_party:
  - name: terraform_1_9_0
    file_path: "terraform"
    url:
      darwin_arm64: "https://example.com/terraform_1_9_0_darwin_arm64.zip"
    sha256:
      darwin_arm64: "abc123"
`
	if err := os.WriteFile(workspaceFilePath, []byte(workspaceContent), 0644); err != nil {
		t.Fatalf("Failed to write workspace file: %v", err)
	}

	// Mock configuration
	obj.WorkspaceFile = "kamaji.workspace.yaml"
	Config.TmpDir = tmpDir
	Config.WorkspaceDir = tmpDir // Ensure the workspace root is detected

	// Call InitRuntime
	InitRuntime("")

	// Check if the cache directory is created
	cacheDir := filepath.Join(tmpDir, "cache")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Errorf("Cache directory was not created")
	}

	// Check if the sha256 directory is created
	shaDir := filepath.Join(cacheDir, "sha256")
	if _, err := os.Stat(shaDir); os.IsNotExist(err) {
		t.Errorf("SHA256 directory was not created")
	}

	// Add logging for directory paths
	fmt.Printf("Test Cache directory path: %s\n", cacheDir)
	fmt.Printf("Test SHA256 directory path: %s\n", shaDir)

	// Check directory permissions
	cacheInfo, err := os.Stat(cacheDir)
	if err != nil {
		t.Errorf("Failed to stat test cache directory: %v", err)
	} else if cacheInfo.Mode().Perm() != 0755 {
		t.Errorf("Test Cache directory permissions are incorrect: %v", cacheInfo.Mode().Perm())
	}

	shaInfo, err := os.Stat(shaDir)
	if err != nil {
		t.Errorf("Failed to stat test SHA256 directory: %v", err)
	} else if shaInfo.Mode().Perm() != 0755 {
		t.Errorf("Test SHA256 directory permissions are incorrect: %v", shaInfo.Mode().Perm())
	}
}

func TestSetupPythonEnv(t *testing.T) {
	// Setup a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "kamaji_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock configuration
	Config.TmpDir = tmpDir

	// Call SetupPythonEnv
	err = SetupPythonEnv()
	if err != nil {
		t.Errorf("Failed to set up Python environment: %v", err)
	}

	// Check if the virtual environment directory is created
	venvDir := filepath.Join(tmpDir, "venv")
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		t.Errorf("Virtual environment directory was not created")
	}
}
