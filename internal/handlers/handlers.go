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
	// StorageManager provides multi-backend spreadsheet storage (e.g.,
	// MongoDB + MySQL). It is optional and may be nil in some contexts
	// (tests, legacy initialization).
	StorageManager *storage.StorageManager
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

	// Initialize email service.
	// NOTE: The underlying SESService is now a console-logging stub, so this
	// does not require real AWS credentials for local development.
	emailService, err := email.NewSESService()
		if err != nil {
		log.Printf("Failed to initialize email service (stub): %v", err)
			log.Println("Email functionality will be disabled")
	} else {
		log.Println("Email service (console stub) initialized successfully")
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

// NewHandlerWithStorageManager wraps NewHandler and additionally attaches
// a StorageManager instance so handlers can take advantage of multiple
// underlying storage backends.
func NewHandlerWithStorageManager(cfg *config.Config, mgr *storage.StorageManager) *Handler {
	h := NewHandler(cfg)
	h.StorageManager = mgr
	return h
}

