package rt

import "testing"

func TestStrictWorkspace(t *testing.T) {
	for _, body := range []string{"limits:\n  max_download_byte: 1\n", "rules_directory: first\n---\nrules_directory: second\n", "third_party:\n  - name: tool\n  - name: tool\n"} {
		t.Run(body, func(t *testing.T) {
			root := runtimeFixture(t)
			writeWorkspace(t, root, body)
			if _, err := readWorkspaceConfig(""); err == nil {
				t.Fatal("invalid workspace accepted")
			}
		})
	}
}
