package storage

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	_ "modernc.org/sqlite"
)

// SQLiteStorage implements the Storage interface using a SQLite database.
// It is intended for local experiments and simple deployments.
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage creates a new SQLite-backed storage instance.
// Example DSN: "file:touchcalc.db?_pragma=foreign_keys(1)".
func NewSQLiteStorage(dsn string) (*SQLiteStorage, error) {
	if dsn == "" {
		dsn = "file:touchcalc.db?_pragma=foreign_keys(1)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	storage := &SQLiteStorage{db: db}
	if err := storage.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize SQLite tables: %w", err)
	}

	return storage, nil
}

func (s *SQLiteStorage) initTables() error {
	query := `
CREATE TABLE IF NOT EXISTS storage_items (
    path TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    data TEXT
);`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStorage) pathToString(path []string) string {
	return strings.Join(path, "/")
}

// PutItem stores or updates a low-level item.
func (s *SQLiteStorage) PutItem(path string, data string, bucket ...string) error {
	const q = `
INSERT INTO storage_items (path, type, data)
VALUES (?, 'item', ?)
ON CONFLICT(path) DO UPDATE SET data = excluded.data;
`
	_, err := s.db.Exec(q, path, data)
	return err
}

// GetItem retrieves a raw item by path.
func (s *SQLiteStorage) GetItem(path string, bucket ...string) (string, error) {
	const q = `SELECT data FROM storage_items WHERE path = ?`
	var data string
	err := s.db.QueryRow(q, path).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", err
	}
	return data, nil
}

// ExistsItem checks if an item exists at the given path.
func (s *SQLiteStorage) ExistsItem(path string, bucket ...string) (bool, error) {
	const q = `SELECT 1 FROM storage_items WHERE path = ? LIMIT 1`
	var dummy int
	err := s.db.QueryRow(q, path).Scan(&dummy)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteItem removes a raw item.
func (s *SQLiteStorage) DeleteItem(path string, bucket ...string) error {
	const q = `DELETE FROM storage_items WHERE path = ?`
	_, err := s.db.Exec(q, path)
	return err
}

// CreateDir creates a directory entry, if it doesn't already exist.
func (s *SQLiteStorage) CreateDir(path []string) error {
	if len(path) == 0 {
		return fmt.Errorf("invalid path: cannot be empty")
	}

	spath := s.pathToString(path)
	exists, err := s.ExistsItem(spath)
	if err != nil {
		return err
	}
	if exists {
		// Idempotent: do not error if directory already exists.
		return nil
	}

	dirData := models.NewStorageItem(path, "dir", []string{})
	dataJSON, err := dirData.ToJSON()
	if err != nil {
		return err
	}

	return s.PutItem(spath, dataJSON)
}

// DeleteDir deletes all items under the directory path.
func (s *SQLiteStorage) DeleteDir(path []string) error {
	if len(path) == 0 {
		return fmt.Errorf("invalid path: cannot be empty")
	}
	spath := s.pathToString(path)
	const q = `DELETE FROM storage_items WHERE path = ? OR path LIKE ?`
	_, err := s.db.Exec(q, spath, spath+"/%")
	return err
}

// GetFile retrieves a file or directory as a StorageItem.
func (s *SQLiteStorage) GetFile(path []string) (*models.StorageItem, error) {
	spath := s.pathToString(path)
	data, err := s.GetItem(spath)
	if err != nil {
		return nil, err
	}
	return models.StorageItemFromJSON(data)
}

// CreateFile creates a new file; errors if it already exists.
func (s *SQLiteStorage) CreateFile(path []string, data string) error {
	if len(path) == 0 {
		return fmt.Errorf("invalid path: cannot be empty")
	}

	spath := s.pathToString(path)
	exists, err := s.ExistsItem(spath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("file already exists")
	}

	// Ensure parent directories exist logically (dir entries).
	if len(path) > 1 {
		parentPath := path[:len(path)-1]
		if err := s.CreateDir(parentPath); err != nil {
			return fmt.Errorf("failed to create parent directories: %w", err)
		}
	}

	fileData := models.NewStorageItem(path, "file", data)
	dataJSON, err := fileData.ToJSON()
	if err != nil {
		return err
	}

	return s.PutItem(spath, dataJSON)
}

// UpdateFile updates an existing file.
func (s *SQLiteStorage) UpdateFile(path []string, data string) error {
	fileItem, err := s.GetFile(path)
	if err != nil {
		return err
	}
	if fileItem.Type != "file" {
		return fmt.Errorf("path is not a file")
	}

	fileItem.Data = data
	dataJSON, err := fileItem.ToJSON()
	if err != nil {
		return err
	}

	spath := s.pathToString(path)
	return s.PutItem(spath, dataJSON)
}

// DeleteFile deletes a file and, optionally, could clean up parent directory listings.
func (s *SQLiteStorage) DeleteFile(path []string) error {
	if len(path) == 0 {
		return fmt.Errorf("invalid path: cannot be empty")
	}
	spath := s.pathToString(path)
	return s.DeleteItem(spath)
}


