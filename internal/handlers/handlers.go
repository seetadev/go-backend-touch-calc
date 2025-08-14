package handlers

import (
    "log"

    "github.com/c4gt/tornado-nginx-go-backend/internal/auth"
    "github.com/c4gt/tornado-nginx-go-backend/internal/config"
    "github.com/c4gt/tornado-nginx-go-backend/internal/email"
    "github.com/c4gt/tornado-nginx-go-backend/internal/session"
    "github.com/c4gt/tornado-nginx-go-backend/internal/storage"
)

type Handler struct {
    Config  *config.Config
    Storage storage.Storage
    Session *session.Manager
    Auth    *AuthHandler
    WebApp  *WebAppHandler
    Email   *EmailHandler
    App     *AppHandler
    Dropbox *DropboxHandler
}

func NewHandler(cfg *config.Config) *Handler {
    // Initialize storage with proper error handling
    storageBackend, err := storage.NewStorage(cfg)
    if err != nil {
        log.Fatalf("Failed to initialize storage backend (%s): %v", cfg.StorageBackend, err)
    }

    // Initialize session manager
    sessionManager := session.NewManager()

    // Initialize auth service
    authService := auth.NewService(storageBackend)

    // Initialize email service (with fallback if AWS not configured)
    var emailService *email.SESService
    if cfg.AWSAccessKey != "" && cfg.AWSSecretKey != "" && 
       cfg.AWSAccessKey != "your_aws_access_key" && cfg.AWSSecretKey != "your_aws_secret_key" {
        emailService, err = email.NewSESService()
        if err != nil {
            log.Printf("Failed to initialize SES email service: %v", err)
            log.Println("Email functionality will be disabled")
        } else {
            log.Println("Email service initialized successfully")
        }
    } else {
        log.Println("AWS credentials not provided or using placeholder values, email functionality disabled")
    }

    h := &Handler{
        Config:  cfg,
        Storage: storageBackend,
        Session: sessionManager,
    }

    // Initialize sub-handlers
    h.Auth = NewAuthHandler(h, authService)
    h.WebApp = NewWebAppHandler(h)
    h.Email = NewEmailHandler(h, emailService)
    h.App = NewAppHandler(h)
    h.Dropbox = NewDropboxHandler(h)

    return h
}
