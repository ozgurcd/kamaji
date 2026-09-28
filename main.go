package main

import (
	"fmt"
	"kamaji/cmd"
	"kamaji/internal/process"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(process.ExitCode(err))
	}
}
