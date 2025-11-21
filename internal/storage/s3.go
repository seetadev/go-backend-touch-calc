package storage

/*
   NOTE: Original AWS S3 / MinIO implementation
   --------------------------------------------
   This block is intentionally commented out so the project can run
   without AWS or MinIO SDK modules. The live implementation below
   keeps the same Storage interface but backs it with local file
   storage instead.

   import (
   	"context"
   	"fmt"
   	"strings"

   	"github.com/aws/aws-sdk-go-v2/aws"
   	"github.com/aws/aws-sdk-go-v2/config"
   	"github.com/aws/aws-sdk-go-v2/credentials"
   	"github.com/aws/aws-sdk-go-v2/service/s3"
   	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
   )

   type S3Storage struct {
   	client     *s3.Client
   	bucketName string
   }

   func NewS3Storage(bucketName, endpoint, accessKey, secretKey, region string, useSSL bool) (*S3Storage, error) {
   	// ... original AWS/MinIO configuration and client creation ...
   }

   // func (s *S3Storage) PutItem(path string, data string, bucket ...string) error              { ... }
   // func (s *S3Storage) GetItem(path string, bucket ...string) (string, error)                 { ... }
   // func (s *S3Storage) ExistsItem(path string, bucket ...string) (bool, error)                { ... }
   // func (s *S3Storage) DeleteItem(path string, bucket ...string) error                        { ... }
   // func (s *S3Storage) CreateDir(path []string) error                                         { ... }
   // func (s *S3Storage) DeleteDir(path []string) error                                         { ... }
   // func (s *S3Storage) GetFile(path []string) (*models.StorageItem, error)                    { ... }
   // func (s *S3Storage) CreateFile(path []string, data string) error                           { ... }
   // func (s *S3Storage) UpdateFile(path []string, data string) error                           { ... }
   // func (s *S3Storage) DeleteFile(path []string) error                                        { ... }
*/

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
)

// S3Storage now implements the Storage interface using the local filesystem
// instead of AWS S3. The bucket name is treated as a top-level directory.
type S3Storage struct {
	bucketName string
}

// NewS3Storage creates a new local-filesystem-backed storage.
// The endpoint/accessKey/secretKey/region/useSSL parameters are kept for
// compatibility but ignored in this stub implementation.
func NewS3Storage(bucketName, endpoint, accessKey, secretKey, region string, useSSL bool) (*S3Storage, error) {
	if bucketName == "" {
		bucketName = "local-bucket"
	}

	return &S3Storage{
		bucketName: bucketName,
	}, nil
}

func (s *S3Storage) pathToString(path []string) string {
	return strings.Join(path, "/")
}

// baseDir returns the root directory for the current bucket.
func (s *S3Storage) baseDir(bucket ...string) string {
	bucketName := s.bucketName
	if len(bucket) > 0 && bucket[0] != "" {
		bucketName = bucket[0]
	}
	// Store everything under ./data/<bucketName>
	return filepath.Join("data", bucketName)
}

func (s *S3Storage) fullPath(path string, bucket ...string) string {
	return filepath.Join(s.baseDir(bucket...), filepath.FromSlash(path))
}

func (s *S3Storage) PutItem(path string, data string, bucket ...string) error {
	fp := s.fullPath(path, bucket...)
	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fp, []byte(data), 0o644)
}

func (s *S3Storage) GetItem(path string, bucket ...string) (string, error) {
	fp := s.fullPath(path, bucket...)
	// First check what exists at this path.
	info, err := os.Stat(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", err
	}

	// If this is a directory, synthesize a directory StorageItem JSON so that
	// higher-level code (e.g., auth and webapp handlers) can treat it as an
	// existing directory, matching the semantics of the S3-based implementation.
	if info.IsDir() {
		pathParts := strings.Split(path, "/")
		dirData := models.NewStorageItem(pathParts, "dir", []string{})
		dataJSON, err := dirData.ToJSON()
		if err != nil {
			return "", err
		}
		return dataJSON, nil
	}

	bytes, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	return string(bytes), nil
}

