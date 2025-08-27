package target

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestParseBuildFile(t *testing.T) {
	// Create a temporary build file
	buildFileContent := `
targets:
  - name: "test_target"
    rule: "test_rule.py"
    config:
      key: "value"
`
	tmpFile, err := ioutil.TempFile("", "BUILD.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(buildFileContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Call ParseBuildFile
	target, err := ParseBuildFile(tmpFile.Name(), "test_target")
	if err != nil {
		t.Errorf("ParseBuildFile failed: %v", err)
	}

	// Check if the target is correctly parsed
	if target.Name != "test_target" {
		t.Errorf("Expected target name 'test_target', got %s", target.Name)
	}
	if target.Rule != "test_rule.py" {
		t.Errorf("Expected rule 'test_rule.py', got %s", target.Rule)
	}
	if target.Config["key"] != "value" {
		t.Errorf("Expected config key 'value', got %v", target.Config["key"])
	}
}
