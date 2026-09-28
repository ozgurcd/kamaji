package process

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

// Run starts a private process group. Cancellation first sends TERM, then KILL
// after grace, covering descendants which remain in that group. started says
// whether a child existed, so callers can distinguish setup from execution.
func Run(ctx context.Context, command *exec.Cmd, grace time.Duration) (started bool, result error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if grace <= 0 {
		grace = 2 * time.Second
	}
	if command.WaitDelay == 0 {
		command.WaitDelay = grace
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setpgid = true
	command.SysProcAttr.Pgid = 0
	restore, err := foreground(ctx, command)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, restore()) }()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := command.Start(); err != nil {
		return false, err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		return true, err
	case <-ctx.Done():
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		// Always finish the group grace period, even if its leader exits first.
		// Otherwise a descendant that ignores TERM could outlive canceled execution.
		timer := time.NewTimer(grace)
		defer timer.Stop()
		<-timer.C
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		return true, errors.Join(ctx.Err(), <-done)
	}
}

// ExitCode preserves child status while distinguishing deadline and cancellation.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return 124
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	var child *exec.ExitError
	if errors.As(err, &child) {
		if status, ok := child.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		if code := child.ExitCode(); code > 0 {
			return code
		}
	}
	return 1
}
