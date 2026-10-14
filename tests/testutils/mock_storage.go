package testutils

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	"github.com/c4gt/tornado-nginx-go-backend/internal/storage"
)

// MockStorage is an in-memory storage.Storage that follows the semantics of the
// S3/MinIO backend: directories and files are separate items, a file needs an
// existing parent directory, and creating something that already exists fails.
type MockStorage struct {
	mu   sync.Mutex
	data map[string]string
}

func NewMockStorage() *MockStorage {
	return &MockStorage{data: make(map[string]string)}
}

func (m *MockStorage) pathToString(path []string) string {
	return strings.Join(path, "/")
}

func (m *MockStorage) put(path []string, itemType string, data interface{}) error {
	dataJSON, err := models.NewStorageItem(path, itemType, data).ToJSON()
	if err != nil {
		return err
	}
	m.data[m.pathToString(path)] = dataJSON
	return nil
}

func (m *MockStorage) CreateDir(path []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.data[m.pathToString(path)]; exists {
		return fmt.Errorf("directory already exists")
	}
	return m.put(path, "dir", []string{})
}

func (m *MockStorage) DeleteDir(path []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, m.pathToString(path))
	return nil
}

func (m *MockStorage) CreateFile(path []string, data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(path) <= 1 {
		return fmt.Errorf("invalid path: must have parent directory")
	}
	if _, ok := m.data[m.pathToString(path[:len(path)-1])]; !ok {
		return fmt.Errorf("parent directory does not exist")
	}
	if _, exists := m.data[m.pathToString(path)]; exists {
		return fmt.Errorf("file already exists")
	}
	return m.put(path, "file", data)
}

func (m *MockStorage) GetFile(path []string) (*models.StorageItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, found := m.data[m.pathToString(path)]
	if !found {
		return nil, storage.ErrNotFound
	}
	return models.StorageItemFromJSON(data)
}

func (m *MockStorage) UpdateFile(path []string, data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, found := m.data[m.pathToString(path)]; !found {
		return storage.ErrNotFound
	}
	return m.put(path, "file", data)
}

func (m *MockStorage) DeleteFile(path []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, m.pathToString(path))
	return nil
}

func (m *MockStorage) PutItem(path string, data string, bucket ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[path] = data
	return nil
}

func (m *MockStorage) GetItem(path string, bucket ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[path]
	if !ok {
		return "", errors.New("item not found")
	}
	return v, nil
}

func (m *MockStorage) ExistsItem(path string, bucket ...string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.data[path]
	return ok, nil
}

func (m *MockStorage) DeleteItem(path string, bucket ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, path)
	return nil
}
