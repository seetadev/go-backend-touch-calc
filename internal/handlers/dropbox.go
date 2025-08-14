package handlers

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "path/filepath"

    "github.com/c4gt/tornado-nginx-go-backend/internal/session"
    "github.com/gin-gonic/gin"
)

type DropboxHandler struct {
    handler *Handler
}

func NewDropboxHandler(h *Handler) *DropboxHandler {
    return &DropboxHandler{
        handler: h,
    }
}

type DropboxConfig struct {
    Key    string `json:"key"`
    Secret string `json:"secret"`
}

type DropboxRequest struct {
    Action     string `json:"action" form:"action"`
    String     string `json:"string" form:"string"`
    Name       string `json:"name" form:"name"`
    FName      string `json:"fname" form:"fname"`
    SessionID  string `json:"sessionid" form:"sessionid"`
}

func (h *DropboxHandler) HandleDropboxGet(c *gin.Context) {
    param1 := c.Param("param1")
    action := c.Query("action")
    sessionID, _ := c.Cookie("session")

    sessionObj := h.handler.Session.GetOrCreate(sessionID)

    switch action {
    case "dropbox-auth-start":
        h.handleDropboxAuthStart(c, param1, sessionID, sessionObj)
    case "dropbox-auth-finish":
        h.handleDropboxAuthFinish(c, param1, sessionID, sessionObj)
    case "getLogin":
        h.handleGetLogin(c, sessionObj)
    case "logout":
        h.handleDropboxLogout(c, sessionID, sessionObj)
    default:
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
    }
}

func (h *DropboxHandler) HandleDropboxPost(c *gin.Context) {
    sessionID, _ := c.Cookie("session")
    sessionObj := h.handler.Session.GetOrCreate(sessionID)

    var req DropboxRequest
    if err := c.ShouldBind(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }

    // Check if user is logged in to Dropbox
    token, exists := sessionObj.GetString("dbToken")
    if !exists || token == "" {
        c.JSON(http.StatusUnauthorized, gin.H{
            "data": "Please login to dropbox",
        })
        return
    }

    switch req.Action {
    case "upload":
        h.handleDropboxUpload(c, req, token)
    case "listdir":
        h.handleDropboxListDir(c, token)
    case "view":
        h.handleDropboxView(c, req, token)
    case "delete":
        h.handleDropboxDelete(c, req, token)
    case "logout":
        h.handleDropboxLogoutPost(c, sessionID, sessionObj)
    default:
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
    }
}

func (h *DropboxHandler) handleDropboxAuthStart(c *gin.Context, appName, sessionID string, sessionObj *session.Session) {
    // Get Dropbox config for the app
    config, err := h.getDropboxConfig(appName)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load Dropbox config"})
        return
    }

    // Clear existing Dropbox session data
    sessionObj.RemoveValue("dbLogin")
    sessionObj.RemoveValue("dbToken")
    h.handler.Session.Set(sessionID, sessionObj)

    // Generate OAuth URL (simplified - in a real implementation, you'd use the Dropbox SDK)
    redirectURI := fmt.Sprintf("https://%s/browser/%s/dropbox?action=dropbox-auth-finish", c.Request.Host, appName)
    authorizeURL := fmt.Sprintf("https://www.dropbox.com/oauth2/authorize?client_id=%s&redirect_uri=%s&response_type=code", 
        config.Key, redirectURI)

    c.JSON(http.StatusOK, gin.H{
        "url": authorizeURL,
    })
}

func (h *DropboxHandler) handleDropboxAuthFinish(c *gin.Context, appName, sessionID string, sessionObj *session.Session) {
    code := c.Query("code")
    if code == "" {
        appURL, _ := sessionObj.GetString("appUrl")
        if appURL != "" {
            c.Redirect(http.StatusFound, appURL)
        } else {
            c.JSON(http.StatusBadRequest, gin.H{"error": "No authorization code"})
        }
        return
    }

    // Simplified - in reality, you'd exchange the code for a token
    // For now, we'll simulate a successful auth
    sessionObj.SetValue("dbToken", "simulated_access_token")
    sessionObj.SetValue("dbLogin", "1")
    h.handler.Session.Set(sessionID, sessionObj)

    appURL, _ := sessionObj.GetString("appUrl")
    if appURL != "" {
        c.Redirect(http.StatusFound, appURL)
    } else {
        c.JSON(http.StatusOK, gin.H{"status": "success"})
    }
}

