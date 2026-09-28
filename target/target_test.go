package target

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"kamaji/execroot"
	"kamaji/obj"
)

func put(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

// Independent processes deliberately publish the same digest concurrently.
// Each uses a different alias and verifies its own private extraction result.
// HTTP is replaced by a local transport in every process; no sockets are used.
func TestCacheConcurrentProcesses(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, name := range []string{"bin/alpha", "bin/beta"} {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("fixture payload " + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	payload := archive.Bytes()
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	if worker := os.Getenv("KAMAJI_TEST_CACHE_WORKER"); worker != "" {
		base := os.Getenv("KAMAJI_TEST_CACHE_ROOT")
		if !filepath.IsAbs(base) {
			t.Fatal("worker requires an absolute fixture root")
		}
		testRuntime.Config = obj.RuntimeConfig{CacheDir: filepath.Join(base, "cache"), TmpDir: filepath.Join(base, "worker-"+worker), Platform: "test_platform", ThirdPartyFiles: map[string]obj.ThirdPartyFileInfo{}, ThirdPartyFinalPaths: map[string]string{}}
		downloadClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload))}, nil
		})}
		name := "bin/alpha"
		if worker == "1" || worker == "3" {
			name = "bin/beta"
		}
		entry := obj.ThirdPartyConfig{Name: "tool", FilePath: name, SHA256s: map[string]string{"test_platform": digest}, URLs: map[string]string{"test_platform": "https://fixture.invalid/tool"}}
		for i := 0; i < 20; i++ {
			if err := downloadAndCacheFile(entry); err != nil {
				t.Fatal(err)
			}
			if err := (&execroot.Preparer{Runtime: &testRuntime}).CreateExecRootDir(obj.ExecTarget{Name: "fixture"}); err != nil {
				t.Fatal(err)
			}
			if err := (&execroot.Preparer{Runtime: &testRuntime}).CopyThirdPartyIntoExecRootDir(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(testRuntime.Config.ThirdPartyFinalPaths["tool"])
			if err != nil || !bytes.Equal(data, []byte("fixture payload "+name)) {
				t.Fatal("shared cache corrupted extraction")
			}
			if !strings.HasPrefix(testRuntime.Config.ThirdPartyFinalPaths["tool"], testRuntime.Config.ExecRootDir+string(filepath.Separator)) {
				t.Fatal("artifact escaped its private execution directory")
			}
			if err := os.RemoveAll(testRuntime.Config.ExecRootDir); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	base := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	type child struct {
		command *exec.Cmd
		output  bytes.Buffer
	}
	children := make([]*child, 0, 4)
	for i := 0; i < 4; i++ {
		process := &child{command: exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCacheConcurrentProcesses$")}
		process.command.Env = append(os.Environ(), fmt.Sprintf("KAMAJI_TEST_CACHE_WORKER=%d", i), "KAMAJI_TEST_CACHE_ROOT="+base)
		process.command.Stdout = &process.output
		process.command.Stderr = &process.output
		if err := process.command.Start(); err != nil {
			cancel()
			for _, started := range children {
				_ = started.command.Wait()
			}
			t.Fatal(err)
		}
		children = append(children, process)
	}
	for _, process := range children {
		if err := process.command.Wait(); err != nil {
			t.Errorf("cache worker failed: %v\n%s", err, process.output.String())
		}
	}
	data, err := os.ReadFile(filepath.Join(base, "cache", digest, "file"))
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatal("published cache payload is incomplete")
	}
	entries, err := os.ReadDir(filepath.Join(base, "cache", digest))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "file" && entry.Name() != "metadata" {
			t.Errorf("shared cache contains scratch artifact: %s", entry.Name())
		}
	}
}

func targetFixture(t *testing.T) string {
	t.Helper()
	old := testRuntime.Config
	t.Cleanup(func() { testRuntime.Config = old })
	dir := t.TempDir()
	testRuntime.Config = obj.RuntimeConfig{CacheDir: filepath.Join(dir, "cache"), Platform: "test_platform", ThirdPartyFiles: map[string]obj.ThirdPartyFileInfo{}}
	testRuntime.Config.WorkspaceConfig.RulesDir = filepath.Join(dir, "rules")
	testRuntime.Config.ExecTarget.Rule = "demo/run.py"
	return dir
}

