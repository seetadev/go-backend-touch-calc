package storage

import (
	"errors"
	"fmt"
	"log"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
)

var (
	ErrNotFound = errors.New("item not found")
)

// Storage defines the interface for storage operations used by the
// existing S3-style path-based storage implementations.
type Storage interface {
	// File operations
	CreateFile(path []string, data string) error
	GetFile(path []string) (*models.StorageItem, error)
	UpdateFile(path []string, data string) error
	DeleteFile(path []string) error
	
	// Directory operations
	CreateDir(path []string) error
	DeleteDir(path []string) error
	
	// Item operations (low-level)
	PutItem(path string, data string, bucket ...string) error
	GetItem(path string, bucket ...string) (string, error)
	ExistsItem(path string, bucket ...string) (bool, error)
	DeleteItem(path string, bucket ...string) error
}

// StorageBackend defines a higher-level interface focused specifically on
// spreadsheet persistence. Implementations can be backed by MongoDB, S3,
// local filesystem, etc.
type StorageBackend interface {
	SaveSpreadsheet(userID, fileName string, data []byte) error
	LoadSpreadsheet(userID, fileName string) ([]byte, error)
	ListSpreadsheets(userID string) ([]string, error)
	DeleteSpreadsheet(userID, fileName string) error
}

// StorageManager coordinates multiple StorageBackend implementations.
// It can replicate writes across all backends and read from the first
// available backend in registration order.
type StorageManager struct {
	backends map[string]StorageBackend
	order    []string
}

// NewStorageManager creates an empty StorageManager.
func NewStorageManager() *StorageManager {
	return &StorageManager{
		backends: make(map[string]StorageBackend),
		order:    make([]string, 0),
	}
}

// RegisterBackend registers a named StorageBackend. If a backend with the
// same name already exists, it is replaced and a log message is emitted.
func (m *StorageManager) RegisterBackend(name string, backend StorageBackend) error {
	if name == "" {
		return fmt.Errorf("storage backend name cannot be empty")
	}
	if backend == nil {
		return fmt.Errorf("storage backend %q is nil", name)
	}

	if m.backends == nil {
		m.backends = make(map[string]StorageBackend)
	}

	if _, exists := m.backends[name]; !exists {
		m.order = append(m.order, name)
	} else {
		log.Printf("StorageManager: backend %q already registered, overriding", name)
	}

	m.backends[name] = backend
	return nil
}

// SaveToAll saves the spreadsheet data to all registered backends. If one or
// more backends fail, it logs the individual errors and returns a combined
// error while still attempting to write to every backend.
func (m *StorageManager) SaveToAll(userID, fileName string, data []byte) error {
	if len(m.order) == 0 {
		return fmt.Errorf("no storage backends registered")
	}

	var errs []error

	for _, name := range m.order {
		backend := m.backends[name]
		if backend == nil {
			continue
		}

		if err := backend.SaveSpreadsheet(userID, fileName, data); err != nil {
			log.Printf("StorageManager: failed to save %q for user %q to backend %q: %v",
				fileName, userID, name, err)
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// LoadFromPrimary attempts to load spreadsheet data from the registered
// backends in registration order. It returns the first successful result.
// If all backends report ErrNotFound, ErrNotFound is returned. If one or
// more backends fail with other errors and none succeed, the last error
// is returned.
func (m *StorageManager) LoadFromPrimary(userID, fileName string) ([]byte, error) {
	if len(m.order) == 0 {
		return nil, fmt.Errorf("no storage backends registered")
	}

	var lastErr error

	for _, name := range m.order {
		backend := m.backends[name]
		if backend == nil {
			continue
		}

		data, err := backend.LoadSpreadsheet(userID, fileName)
		if err == nil {
			log.Printf("StorageManager: loaded %q for user %q from backend %q",
				fileName, userID, name)
			return data, nil
		}

		if errors.Is(err, ErrNotFound) {
			log.Printf("StorageManager: %q for user %q not found in backend %q",
				fileName, userID, name)
			lastErr = ErrNotFound
			continue
		}

		log.Printf("StorageManager: error loading %q for user %q from backend %q: %v",
			fileName, userID, name, err)
		lastErr = err
	}

	if lastErr == nil || errors.Is(lastErr, ErrNotFound) {
		return nil, ErrNotFound
	}

	return nil, lastErr
}
