//go:build windows

package storage

import (
	"os"
	"golang.org/x/sys/windows"
)

type windowsFileLock struct {
	file *os.File
}

func NewFileLock(file *os.File) FileLock {
	return &windowsFileLock{file: file}
}

func (l *windowsFileLock) Lock() error {
	var overlapped windows.Overlapped
	return windows.LockFileEx(
		windows.Handle(l.file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0,
		1,
		0,
		&overlapped,
	)
}

func (l *windowsFileLock) RLock() error {
	var overlapped windows.Overlapped
	return windows.LockFileEx(
		windows.Handle(l.file.Fd()),
		0, // Shared lock
		0,
		1,
		0,
		&overlapped,
	)
}

func (l *windowsFileLock) Unlock() error {
	var overlapped windows.Overlapped
	return windows.UnlockFileEx(
		windows.Handle(l.file.Fd()),
		0,
		1,
		0,
		&overlapped,
	)
}
