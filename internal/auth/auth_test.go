package auth

import (
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	"github.com/c4gt/tornado-nginx-go-backend/internal/storage"
)

// MockStorage implements the Storage interface for testing
type MockStorage struct {
	files map[string]*models.StorageItem
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		files: make(map[string]*models.StorageItem),
	}
}

func (m *MockStorage) pathToString(path []string) string {
	result := ""
	for i, part := range path {
		if i > 0 {
			result += "/"
		}
		result += part
	}
	return result
}

func (m *MockStorage) CreateFile(path []string, data string) error {
	key := m.pathToString(path)
	m.files[key] = models.NewStorageItem(path, "file", data)
	return nil
}

func (m *MockStorage) GetFile(path []string) (*models.StorageItem, error) {
	key := m.pathToString(path)
	item, exists := m.files[key]
	if !exists {
		return nil, storage.ErrNotFound
	}
	return item, nil
}

func (m *MockStorage) UpdateFile(path []string, data string) error {
	key := m.pathToString(path)
	if _, exists := m.files[key]; !exists {
		return storage.ErrNotFound
	}
	m.files[key] = models.NewStorageItem(path, "file", data)
	return nil
}

func (m *MockStorage) DeleteFile(path []string) error {
	key := m.pathToString(path)
	if _, exists := m.files[key]; !exists {
		return storage.ErrNotFound
	}
	delete(m.files, key)
	return nil
}

func (m *MockStorage) CreateDir(path []string) error {
	key := m.pathToString(path)
	m.files[key] = models.NewStorageItem(path, "dir", []string{})
	return nil
}

func (m *MockStorage) DeleteDir(path []string) error {
	key := m.pathToString(path)
	if _, exists := m.files[key]; !exists {
		return storage.ErrNotFound
	}
	delete(m.files, key)
	return nil
}

func (m *MockStorage) PutItem(path string, data string, bucket ...string) error {
	// Not implemented for mock
	return nil
}

func (m *MockStorage) GetItem(path string, bucket ...string) (string, error) {
	// Not implemented for mock
	return "", nil
}

func (m *MockStorage) ExistsItem(path string, bucket ...string) (bool, error) {
	// Not implemented for mock
	return false, nil
}

func (m *MockStorage) DeleteItem(path string, bucket ...string) error {
	// Not implemented for mock
	return nil
}

func TestCreateUser(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)

	// Create user directory first
	err := mockStorage.CreateDir([]string{"home", UserDir})
	if err != nil {
		t.Fatalf("Failed to create user directory: %v", err)
	}

	email := "test@example.com"
	password := "testpassword"

	err = service.CreateUser(email, password)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Verify user exists
	exists, err := service.UserExists(email)
	if err != nil {
		t.Fatalf("UserExists failed: %v", err)
	}
	if !exists {
		t.Error("User should exist after creation")
	}
}

func TestAuthenticateUser(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)

	// Create user directory first
	err := mockStorage.CreateDir([]string{"home", UserDir})
	if err != nil {
		t.Fatalf("Failed to create user directory: %v", err)
	}

	email := "test@example.com"
	password := "testpassword"

	// Create user
	err = service.CreateUser(email, password)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Test correct password
	authenticated, err := service.AuthenticateUser(email, password)
	if err != nil {
		t.Fatalf("AuthenticateUser failed: %v", err)
	}
	if !authenticated {
		t.Error("Authentication should succeed with correct password")
	}

	// Test incorrect password
	authenticated, err = service.AuthenticateUser(email, "wrongpassword")
	if err != nil {
		t.Fatalf("AuthenticateUser failed: %v", err)
	}
	if authenticated {
		t.Error("Authentication should fail with incorrect password")
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"test@example.com", true},
		{"user@domain.org", true},
		{"invalid.email", false},
		{"@domain.com", false},
		{"user@", false},
		{"", false},
		{"a@b", true}, // minimal valid email
	}

	for _, test := range tests {
		result := ValidateEmail(test.email)
		if result != test.valid {
			t.Errorf("ValidateEmail(%q) = %v, want %v", test.email, result, test.valid)
		}
	}
}

func TestUpdatePassword(t *testing.T) {
	mockStorage := NewMockStorage()
	service := NewService(mockStorage)

	// Create user directory first
	err := mockStorage.CreateDir([]string{"home", UserDir})
	if err != nil {
		t.Fatalf("Failed to create user directory: %v", err)
	}

	email := "test@example.com"
	oldPassword := "oldpassword"
	newPassword := "newpassword"

	// Create user
	err = service.CreateUser(email, oldPassword)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Update password
	err = service.UpdatePassword(email, newPassword)
	if err != nil {
		t.Fatalf("UpdatePassword failed: %v", err)
	}

	// Test old password should fail
	authenticated, err := service.AuthenticateUser(email, oldPassword)
	if err != nil {
		t.Fatalf("AuthenticateUser failed: %v", err)
	}
	if authenticated {
		t.Error("Authentication should fail with old password")
	}

	// Test new password should succeed
	authenticated, err = service.AuthenticateUser(email, newPassword)
	if err != nil {
		t.Fatalf("AuthenticateUser failed: %v", err)
	}
	if !authenticated {
		t.Error("Authentication should succeed with new password")
	}
}