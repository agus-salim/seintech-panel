package storage

// FileLock represents an advisory file lock handle
type FileLock interface {
	Lock() error
	RLock() error
	Unlock() error
}
