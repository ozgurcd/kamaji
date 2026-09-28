package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"unsafe"
)

// A controlling terminal is a process-wide resource, so interactive ownership
// transfers are serialized even when callers use independent runtime instances.
var terminalGate = make(chan struct{}, 1)

func foreground(ctx context.Context, command *exec.Cmd) (func() error, error) {
	input, ok := command.Stdin.(*os.File)
	if !ok {
		return func() error { return nil }, nil
	}
	var group int32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, input.Fd(), uintptr(syscall.TIOCGPGRP), uintptr(unsafe.Pointer(&group)))
	if errno != 0 {
		return func() error { return nil }, nil
	} // pipe or non-controlling input
	select {
	case terminalGate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// Refresh after waiting for a previous interactive command to release the TTY.
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, input.Fd(), uintptr(syscall.TIOCGPGRP), uintptr(unsafe.Pointer(&group)))
	if errno != 0 || int(group) != syscall.Getpgrp() {
		<-terminalGate
		return nil, fmt.Errorf("interactive execution requires the foreground terminal")
	}
	ignored := signal.Ignored(syscall.SIGTTOU)
	signal.Ignore(syscall.SIGTTOU)
	command.SysProcAttr.Foreground = true
	command.SysProcAttr.Ctty = int(input.Fd())
	return func() error {
		defer func() { <-terminalGate }()
		if !ignored {
			defer signal.Reset(syscall.SIGTTOU)
		}
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, input.Fd(), uintptr(syscall.TIOCSPGRP), uintptr(unsafe.Pointer(&group)))
		if errno != 0 {
			return fmt.Errorf("restore terminal foreground: %w", errno)
		}
		return nil
	}, nil
}