func TestParseBuildFile(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    bool
	}{
		{"valid", "targets:\n- name: demo\n  rule: demo/run.py\n  config: {value: value, enabled: true, count: 2}\n", false},
		{"not found", "targets: []\n", true},
		{"malformed", "targets: [", true},
		{"empty", "", true},
		{"duplicate", "targets:\n- {name: demo, rule: one.py}\n- {name: demo, rule: two.py}\n", true},
		{"missing rule", "targets:\n- {name: demo}\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "BUILD.yaml")
			put(t, p, []byte(tc.body))
			got, err := ParseBuildFile(p, "demo")
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v; want error=%v", err, tc.wantErr)
			}
			if !tc.wantErr && (got.Name != "demo" || got.Rule != "demo/run.py" || got.Config["count"] != 2) {
				t.Fatalf("target = %#v", got)
			}
		})
	}
	if _, err := ParseBuildFile(filepath.Join(t.TempDir(), "missing"), "demo"); err == nil {
		t.Fatal("missing build file accepted")
	}
}

func TestYAMLBooleanAndStringScalars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "BUILD.yaml")
	put(t, path, []byte("targets:\n- name: demo\n  rule: demo/run.py\n  config:\n    enabled: true\n    disabled: false\n    answer: yes\n    switch: on\n    count: 2\n"))
	got, err := ParseBuildFile(path, "demo")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"enabled": true, "disabled": false, "answer": "yes", "switch": "on", "count": 2}
	if !reflect.DeepEqual(got.Config, want) {
		t.Fatal("YAML boolean/string scalar semantics differ from the documented contract")
	}
}

func TestLoadExpectedVariables(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    bool
	}{
		{"types", "variables: {value: string}", false},
		{"definitions", "variables: {value: {type: string, mandatory: true}}", false},
		{"missing section", "other: value", true},
		{"wrong section", "variables: [one, two]", true},
		{"non-string key", "variables: {1: string}", true},
		{"malformed", "variables: [", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "definition.yaml")
			put(t, p, []byte(tc.body))
			got, err := loadExpectedVariables(p)
			if (err != nil) != tc.wantErr {
				t.Fatalf("schema error = %v", err)
			}
			if !tc.wantErr && len(got) != 1 {
				t.Fatalf("schema = %#v", got)
			}
		})
	}
	if _, err := loadExpectedVariables(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing schema accepted")
	}
}

func TestValidateTargetVariables(t *testing.T) {
	for _, tc := range []struct {
		name, definition string
		values           map[string]any
		wantErr          bool
		want             map[string]any
	}{
		{"valid", "variables: {name: {type: string, mandatory: true}}", map[string]any{"name": "demo"}, false, nil},
		{"required missing", "variables: {name: {type: string, mandatory: true}}", map[string]any{}, true, nil},
		{"optional missing", "variables: {name: {type: string}}", map[string]any{}, false, nil},
		{"default applied", "variables: {count: {type: int, default: 3}}", map[string]any{}, false, map[string]any{"count": 3}},
		{"default not overriding", "variables: {count: {type: int, default: 3}}", map[string]any{"count": 7}, false, map[string]any{"count": 7}},
		{"wrong type", "variables: {count: int}", map[string]any{"count": "wrong"}, true, nil},
		{"invalid definition", "variables: {name: {type: [bad]}}", map[string]any{}, true, nil},
		{"missing type", "variables: {name: {mandatory: true}}", map[string]any{}, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targetFixture(t)
			p := filepath.Join(testRuntime.Config.WorkspaceConfig.RulesDir, "demo/rule_definition.yaml")
			put(t, p, []byte(tc.definition))
			err := ValidateTargetVariables(tc.values)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validation error = %v; want error=%v", err, tc.wantErr)
			}
			if tc.want != nil && !reflect.DeepEqual(tc.values, tc.want) {
				t.Fatalf("values = %#v; want %#v", tc.values, tc.want)
			}
		})
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type observedBody struct {
	io.Reader
	closed bool
}

func (b *observedBody) Close() error { b.closed = true; return nil }

func installTransport(t *testing.T, f transportFunc) {
	t.Helper()
	old := downloadClient
	t.Cleanup(func() { downloadClient = old })
	downloadClient = &http.Client{Transport: f}
}

func TestDownloadFile(t *testing.T) {
	for _, tc := range []struct {
		name         string
		status       int
		transportErr bool
	}{
		{"success", 200, false}, {"HTTP failure", 404, false}, {"transport failure", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &observedBody{Reader: strings.NewReader("payload")}
			installTransport(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "fixture.invalid" {
					t.Fatal("unexpected request host")
				}
				if tc.transportErr {
					return nil, errors.New("offline fixture failure")
				}
				return &http.Response{StatusCode: tc.status, Status: http.StatusText(tc.status), Body: body, Header: make(http.Header)}, nil
			})
			p := filepath.Join(t.TempDir(), "file")
			err := downloadFile("https://fixture.invalid/tool", p)
			if (err != nil) != (tc.transportErr || tc.status != 200) {
				t.Fatalf("download error = %v", err)
			}
			if !tc.transportErr && !body.closed {
				t.Error("response body not closed")
			}
			if err == nil {
				b, e := os.ReadFile(p)
				if e != nil || string(b) != "payload" {
					t.Fatalf("download content = %q, %v", b, e)
				}
			}
		})
	}
}

