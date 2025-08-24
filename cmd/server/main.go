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
	setupTemplateRoutes(router)
	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func setupRoutes(router *gin.Engine, handler *handlers.Handler) {
	// Static files
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "tornado-nginx-go-backend",
		})
	})

	// API routes
	api := router.Group("/")
	{
		// Authentication routes
		api.POST("/iauth", handler.Auth.HandleAuth)
		api.POST("/login", handler.Auth.HandleLogin)
		api.POST("/register", handler.Auth.HandleRegister)
		api.POST("/logout", handler.Auth.HandleLogout)
		api.GET("/pwreset", handler.Auth.HandlePasswordResetGet)
		api.POST("/pwreset", handler.Auth.HandlePasswordResetPost)

		// Web app routes
		api.POST("/iwebapp", handler.WebApp.HandleWebApp)

		// // Email routes
		// api.POST("/irunasemailer", handler.Email.HandleRunAsEmail)

		// Browser/app routes
		api.GET("/browser", handler.App.HandleLanding)
		api.GET("/browser/:param1/:paramCode/:param2", handler.App.HandleAmazonWebApp)
		api.GET("/browser/:param1/dropbox", handler.Dropbox.HandleDropboxGet)
		api.POST("/browser/:param1/dropbox", handler.Dropbox.HandleDropboxPost)

		// // Generic browser verification
		// api.GET("/browser/", handler.App.HandleGoogleVerification)
	}
}

func setupTemplateRoutes(router *gin.Engine) {
	// Render base.html at /base
	router.GET("/base", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base.html", gin.H{"title": "Aspiring Investments"})
	})

	// Render allusersheets.html at /allusersheets
	router.GET("/allusersheets", func(c *gin.Context) {
		c.HTML(http.StatusOK, "allusersheets.html", gin.H{"title": "All User Sheets"})
	})

	// Render importcollabload.html at /importcollabload
	router.GET("/importcollabload", func(c *gin.Context) {
		c.HTML(http.StatusOK, "importcollabload.html", gin.H{"title": "Import Collab Load"})
	})

	// Render lostpassword.html at /lostpassword
	router.GET("/lostpassword", func(c *gin.Context) {
		c.HTML(http.StatusOK, "lostpassword.html", gin.H{"title": "Lost Password"})
	})
	// Render lostpassword-baduser.html at /lostpassword-baduser
	router.GET("/lostpassword-baduser", func(c *gin.Context) {
		reguser := c.Query("reguser")
		c.HTML(http.StatusOK, "lostpassword-baduser.html", gin.H{
			"title":   "Lost Password - Bad User",
			"reguser": reguser,
		})
	})
	// Render lostpassword-sentemail.html at /lostpassword-sentemail
	router.GET("/lostpassword-sentemail", func(c *gin.Context) {
		reguser := c.Query("reguser")
		c.HTML(http.StatusOK, "lostpassword-sentemail.html", gin.H{
			"title":   "Lost Password - Sent Email",
			"reguser": reguser,
		})
	})

	// Render pwreset-invalid.html at /pwreset-invalid
	router.GET("/pwreset-invalid", func(c *gin.Context) {
		c.HTML(http.StatusOK, "pwreset-invalid.html", gin.H{"title": "Password Reset - Invalid"})
	})
	// Render pwreset-ok.html at /pwreset-ok
	router.GET("/pwreset-ok", func(c *gin.Context) {
		c.HTML(http.StatusOK, "pwreset-ok.html", gin.H{"title": "Password Reset - OK"})
	})

	// Render userlogin.html at /userlogin
	router.GET("/userlogin", func(c *gin.Context) {
		c.HTML(http.StatusOK, "userlogin.html", gin.H{"title": "User Login"})
	})

	// Render userregister.html at /userregister
	router.GET("/userregister", func(c *gin.Context) {
		c.HTML(http.StatusOK, "userregister.html", gin.H{"title": "User Register"})
	})
	// Render userregister-exists.html at /userregister-exists
	router.GET("/userregister-exists", func(c *gin.Context) {
		c.HTML(http.StatusOK, "userregister-exists.html", gin.H{"title": "User Register - Exists"})
	})
	// Render userregister-ok.html at /userregister-ok
	router.GET("/userregister-ok", func(c *gin.Context) {
		c.HTML(http.StatusOK, "userregister-ok.html", gin.H{"title": "User Register - OK"})
	})
}