func (h *DropboxHandler) handleGetLogin(c *gin.Context, sessionObj *session.Session) {
    login, exists := sessionObj.GetString("dbLogin")
    if !exists {
        login = ""
    }
    
    c.JSON(http.StatusOK, gin.H{
        "login": login,
    })
}

func (h *DropboxHandler) handleDropboxLogout(c *gin.Context, sessionID string, sessionObj *session.Session) {
    sessionObj.RemoveValue("dbLogin")
    sessionObj.RemoveValue("dbToken")
    h.handler.Session.Set(sessionID, sessionObj)
    
    c.JSON(http.StatusOK, gin.H{
        "status": 1,
    })
}

func (h *DropboxHandler) handleDropboxUpload(c *gin.Context, req DropboxRequest, token string) {
    // In a real implementation, you would use the Dropbox API to upload the file
    // For now, we'll simulate a successful upload
    c.JSON(http.StatusOK, gin.H{
        "data": "Done",
    })
}

func (h *DropboxHandler) handleDropboxListDir(c *gin.Context, token string) {
    // In a real implementation, you would use the Dropbox API to list directory contents
    // For now, we'll return a simulated response
    c.JSON(http.StatusOK, gin.H{
        "contents": []map[string]interface{}{
            {
                "name":     "example.txt",
                "is_dir":   false,
                "size":     1024,
            },
        },
    })
}

func (h *DropboxHandler) handleDropboxView(c *gin.Context, req DropboxRequest, token string) {
    // In a real implementation, you would use the Dropbox API to download and return the file
    // For now, we'll return simulated file content
    c.JSON(http.StatusOK, gin.H{
        "text": "Simulated file content",
    })
}

func (h *DropboxHandler) handleDropboxDelete(c *gin.Context, req DropboxRequest, token string) {
    // In a real implementation, you would use the Dropbox API to delete the file
    // For now, we'll simulate a successful deletion
    c.JSON(http.StatusOK, gin.H{
        "data": "Done",
    })
}

func (h *DropboxHandler) handleDropboxLogoutPost(c *gin.Context, sessionID string, sessionObj *session.Session) {
    sessionObj.RemoveValue("dbLogin")
    sessionObj.RemoveValue("dbToken")
    h.handler.Session.Set(sessionID, sessionObj)
    
    c.JSON(http.StatusOK, gin.H{
        "data": "Done",
    })
}

func (h *DropboxHandler) getDropboxConfig(appName string) (*DropboxConfig, error) {
    mscPath := "webappTemplates/"
    configFile := filepath.Join(mscPath, appName, appName+".config.txt")
    
    data, err := ioutil.ReadFile(configFile)
    if err != nil {
        // Return default config if file doesn't exist
        return &DropboxConfig{
            Key:    "default_key",
            Secret: "default_secret",
        }, nil
    }

    var config map[string]interface{}
    err = json.Unmarshal(data, &config)
    if err != nil {
        return &DropboxConfig{
            Key:    "default_key",
            Secret: "default_secret",
        }, nil
    }

    dropboxConfig, ok := config["dropbox"].(map[string]interface{})
    if !ok {
        return &DropboxConfig{
            Key:    "default_key",
            Secret: "default_secret",
        }, nil
    }

    key, ok1 := dropboxConfig["key"].(string)
    secret, ok2 := dropboxConfig["secret"].(string)
    
    if !ok1 || !ok2 {
        return &DropboxConfig{
            Key:    "default_key",
            Secret: "default_secret",
        }, nil
    }

    return &DropboxConfig{
        Key:    key,
        Secret: secret,
    }, nil
}
