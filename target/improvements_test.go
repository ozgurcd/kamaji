package target

import (
	"bytes"
	"errors"
	"io"
	"kamaji/obj"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestDownloadErrorRedaction(t *testing.T) {
	for _, scenario := range []string{"transport", "redirect", "body"} {
		t.Run(scenario, func(t *testing.T) {
			root := targetFixture(t)
			installTransport(t, transportFunc(func(*http.Request) (*http.Response, error) {
				switch scenario {
				case "redirect":
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://fixture.invalid/next?query=PUBLIC_MARKER"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
				case "body":
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(iotest.ErrReader(errors.New("body PUBLIC_MARKER")))}, nil
				default:
					return nil, errors.New("transport PUBLIC_MARKER")
				}
			}))
			downloadClient.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("redirect PUBLIC_MARKER") }
			err := downloadFile("https://fixture.invalid/tool?query=PUBLIC_MARKER", filepath.Join(root, "file"))
			if err == nil || strings.Contains(err.Error(), "PUBLIC_MARKER") {
				t.Fatal("download error contains supplied data")
			}
		})
	}
}

func TestDuplicateDependencyRejected(t *testing.T) {
	workspace := obj.WorkspaceConfig{ThirdParty: []obj.ThirdPartyConfig{{Name: "tool"}, {Name: "tool"}}}
	if _, err := findThirdPartyConfig(workspace, "tool"); err == nil {
		t.Fatal("duplicate dependency accepted")
	}
}

func TestRuleSchemaLanguageMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rule_definition.yaml")
	if err := os.WriteFile(path, []byte("language: python\nvariables: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadExpectedVariables(path); err != nil {
		t.Fatal(err)
	}
}

func TestCachedMetadataNotReplaced(t *testing.T) {
	targetFixture(t)
	data, entry := artifact(t)
	downloads := 0
	installTransport(t, transportFunc(func(*http.Request) (*http.Response, error) {
		downloads++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data))}, nil
	}))
	workspace := obj.WorkspaceConfig{ThirdParty: []obj.ThirdPartyConfig{entry}}
	if err := downloadThirdParty(workspace, entry.Name); err != nil {
		t.Fatal(err)
	}
	dir, err := cachePath(entry)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(filepath.Join(dir, "metadata"))
	if err != nil {
		t.Fatal(err)
	}
	if err := InitThirdPartyUsedInTarget(workspace, obj.ExecTarget{Config: map[string]any{"one": "@@tool", "two": "@@tool", "three": "@@tool"}}); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(filepath.Join(dir, "metadata"))
	if err != nil {
		t.Fatal(err)
	}
	if downloads != 1 || !os.SameFile(before, after) {
		t.Fatal("cache hit downloaded or rewrote unchanged metadata")
	}
}
