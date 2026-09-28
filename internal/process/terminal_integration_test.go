//go:build integration

package process

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestTerminalInteraction is run explicitly in a PTY with one synthetic input line.
func TestTerminalInteraction(t *testing.T) {
	command := helperCommand("terminal", "")
	var output bytes.Buffer
	command.Stdin = os.Stdin
	command.Stdout = &output
	command.Stderr = os.Stderr
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	started, err := Run(ctx, command, 100*time.Millisecond)
	if !started || err != nil || !strings.Contains(output.String(), "hello\n") {
		t.Fatalf("terminal input failed: started=%v err=%v", started, err)
	}
}
