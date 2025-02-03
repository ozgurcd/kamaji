package main

import (
	"flag"
	"fmt"
	"kamaji/rt"
	"kamaji/runner"
	"kamaji/tools"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	rt.Init()

	buildFileName := flag.String("build", "BUILD.yaml", "name of the build file")
	debugModeFlag := flag.Bool("debug", false, "debug mode")
	cleanupFlag := flag.Bool("cleanup", false, "cleanup mode")

	flag.Parse()

	if *cleanupFlag {
		for _, dirName := range []string{"cache", "sandbox"} {
			dir := filepath.Join(rt.Config.TmpDir, dirName)
			if _, err := os.Stat(dir); err == nil {
				os.RemoveAll(dir)
			}
		}
		os.Exit(0)
	}

	if *debugModeFlag {
		rt.Config.DebugMode = true
	} else {
		rt.Config.DebugMode = false
	}

	targetName := strings.TrimSpace(flag.Arg(0))
	if targetName == "" {
		fmt.Printf("Target name is required\n")
		os.Exit(1)
	}
	if rt.Config.DebugMode {
		log.Printf("Target name is %s\n", targetName)
	}

	var pythonArgs []string
	for i, arg := range os.Args {
		if arg == "--" {
			pythonArgs = os.Args[i+1:]
			break
		}
	}

	target, err := tools.ParseBuildFile(*buildFileName, targetName)
	if err != nil {
		log.Fatalf("Error parsing build file: %s\n", err.Error())
	}

	if target.Name == "" {
		log.Fatalf("Target not found in build file\n")
	}

	err = tools.InitThirdPartyUsedInTarget(rt.Config.WorkspaceConfig, target)
	if err != nil {
		log.Fatalf("Error initializing third party used in target: %s\n", err.Error())
	}

	err = runner.Run(rt.Config.WorkspaceConfig, target, pythonArgs...)
	if err != nil {
		log.Fatalf("Error running target: %s\n", err.Error())
	}

	if rt.Config.DebugMode {
		log.Printf("Cleaning up sandbox directory: %s\n", rt.Config.SandboxDir)
	}
	os.RemoveAll(rt.Config.SandboxDir)
}
