package logging

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger is a structured (slog JSON) logger that tees to stderr and, when a
// runDir is given, appends to run.log.jsonl for later diagnosis.
type Logger struct {
	*slog.Logger
	file *os.File
	mu   sync.Mutex
}

// New creates a logger writing to stderr and (optionally) a run log file.
// Debug enables slog debug level.
func New(runDir string, debug bool) (*Logger, error) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	var writers []io.Writer
	writers = append(writers, os.Stderr)
	var file *os.File
	if runDir != "" {
		if err := os.MkdirAll(runDir, 0755); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(filepath.Join(runDir, "run.log.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		file = f
		writers = append(writers, f)
	}
	h := slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level})
	return &Logger{Logger: slog.New(h), file: file}, nil
}

// Close closes the underlying log file if one was opened.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// WriteJSONL appends a JSON-encoded value as one line to path, creating parent
// directories as needed. Used for ad-hoc JSONL artifacts outside the main logger.
func WriteJSONL(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

// RunID returns a UTC timestamp string used as a unique run directory name.
// Note: callers that need monotonic IDs across rapid runs should append a
// disambiguator; the second-resolution stamp can collide for concurrent runs.
func RunID() string { return time.Now().UTC().Format("20060102T150405Z") }
