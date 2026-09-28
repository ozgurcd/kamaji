package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestChildProcess(t *testing.T) {
	mode := os.Getenv("KAMAJI_CHILD_MODE")
	if mode == "" {
		return
	}
	if mode == "exit" {
		os.Exit(23)
	}
	if mode == "terminal" {
		fmt.Fprintln(os.Stderr, "terminal ready")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			os.Exit(5)
		}
		fmt.Print(line)
		return
	}
	if mode == "ignore" {
		signal.Ignore(syscall.SIGTERM)
		fmt.Println("ready")
		<-time.After(time.Hour)
		return
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	defer signal.Stop(signals)
	if mode == "tree" {
		child := exec.Command(os.Args[0], "-test.run=^TestChildProcess$")
		child.Env = append(os.Environ(), "KAMAJI_CHILD_MODE=term")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Run(); err != nil {
			os.Exit(3)
		}
		return
	}
	fmt.Println("ready")
	<-signals
	if err := os.WriteFile(os.Getenv("KAMAJI_CHILD_MARKER"), []byte("terminated"), 0600); err != nil {
		os.Exit(4)
	}
}

func helperCommand(mode, marker string) *exec.Cmd {
	command := exec.Command(os.Args[0], "-test.run=^TestChildProcess$")
	command.Env = append(os.Environ(), "KAMAJI_CHILD_MODE="+mode, "KAMAJI_CHILD_MARKER="+marker)
	return command
}

func TestCancellationTerminatesProcessGroup(t *testing.T) {
	for _, mode := range []string{"tree", "ignore"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "terminated")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			command := helperCommand(mode, marker)
			output, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, err := Run(ctx, command, 150*time.Millisecond); done <- err }()
			ready := make(chan bool, 1)
			go func() { scanner := bufio.NewScanner(output); ready <- scanner.Scan() && scanner.Text() == "ready" }()
			select {
			case ok := <-ready:
				if !ok {
					t.Fatal("child did not become ready")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("child startup timed out")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) || ExitCode(err) != 130 {
					t.Fatalf("cancellation lost: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("child did not terminate")
			}
			if mode == "tree" {
				if _, err := os.Stat(marker); err != nil {
					t.Fatal("descendant did not receive graceful termination")
				}
			}
		})
	}
}

func TestDeadlineAndExitCodes(t *testing.T) {
	started, err := Run(context.Background(), helperCommand("exit", ""), time.Millisecond)
	if !started || ExitCode(err) != 23 {
		t.Fatalf("child status lost: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started, err = Run(ctx, helperCommand("ignore", ""), 10*time.Millisecond)
	if !started || !errors.Is(err, context.DeadlineExceeded) || ExitCode(err) != 124 {
		t.Fatalf("deadline lost: %v", err)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	started, err = Run(canceled, helperCommand("exit", ""), time.Millisecond)
	if started || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled context started a child")
	}
	if ExitCode(nil) != 0 || ExitCode(errors.New("fixture")) != 1 {
		t.Fatal("wrapper exit codes incorrect")
	}
}
