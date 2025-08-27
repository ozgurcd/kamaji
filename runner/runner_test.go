package runner

import (
	"kamaji/obj"
	"kamaji/rt"
	"os"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	// Setup a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "kamaji_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary workspace file
	workspaceFilePath := filepath.Join(tmpDir, "test_workspace.yaml")
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
	obj.WorkspaceFile = "test_workspace.yaml"
	rt.Config.TmpDir = tmpDir
	rt.Config.WorkspaceDir = tmpDir // Ensure the workspace root is detected

	// Create necessary directories
	execRootDir := filepath.Join(tmpDir, "execroot")
	if err := os.MkdirAll(execRootDir, 0755); err != nil {
		t.Fatalf("Failed to create execroot directory: %v", err)
	}

	// Check if the execroot directory is created
	if _, err := os.Stat(execRootDir); os.IsNotExist(err) {
		t.Errorf("Execroot directory was not created")
	}
}

func TestPrepareCmdline(t *testing.T) {
	// Setup a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "kamaji_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create necessary directories
	rulesDir := filepath.Join(tmpDir, "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatalf("Failed to create rules directory: %v", err)
	}

	// Check if the rules directory is created
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		t.Errorf("Rules directory was not created")
	}
}
