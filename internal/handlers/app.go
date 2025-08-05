package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type AppHandler struct {
	handler *Handler
}

func NewAppHandler(h *Handler) *AppHandler {
	return &AppHandler{
		handler: h,
	}
}

// HandleLanding handles the landing page
func (h *AppHandler) HandleLanding(c *gin.Context) {
	c.HTML(http.StatusOK, "landing-page.html", gin.H{})
}

// HandleGoogleVerification handles Google verification files
func (h *AppHandler) HandleGoogleVerification(c *gin.Context) {
	slug := c.Param("filepath")
	// Remove leading slash
	if len(slug) > 0 && slug[0] == '/' {
		slug = slug[1:]
	}
	
	c.HTML(http.StatusOK, slug, gin.H{})
}

// HandleAmazonWebApp handles the Amazon web app routes
func (h *AppHandler) HandleAmazonWebApp(c *gin.Context) {
	param1 := c.Param("param1")
	paramCode := c.Param("paramCode")
	param2 := c.Param("param2")

	// Get or create session
	sessionID := h.getOrCreateSession(c, param1)

	if param2 == "index.html" {
		h.handleWebAppIndex(c, param1, paramCode, sessionID)
	} else if param2 == "appsplash.png" {
		h.handleAppSplash(c, param1)
	} else {
		h.handleStaticFile(c, param2)
	}
}

func (h *AppHandler) handleWebAppIndex(c *gin.Context, appName, paramCode, sessionID string) {
	mscPath := "webappTemplates/"
	
	// Read MSC file
	mscFile := filepath.Join(mscPath, appName, appName+".msc.txt")
	mscData, err := ioutil.ReadFile(mscFile)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MSC file not found"})
		return
	}

	// Read config file
	configFile := filepath.Join(mscPath, appName, appName+".config.txt")
	configData, err := ioutil.ReadFile(configFile)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config file not found"})
		return
	}

	var config map[string]interface{}
	err = json.Unmarshal(configData, &config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid config file"})
		return
	}

	// Validate code
	if code, ok := config["code"].(string); !ok || code != paramCode {
		c.String(http.StatusUnauthorized, "Invalid code")
		return
	}

	// Get footers
	footers := []string{"1", "2", "3", "4", "5", "6", "7"}
	if configFooters, ok := config["footers"].([]interface{}); ok {
		footers = make([]string, len(configFooters))
		for i, footer := range configFooters {
			if str, ok := footer.(string); ok {
				footers[i] = str
			}
		}
	}

	// Get session and set app info
	session := h.handler.Session.GetOrCreate(sessionID)
	session.SetValue("appName", appName)
	session.SetValue("appUrl", c.Request.RequestURI)
	h.handler.Session.Set(sessionID, session)

	// Check dropbox login status
	dbLogin := 0
	if login, exists := session.GetString("dbLogin"); exists && login == "1" {
		dbLogin = 1
	}

	// Render template
	c.HTML(http.StatusOK, "amazonwebapp.html", gin.H{
		"fname":         appName,
		"sheetstr":      string(mscData),
		"sheetmscestr":  "",
		"appjsfiles":    "",
		"appstylefiles": "",
		"sheets":        footers,
		"sessionid":     sessionID,
		"dbLogin":       dbLogin,
	})
}

func (h *AppHandler) handleAppSplash(c *gin.Context, appName string) {
	mscPath := "webappTemplates/"
	splashFile := filepath.Join(mscPath, appName, "appsplash.png")
	
	data, err := ioutil.ReadFile(splashFile)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Splash image not found"})
		return
	}

	c.Header("Content-Type", "image/png")
	c.Data(http.StatusOK, "image/png", data)
}

func (h *AppHandler) handleStaticFile(c *gin.Context, filename string) {
	staticPath := filepath.Join(h.handler.Config.StaticPath, "runappios43c", filename)
	
	// Check if file exists
	if _, err := os.Stat(staticPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(staticPath)
}

func (h *AppHandler) getOrCreateSession(c *gin.Context, appName string) string {
	sessionID, err := c.Cookie("session")
	cookieSessionPath := "/webapps/" + appName

	if err != nil || sessionID == "" {
		// Create new session
		sessionID = h.generateRandomString(16)
		c.SetCookie("session", sessionID, 3600*24, cookieSessionPath, "", false, true)
		session := h.handler.Session.GetOrCreate(sessionID)
		h.handler.Session.Set(sessionID, session)
		return sessionID
	}

	// Check if session exists and is valid
	session, exists := h.handler.Session.Get(sessionID)
	if !exists {
		// Session doesn't exist, create new one
		sessionID = h.generateRandomString(16)
		c.SetCookie("session", sessionID, 3600*24, cookieSessionPath, "", false, true)
		session = h.handler.Session.GetOrCreate(sessionID)
		h.handler.Session.Set(sessionID, session)
		return sessionID
	}

	// Check if session is for the right app
	if sessionAppName, exists := session.GetString("appName"); exists && sessionAppName != appName {
		// Wrong app, create new session
		sessionID = h.generateRandomString(16)
		c.SetCookie("session", sessionID, 3600*24, cookieSessionPath, "", false, true)
		session = h.handler.Session.GetOrCreate(sessionID)
		h.handler.Session.Set(sessionID, session)
		return sessionID
	}

	return sessionID
}

func (h *AppHandler) generateRandomString(length int) string {
	// Try to generate unique ID
	for i := 0; i < 100; i++ {
		bytes := make([]byte, length)
		rand.Read(bytes)
		id := base64.URLEncoding.EncodeToString(bytes)[:length]
		
		// Check if ID already exists
		if _, exists := h.handler.Session.Get(id); !exists {
			return id
		}
	}
	
	// Fallback if all attempts failed
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}