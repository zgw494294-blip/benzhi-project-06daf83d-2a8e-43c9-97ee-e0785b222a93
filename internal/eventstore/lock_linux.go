package eventstore

import "syscall"

// acquireLock takes an exclusive flock on the open lock file descriptor.
func acquireLock(fd int) error  { return syscall.Flock(fd, syscall.LOCK_EX) }
func releaseLock(fd int) error { return syscall.Flock(fd, syscall.LOCK_UN) }
