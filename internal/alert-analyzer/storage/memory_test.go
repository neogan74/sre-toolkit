package storage

import (
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
	"github.com/stretchr/testify/assert"
)

func TestNewMemoryStorage(t *testing.T) {
	s := NewMemoryStorage()
	assert.NotNil(t, s)
	assert.False(t, s.HasData())
}

func TestMemoryStorage_StoreAndRetrieve(t *testing.T) {
	s := NewMemoryStorage()

	history := &collector.AlertHistory{
		Alerts:    []collector.Alert{{Name: "TestAlert"}},
		StartTime: time.Now(),
	}

	// Test Retrieve on empty storage
	retrieved, err := s.Retrieve()
	assert.Error(t, err)
	assert.Nil(t, retrieved)

	// Test Store
	err = s.Store(history)
	assert.NoError(t, err)
	assert.True(t, s.HasData())

	// Test Retrieve
	retrieved, err = s.Retrieve()
	assert.NoError(t, err)
	assert.Equal(t, history, retrieved)
}

func TestMemoryStorage_StoreNil(t *testing.T) {
	s := NewMemoryStorage()
	err := s.Store(nil)
	assert.Error(t, err)
}

func TestMemoryStorage_Clear(t *testing.T) {
	s := NewMemoryStorage()
	history := &collector.AlertHistory{
		Alerts: []collector.Alert{{Name: "TestAlert"}},
	}
	_ = s.Store(history)
	assert.True(t, s.HasData())

	err := s.Clear()
	assert.NoError(t, err)
	assert.False(t, s.HasData())

	_, err = s.Retrieve()
	assert.Error(t, err)
}
