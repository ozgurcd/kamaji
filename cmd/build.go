package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"kamaji/buildsys"
)

func (a *application) graphCommands() []*cobra.Command {
	var commands []*cobra.Command
	for _, verb := range []string{"build", "plan", "affected", "history", "clean"} {
		var filename, expected string
		var jsonOutput, events, effects, noCache bool
		var dryRun, cache, history bool
		var jobs int
		var timeout time.Duration
		command := &cobra.Command{Use: verb + " [targets...]", Args: cobra.ArbitraryArgs}
		switch verb {
		case "build":
			command.Short = "Build a TOML dependency graph with verified local caching"
		case "plan":
			command.Short = "Inspect a build plan without running commands or writing state"
		case "affected":
			command.Use = "affected <project-relative paths...>"
			command.Args = cobra.MinimumNArgs(1)
			command.Short = "List targets affected by changed or deleted paths"
		case "history":
			command.Use = "history <run-id>"
			command.Args = cobra.ExactArgs(1)
			command.Short = "Read a structured build evidence record"
		case "clean":
			command.Short = "Remove declared build outputs; optionally clear cache or history"
		}
		command.Flags().StringVarP(&filename, "file", "f", "", "Build document (default: discover kamaji.toml)")
		command.Flags().BoolVar(&jsonOutput, "json", false, "Write a versioned JSON document to stdout")
		if verb == "clean" {
			command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview removals without writing state")
			command.Flags().BoolVar(&cache, "cache", false, "Also remove the project's entire build artifact cache")
			command.Flags().BoolVar(&history, "history", false, "Also remove the project's build evidence records")
		}
		if verb == "build" {
			command.Flags().IntVarP(&jobs, "jobs", "j", runtime.GOMAXPROCS(0), "Parallel action slot budget")
			command.Flags().BoolVar(&effects, "allow-effects", false, "Allow targets declared to have external effects")
			command.Flags().BoolVar(&noCache, "no-cache", false, "Execute actions without reading or writing the artifact cache")
			command.Flags().BoolVar(&events, "events", false, "Stream versioned JSON events followed by the final result")
			command.Flags().StringVar(&expected, "expect-plan", "", "Refuse execution unless the current plan has this ID")
			command.Flags().DurationVar(&timeout, "timeout", 0, "Whole-build deadline (zero disables)")
		}
		if verb == "build" || verb == "plan" || verb == "clean" {
			command.ValidArgsFunction = func(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
				cwd, err := a.runtime.WorkingDirectory()
				if err != nil {
					return nil, cobra.ShellCompDirectiveError
				}
				p, err := buildsys.Load(cwd, filename)
				if err != nil {
					return nil, cobra.ShellCompDirectiveError
				}
				return graphCompletions(p, args, prefix), cobra.ShellCompDirectiveNoFileComp
			}
		}
		command.RunE = func(c *cobra.Command, args []string) error {
			cwd, err := a.runtime.WorkingDirectory()
			if err != nil {
				return err
			}
			p, err := buildsys.Load(cwd, filename)
			if err != nil {
				return graphError(c, jsonOutput || events, err)
			}
			encoder := json.NewEncoder(c.OutOrStdout())
			switch verb {
			case "clean":
				paths, err := p.Clean(args, buildsys.CleanOptions{DryRun: dryRun, Cache: cache, History: history})
				if err != nil {
					return graphError(c, jsonOutput, err)
				}
				if jsonOutput {
					return encoder.Encode(struct {
						Schema string   `json:"schema"`
						DryRun bool     `json:"dry_run"`
						Paths  []string `json:"paths"`
					}{"kamaji.clean.v1", dryRun, paths})
				}
				for _, path := range paths {
					if dryRun {
						c.Printf("would remove %s\n", path)
					} else {
						c.Printf("removed %s\n", path)
					}
				}
				return nil
			case "history":
				data, err := p.ReadRun(args[0])
				if err != nil {
					return graphError(c, jsonOutput, err)
				}
				_, err = c.OutOrStdout().Write(data)
				return err
			case "affected":
				names, err := p.Affected(args)
				if err != nil {
					return graphError(c, jsonOutput, err)
				}
				if jsonOutput {
					return encoder.Encode(struct {
						Schema  string   `json:"schema"`
						Targets []string `json:"targets"`
					}{"kamaji.affected.v1", names})
				}
				for _, name := range names {
					c.Println(name)
				}
				return nil
			case "plan":
				plan, err := p.Plan(args)
				if err != nil {
					return graphError(c, jsonOutput, err)
				}
				if jsonOutput {
					return encoder.Encode(plan)
				}
				c.Printf("Plan %s\n", plan.ID)
				for _, target := range plan.Targets {
					c.Printf("%s: %s (%s); effect=%s\n", target.Name, target.Status, target.Reason, target.Effect)
				}
				return nil
			}
			if timeout < 0 || jsonOutput && events {
				return graphError(c, jsonOutput || events, fmt.Errorf("use a nonnegative timeout and choose either --json or --events"))
			}
			ctx := c.Context()
			if timeout > 0 {
				var cancel func()
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
			output := c.OutOrStdout()
			if jsonOutput || events {
				output = c.ErrOrStderr()
			}
			options := buildsys.Options{Jobs: jobs, AllowEffects: effects, ExpectPlan: expected, NoCache: noCache, Stdout: output, Stderr: c.ErrOrStderr()}
			var eventError error
			if events {
				options.OnEvent = func(event buildsys.Event) {
					if eventError == nil {
						eventError = encoder.Encode(event)
					}
				}
			}
			result, buildErr := p.Build(ctx, args, options)
			if result == nil {
				return graphError(c, jsonOutput || events, buildErr)
			}
			if jsonOutput || events {
				if err := encoder.Encode(result); err != nil {
					return err
				}
			} else {
				for _, target := range result.Targets {
					c.Printf("%s: %s\n", target.Name, target.Status)
				}
				c.Printf("Run %s\n", result.ID)
			}
			if eventError != nil {
				return eventError
			}
			return buildErr
		}
		commands = append(commands, command)
	}
	commands = append(commands, &cobra.Command{Use: "capabilities", Short: "Describe the versioned build automation interface as JSON", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return json.NewEncoder(c.OutOrStdout()).Encode(struct {
			Schema    string   `json:"schema"`
			Version   string   `json:"version"`
			Formats   []string `json:"build_formats"`
			Commands  []string `json:"build_commands"`
			Contracts []string `json:"contracts"`
			Sandbox   bool     `json:"os_sandbox"`
		}{"kamaji.capabilities.v1", Version, []string{"toml", "yaml"}, []string{"build", "plan", "affected", "history", "clean"}, []string{"kamaji.plan.v1", "kamaji.result.v1", "kamaji.event.v1", "kamaji.affected.v1", "kamaji.clean.v1", "kamaji.error.v1"}, false})
	}})
	return commands
}