func artifact(t *testing.T) ([]byte, obj.ThirdPartyConfig) {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, err := w.Create("tool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return data, obj.ThirdPartyConfig{Name: "tool", FilePath: "tool", SHA256s: map[string]string{testRuntime.Config.Platform: hash}, URLs: map[string]string{testRuntime.Config.Platform: "https://fixture.invalid/tool"}}
}

func TestThirdPartyDownloadAndCache(t *testing.T) {
	for _, initial := range []string{"absent", "partial", "missing metadata", "valid"} {
		t.Run(initial, func(t *testing.T) {
			targetFixture(t)
			data, tp := artifact(t)
			testRuntime.Config.WorkspaceConfig.ThirdParty = []obj.ThirdPartyConfig{tp}
			cache := filepath.Join(testRuntime.Config.CacheDir, tp.SHA256s[testRuntime.Config.Platform])
			if initial != "absent" {
				content := data
				if initial == "partial" {
					content = []byte("incomplete")
				}
				put(t, filepath.Join(cache, "file"), content)
			}
			if initial == "valid" {
				put(t, filepath.Join(cache, "metadata"), []byte("tool,application/zip"))
			}
			requests := 0
			installTransport(t, func(*http.Request) (*http.Response, error) {
				requests++
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
			})
			if err := InitThirdPartyUsedInTarget(testRuntime.Config.WorkspaceConfig, obj.ExecTarget{Config: map[string]any{"executable": "@@tool", "unchanged": true}}); err != nil {
				t.Fatal(err)
			}
			if initial == "valid" && requests != 0 {
				t.Fatal("valid cache triggered download")
			}
			if _, err := os.Stat(filepath.Join(cache, "metadata")); err != nil {
				t.Fatal("metadata not recovered")
			}
			if got := testRuntime.Config.ThirdPartyFiles["tool"]; got.FileName != cache || got.FinalName != "tool" {
				t.Fatalf("resolved cache entry = %#v", got)
			}
			if err := validateCachedFile(tp); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestThirdPartyFailures(t *testing.T) {
	targetFixture(t)
	if _, err := findThirdPartyConfig(obj.WorkspaceConfig{}, "missing"); err == nil {
		t.Fatal("unknown dependency accepted")
	}
	if err := InitThirdPartyUsedInTarget(obj.WorkspaceConfig{}, obj.ExecTarget{Config: map[string]any{"executable": "@@missing"}}); err == nil {
		t.Fatal("unknown dependency accepted")
	}
	_, tp := artifact(t)
	installTransport(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("wrong digest")), Header: make(http.Header)}, nil
	})
	if err := downloadAndCacheFile(tp); err == nil {
		t.Fatal("hash mismatch accepted")
	}
	if _, err := os.Stat(filepath.Join(testRuntime.Config.CacheDir, tp.SHA256s[testRuntime.Config.Platform], "file")); !os.IsNotExist(err) {
		t.Fatal("invalid download published")
	}
	tp.SHA256s[testRuntime.Config.Platform] = "../escape"
	if err := downloadAndCacheFile(tp); err == nil {
		t.Fatal("invalid checksum path accepted")
	}
	tp.SHA256s = map[string]string{}
	if err := downloadAndCacheFile(tp); err == nil {
		t.Fatal("missing platform checksum accepted")
	}
}

type brokenReader struct{}

func TestDownloadResourceLimits(t *testing.T) {
	for _, scenario := range []string{"declared oversized", "stream oversized", "exact boundary", "under boundary", "invalid config"} {
		t.Run(scenario, func(t *testing.T) {
			targetFixture(t)
			testRuntime.Config.WorkspaceConfig.Limits.MaxDownloadBytes = 4
			payload := "12345"
			length := int64(-1)
			switch scenario {
			case "declared oversized":
				length = 5
			case "exact boundary":
				payload = "1234"
			case "under boundary":
				payload = "123"
			case "invalid config":
				testRuntime.Config.WorkspaceConfig.Limits.MaxDownloadBytes = -1
			}
			body := &observedBody{Reader: strings.NewReader(payload)}
			installTransport(t, func(*http.Request) (*http.Response, error) {
				if scenario == "invalid config" {
					t.Fatal("invalid limit made HTTP request")
				}
				return &http.Response{StatusCode: 200, ContentLength: length, Body: body}, nil
			})
			path := filepath.Join(t.TempDir(), "file")
			err := downloadFile("https://fixture.invalid/tool", path)
			wantErr := scenario != "exact boundary" && scenario != "under boundary"
			if (err != nil) != wantErr {
				t.Fatalf("error=%v expectedError=%v", err, wantErr)
			}
			if scenario != "invalid config" && !body.closed {
				t.Fatal("body not closed")
			}
			if info, statErr := os.Stat(path); statErr == nil && info.Size() > 4 {
				t.Fatal("download exceeded byte limit on disk")
			}
		})
	}
}

func TestCacheRespectsDownloadLimit(t *testing.T) {
	targetFixture(t)
	data, entry := artifact(t)
	put(t, filepath.Join(testRuntime.Config.CacheDir, entry.SHA256s[testRuntime.Config.Platform], "file"), data)
	testRuntime.Config.WorkspaceConfig.Limits.MaxDownloadBytes = int64(len(data) - 1)
	if err := validateCachedFile(entry); err == nil {
		t.Fatal("oversized cached artifact accepted")
	}
}

func TestFailedDownloadPreservesVerifiedCache(t *testing.T) {
	targetFixture(t)
	data, entry := artifact(t)
	path := filepath.Join(testRuntime.Config.CacheDir, entry.SHA256s[testRuntime.Config.Platform], "file")
	put(t, path, data)
	installTransport(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("invalid replacement"))}, nil
	})
	if err := downloadAndCacheFile(entry); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("failed writer replaced a verified artifact")
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "download-") {
			t.Fatal("failed download left temporary data")
		}
	}
}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("interrupted body") }

