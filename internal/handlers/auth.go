package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/c4gt/tornado-nginx-go-backend/internal/auth"
	"github.com/c4gt/tornado-nginx-go-backend/internal/email"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	handler *Handler
	service *auth.Service
}

func NewAuthHandler(h *Handler, service *auth.Service) *AuthHandler {
	return &AuthHandler{
		handler: h,
		service: service,
	}
}

// AuthRequest represents the request structure for authentication
type AuthRequest struct {
	Action   string `json:"action" form:"action"`
	Email    string `json:"email" form:"email"`
	Password string `json:"pwd" form:"pwd"`
}

// HandleAuth handles the /iauth endpoint
func (h *AuthHandler) HandleAuth(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	switch req.Action {
	case "login":
		h.handleLogin(c, req.Email, req.Password)
	case "register":
		h.handleRegister(c, req.Email, req.Password)
	case "logout":
		h.handleLogout(c)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
	}
}

// HandleLogin handles login requests
func (h *AuthHandler) HandleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" form:"email"`
		Password string `json:"password" form:"password"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	h.handleLogin(c, req.Email, req.Password)
}

// HandleRegister handles registration requests
func (h *AuthHandler) HandleRegister(c *gin.Context) {
	var req struct {
		Email    string `json:"email" form:"email"`
		Password string `json:"password" form:"password"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	h.handleRegister(c, req.Email, req.Password)
}

// HandleLogout handles logout requests
func (h *AuthHandler) HandleLogout(c *gin.Context) {
	h.handleLogout(c)
}

func (h *AuthHandler) handleLogin(c *gin.Context, email, password string) {
	if !auth.ValidateEmail(email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "usererror",
			"result": "fail",
		})
		return
	}

	authenticated, err := h.service.AuthenticateUser(email, password)
	if err != nil {
		exists, _ := h.service.UserExists(email)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"data":   "usererror",
				"result": "fail",
			})
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"data":   "authfail",
				"result": "fail",
			})
		}
		return
	}

	if authenticated {
		h.setCurrentUser(c, email)
		c.JSON(http.StatusOK, gin.H{
			"data":   "success",
			"result": "ok",
		})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{
			"data":   "authfail",
			"result": "fail",
		})
	}
}

func (h *AuthHandler) handleRegister(c *gin.Context, email, password string) {
	if !auth.ValidateEmail(email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "usererror",
			"result": "fail",
		})
		return
	}

	exists, err := h.service.UserExists(email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"data":   "error",
			"result": "fail",
		})
		return
	}

	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"data":   "userexists",
			"result": "fail",
		})
		return
	}

	// Create user directory
	userDir := []string{"home", "users"}
	_, err = h.handler.Storage.GetFile(userDir)
	if err != nil {
		err = h.handler.Storage.CreateDir(userDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"data":   "error",
				"result": "fail",
			})
			return
		}
	}

	err = h.service.CreateUser(email, password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"data":   "error",
			"result": "fail",
		})
		return
	}

	// Create user home directory
	userHomePath := []string{"home", email}
	err = h.handler.Storage.CreateDir(userHomePath)
	if err != nil {
		// Log error but don't fail the registration
		fmt.Printf("Failed to create user home directory: %v\n", err)
	}

	h.setCurrentUser(c, email)
	c.JSON(http.StatusOK, gin.H{
		"data":   "success",
		"result": "ok",
	})
}

func (h *AuthHandler) handleLogout(c *gin.Context) {
	h.clearCurrentUser(c)
	c.JSON(http.StatusOK, gin.H{
		"result": "ok",
	})
}

// HandlePasswordResetGet handles GET requests for password reset
func (h *AuthHandler) HandlePasswordResetGet(c *gin.Context) {
	user := c.Query("u")
	dongle := c.Query("d")

	if user == "" || dongle == "" {
		c.HTML(http.StatusBadRequest, "pwreset-invalid.html", gin.H{
			"user":    nil,
			"reguser": user,
		})
		return
	}

	userDongle, err := h.service.GetUserDongle(user)
	if err != nil || userDongle != dongle {
		c.HTML(http.StatusBadRequest, "pwreset-invalid.html", gin.H{
			"user":    nil,
			"reguser": user,
		})
		return
	}

	c.HTML(http.StatusOK, "pwreset.html", gin.H{
		"user":    nil,
		"reguser": user,
	})
}

// HandlePasswordResetPost handles POST requests for password reset
func (h *AuthHandler) HandlePasswordResetPost(c *gin.Context) {
	var req struct {
		Email    string `json:"email" form:"email"`
		Password string `json:"password" form:"password"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "pwreset-invalid.html", gin.H{
			"user":    nil,
			"reguser": req.Email,
		})
		return
	}

	exists, err := h.service.UserExists(req.Email)
	if err != nil || !exists {
		c.HTML(http.StatusBadRequest, "lostpassword-baduser.html", gin.H{
			"user":    nil,
			"reguser": req.Email,
		})
		return
	}

	err = h.service.UpdatePassword(req.Email, req.Password)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "pwreset-invalid.html", gin.H{
			"user":    nil,
			"reguser": req.Email,
		})
		return
	}

	c.HTML(http.StatusOK, "pwreset-ok.html", gin.H{
		"user":    nil,
		"reguser": req.Email,
	})
}

// HandleLostPassword handles lost password requests
func (h *AuthHandler) HandleLostPassword(c *gin.Context) {
	if c.Request.Method == "GET" {
		c.HTML(http.StatusOK, "lostpassword.html", gin.H{
			"user": nil,
		})
		return
	}

	var req struct {
		Email string `json:"email" form:"email"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "lostpassword.html", gin.H{
			"user": nil,
		})
		return
	}

	exists, err := h.service.UserExists(req.Email)
	if err != nil || !exists {
		c.HTML(http.StatusBadRequest, "lostpassword-baduser.html", gin.H{
			"user":    nil,
			"reguser": req.Email,
		})
		return
	}

	// Generate dongle and send email
	dongle := h.generateRandomString(20)
	err = h.service.SetUserDongle(req.Email, dongle)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "lostpassword.html", gin.H{
			"user": nil,
		})
		return
	}

	// Send password reset email
	err = h.sendLostPasswordEmail(req.Email, dongle, c.Request.Host)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "lostpassword.html", gin.H{
			"user": nil,
		})
		return
	}

	c.HTML(http.StatusOK, "lostpassword-sentemail.html", gin.H{
		"user":    nil,
		"reguser": req.Email,
	})
}

func (h *AuthHandler) setCurrentUser(c *gin.Context, user string) {
	userJSON, _ := json.Marshal(user)
	// Set secure cookie
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("user", string(userJSON), 3600*24, "/", "", false, true)
}

func (h *AuthHandler) clearCurrentUser(c *gin.Context) {
	c.SetCookie("user", "", -1, "/", "", false, true)
}

func (h *AuthHandler) generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

func (h *AuthHandler) sendLostPasswordEmail(userEmail, dongle, host string) error {
	// This would need the email service to be implemented
	// For now, we'll return nil
	link := fmt.Sprintf("http://%s/pwreset?u=%s&d=%s", host, userEmail, dongle)
	message := email.NewMessage()
	message.Subject = "Reset Password"
	message.BodyText = fmt.Sprintf("Please click the following link to reset password for user %s\n%s", userEmail, link)

	// Note: This assumes we have access to the email service and from email
	// We would need to implement this properly with the actual email service
	return nil
}