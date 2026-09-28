package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kamaji/internal/process"
	"kamaji/target"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"kamaji/execroot"
	"kamaji/obj"
	"kamaji/tools"
	"kamaji/utils"
)

func (scope *Executor) prepareArgs(target obj.ExecTarget) ([]string, error) {
	rule, err := (&tools.Context{Runtime: scope.Runtime}).GetRule(target)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(rule) {
		rule = filepath.Join(scope.Runtime.Config.WorkspaceConfig.RulesDir, rule)
	}
	args := []string{rule}
	keys := make([]string, 0, len(target.Config))
	for key := range target.Config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "" || strings.ContainsAny(key, "=\x00") {
			return nil, fmt.Errorf("invalid option name")
		}
		value := target.Config[key]
		var rendered string
		switch typed := value.(type) {
		case string:
			rendered = typed
			if strings.HasPrefix(typed, "@@") {
				var exists bool
				rendered, exists = scope.Runtime.Config.ThirdPartyFinalPaths[typed[2:]]
				if !exists {
					return nil, fmt.Errorf("third party file not found: %s", typed[2:])
				}
			}
		default:
			// Normalize YAML maps recursively before JSON encoding structured values.
			normalized := tools.NormalizeMap(map[any]any{"value": value})["value"]
			data, err := json.Marshal(normalized)
			if err != nil {
				return nil, fmt.Errorf("encode option %q: %w", key, err)
			}
			rendered = string(data)
		}
		args = append(args, "--"+key+"="+rendered)
	}
	return args, nil
}

func (scope *Executor) Run(workspace obj.WorkspaceConfig, selected obj.ExecTarget, pythonArgs ...string) (result error) {
	ctx := scope.Runtime.ExecutionContext()
	if scope.Runtime.Timeout < 0 {
		return fmt.Errorf("execution timeout cannot be negative")
	}
	if scope.Runtime.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, scope.Runtime.Timeout)
		defer cancel()
	}
	previousContext := scope.Runtime.Context
	scope.Runtime.Context = ctx
	defer func() { scope.Runtime.Context = previousContext }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Preflight(workspace, selected); err != nil {
		return err
	}
	python, err := scope.Interpreter()
	if err != nil {
		return err
	}
	cwd, err := scope.Runtime.WorkingDirectory()
	if err != nil {
		return err
	}
	lease, err := scope.Runtime.RuntimeLease(false)
	if err != nil {
		return err
	}
	defer lease.Close()
	if err := (&target.Manager{Runtime: scope.Runtime}).InitThirdPartyUsedInTarget(workspace, selected); err != nil {
		return err
	}
	if err := (&execroot.Preparer{Runtime: scope.Runtime}).CreateExecRootDir(selected); err != nil {
		return err
	}
	started := false
	defer func() { result = errors.Join(result, scope.Runtime.FinishExecution(started)) }()
	if err := (&execroot.Preparer{Runtime: scope.Runtime}).CopyThirdPartyIntoExecRootDir(); err != nil {
		return err
	}
	args, err := scope.prepareArgs(selected)
	if err != nil {
		return err
	}
	info, err := os.Stat(args[0])
	if err != nil {
		return fmt.Errorf("rule: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("rule must be a regular file")
	}
	command := exec.Command(python, append(args, pythonArgs...)...)
	command.Dir = cwd
	if scope.Runtime.Config.Isolated {
		command.Dir = filepath.Join(scope.Runtime.Config.ExecRootDir, "origin")
		if err := utils.CopyDirectory(cwd, command.Dir); err != nil {
			return fmt.Errorf("isolate workspace: %w", err)
		}
	}
	common := workspace.RulesCommonDir
	if common == "" {
		common = "common"
	}
	pythonPath := common
	if !filepath.IsAbs(pythonPath) {
		pythonPath = filepath.Join(workspace.RulesDir, common)
	}
	// Environ uses command.Dir for PWD; exec.Cmd resolves duplicate variables by
	// keeping the final value, so inherited values cannot override these settings.
	command.Env = append(command.Environ(), "KAMAJI_ORGANIZATION_DOMAIN="+workspace.WorkspaceVars[0].Org_Domain, "PYTHONPATH="+pythonPath)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	run := scope.RunCommand
	if run == nil {
		run = func(c *exec.Cmd) error {
			var err error
			started, err = process.Run(ctx, c, scope.Runtime.KillGrace)
			return err
		}
	} else {
		started = true
	}
	if err := run(command); err != nil {
		return fmt.Errorf("rule execution failed: %w", err)
	}
	return nil
}

func (scope *Executor) ValidateRule(selected obj.ExecTarget) error {
	if selected.Name == "" || selected.Name == "." || selected.Name == ".." || strings.ContainsAny(selected.Name, `/\`) {
		return fmt.Errorf("invalid target name")
	}
	rule, err := (&tools.Context{Runtime: scope.Runtime}).GetRule(selected)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(rule) {
		rule = filepath.Join(scope.Runtime.Config.WorkspaceConfig.RulesDir, rule)
	}
	info, err := os.Stat(rule)
	if err != nil {
		return fmt.Errorf("rule: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("rule must be a regular file")
	}
	return nil
}
