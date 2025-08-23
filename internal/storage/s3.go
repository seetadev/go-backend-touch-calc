package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
)

type S3Storage struct {
	client     *s3.Client
	bucketName string
}

func NewS3Storage(bucketName string) (*S3Storage, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	return &S3Storage{
		client:     client,
		bucketName: bucketName,
	}, nil
}

func (s *S3Storage) pathToString(path []string) string {
	return strings.Join(path, "/")
}

func (s *S3Storage) PutItem(path string, data string, bucket ...string) error {
	bucketName := s.bucketName
	if len(bucket) > 0 && bucket[0] != "" {
		bucketName = bucket[0]
	}

	_, err := s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(path),
		Body:   strings.NewReader(data),
	})

	return err
}

func (s *S3Storage) GetItem(path string, bucket ...string) (string, error) {
	bucketName := s.bucketName
	if len(bucket) > 0 && bucket[0] != "" {
		bucketName = bucket[0]
	}

	result, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(path),
	})
	if err != nil {
		return "", err
	}
	defer result.Body.Close()

	// Read the content
	var content strings.Builder
	buffer := make([]byte, 1024)
	for {
		n, err := result.Body.Read(buffer)
		if n > 0 {
			content.Write(buffer[:n])
		}
		if err != nil {
			break
		}
	}

	return content.String(), nil
}

func (s *S3Storage) ExistsItem(path string, bucket ...string) (bool, error) {
	bucketName := s.bucketName
	if len(bucket) > 0 && bucket[0] != "" {
		bucketName = bucket[0]
	}

	_, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(path),
	})

	if err != nil {
		// Check if the error is "not found"
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s *S3Storage) DeleteItem(path string, bucket ...string) error {
	bucketName := s.bucketName
	if len(bucket) > 0 && bucket[0] != "" {
		bucketName = bucket[0]
	}

	_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(path),
	})

	return err
}

func (s *S3Storage) CreateDir(path []string) error {
	spath := s.pathToString(path)
	
	// Check if directory already exists
	exists, err := s.ExistsItem(spath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("directory already exists")
	}

	// Create directory metadata
	dirData := models.NewStorageItem(path, "dir", []string{})
	dataJSON, err := dirData.ToJSON()
	if err != nil {
		return err
	}

	return s.PutItem(spath, dataJSON)
}

func (s *S3Storage) DeleteDir(path []string) error {
	// TODO: Implement directory deletion
	// This should recursively delete all files in the directory
	return fmt.Errorf("delete directory not implemented")
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