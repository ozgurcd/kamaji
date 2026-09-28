package fsutil

import (
	"fmt"
	"os"
	"syscall"
)

// Lock uses a persistent inode. Never remove the lock file: another process
// could otherwise acquire a different inode while an existing lease is held.
func Lock(path string, exclusive bool) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0077 != 0 {
		file.Close()
		return nil, fmt.Errorf("unsafe lock file")
	}
	mode := syscall.LOCK_SH | syscall.LOCK_NB
	if exclusive {
		mode = syscall.LOCK_EX | syscall.LOCK_NB
	}
	if err := syscall.Flock(fd, mode); err != nil {
		file.Close()
		return nil, fmt.Errorf("another operation is using this directory; retry when it finishes")
	}
	return file, nil
}
