package testutils

import (
	"errors"
	"strings"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
)

type MockStorage struct {
	data map[string]string
}

func NewMockStorage() *MockStorage {
	return &MockStorage{data: make(map[string]string)}
}

func (m *MockStorage) pathToString(path []string) string {
	return strings.Join(path, "/")
}

func (m *MockStorage) CreateDir(path []string) error {
	spath := m.pathToString(path)
	m.data[spath] = `{"path":["` + strings.Join(path, `","`) + `"],"type":"dir","data":[]}`
	return nil
}

func (m *MockStorage) DeleteDir(path []string) error {
	delete(m.data, m.pathToString(path))
	return nil
}

func (m *MockStorage) CreateFile(path []string, data string) error {
	spath := m.pathToString(path)
	m.data[spath] = data
	return nil
}

func (m *MockStorage) GetFile(path []string) (*models.StorageItem, error) {
	spath := m.pathToString(path)
	data, found := m.data[spath]
	if !found {
		return nil, errors.New("item not found")
	}
	return models.StorageItemFromJSON(data)
}

func (m *MockStorage) UpdateFile(path []string, data string) error {
	spath := m.pathToString(path)
	m.data[spath] = data
	return nil
}

func (m *MockStorage) DeleteFile(path []string) error {
	delete(m.data, m.pathToString(path))
	return nil
}

func (m *MockStorage) PutItem(path string, data string, bucket ...string) error {
	m.data[path] = data
	return nil
}

func (m *MockStorage) GetItem(path string, bucket ...string) (string, error) {
	v, ok := m.data[path]
	if !ok {
		return "", errors.New("item not found")
	}
	return v, nil
}

func (m *MockStorage) ExistsItem(path string, bucket ...string) (bool, error) {
	_, ok := m.data[path]
	return ok, nil
}

func (m *MockStorage) DeleteItem(path string, bucket ...string) error {
	delete(m.data, path)
	return nil
}
