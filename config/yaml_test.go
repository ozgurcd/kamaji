package config

import (
	"strings"
	"testing"
)

func TestStrictYAMLDiagnostics(t *testing.T) {
	for _, body := range []string{"python: [PUBLIC_MARKER]\n", "python: one\npython: PUBLIC_MARKER\n", "pythno: PUBLIC_MARKER\n", "python: first\n---\npython: PUBLIC_MARKER\n"} {
		var config struct {
			Python string `yaml:"python"`
		}
		err := DecodeYAML(strings.NewReader(body), &config)
		if err == nil {
			t.Fatal("invalid YAML accepted")
		}
		if strings.Contains(err.Error(), "PUBLIC_MARKER") {
			t.Fatal("diagnostic included scalar data")
		}
	}
}