func TestDownloadFailuresCloseBody(t *testing.T) {
	for _, scenario := range []string{"body read failure", "destination failure"} {
		t.Run(scenario, func(t *testing.T) {
			targetFixture(t)
			body := &observedBody{Reader: brokenReader{}}
			installTransport(t, func(*http.Request) (*http.Response, error) { return &http.Response{StatusCode: 200, Body: body}, nil })
			path := filepath.Join(t.TempDir(), "file")
			if scenario == "destination failure" {
				path = t.TempDir()
			}
			if err := downloadFile("https://fixture.invalid/tool", path); err == nil {
				t.Fatal("download error swallowed")
			}
			if !body.closed {
				t.Fatal("response body not closed")
			}
		})
	}
}

func TestCacheValidationAndRejection(t *testing.T) {
	for _, scenario := range []string{"missing URL", "unsafe file path", "invalid digest", "blocked cache", "valid cache", "corrupt cache"} {
		t.Run(scenario, func(t *testing.T) {
			root := targetFixture(t)
			data, entry := artifact(t)
			installTransport(t, func(*http.Request) (*http.Response, error) {
				t.Fatal("invalid cache input made an HTTP request")
				return nil, errors.New("unexpected request")
			})
			switch scenario {
			case "missing URL":
				entry.URLs = nil
			case "unsafe file path":
				entry.FilePath = "../escape"
			case "invalid digest":
				entry.SHA256s[testRuntime.Config.Platform] = "not-a-digest"
			case "blocked cache":
				testRuntime.Config.CacheDir = filepath.Join(root, "blocked")
				put(t, testRuntime.Config.CacheDir, []byte("file"))
			case "valid cache", "corrupt cache":
				if scenario == "corrupt cache" {
					data = []byte("corrupt")
				}
				put(t, filepath.Join(testRuntime.Config.CacheDir, entry.SHA256s[testRuntime.Config.Platform], "file"), data)
			}
			testRuntime.Config.WorkspaceConfig.ThirdParty = []obj.ThirdPartyConfig{entry}
			if scenario == "valid cache" {
				testRuntime.Config.ThirdPartyFiles = nil
				if err := downloadThirdParty(testRuntime.Config.WorkspaceConfig, entry.Name); err != nil {
					t.Fatal(err)
				}
				if testRuntime.Config.ThirdPartyFiles[entry.Name].FinalName != entry.FilePath {
					t.Fatal("cache not registered")
				}
			} else if scenario == "corrupt cache" {
				if err := validateCachedFile(entry); err == nil {
					t.Fatal("corrupt cache accepted")
				}
			} else {
				if err := downloadAndCacheFile(entry); err == nil {
					t.Fatal("invalid download input accepted")
				}
			}
		})
	}
}
