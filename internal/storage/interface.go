package storage

import (
	"errors"
	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
)

var (
	ErrNotFound = errors.New("item not found")
)

// Storage defines the interface for storage operations
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