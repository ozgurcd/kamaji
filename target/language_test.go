package target

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuleLanguages(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"", "python"}, {"python", "python"}, {"go", "go"}, {"golang", "go"}, {"ruby", ""},
	} {
		t.Run(test.input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rule_definition.yaml")
			if err := os.WriteFile(path, []byte("language: "+test.input+"\nvariables: {}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			schema, err := readSchema(path)
			if test.want == "" {
				if err == nil {
					t.Fatal("unsupported language accepted")
				}
				return
			}
			if err != nil || schema.Language != test.want {
				t.Fatalf("language = %q, error = %v", schema.Language, err)
			}
		})
	}
}

func TestGenericExecutionSchemas(t *testing.T) {
	for _, test := range []struct {
		name, definition string
		valid            bool
	}{
		{"javascript", "language: javascript\nexecution:\n  mode: interpreter\n  command: [node, --enable-source-maps]\n", true},
		{"rust", "language: rust\nexecution:\n  mode: executable\n", true},
		{"custom-python", "language: python\nexecution:\n  mode: interpreter\n  command: [python3, -I]\n", true},
		{"unknown-mode", "language: anything\nexecution:\n  mode: shell\n", false},
		{"missing-command", "language: anything\nexecution:\n  mode: interpreter\n", false},
		{"empty-program", "language: anything\nexecution:\n  mode: interpreter\n  command: ['', flag]\n", false},
		{"nul-argument", "language: anything\nexecution:\n  mode: interpreter\n  command: [tool, \"bad\\0argument\"]\n", false},
		{"native-command", "language: rust\nexecution:\n  mode: executable\n  command: [tool]\n", false},
		{"typo", "language: javascript\nexecution:\n  mode: interpreter\n  commmand: [node]\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rule_definition.yaml")
			if err := os.WriteFile(path, []byte(test.definition+"variables: {}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := readSchema(path)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}
