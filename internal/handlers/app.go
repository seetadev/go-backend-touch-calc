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
    user := h.getCurrentUser(c)
    fmt.Printf("DEBUG: Landing page - current user: '%s'\n", user)
    
    // Get session info for debugging
    sessionID, _ := c.Cookie("session")
    
    templateData := gin.H{
        "user": user,
        "storage_backend": h.handler.Config.StorageBackend,
        "environment": h.handler.Config.Environment,
        "session_id": sessionID,
        "debug": h.handler.Config.Environment == "development",
    }
    
    c.HTML(http.StatusOK, "landing-page.html", templateData)
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

    // Check if user is logged in
    user := h.getCurrentUser(c)
    if user == "" {
        c.Redirect(http.StatusFound, "/browser")
        return
    }

    // Get or create session
    sessionID := h.getOrCreateSession(c, param1)

    if param2 == "index.html" {
        h.handleWebAppIndex(c, param1, paramCode, sessionID, user)
    } else if param2 == "appsplash.png" {
        h.handleAppSplash(c, param1)
    } else {
        h.handleStaticFile(c, param2)
    }
}

func (h *AppHandler) handleWebAppIndex(c *gin.Context, appName, paramCode, sessionID, user string) {
    mscPath := "webappTemplates/"
    
    // Try to load existing spreadsheet data from storage first
    var mscData []byte
    user = h.getCurrentUser(c)
    if user != "" {
        // Try to load existing file from storage
        path := []string{"home", user, "securestore", appName, appName + ".msc"}
        item, err := h.handler.Storage.GetFile(path)
        if err == nil && item != nil {
            if dataStr, ok := item.Data.(string); ok {
                var fileData map[string]interface{}
                if err := json.Unmarshal([]byte(dataStr), &fileData); err == nil {
                    if content, exists := fileData["content"]; exists {
                        if contentStr, ok := content.(string); ok {
                            mscData = []byte(contentStr)
                        }
                    }
                }
            }
        }
    }

    // If no stored data found, try file system
    if len(mscData) == 0 {
        mscFile := filepath.Join(mscPath, appName, appName+".msc.txt")
        var err error
        mscData, err = ioutil.ReadFile(mscFile)
        if err != nil {
            mscData = []byte("A1:Welcome to TouchCalc\nB1:Hello " + user + "\nA2:Start editing here\nB2:Your data auto-saves")
        }
    }

    // Read config file
    configFile := filepath.Join(mscPath, appName, appName+".config.txt")
    configData, err := ioutil.ReadFile(configFile)
    var config map[string]interface{}
    
    if err != nil {
        // Default config if file doesn't exist
        config = map[string]interface{}{
            "code": paramCode,
            "footers": []string{"Sheet1", "Sheet2", "Sheet3", "Sheet4", "Sheet5", "Sheet6", "Sheet7"},
        }
    } else {
        err = json.Unmarshal(configData, &config)
        if err != nil {
            config = map[string]interface{}{
                "code": paramCode,
                "footers": []string{"Sheet1", "Sheet2", "Sheet3", "Sheet4", "Sheet5", "Sheet6", "Sheet7"},
            }
        }
    }

    // Validate code (skip validation if no code specified)
    if code, ok := config["code"].(string); ok && code != "" && code != paramCode {
        c.String(http.StatusUnauthorized, "Invalid access code")
        return
    }

    // Get footers
    footers := []string{"Sheet1", "Sheet2", "Sheet3", "Sheet4", "Sheet5", "Sheet6", "Sheet7"}
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
    session.SetValue("user", user)
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
        "user":          user,
        "storage":       h.handler.Config.StorageBackend,
    })
}

func (h *AppHandler) handleAppSplash(c *gin.Context, appName string) {
    mscPath := "webappTemplates/"
    splashFile := filepath.Join(mscPath, appName, "appsplash.png")
    
    data, err := ioutil.ReadFile(splashFile)
    if err != nil {
        // Return a default 1x1 PNG if splash doesn't exist
        c.Header("Content-Type", "image/png")
        c.Data(http.StatusOK, "image/png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
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
    cookieSessionPath := "/browser/" + appName

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

func (h *AppHandler) getCurrentUser(c *gin.Context) string {
    userCookie, err := c.Cookie("user")
    if err != nil {
        return ""
    }

    // Handle both JSON format and plain text format
    if len(userCookie) > 0 && userCookie[0] == '"' && userCookie[len(userCookie)-1] == '"' {
        // JSON format
        var user string
        err = json.Unmarshal([]byte(userCookie), &user)
        if err != nil {
            return ""
        }
        return user
    }
    
    // Plain text format
    return userCookie
}
