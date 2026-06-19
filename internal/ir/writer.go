package ir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Writer appends JSONL action records to a file with a mutex and fsync so
// records survive a crash mid-run.
type Writer struct {
	mu sync.Mutex
	f  *os.File
}

// NewWriter opens (creating parents) an append-only IR file at path.
func NewWriter(path string) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &Writer{f: f}, nil
}

// Write serializes one record and fsyncs it to disk.
func (w *Writer) Write(rec ActionRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err := w.f.Write(append(data, '\n')); err != nil {
		return err
	}
	return w.f.Sync()
}

// Close closes the IR file; safe to call on a nil receiver.
func (w *Writer) Close() error {
	if w == nil || w.f == nil {
		return nil
	}
	return w.f.Close()
}
