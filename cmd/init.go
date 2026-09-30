package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

func (a *application) initWorkspace(c *cobra.Command, args []string) (result error) {
	if a.template == "build" {
		return a.initBuildProject(c, args)
	}
	if a.template != "minimal" {
		return fmt.Errorf("unknown template; available: build, minimal")
	}
	root, err := a.runtime.WorkingDirectory()
	if err != nil {
		return err
	}
	if len(args) > 0 {
		if filepath.IsAbs(args[0]) {
			root = args[0]
		} else {
			root = filepath.Join(root, args[0])
		}
	}
	files := map[string]string{
		"kamaji.workspace.yaml":            "rules_directory: //rules\nworkspace_vars:\n  - org_domain: example.com\n",
		"BUILD.yaml":                       "targets:\n  - name: hello\n    description: Print a greeting\n    rule: hello/rule.py\n    config:\n      greeting: Hello from Kamaji\n",
		"rules/hello/rule_definition.yaml": "description: Print a greeting and optional extra arguments\nlanguage: python\nallow_unknown: false\nvariables:\n  greeting:\n    type: string\n    description: Greeting to print\n    default: Hello\n",
		"rules/hello/rule.py":              "import argparse\n\nparser = argparse.ArgumentParser()\nparser.add_argument('--greeting', required=True)\nparser.add_argument('extra', nargs='*')\nargs = parser.parse_args()\nprint(args.greeting)\nfor value in args.extra:\n    print(value)\n",
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	// Preflight every destination before creating any files. Symlinked directories
	// are rejected; a concurrent file creation is also protected by O_EXCL.
	for _, name := range names {
		path := filepath.Join(root, name)
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("refusing to overwrite %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
		for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err == nil && !info.IsDir() {
				return fmt.Errorf("destination parent is not a real directory: %s", parent)
			}
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if parent == root || parent == filepath.Dir(parent) {
				break
			}
		}
	}
	created := []string{}
	defer func() {
		if result != nil {
			for _, path := range created {
				_ = os.Remove(path)
			}
		}
	}()
	for _, name := range names {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		created = append(created, path)
		_, writeErr := f.WriteString(files[name])
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	c.Printf("Workspace created in %s.\nNext: kamaji validate hello, then kamaji run hello from that directory.\n", root)
	return nil
}
