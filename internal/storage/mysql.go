package storage

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	_ "github.com/go-sql-driver/mysql"
)

type MySQLStorage struct {
	db *sql.DB
}

func NewMySQLStorage(dsn string) (*MySQLStorage, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	// Basic connection pool configuration
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	storage := &MySQLStorage{db: db}

	// Initialize tables
	if err := storage.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return storage, nil
}

// initTables creates the underlying tables if they do not already exist.
// NOTE: We keep the original storage_items table for compatibility and
// add a new spreadsheets table for the higher-level StorageBackend API.
func (m *MySQLStorage) initTables() error {
	// Original storage_items schema (preserved behavior).
	createStorageItems := `
    CREATE TABLE IF NOT EXISTS storage_items (
        path VARCHAR(512) PRIMARY KEY,
        type VARCHAR(10) NOT NULL,
        data LONGTEXT
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
    `

	if _, err := m.db.Exec(createStorageItems); err != nil {
		return fmt.Errorf("failed to create storage_items table: %w", err)
	}

	// New spreadsheets table to support StorageBackend operations.
	createSpreadsheets := `
CREATE TABLE IF NOT EXISTS spreadsheets (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id   VARCHAR(255) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    data      LONGTEXT     NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_user_file (user_id, file_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`

	if _, err := m.db.Exec(createSpreadsheets); err != nil {
		return fmt.Errorf("failed to create spreadsheets table: %w", err)
	}

	return nil
}

func (m *MySQLStorage) pathToString(path []string) string {
	return strings.Join(path, "/")
}

func (m *MySQLStorage) PutItem(path string, data string, bucket ...string) error {
	query := `
    INSERT INTO storage_items (path, type, data) 
    VALUES (?, 'item', ?) 
    ON DUPLICATE KEY UPDATE data = VALUES(data)
    `

	_, err := m.db.Exec(query, path, data)
	return err
}

func (m *MySQLStorage) GetItem(path string, bucket ...string) (string, error) {
	query := "SELECT data FROM storage_items WHERE path = ?"

	var data string
	err := m.db.QueryRow(query, path).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", err
	}

	return data, nil
}

