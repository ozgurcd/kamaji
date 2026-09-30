// Package buildsys implements local artifact builds with explicit dependencies.
package buildsys

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"kamaji/config"
)

const SchemaVersion = 1

var ErrNoProject = errors.New("no kamaji.toml found; create one with kamaji init --template build")

// Target is either an argv action or a dependency-only aggregate. Cache is opt-in.
type Target struct {
	Name        string            `toml:"name" yaml:"name" json:"name"`
	Description string            `toml:"description" yaml:"description" json:"description,omitempty"`
	Command     []string          `toml:"command" yaml:"command" json:"command,omitempty"`
	Deps        []string          `toml:"deps" yaml:"deps" json:"deps,omitempty"`
	Inputs      []string          `toml:"inputs" yaml:"inputs" json:"inputs,omitempty"`
	Outputs     []string          `toml:"outputs" yaml:"outputs" json:"outputs,omitempty"`
	Env         map[string]string `toml:"env" yaml:"env" json:"env,omitempty"`
	PassEnv     []string          `toml:"pass_env" yaml:"pass_env" json:"pass_env,omitempty"`
	Tools       []string          `toml:"tools" yaml:"tools" json:"tools,omitempty"`
	Cache       bool              `toml:"cache" yaml:"cache" json:"cache"`
	Timeout     string            `toml:"timeout" yaml:"timeout" json:"timeout,omitempty"`
	Effect      string            `toml:"effect" yaml:"effect" json:"effect"`
	Slots       int               `toml:"slots" yaml:"slots" json:"slots"`
}

type Document struct {
	Version int      `toml:"version" yaml:"version" json:"version"`
	Default []string `toml:"default" yaml:"default" json:"default,omitempty"`
	Targets []Target `toml:"targets" yaml:"targets" json:"targets"`
}

type Project struct {
	Document
	Root string
	File string
}

// Load discovers one build document, using its directory as the project root.
// Explicit filenames are relative to cwd. Discovery never writes project state.
func Load(cwd, filename string) (*Project, error) {
	root, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	if filename != "" {
		if !filepath.IsAbs(filename) {
			filename = filepath.Join(root, filename)
		}
	} else {
		for {
			var found []string
			for _, name := range []string{"kamaji.toml", "kamaji.yaml", "kamaji.yml"} {
				candidate := filepath.Join(root, name)
				if _, err := os.Lstat(candidate); err == nil {
					found = append(found, candidate)
				} else if !os.IsNotExist(err) {
					return nil, err
				}
			}
			if len(found) > 1 {
				return nil, fmt.Errorf("multiple build documents in %s; select one with --file", root)
			}
			if len(found) == 1 {
				filename = found[0]
				break
			}
			parent := filepath.Dir(root)
			if root == parent {
				return nil, ErrNoProject
			}
			root = parent
		}
	}
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return nil, fmt.Errorf("build document must be a regular file of at most 4 MiB")
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (4<<20)+1))
	if err != nil || len(data) > 4<<20 {
		return nil, fmt.Errorf("could not read bounded build document")
	}
	p, err := Parse(filename, data)
	if err != nil {
		return nil, err
	}
	p.Root = filepath.Dir(filename)
	p.File = filename
	// Absolute executables can only be classified as project-local once the
	// document's root is known. Parse also validates root-independent rules.
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func Parse(filename string, data []byte) (*Project, error) {
	var doc Document
	var err error
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".toml":
		err = toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&doc)
	case ".yaml", ".yml":
		err = config.DecodeYAML(bytes.NewReader(data), &doc)
	default:
		return nil, fmt.Errorf("build document must use .toml, .yaml, or .yml")
	}
	if err != nil {
		// Parser diagnostics may contain configuration values. Keep them private.
		return nil, fmt.Errorf("invalid build document: check syntax, field names, and value types")
	}
	p := &Project{Document: doc}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

