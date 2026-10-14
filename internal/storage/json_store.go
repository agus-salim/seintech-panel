package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JSONStore manages a flat-file JSON storage with in-process sync.RWMutex and OS-level flock.
type JSONStore struct {
	filePath string
	lockPath string
	mu       sync.RWMutex
}

// NewJSONStore creates a new JSONStore instance for a given file path.
func NewJSONStore(filePath string) (*JSONStore, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	lockPath := filePath + ".lock"

	store := &JSONStore{
		filePath: filePath,
		lockPath: lockPath,
	}

	return store, nil
}

// acquireLock acquires an OS-level file lock (exclusive or shared).
func (s *JSONStore) acquireLock(exclusive bool) (FileLock, *os.File, error) {
	file, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open lock file %s: %w", s.lockPath, err)
	}

	fl := NewFileLock(file)
	if exclusive {
		if err := fl.Lock(); err != nil {
			file.Close()
			return nil, nil, err
		}
	} else {
		if err := fl.RLock(); err != nil {
			file.Close()
			return nil, nil, err
		}
	}

	return fl, file, nil
}

// Read loads and unmarshals JSON data from the file with shared lock.
func (s *JSONStore) Read(v any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fl, file, err := s.acquireLock(false)
	if err != nil {
		return fmt.Errorf("read lock error: %w", err)
	}
	defer func() {
		fl.Unlock()
		file.Close()
	}()

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return os.ErrNotExist
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed reading file %s: %w", s.filePath, err)
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshal error in %s: %w", s.filePath, err)
	}

	return nil
}

// Write atomically saves data to the file with exclusive lock and temporary file rename.
func (s *JSONStore) Write(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fl, file, err := s.acquireLock(true)
	if err != nil {
		return fmt.Errorf("write lock error: %w", err)
	}
	defer func() {
		fl.Unlock()
		file.Close()
	}()

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	tmpFile := filepath.Join(dir, fmt.Sprintf(".tmp_%d_%s", time.Now().UnixNano(), filepath.Base(s.filePath)))

	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("cannot create tmp file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("cannot write tmp file: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("cannot sync tmp file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("cannot close tmp file: %w", err)
	}

	// Atomic rename replaces the existing file safely
	if err := os.Rename(tmpFile, s.filePath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	return nil
}
