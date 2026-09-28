package fsutil

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLockAcrossProcesses(t *testing.T) {
	if os.Getenv("KAMAJI_TEST_LOCK_CHILD") == "1" {
		lease, err := Lock(os.Getenv("KAMAJI_TEST_LOCK_PATH"), false)
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Close()
		if _, err := os.Stdout.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
		_, _ = io.CopyN(io.Discard, os.Stdin, 1)
		return
	}
	path := filepath.Join(t.TempDir(), "lock")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLockAcrossProcesses$")
	child.Env = []string{"KAMAJI_TEST_LOCK_CHILD=1", "KAMAJI_TEST_LOCK_PATH=" + path}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { input.Close(); cancel(); _ = child.Wait() })
	var ready [1]byte
	if _, err := io.ReadFull(output, ready[:]); err != nil {
		t.Fatal(err)
	}
	if lease, err := Lock(path, true); err == nil {
		lease.Close()
		t.Fatal("exclusive lock acquired while another process was active")
	}
	shared, err := Lock(path, false)
	if err != nil {
		t.Fatal(err)
	}
	shared.Close()
	input.Close()
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	exclusive, err := Lock(path, true)
	if err != nil {
		t.Fatal(err)
	}
	exclusive.Close()
}