func (m *MySQLStorage) ExistsItem(path string, bucket ...string) (bool, error) {
	query := "SELECT COUNT(*) FROM storage_items WHERE path = ?"

	var count int
	err := m.db.QueryRow(query, path).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (m *MySQLStorage) DeleteItem(path string, bucket ...string) error {
	query := "DELETE FROM storage_items WHERE path = ?"

	_, err := m.db.Exec(query, path)
	return err
}

func (m *MySQLStorage) CreateDir(path []string) error {
	spath := m.pathToString(path)

	exists, err := m.ExistsItem(spath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("directory already exists")
	}

	dirData := models.NewStorageItem(path, "dir", []string{})
	dataJSON, err := dirData.ToJSON()
	if err != nil {
		return err
	}

	return m.PutItem(spath, dataJSON)
}

func (m *MySQLStorage) DeleteDir(path []string) error {
	spath := m.pathToString(path)
	query := "DELETE FROM storage_items WHERE path LIKE ?"

	_, err := m.db.Exec(query, spath+"%")
	return err
}

func (m *MySQLStorage) GetFile(path []string) (*models.StorageItem, error) {
	spath := m.pathToString(path)
	data, err := m.GetItem(spath)
	if err != nil {
		return nil, err
	}

	return models.StorageItemFromJSON(data)
}

func (m *MySQLStorage) CreateFile(path []string, data string) error {
	if len(path) <= 1 {
		return fmt.Errorf("invalid path: must have parent directory")
	}

	parentPath := path[:len(path)-1]
	parentItem, err := m.GetFile(parentPath)
	if err != nil {
		return fmt.Errorf("parent directory does not exist")
	}

	spath := m.pathToString(path)
	exists, err := m.ExistsItem(spath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("file already exists")
	}

	fileData := models.NewStorageItem(path, "file", data)
	dataJSON, err := fileData.ToJSON()
	if err != nil {
		return err
	}

	err = m.PutItem(spath, dataJSON)
	if err != nil {
		return err
	}

	fileName := path[len(path)-1]

	var filesList []string
	if parentData, ok := parentItem.Data.([]interface{}); ok {
		for _, item := range parentData {
			if str, ok := item.(string); ok {
				filesList = append(filesList, str)
			}
		}
	}

	filesList = append(filesList, fileName)
	parentItem.Data = filesList

	parentJSON, err := parentItem.ToJSON()
	if err != nil {
		return err
	}

	parentSPath := m.pathToString(parentPath)
	return m.PutItem(parentSPath, parentJSON)
}

func (m *MySQLStorage) UpdateFile(path []string, data string) error {
	fileItem, err := m.GetFile(path)
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

	spath := m.pathToString(path)
	return m.PutItem(spath, dataJSON)
}

func (m *MySQLStorage) DeleteFile(path []string) error {
	fileItem, err := m.GetFile(path)
	if err != nil {
		return err
	}
	if fileItem.Type != "file" {
		return fmt.Errorf("path is not a file")
	}

	if len(path) > 1 {
		parentPath := path[:len(path)-1]
		parentItem, err := m.GetFile(parentPath)
		if err != nil {
			return err
		}

		fileName := path[len(path)-1]

		var filesList []string
		if parentData, ok := parentItem.Data.([]interface{}); ok {
			for _, item := range parentData {
				if str, ok := item.(string); ok && str != fileName {
					filesList = append(filesList, str)
				}
			}
		}

		parentItem.Data = filesList

		parentJSON, err := parentItem.ToJSON()
		if err != nil {
			return err
		}

		parentSPath := m.pathToString(parentPath)
		err = m.PutItem(parentSPath, parentJSON)
		if err != nil {
			return err
		}
	}

	spath := m.pathToString(path)
	return m.DeleteItem(spath)
}

// SaveSpreadsheet implements the StorageBackend interface for MySQLStorage.
// It uses an upsert to either insert a new row or update an existing one.
func (m *MySQLStorage) SaveSpreadsheet(userID, fileName string, data []byte) error {
	if m.db == nil {
		return fmt.Errorf("mysql storage is not initialized")
	}

	now := time.Now().UTC()
	query := `
INSERT INTO spreadsheets (user_id, file_name, data, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    data = VALUES(data),
    updated_at = VALUES(updated_at);
`

	stmt, err := m.db.Prepare(query)
	if err != nil {
		log.Printf("MySQLStorage: failed to prepare SaveSpreadsheet statement: %v", err)
		return fmt.Errorf("prepare SaveSpreadsheet: %w", err)
	}
	defer stmt.Close()

	if _, err := stmt.Exec(userID, fileName, string(data), now, now); err != nil {
		log.Printf("MySQLStorage: failed to execute SaveSpreadsheet for user=%s file=%s: %v",
			userID, fileName, err)
		return fmt.Errorf("exec SaveSpreadsheet: %w", err)
	}

	return nil
}

// LoadSpreadsheet implements the StorageBackend interface for MySQLStorage.
func (m *MySQLStorage) LoadSpreadsheet(userID, fileName string) ([]byte, error) {
	if m.db == nil {
		return nil, fmt.Errorf("mysql storage is not initialized")
	}

	query := `
SELECT data
FROM spreadsheets
WHERE user_id = ? AND file_name = ?
`

	stmt, err := m.db.Prepare(query)
	if err != nil {
		log.Printf("MySQLStorage: failed to prepare LoadSpreadsheet statement: %v", err)
		return nil, fmt.Errorf("prepare LoadSpreadsheet: %w", err)
	}
	defer stmt.Close()

	var data string
	if err := stmt.QueryRow(userID, fileName).Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		log.Printf("MySQLStorage: failed to load spreadsheet user=%s file=%s: %v",
			userID, fileName, err)
		return nil, fmt.Errorf("query LoadSpreadsheet: %w", err)
	}

	return []byte(data), nil
}

// ListSpreadsheets implements the StorageBackend interface for MySQLStorage.
func (m *MySQLStorage) ListSpreadsheets(userID string) ([]string, error) {
	if m.db == nil {
		return nil, fmt.Errorf("mysql storage is not initialized")
	}

	query := `
SELECT file_name
FROM spreadsheets
WHERE user_id = ?
ORDER BY file_name;
`

	stmt, err := m.db.Prepare(query)
	if err != nil {
		log.Printf("MySQLStorage: failed to prepare ListSpreadsheets statement: %v", err)
		return nil, fmt.Errorf("prepare ListSpreadsheets: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
	if err != nil {
		log.Printf("MySQLStorage: failed to query ListSpreadsheets for user=%s: %v", userID, err)
		return nil, fmt.Errorf("query ListSpreadsheets: %w", err)
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Printf("MySQLStorage: failed to scan ListSpreadsheets row: %v", err)
			return nil, fmt.Errorf("scan ListSpreadsheets: %w", err)
		}
		files = append(files, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in ListSpreadsheets: %w", err)
	}

	return files, nil
}

// DeleteSpreadsheet implements the StorageBackend interface for MySQLStorage.
func (m *MySQLStorage) DeleteSpreadsheet(userID, fileName string) error {
	if m.db == nil {
		return fmt.Errorf("mysql storage is not initialized")
	}

	query := `
DELETE FROM spreadsheets
WHERE user_id = ? AND file_name = ?
`

	stmt, err := m.db.Prepare(query)
	if err != nil {
		log.Printf("MySQLStorage: failed to prepare DeleteSpreadsheet statement: %v", err)
		return fmt.Errorf("prepare DeleteSpreadsheet: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(userID, fileName)
	if err != nil {
		log.Printf("MySQLStorage: failed to execute DeleteSpreadsheet for user=%s file=%s: %v",
			userID, fileName, err)
		return fmt.Errorf("exec DeleteSpreadsheet: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected in DeleteSpreadsheet: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

