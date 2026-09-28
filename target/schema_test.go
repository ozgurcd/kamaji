package target

import (
	"path/filepath"
	"testing"
)

func loadExpectedVariables(path string) (map[string]any, error) {
	schema, err := readSchema(path)
	return schema.Variables, err
}

func TestExtendedSchema(t *testing.T) {
	for _, tc := range []struct {
		name, schema string
		values       map[string]any
		fail         bool
	}{
		{"description", "description: Example\nvariables: {name: {type: string, description: Display name}}", map[string]any{"name": "ok"}, false},
		{"enum accepted", "variables: {mode: {type: string, enum: [fast, safe]}}", map[string]any{"mode": "safe"}, false},
		{"enum rejected", "variables: {mode: {type: string, enum: [fast, safe]}}", map[string]any{"mode": "other"}, true},
		{"lower bound", "variables: {count: {type: int, minimum: 2, maximum: 5}}", map[string]any{"count": 1}, true},
		{"upper bound", "variables: {count: {type: int, minimum: 2, maximum: 5}}", map[string]any{"count": 6}, true},
		{"bounds inclusive", "variables: {count: {type: int, minimum: 2, maximum: 5}}", map[string]any{"count": 2}, false},
		{"inverted bounds", "variables: {count: {type: int, minimum: 5, maximum: 2}}", map[string]any{}, true},
		{"structured", "variables: {options: map, names: list, ratio: number}", map[string]any{"options": map[string]any{"nested": true}, "names": []any{"a"}, "ratio": 1.5}, false},
		{"strict", "allow_unknown: false\nvariables: {name: string}", map[string]any{"naem": "typo"}, true},
		{"compatible", "variables: {name: string}", map[string]any{"extra": "allowed"}, false},
		{"invalid default", "variables: {count: {type: int, minimum: 2, default: 1}}", map[string]any{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targetFixture(t)
			put(t, filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "demo/rule_definition.yaml"), []byte(tc.schema))
			err := ValidateTargetVariables(tc.values)
			if (err != nil) != tc.fail {
				t.Fatalf("error=%v; want failure=%v", err, tc.fail)
			}
		})
	}
}
