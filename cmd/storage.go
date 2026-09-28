package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"kamaji/rt"

	"github.com/spf13/cobra"
)

func printEntries(c *cobra.Command, entries []rt.StorageEntry, jsonOutput bool) error {
	if jsonOutput {
		return json.NewEncoder(c.OutOrStdout()).Encode(entries)
	}
	for _, entry := range entries {
		c.Printf("%s\t%d bytes\t%s\t%s\n", entry.Name, entry.Bytes, entry.Status, entry.Path)
	}
	if len(entries) == 0 {
		c.Println("No entries.")
	}
	return nil
}
func (a *application) cacheCommand() *cobra.Command {
	cache := &cobra.Command{Use: "cache", Short: "Inspect or remove downloaded dependencies"}
	var jsonStatus, dryClean, includeRuns, dryPrune bool
	var budget int64
	status := &cobra.Command{Use: "status", Short: "Show cache entries and sizes", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		entries, err := a.runtime.StorageEntries("cache")
		if err != nil {
			return err
		}
		return printEntries(c, entries, jsonStatus)
	}}
	status.Flags().BoolVar(&jsonStatus, "json", false, "Emit JSON")
	clean := &cobra.Command{Use: "clean", Short: "Remove cached downloads (optionally execution files)", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return a.clean(dryClean, includeRuns, c) }}
	clean.Flags().BoolVar(&dryClean, "dry-run", false, "Preview paths and bytes without changing storage")
	clean.Flags().BoolVar(&includeRuns, "runs", false, "Also remove execution files")
	prune := &cobra.Command{Use: "prune", Short: "Remove oldest cached payloads to meet a byte budget", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		entries, err := a.runtime.CachePrune(budget, dryPrune)
		verb := "Removed"
		if dryPrune {
			verb = "Would remove"
		}
		for _, entry := range entries {
			c.Printf("%s %s (%d bytes)\n", verb, entry.Path, entry.Bytes)
		}
		if err != nil {
			return err
		}
		c.Printf("Cache budget: %d bytes.\n", budget)
		return nil
	}}
	prune.Flags().Int64Var(&budget, "max-bytes", 4<<30, "Maximum bytes to retain (0 removes all recognized entries)")
	prune.Flags().BoolVar(&dryPrune, "dry-run", false, "Preview removals without changing storage")
	cache.AddCommand(status, clean, prune)
	return cache
}
func (a *application) clean(dryRun, includeRuns bool, c *cobra.Command) error {
	kinds := []string{"cache"}
	if includeRuns {
		kinds = append(kinds, "execroot")
	}
	if dryRun {
		for _, kind := range kinds {
			entries, err := a.runtime.StorageEntries(kind)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				c.Printf("Would remove %s (%d bytes)\n", entry.Path, entry.Bytes)
			}
		}
		return nil
	}
	lease, err := a.runtime.RuntimeLease(true)
	if err != nil {
		return err
	}
	defer lease.Close()
	remove := a.options.RemoveAll
	if remove == nil {
		remove = os.RemoveAll
	}
	for _, kind := range kinds {
		path := filepath.Join(a.runtime.Config.TmpDir, kind)
		if err := rt.CheckPrivateDir(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := remove(path); err != nil {
			return fmt.Errorf("cleanup %s: %w", kind, err)
		}
	}
	c.Println("Requested runtime storage removed; Python environments preserved.")
	return nil
}
func (a *application) runsCommand() *cobra.Command {
	runs := &cobra.Command{Use: "runs", Short: "Inspect execution directories without reading their contents"}
	var jsonList, jsonShow bool
	list := &cobra.Command{Use: "list", Short: "List retained or incomplete execution directories", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		entries, err := a.runtime.StorageEntries("execroot")
		if err != nil {
			return err
		}
		return printEntries(c, entries, jsonList)
	}}
	list.Flags().BoolVar(&jsonList, "json", false, "Emit JSON")
	show := &cobra.Command{Use: "show <name>", Short: "Show an execution directory's path, size and completion state", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		if !filepath.IsLocal(args[0]) || filepath.Base(args[0]) != args[0] || args[0] == "." {
			return fmt.Errorf("invalid execution name")
		}
		entries, err := a.runtime.StorageEntries("execroot")
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name == args[0] {
				return printEntries(c, []rt.StorageEntry{entry}, jsonShow)
			}
		}
		return fmt.Errorf("execution directory not found")
	}}
	show.Flags().BoolVar(&jsonShow, "json", false, "Emit JSON")
	runs.AddCommand(list, show)
	return runs
}
