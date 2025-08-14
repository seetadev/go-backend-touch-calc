package storage

import (
    "database/sql"
    // "encoding/json"
    "fmt"
    "strings"

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

func (m *MySQLStorage) initTables() error {
    query := `
    CREATE TABLE IF NOT EXISTS storage_items (
        path VARCHAR(512) PRIMARY KEY,
        type VARCHAR(10) NOT NULL,
        data LONGTEXT
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
    `
    
    _, err := m.db.Exec(query)
    return err
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