var targetName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]*$`)
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (p *Project) validate() error {
	if p.Version != SchemaVersion || len(p.Targets) == 0 {
		return fmt.Errorf("build document requires version = 1 and at least one target")
	}
	seen := map[string]bool{}
	for i := range p.Targets {
		t := &p.Targets[i]
		if !targetName.MatchString(t.Name) || seen[t.Name] {
			return fmt.Errorf("invalid or duplicate target name %q", t.Name)
		}
		seen[t.Name] = true
		if t.Slots == 0 {
			t.Slots = 1
		}
		if t.Slots < 1 {
			return fmt.Errorf("target %s: slots must be positive", t.Name)
		}
		if t.Effect == "" {
			t.Effect = "build"
		}
		if t.Effect != "build" && t.Effect != "external" {
			return fmt.Errorf("target %s: effect must be build or external", t.Name)
		}
		if t.Cache && (len(t.Outputs) == 0 || t.Effect == "external") {
			return fmt.Errorf("target %s: caching requires outputs and a build effect", t.Name)
		}
		if len(t.Command) == 0 && (len(t.Deps) == 0 || len(t.Outputs) != 0 || t.Cache || t.Effect != "build") {
			return fmt.Errorf("target %s: specify command or dependency-only aggregate", t.Name)
		}
		for j, arg := range t.Command {
			if strings.ContainsRune(arg, 0) || j == 0 && arg == "" {
				return fmt.Errorf("target %s: invalid command argument", t.Name)
			}
		}
		if t.Timeout != "" {
			if duration, err := time.ParseDuration(t.Timeout); err != nil || duration <= 0 {
				return fmt.Errorf("target %s: timeout must be a positive duration", t.Name)
			}
		}
		for key, value := range t.Env {
			if !envName.MatchString(key) || strings.ContainsRune(value, 0) {
				return fmt.Errorf("target %s: invalid environment entry", t.Name)
			}
		}
		for _, key := range t.PassEnv {
			if !envName.MatchString(key) {
				return fmt.Errorf("target %s: invalid pass_env entry", t.Name)
			}
		}
		for _, input := range t.Inputs {
			if err := validPath(input, true); err != nil {
				return fmt.Errorf("target %s input: %w", t.Name, err)
			}
		}
		for _, output := range t.Outputs {
			if err := validPath(output, false); err != nil {
				return fmt.Errorf("target %s output: %w", t.Name, err)
			}
			for _, input := range t.Inputs {
				if intersectsTree(input, output) {
					return fmt.Errorf("target %s: input and output paths overlap", t.Name)
				}
			}
		}
	}
	// Selecting all targets checks unknown dependencies and cycles, even in
	// targets outside the default build. Validate the defaults separately.
	all := make([]string, 0, len(p.Targets))
	for _, t := range p.Targets {
		all = append(all, t.Name)
	}
	if _, err := p.Select(all); err != nil {
		return err
	}
	if _, err := p.Select(p.Default); err != nil {
		return err
	}
	type owner struct{ name, path string }
	var outputs []owner
	for _, t := range p.Targets {
		for _, output := range t.Outputs {
			for _, old := range outputs {
				if within(old.path, output) || within(output, old.path) {
					return fmt.Errorf("overlapping outputs from %s and %s", old.name, t.Name)
				}
			}
			outputs = append(outputs, owner{t.Name, output})
		}
	}
	for _, t := range p.Targets {
		order, _ := p.Select([]string{t.Name})
		ancestors := map[string]bool{}
		for _, name := range order {
			ancestors[name] = true
		}
		programs := append([]string(nil), t.Tools...)
		if len(t.Command) > 0 {
			programs = append(programs, t.Command[0])
		}
		consumed := append([]string(nil), t.Inputs...)
		for _, program := range programs {
			if relative, local := localProgramPath(p.Root, program); local {
				consumed = append(consumed, relative)
			}
		}
		for _, output := range outputs {
			for _, input := range consumed {
				if !intersectsTree(input, output.path) {
					continue
				}
				if output.name == t.Name {
					return fmt.Errorf("target %s: input or executable and output paths overlap", t.Name)
				}
				if !ancestors[output.name] {
					return fmt.Errorf("target %s consumes output of %s without a dependency", t.Name, output.name)
				}
			}
			for _, program := range programs {
				if program == output.path && !strings.Contains(program, "/") {
					return fmt.Errorf("target %s: use ./%s for a project-root executable", t.Name, output.path)
				}
			}
		}
	}
	return nil
}

func validPath(value string, pattern bool) error {
	if value == "" || value == "." || strings.ContainsAny(value, "\\\x00") || strings.HasPrefix(value, "/") || path.Clean(value) != value || value == ".." || strings.HasPrefix(value, "../") {
		return fmt.Errorf("paths must be normalized project-relative paths")
	}
	first := strings.Split(value, "/")[0]
	if reserved(first) {
		return fmt.Errorf("path uses reserved project storage")
	}
	if !pattern && strings.ContainsAny(value, "*?[") {
		return fmt.Errorf("output paths cannot contain wildcards")
	}
	if _, err := path.Match(value, ""); err != nil {
		return fmt.Errorf("invalid path pattern")
	}
	return nil
}

func reserved(name string) bool {
	return name == ".git" || name == ".kamaji" || name == ".audit" || name == ".gograph"
}

func within(parent, child string) bool {
	return child == parent || strings.HasPrefix(child, parent+"/")
}

// covers supports recursive ** segments; literal directories include descendants.
// intersectsTree reports whether an input can consume the root or any of its
// descendants. Outputs may be directories even before they exist. Matching only
// the root itself would miss patterns such as **/*.txt against generated/.
func intersectsTree(pattern, root string) bool {
	if !strings.ContainsAny(pattern, "*?[") {
		return within(pattern, root) || within(root, pattern)
	}
	parts, names := strings.Split(pattern, "/"), strings.Split(root, "/")
	type position struct{ pattern, name int }
	visited := map[position]bool{}
	var match func(int, int) bool
	match = func(i, j int) bool {
		if j == len(names) {
			return true // The remaining valid pattern can match descendants.
		}
		if i == len(parts) || visited[position{i, j}] {
			return false
		}
		visited[position{i, j}] = true
		if parts[i] == "**" {
			return match(i+1, j) || match(i, j+1)
		}
		ok, _ := path.Match(parts[i], names[j])
		return ok && match(i+1, j+1)
	}
	return match(0, 0)
}

func covers(pattern, name string) bool {
	if !strings.ContainsAny(pattern, "*?[") {
		return within(pattern, name)
	}
	var match func([]string, []string) bool
	match = func(p, n []string) bool {
		if len(p) == 0 {
			return len(n) == 0
		}
		if p[0] == "**" {
			return match(p[1:], n) || len(n) > 0 && match(p, n[1:])
		}
		if len(n) == 0 {
			return false
		}
		ok, _ := path.Match(p[0], n[0])
		return ok && match(p[1:], n[1:])
	}
	return match(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func (p *Project) Index() map[string]Target {
	index := make(map[string]Target, len(p.Targets))
	for _, t := range p.Targets {
		index[t.Name] = t
	}
	return index
}

// Select returns a deterministic topological order of the requested closure.
func (p *Project) Select(names []string) ([]string, error) {
	index := p.Index()
	if len(names) == 0 {
		names = p.Default
	}
	if len(names) == 0 {
		for name := range index {
			names = append(names, name)
		}
	}
	names = append([]string(nil), names...)
	sort.Strings(names)
	var order []string
	state := map[string]int{}
	var visit func(string) error
	visit = func(name string) error {
		t, ok := index[name]
		if !ok {
			return fmt.Errorf("unknown target %q", name)
		}
		if state[name] == 1 {
			return fmt.Errorf("dependency cycle at %s", name)
		}
		if state[name] == 2 {
			return nil
		}
		state[name] = 1
		deps := append([]string(nil), t.Deps...)
		sort.Strings(deps)
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = 2
		order = append(order, name)
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return order, nil
}
