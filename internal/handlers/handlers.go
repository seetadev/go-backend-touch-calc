package handlers

import (
	"github.com/c4gt/tornado-nginx-go-backend/internal/auth"
	"github.com/c4gt/tornado-nginx-go-backend/internal/config"
	"github.com/c4gt/tornado-nginx-go-backend/internal/email"
	"github.com/c4gt/tornado-nginx-go-backend/internal/session"
	"github.com/c4gt/tornado-nginx-go-backend/internal/storage"
)

// Handler contains all the route handlers
type Handler struct {
	Config    *config.Config
	Storage   storage.Storage
	Auth      *AuthHandler
	WebApp    *WebAppHandler
	Email     *EmailHandler
	App       *AppHandler
	Dropbox   *DropboxHandler
	Session   *session.Manager
}

// NewHandler creates a new handler instance
func NewHandler(cfg *config.Config) *Handler {
	// Initialize storage
	s3Storage, err := storage.NewS3Storage(cfg.S3Bucket)
	if err != nil {
		panic("Failed to initialize S3 storage: " + err.Error())
	}

	// Initialize services
	authService := auth.NewService(s3Storage)
	emailService, err := email.NewSESService()
	if err != nil {
		panic("Failed to initialize email service: " + err.Error())
	}
	
	sessionManager := session.NewManager()

	// Create handler
	h := &Handler{
		Config:  cfg,
		Storage: s3Storage,
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