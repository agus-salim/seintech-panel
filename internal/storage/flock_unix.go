//go:build unix || linux || darwin

package storage

import (
	"os"
	"golang.org/x/sys/unix"
)

type unixFileLock struct {
	file *os.File
}

func NewFileLock(file *os.File) FileLock {
	return &unixFileLock{file: file}
}

func (l *unixFileLock) Lock() error {
	return unix.Flock(int(l.file.Fd()), unix.LOCK_EX)
}

func (l *unixFileLock) RLock() error {
	return unix.Flock(int(l.file.Fd()), unix.LOCK_SH)
}

func (l *unixFileLock) Unlock() error {
	return unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
}