func (s *S3Storage) ExistsItem(path string, bucket ...string) (bool, error) {
	fp := s.fullPath(path, bucket...)
	_, err := os.Stat(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Storage) DeleteItem(path string, bucket ...string) error {
	fp := s.fullPath(path, bucket...)
	if err := os.Remove(fp); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func (s *S3Storage) CreateDir(path []string) error {
	if len(path) == 0 {
		return fmt.Errorf("invalid path: cannot be empty")
	}

	// Resolve the actual filesystem directory path for this logical path.
	dirPath := s.fullPath(s.pathToString(path))

	// If something already exists at this path and it's a file, remove it
	// so we can create a directory instead. This heals older runs where
	// directories were stored as metadata files.
	if info, err := os.Stat(dirPath); err == nil {
		if !info.IsDir() {
			if rmErr := os.Remove(dirPath); rmErr != nil {
				return rmErr
			}
		} else {
			// Directory already exists; treat as success (idempotent).
			return nil
		}
	}

	// Create the directory (and parents) on the local filesystem.
	return os.MkdirAll(dirPath, 0o755)
}

func (s *S3Storage) DeleteDir(path []string) error {
	// Recursively delete all files under the directory path in the local filesystem.
	spath := s.pathToString(path)
	root := s.baseDir()
	dirPrefix := filepath.Join(root, filepath.FromSlash(spath))

	// If directory doesn't exist, treat as success.
	if _, err := os.Stat(dirPrefix); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(dirPrefix)
}

func (s *S3Storage) GetFile(path []string) (*models.StorageItem, error) {
	spath := s.pathToString(path)
	data, err := s.GetItem(spath)
	if err != nil {
		return nil, err
	}

	return models.StorageItemFromJSON(data)
}

func (s *S3Storage) CreateFile(path []string, data string) error {
	// Check if parent directory exists
	if len(path) <= 1 {
		return fmt.Errorf("invalid path: must have parent directory")
	}

	parentPath := path[:len(path)-1]
	parentItem, err := s.GetFile(parentPath)
	if err != nil {
		return fmt.Errorf("parent directory does not exist")
	}

	// Check if file already exists
	spath := s.pathToString(path)
	exists, err := s.ExistsItem(spath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("file already exists")
	}

	// Create file metadata
	fileData := models.NewStorageItem(path, "file", data)
	dataJSON, err := fileData.ToJSON()
	if err != nil {
		return err
	}

	// Save the file
	err = s.PutItem(spath, dataJSON)
	if err != nil {
		return err
	}

	// Update parent directory
	fileName := path[len(path)-1]

	// Parse parent directory data
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

	// Save updated parent directory
	parentJSON, err := parentItem.ToJSON()
	if err != nil {
		return err
	}

	parentSPath := s.pathToString(parentPath)
	return s.PutItem(parentSPath, parentJSON)
}

func (s *S3Storage) UpdateFile(path []string, data string) error {
	// Check if file exists
	fileItem, err := s.GetFile(path)
	if err != nil {
		return err
	}
	if fileItem.Type != "file" {
		return fmt.Errorf("path is not a file")
	}

	// Update file data
	fileItem.Data = data
	dataJSON, err := fileItem.ToJSON()
	if err != nil {
		return err
	}

	spath := s.pathToString(path)
	return s.PutItem(spath, dataJSON)
}

func (s *S3Storage) DeleteFile(path []string) error {
	// Get file to ensure it exists and is a file
	fileItem, err := s.GetFile(path)
	if err != nil {
		return err
	}
	if fileItem.Type != "file" {
		return fmt.Errorf("path is not a file")
	}

	// Update parent directory
	if len(path) > 1 {
		parentPath := path[:len(path)-1]
		parentItem, err := s.GetFile(parentPath)
		if err != nil {
			return err
		}

		fileName := path[len(path)-1]

		// Parse parent directory data
		var filesList []string
		if parentData, ok := parentItem.Data.([]interface{}); ok {
			for _, item := range parentData {
				if str, ok := item.(string); ok && str != fileName {
					filesList = append(filesList, str)
				}
			}
		}

		parentItem.Data = filesList

		// Save updated parent directory
		parentJSON, err := parentItem.ToJSON()
		if err != nil {
			return err
		}

		parentSPath := s.pathToString(parentPath)
		err = s.PutItem(parentSPath, parentJSON)
		if err != nil {
			return err
		}
	}

	// Delete the file
	spath := s.pathToString(path)
	return s.DeleteItem(spath)
}

