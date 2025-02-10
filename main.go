package main

import (
	"flag"
	"fmt"
	"kamaji/obj"
	"kamaji/rt"
	"kamaji/runner"
	"kamaji/target"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	obj.WorkspaceFile = "kamaji.workspace.yaml"
	rt.Init()

	buildFileName := flag.String("build", "BUILD.yaml", "name of the build file")
	debugModeFlag := flag.Bool("debug", false, "debug mode")
	cleanupFlag := flag.Bool("cleanup", false, "cleanup mode")

	flag.Parse()

	if *cleanupFlag {
		for _, dirName := range []string{"cache", "execroot"} {
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

	var restOfTheArgs []string
	for i, arg := range os.Args {
		if arg == "--" {
			restOfTheArgs = os.Args[i+1:]
			break
		}
	}

	execTarget, err := target.ParseBuildFile(*buildFileName, targetName)
	if err != nil {
		log.Fatalf("Error parsing build file: %s\n", err.Error())
	}

	if execTarget.Name == "" {
		log.Fatalf("Target not found in build file\n")
	}

	rt.Config.ExecTarget = execTarget

	err = target.InitThirdPartyUsedInTarget(rt.Config.WorkspaceConfig, execTarget)
	if err != nil {
		log.Fatalf("Error initializing third party used in target: %s\n", err.Error())
	}

	err = runner.Run(rt.Config.WorkspaceConfig, execTarget, restOfTheArgs...)
	if err != nil {
		log.Fatalf("Error running target: %s\n", err.Error())
	}

	if rt.Config.DebugMode {
		log.Printf("Cleaning up execroot directory: %s\n", rt.Config.ExecRootDir)
	}
	os.RemoveAll(rt.Config.ExecRootDir)
}
