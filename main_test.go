package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The test executable exercises main's process exit without building a second
// binary or running any rule, installer, or network operation.
func TestMainProcess(t *testing.T) {
	if os.Getenv("KAMAJI_ENTRYPOINT_HELPER") == "1" {
		for i, arg := range os.Args {
			if arg == "--" {
				os.Args = append([]string{"kamaji"}, os.Args[i+1:]...)
				break
			}
		}
		main()
		os.Exit(0)
	}
	for _, tc := range []struct {
		name    string
		args    []string
		status  int
		message string
	}{
		{"help", []string{"--help"}, 0, "Usage:"},
		{"missing target", nil, 1, "provide a build target"},
		{"unknown flag", []string{"--not-a-flag"}, 1, "unknown flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestMainProcess$", "--"}, tc.args...)
			child := exec.Command(os.Args[0], args...)
			child.Dir = t.TempDir()
			child.Env = append(os.Environ(), "KAMAJI_ENTRYPOINT_HELPER=1")
			out, err := child.CombinedOutput()
			status := 0
			if err != nil {
				exit, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				status = exit.ExitCode()
			}
			if status != tc.status || !strings.Contains(string(out), tc.message) {
				t.Fatalf("exit=%d output=%q; want exit=%d containing %q", status, out, tc.status, tc.message)
			}
		})
	}
}
