package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveConfigValue(t *testing.T) {
	for _, tc := range []struct{ name, flag, env, yaml, fallback, want string }{
		{"flag wins", "flag", "env", "yaml", "default", "flag"},
		{"environment wins", "", "env", "yaml", "default", "env"},
		{"yaml wins", "", "", "yaml", "default", "yaml"},
		{"default", "", "", "", "default", "default"},
		{"empty default", "", "", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KAMAJI_TEST_CONFIG", tc.env)
			if got := ResolveConfigValue(tc.flag, "KAMAJI_TEST_CONFIG", tc.yaml, tc.fallback); got != tc.want {
				t.Fatalf("value = %q; want %q", got, tc.want)
			}
		})
	}
}

func TestLoadUserConfig(t *testing.T) {
	loader := Loader{}
	for _, tc := range []struct {
		name, content string
		create        bool
		want          map[string]string
	}{
		{"missing", "", false, map[string]string{}},
		{"valid", "python: /fixture/python\n", true, map[string]string{"python": "/fixture/python"}},
		{"empty", "", true, map[string]string{}},
		{"malformed", "python: [unterminated", true, map[string]string{}},
		{"wrong type", "python: [one, two]\n", true, map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			loader.HomeDir = func() (string, error) { return dir, nil }
			if tc.create {
				if err := os.Mkdir(filepath.Join(dir, ".kamaji"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".kamaji/config.yaml"), []byte(tc.content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := loader.Load()
			wantErr := tc.name == "malformed" || tc.name == "wrong type"
			if (err != nil) != wantErr {
				t.Fatalf("error=%v wantError=%v", err, wantErr)
			}
			if !wantErr && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("config = %#v; want %#v", got, tc.want)
			}
		})
	}
	loader.HomeDir = func() (string, error) { return "", errors.New("no home") }
	if _, err := loader.Load(); err == nil {
		t.Fatal("home lookup failure should return an error")
	}
}