func graphError(c *cobra.Command, structured bool, err error) error {
	if structured && err != nil {
		if writeErr := json.NewEncoder(c.OutOrStdout()).Encode(struct {
			Schema string `json:"schema"`
			Error  string `json:"error"`
		}{"kamaji.error.v1", err.Error()}); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func graphCompletions(p *buildsys.Project, args []string, prefix string) []string {
	used := map[string]bool{}
	for _, arg := range args {
		used[arg] = true
	}
	result := []string{}
	for _, target := range p.Targets {
		if strings.HasPrefix(target.Name, prefix) && !used[target.Name] {
			name := target.Name
			if target.Description != "" {
				name += "\t" + strings.NewReplacer("\n", " ", "\t", " ").Replace(target.Description)
			}
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func (a *application) discoverGraph(c *cobra.Command) (*buildsys.Project, error) {
	filename := ""
	for command := c; command != nil; command = command.Parent() {
		if flag := command.Flags().Lookup("build"); flag != nil && flag.Changed {
			if filepath.Ext(a.buildFile) != ".toml" && filepath.Base(a.buildFile) != "kamaji.yaml" && filepath.Base(a.buildFile) != "kamaji.yml" {
				return nil, nil // Explicit legacy --build selection wins over discovery.
			}
			filename = a.buildFile
			break
		}
	}
	cwd, err := a.runtime.WorkingDirectory()
	if err != nil {
		return nil, err
	}
	p, err := buildsys.Load(cwd, filename)
	if errors.Is(err, buildsys.ErrNoProject) {
		return nil, nil
	}
	return p, err
}

func (a *application) inspectGraph(c *cobra.Command, args []string) (bool, error) {
	p, err := a.discoverGraph(c)
	if err != nil {
		return true, err
	}
	if p == nil {
		return false, nil
	}
	for command := c; command != nil; command = command.Parent() {
		for _, name := range []string{"python", "rules-directory", "user", "install-root", "isolated"} {
			if flag := command.Flags().Lookup(name); flag != nil && flag.Changed {
				return true, fmt.Errorf("--%s applies only to legacy rule projects", name)
			}
		}
	}
	if c.Name() == "targets" {
		for _, name := range graphCompletions(p, nil, "") {
			c.Println(name)
		}
		return true, nil
	}
	if a.isolated {
		return true, fmt.Errorf("graph builds use declared outputs in the project directory; --isolated belongs to legacy rules")
	}
	names := args
	if a.all {
		names = nil
		for _, target := range p.Targets {
			names = append(names, target.Name)
		}
	}
	plan, err := p.Plan(names)
	if err != nil {
		return true, err
	}
	if c.Name() == "explain" && a.jsonOutput {
		return true, json.NewEncoder(c.OutOrStdout()).Encode(plan)
	}
	for _, item := range plan.Targets {
		if c.Name() == "explain" {
			c.Printf("%s: %s (%s); effect=%s\n", item.Name, item.Status, item.Reason, item.Effect)
		} else {
			c.Printf("%s: valid\n", item.Name)
		}
	}
	return true, nil
}

func (a *application) initBuildProject(c *cobra.Command, args []string) error {
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
	for dir := root; ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err == nil && !info.IsDir() {
			return fmt.Errorf("scaffold destination must use real directories")
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	for _, name := range []string{"kamaji.toml", "kamaji.yaml", "kamaji.yml"} {
		if _, err := os.Lstat(filepath.Join(root, name)); err == nil {
			return fmt.Errorf("build document already exists; refusing to overwrite")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	filename := filepath.Join(root, "kamaji.toml")
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString("version = 1\ndefault = [\"hello\"]\n\n[[targets]]\nname = \"hello\"\ndescription = \"A first language-independent build task\"\ncommand = [\"printf\", \"Hello from Kamaji\\n\"]\n")
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(filename)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	c.Printf("Created %s\nNext: kamaji plan, then kamaji build. Add .kamaji/ to your project's .gitignore.\n", filename)
	return nil
}
