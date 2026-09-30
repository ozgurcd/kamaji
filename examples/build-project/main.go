package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	message := flag.String("message", "Hello", "Message to generate or verify")
	output := flag.String("output", "out/greeting.txt", "Artifact path")
	check := flag.Bool("check", false, "Verify the artifact instead of writing it")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	if err := generate(*output, *message, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(output, message string, check bool) error {
	expected := message + "\n"
	if check {
		data, err := os.ReadFile(output)
		if err != nil {
			return err
		}
		if string(data) != expected {
			return fmt.Errorf("greeting artifact does not match expected content")
		}
		fmt.Println("Greeting verified")
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	return os.WriteFile(output, []byte(expected), 0644)
}
