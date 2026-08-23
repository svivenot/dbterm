package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HistoryEntry records a single query execution
type HistoryEntry struct {
	Query      string        `json:"query"`
	Database   string        `json:"database"`
	Server     string        `json:"server"`
	RowCount   int           `json:"row_count"`
	Duration   time.Duration `json:"duration"`
	ExecutedAt time.Time     `json:"executed_at"`
	Success    bool          `json:"success"`
	ErrorMsg   string        `json:"error_msg,omitempty"`
}

// Manager handles query history persistence
type Manager struct {
	Entries []HistoryEntry
	maxSize int
	path    string
}

func NewManager(maxSize int) *Manager {
	if maxSize <= 0 {
		maxSize = 200
	}
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".config", "dbterm", "history.json")

	mgr := &Manager{
		Entries: make([]HistoryEntry, 0),
		maxSize: maxSize,
		path:    p,
	}
	mgr.load()
	return mgr
}

func (m *Manager) Add(entry HistoryEntry) {
	trimmed := strings.TrimSpace(entry.Query)
	if trimmed == "" {
		return
	}
	entry.ExecutedAt = time.Now()

	// Prepend to show newest first
	m.Entries = append([]HistoryEntry{entry}, m.Entries...)
	if len(m.Entries) > m.maxSize {
		m.Entries = m.Entries[:m.maxSize]
	}
	_ = m.save()
}

func (m *Manager) load() {
	if data, err := os.ReadFile(m.path); err == nil {
		var entries []HistoryEntry
		if err := json.Unmarshal(data, &entries); err == nil {
			m.Entries = entries
		}
	}
}

func (m *Manager) save() error {
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.Entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0600)
}
