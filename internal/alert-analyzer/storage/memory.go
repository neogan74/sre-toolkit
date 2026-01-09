// Package storage provides in-memory storage for alert history.
package storage

import (
	"fmt"
	"sync"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
)

// Storage defines the interface for alert history storage
type Storage interface {
	Store(history *collector.AlertHistory) error
	Retrieve() (*collector.AlertHistory, error)
	Clear() error
}

// MemoryStorage implements in-memory storage for alert history
type MemoryStorage struct {
	mu      sync.RWMutex
	history *collector.AlertHistory
}

// NewMemoryStorage creates a new in-memory storage instance
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		history: nil,
	}
}

// Store saves alert history to memory
func (s *MemoryStorage) Store(history *collector.AlertHistory) error {
	if history == nil {
		return fmt.Errorf("cannot store nil history")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = history
	return nil
}

// Retrieve gets the stored alert history
func (s *MemoryStorage) Retrieve() (*collector.AlertHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.history == nil {
		return nil, fmt.Errorf("no alert history stored")
	}

	return s.history, nil
}

// Clear removes stored alert history
func (s *MemoryStorage) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = nil
	return nil
}

// HasData returns true if storage contains data
func (s *MemoryStorage) HasData() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.history != nil
}
