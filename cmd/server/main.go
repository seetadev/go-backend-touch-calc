package main

import (
	"log"
	"net/http"
	"os"

	"github.com/c4gt/tornado-nginx-go-backend/internal/config"
	"github.com/c4gt/tornado-nginx-go-backend/internal/handlers"
	"github.com/c4gt/tornado-nginx-go-backend/internal/storage"
	"github.com/c4gt/tornado-nginx-go-backend/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Load configuration (from environment variables / .env)
	cfg := config.Load()

	// Debug: Print configuration
	log.Printf("Environment: %s", cfg.Environment)
	log.Printf("Primary storage backend: %s", cfg.StorageBackend)
	log.Printf("MongoDB URI: %s", cfg.MongoURI)
	log.Printf("MySQL DSN: %s", cfg.MySQLDSN)

	// ------------------------------------------------------------------
	// Initialize storage backends and StorageManager
	// ------------------------------------------------------------------

	// Initialize MongoDB storage backend (for spreadsheets and path-based storage)
	mongoStore, err := storage.NewMongoStorage(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		log.Printf("Failed to initialize MongoDB storage backend: %v", err)
	} else {
		log.Printf("MongoDB storage backend initialized (db=%s)", cfg.MongoDatabase)
	}

	// Initialize MySQL storage backend
	mysqlStore, err := storage.NewMySQLStorage(cfg.MySQLDSN)
	if err != nil {
		log.Printf("Failed to initialize MySQL storage backend: %v", err)
	} else {
		log.Printf("MySQL storage backend initialized (dsn=%s)", cfg.MySQLDSN)
	}

	// Create StorageManager and register both backends (if available).
	storageManager := storage.NewStorageManager()

	if mongoStore != nil {
		mongoBackend := storage.NewMongoSpreadsheetBackend(mongoStore, "touchcalc")
		if err := storageManager.RegisterBackend("mongodb", mongoBackend); err != nil {
			log.Printf("Failed to register MongoDB backend with StorageManager: %v", err)
		}
	}

	if mysqlStore != nil {
		// MySQLStorage already implements the StorageBackend interface.
		if err := storageManager.RegisterBackend("mysql", mysqlStore); err != nil {
			log.Printf("Failed to register MySQL backend with StorageManager: %v", err)
		}
	}

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Apply middleware
	router.Use(middleware.CORS())
	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())

	// Initialize handlers
	// handler := handlers.NewHandler(cfg) // legacy single-backend initialization
	handler := handlers.NewHandlerWithStorageManager(cfg, storageManager)

	// Setup routes
	setupRoutes(router, handler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("Primary storage backend: %s", cfg.StorageBackend)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func setupRoutes(router *gin.Engine, handler *handlers.Handler) {
	// ------------------------------------------------------------------
	// Static assets
	// ------------------------------------------------------------------

	// Serve all static assets from ./web/static under /static/
	router.Static("/static", "./web/static")
	// Convenience mounts for JS/CSS/Images if referenced directly
	router.StaticFS("/js", http.Dir("./web/static/js"))
	router.StaticFS("/css", http.Dir("./web/static/css"))
	router.StaticFS("/images", http.Dir("./web/static/images"))

	// Existing HTML templates for legacy handlers.
	// Use configured templates path so it works both locally and in Docker.
	router.LoadHTMLGlob(handler.Config.TemplatesPath + "/*")

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "tornado-nginx-go-backend",
			"storage": handler.Config.StorageBackend,
		})
	})

	// ------------------------------------------------------------------
	// Public routes (no authentication required)
	// ------------------------------------------------------------------
	public := router.Group("/")
	{
		// Multi-purpose auth endpoint (JSON-style)
		public.POST("/iauth", handler.Auth.HandleAuth)

		// Authentication pages
		public.GET("/login", handler.Auth.HandleLoginGet)
		public.POST("/login", handler.Auth.HandleLogin)
		public.GET("/register", handler.Auth.HandleRegisterGet)
		public.POST("/register", handler.Auth.HandleRegister)

		// Password reset / lost password flows
		public.GET("/pwreset", handler.Auth.HandlePasswordResetGet)
		public.POST("/pwreset", handler.Auth.HandlePasswordResetPost)
		public.GET("/browser", handler.App.HandleLanding)

		// Spreadsheet web app route:
		// GET /browser/:app/:code/:file
		// AppHandler itself checks authentication via cookies and will
		// redirect to /browser if the user is not logged in.
		public.GET("/browser/:app/:code/:file", handler.App.HandleAmazonWebApp)

		// Google verification and similar static-template checks
		public.GET("/browser/static/*filepath", handler.App.HandleGoogleVerification)
	}

	// ------------------------------------------------------------------
	// Protected routes (authentication required)
	// ------------------------------------------------------------------
	protected := router.Group("/")
	protected.Use(middleware.Authentication())
	{
		// Logout (also allowed via POST if called from forms/JS)
		protected.GET("/logout", handler.Auth.HandleLogout)
		protected.POST("/logout", handler.Auth.HandleLogout)

		// API routes for spreadsheet/webapp save/load/list/etc.
		// These are action-based and handled by WebAppHandler.
		protected.POST("/iwebapp", handler.WebApp.HandleWebApp)

		// Email routes (run-as emailer) - protected so only logged-in users can send.
		protected.POST("/irunasemailer", handler.Email.HandleRunAsEmail)

		// Dropbox integration routes (protected)
		protected.GET("/browser/:app/dropbox", handler.Dropbox.HandleDropboxGet)
		protected.POST("/browser/:app/dropbox", handler.Dropbox.HandleDropboxPost)
	}
}
