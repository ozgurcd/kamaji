package buildsys

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

func privateDir(filename string) error {
	if err := os.Mkdir(filename, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(filename)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("build storage must be a private real directory: %s", filename)
	}
	return nil
}

func (p *Project) storage() (string, error) {
	root := filepath.Join(p.Root, ".kamaji")
	for _, dir := range []string{root, filepath.Join(root, "cache"), filepath.Join(root, "runs")} {
		if err := privateDir(dir); err != nil {
			return "", err
		}
	}
	return root, nil
}

func writeJSON(filename string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(filename), ".record-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filename)
}

func copyArtifact(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	output, err := os.CreateTemp(filepath.Dir(destination), ".artifact-*")
	if err != nil {
		return err
	}
	tmp := output.Name()
	defer os.Remove(tmp)
	_, copyErr := io.Copy(output, input)
	chmodErr := output.Chmod(mode.Perm())
	closeErr := output.Close()
	for _, err := range []error{copyErr, chmodErr, closeErr} {
		if err != nil {
			return err
		}
	}
	return os.Rename(tmp, destination)
}

func copyArtifacts(source, destination string, artifacts []Artifact) error {
	for _, a := range artifacts {
		from, err := safePath(source, a.Path)
		if err != nil {
			return err
		}
		to, err := safePath(destination, a.Path)
		if err != nil {
			return err
		}
		if a.Dir {
			if err := os.MkdirAll(to, 0700); err != nil {
				return err
			}
		} else if err := copyArtifact(from, to, os.FileMode(a.Mode)); err != nil {
			return err
		}
	}
	for i := len(artifacts) - 1; i >= 0; i-- {
		if a := artifacts[i]; a.Dir {
			if err := os.Chmod(filepath.Join(destination, filepath.FromSlash(a.Path)), os.FileMode(a.Mode).Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Project) saveCache(t Target, key string, artifacts []Artifact) error {
	parent := filepath.Join(p.Root, ".kamaji/cache")
	tmp, err := os.MkdirTemp(parent, ".pending-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	tree := filepath.Join(tmp, "tree")
	if err := os.Mkdir(tree, 0700); err != nil {
		return err
	}
	if err := copyArtifacts(p.Root, tree, artifacts); err != nil {
		return err
	}
	copyProject := &Project{Root: tree}
	verified, err := copyProject.snapshot(t.Outputs, nil)
	if err != nil || !reflect.DeepEqual(verified, artifacts) {
		return fmt.Errorf("outputs changed while saving cache for %s", t.Name)
	}
	if err := writeJSON(filepath.Join(tmp, "artifacts.json"), artifacts); err != nil {
		return err
	}
	destination := filepath.Join(parent, key)
	// The workspace lease serializes builds. Replacing an invalid cache entry
	// cannot race another Kamaji writer; RemoveAll never follows a symlink.
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	return os.Rename(tmp, destination)
}

// cached verifies payloads on every reuse. Corrupt entries are misses, never
// trusted artifacts. Restoration is confined to this target's declared outputs.
func (p *Project) cached(t Target, key string, restore bool) ([]Artifact, bool, error) {
	root := filepath.Join(p.Root, ".kamaji/cache", key)
	for _, filename := range []string{filepath.Join(p.Root, ".kamaji"), filepath.Dir(root), root, filepath.Join(root, "tree")} {
		info, err := os.Lstat(filename)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			return nil, false, nil
		}
	}
	meta := filepath.Join(root, "artifacts.json")
	info, err := os.Lstat(meta)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return nil, false, nil
	}
	data, err := os.ReadFile(meta)
	if err != nil {
		return nil, false, nil
	}
	var artifacts []Artifact
	if json.Unmarshal(data, &artifacts) != nil || len(artifacts) == 0 {
		return nil, false, nil
	}
	for _, artifact := range artifacts {
		belongs := false
		for _, output := range t.Outputs {
			belongs = belongs || within(output, artifact.Path)
		}
		if !belongs || validPath(artifact.Path, false) != nil {
			return nil, false, nil
		}
	}
	tree := filepath.Join(root, "tree")
	copyProject := &Project{Root: tree}
	verified, err := copyProject.snapshot(t.Outputs, nil)
	if err != nil || !reflect.DeepEqual(verified, artifacts) {
		return nil, false, nil
	}
	current, currentErr := p.snapshot(t.Outputs, nil)
	if currentErr == nil && reflect.DeepEqual(current, artifacts) {
		return artifacts, true, nil
	}
	if !restore {
		return artifacts, true, nil
	}
	for _, output := range t.Outputs {
		filename, err := safePath(p.Root, output)
		if err != nil {
			return nil, false, err
		}
		if err := os.RemoveAll(filename); err != nil {
			return nil, false, err
		}
	}
	if err := copyArtifacts(tree, p.Root, artifacts); err != nil {
		return nil, false, err
	}
	return artifacts, true, nil
}

// ReadRun accepts only generated hex identifiers, never arbitrary paths.
func (p *Project) ReadRun(id string) ([]byte, error) {
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" {
		return nil, fmt.Errorf("invalid run identifier")
	}
	for _, dir := range []string{filepath.Join(p.Root, ".kamaji"), filepath.Join(p.Root, ".kamaji/runs")} {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("run storage is missing or unsafe")
		}
	}
	filename := filepath.Join(p.Root, ".kamaji/runs", id+".json")
	info, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return nil, fmt.Errorf("run record is missing or invalid")
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var result Result
	if json.Unmarshal(data, &result) != nil || result.Schema != "kamaji.result.v1" || result.ID != id {
		return nil, fmt.Errorf("run record is corrupt or has an unsupported schema")
	}
	return data, nil
}
