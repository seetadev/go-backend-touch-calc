package main

import (
	"log"
	"net/http"
	"os"

	"github.com/c4gt/tornado-nginx-go-backend/internal/config"
	"github.com/c4gt/tornado-nginx-go-backend/internal/handlers"
	"github.com/c4gt/tornado-nginx-go-backend/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Load configuration
	cfg := config.Load()

	// Debug: Print configuration
    log.Printf("Storage backend: %s", cfg.StorageBackend)
    log.Printf("MongoDB URI: %s", cfg.MongoURI)
    log.Printf("MySQL DSN: %s", cfg.MySQLDSN)

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
	handler := handlers.NewHandler(cfg)

	// Setup routes
	setupRoutes(router, handler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("Storage backend: %s", cfg.StorageBackend)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func setupRoutes(router *gin.Engine, handler *handlers.Handler) {
	// Static files
	router.Static("/static", "./web/static")

	router.StaticFS("/js", http.Dir("./web/static/js"))
	router.StaticFS("/css", http.Dir("./web/static/css"))
	router.StaticFS("/images", http.Dir("./web/static/images"))

	router.LoadHTMLGlob("web/templates/*")

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"service": "tornado-nginx-go-backend",
			"storage": handler.Config.StorageBackend,
		})
	})

	// API routes
	api := router.Group("/")
	{
		// Authentication routes
		api.POST("/iauth", handler.Auth.HandleAuth)
		api.GET("/login", handler.Auth.HandleLoginGet)
		api.POST("/login", handler.Auth.HandleLogin)
		api.GET("/register", handler.Auth.HandleRegisterGet)
		api.POST("/register", handler.Auth.HandleRegister)
		api.GET("/logout", handler.Auth.HandleLogout)
		api.POST("/logout", handler.Auth.HandleLogout)
		api.GET("/pwreset", handler.Auth.HandlePasswordResetGet)
		api.POST("/pwreset", handler.Auth.HandlePasswordResetPost)

		// Web app routes
		api.POST("/iwebapp", handler.WebApp.HandleWebApp)
		
		// Email routes
		api.POST("/irunasemailer", handler.Email.HandleRunAsEmail)

		// Browser/app routes
		api.GET("/browser", handler.App.HandleLanding)
		api.GET("/browser/:param1/:paramCode/:param2", handler.App.HandleAmazonWebApp)
		api.GET("/browser/:param1/dropbox", handler.Dropbox.HandleDropboxGet)
		api.POST("/browser/:param1/dropbox", handler.Dropbox.HandleDropboxPost)
		
		// Generic browser verification
		api.GET("/browser/static/*filepath", handler.App.HandleGoogleVerification)
	}
}