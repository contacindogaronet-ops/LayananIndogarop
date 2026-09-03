package logger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

type APILogger struct {
	mu      sync.Mutex
	maxLogs int
	entries []LogEntry
}

func NewAPILogger(maxLogs int) *APILogger {
	return &APILogger{
		maxLogs: maxLogs,
		entries: make([]LogEntry, 0),
	}
}

func (l *APILogger) Log(level, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}

	fmt.Printf("[%s] [%s] %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"), level, message)

	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxLogs {
		l.entries = l.entries[1:]
	}
}

func (l *APILogger) ServeHTTPLogs(w http.ResponseWriter, r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(l.entries)
}
