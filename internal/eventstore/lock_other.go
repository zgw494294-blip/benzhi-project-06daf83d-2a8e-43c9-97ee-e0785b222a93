//go:build !linux

package eventstore

import "syscall"

// acquireLock is a no-op on platforms without flock; Open relies on the
// per-directory lock file plus the append-only, single-syscall frame write.
func acquireLock(fd int) error  { return nil }
func releaseLock(fd int) error { return nil }
