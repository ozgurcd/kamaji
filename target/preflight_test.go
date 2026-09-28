package target

import (
	"strings"
	"testing"

	"kamaji/obj"
	"kamaji/rt"
)

func TestDependencyURLPreflight(t *testing.T) {
	manager := Manager{Runtime: &rt.Runtime{Config: obj.RuntimeConfig{Platform: "fixture"}}}
	for _, tc := range []struct {
		url string
		bad bool
	}{{"https://fixture.invalid/tool", false}, {"", true}, {"file:///tmp/tool", true}, {"https://", true}, {"https://fixture.invalid/\nPUBLIC_MARKER", true}} {
		entry := obj.ThirdPartyConfig{Name: "tool", FilePath: "tool", SHA256s: map[string]string{"fixture": strings.Repeat("0", 64)}, URLs: map[string]string{"fixture": tc.url}}
		err := manager.ValidateDependencies(obj.WorkspaceConfig{ThirdParty: []obj.ThirdPartyConfig{entry}}, obj.ExecTarget{Config: map[string]any{"tool": "@@tool"}})
		if (err != nil) != tc.bad {
			t.Fatalf("unexpected URL validation result; want error=%v", tc.bad)
		}
		if err != nil && strings.Contains(err.Error(), "PUBLIC_MARKER") {
			t.Fatal("preflight disclosed URL data")
		}
	}
}
