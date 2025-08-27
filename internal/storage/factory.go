package storage

import (
    "fmt"
    "log"

    "github.com/c4gt/tornado-nginx-go-backend/internal/config"
)

func NewStorage(cfg *config.Config) (Storage, error) {
    log.Printf("Initializing storage backend: %s", cfg.StorageBackend)
    
    switch cfg.StorageBackend {
    case "mongodb":
        log.Printf("Attempting to connect to MongoDB at: %s", cfg.MongoURI)
        storage, err := NewMongoStorage(cfg.MongoURI, cfg.MongoDatabase)
        if err != nil {
            return nil, fmt.Errorf("failed to initialize MongoDB storage: %w", err)
        }
        log.Printf("Successfully connected to MongoDB")
        return storage, nil
        
    case "mysql":
        log.Printf("Attempting to connect to MySQL with DSN: %s", cfg.MySQLDSN)
        storage, err := NewMySQLStorage(cfg.MySQLDSN)
        if err != nil {
            return nil, fmt.Errorf("failed to initialize MySQL storage: %w", err)
        }
        log.Printf("Successfully connected to MySQL")
        return storage, nil
        
    case "s3":
        if cfg.AWSAccessKey == "" || cfg.AWSSecretKey == "" {
            return nil, fmt.Errorf("AWS credentials required for S3 storage")
        }
        log.Printf("Attempting to connect to AWS S3 bucket: %s", cfg.S3Bucket)
        storage, err := NewS3Storage(cfg.S3Bucket, "", cfg.AWSAccessKey, cfg.AWSSecretKey, cfg.AWSRegion, false)
        if err != nil {
            return nil, fmt.Errorf("failed to initialize S3 storage: %w", err)
        }
        log.Printf("Successfully connected to AWS S3")
        return storage, nil
        
    case "minio":
        if cfg.MinIOAccessKey == "" || cfg.MinIOSecretKey == "" {
            return nil, fmt.Errorf("MinIO credentials required for MinIO storage")
        }
        log.Printf("Attempting to connect to MinIO at: %s, bucket: %s", cfg.MinIOEndpoint, cfg.MinIOBucket)
        useSSL := cfg.MinIOSSL == "true"
        storage, err := NewS3Storage(cfg.MinIOBucket, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.AWSRegion, useSSL)
        if err != nil {
            return nil, fmt.Errorf("failed to initialize MinIO storage: %w", err)
        }
        log.Printf("Successfully connected to MinIO")
        return storage, nil
        
    default:
        return nil, fmt.Errorf("unsupported storage backend: %s", cfg.StorageBackend)
    }
}
