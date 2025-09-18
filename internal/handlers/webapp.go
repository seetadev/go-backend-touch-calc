package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/gin-gonic/gin"
)

type WebAppHandler struct {
    handler *Handler
}

func NewWebAppHandler(h *Handler) *WebAppHandler {
    return &WebAppHandler{
        handler: h,
    }
}

type WebAppRequest struct {
    Action  string `json:"action" form:"action"`
    AppName string `json:"appname" form:"appname"`
    FName   string `json:"fname" form:"fname"`
    Data    string `json:"data" form:"data"`
    Content string `json:"content" form:"content"`
}

func (h *WebAppHandler) HandleWebApp(c *gin.Context) {
    var req WebAppRequest
    if err := c.ShouldBind(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "error",
            "result": "fail",
        })
        return
    }

    // Get current user from cookie
    user := h.getCurrentUser(c)
    if user == "" {
        c.JSON(http.StatusUnauthorized, gin.H{
            "data":   "usererror",
            "result": "fail",
        })
        return
    }

    // Log the action for debugging
    fmt.Printf("DEBUG: WebApp action: %s, user: %s, app: %s, file: %s\n", 
        req.Action, user, req.AppName, req.FName)

    switch req.Action {
    case "savefile":
        h.handleSaveFile(c, user, req)
    case "getfile":
        h.handleGetFile(c, user, req)
    case "delete-file":
        h.handleDeleteFile(c, user, req)
    case "listdir":
        h.handleListDir(c, user, req)
    case "save-multiple":
        h.handleSaveMultiple(c, user, req)
    case "get-data":
        h.handleGetData(c, user, req)
    case "backup":
        h.handleBackup(c, user, req)
    case "restore":
        h.handleRestore(c, user, req)
    case "save":
        h.handleSocialCalcSave(c, user, req)
    case "load":
        h.handleSocialCalcLoad(c, user, req)
    default:
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "invalid action: " + req.Action,
            "result": "fail",
        })
    }
}

func (h *WebAppHandler) handleSaveFile(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.FName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or fname)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Saving file %s for user %s in app %s\n", req.FName, user, req.AppName)

    path := []string{"home", user, "securestore", req.AppName, req.FName}
    // dirPath := []string{"home", user, "securestore", req.AppName}

    // Ensure entire directory structure exists
    err := h.ensureDirectoryStructure(user, req.AppName)
    if err != nil {
        fmt.Printf("DEBUG: Error ensuring directory structure: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to create directory structure: " + err.Error(),
            "result": "fail",
        })
        return
    }

    // Save the data (include metadata for better debugging)
    fileData := map[string]interface{}{
        "content": req.Data,
        "user": user,
        "app": req.AppName,
        "filename": req.FName,
        "timestamp": fmt.Sprintf("%d", getCurrentTimestamp()),
        "storage_backend": h.handler.Config.StorageBackend,
    }

    dataJSON, err := json.Marshal(fileData)
    if err != nil {
        fmt.Printf("DEBUG: Error marshaling file data: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to encode file data",
            "result": "fail",
        })
        return
    }

    // Check if file exists
    _, err = h.handler.Storage.GetFile(path)
    if err != nil {
        // File doesn't exist, create it
        fmt.Printf("DEBUG: Creating new file: %s\n", req.FName)
        err = h.handler.Storage.CreateFile(path, string(dataJSON))
    } else {
        // File exists, update it
        fmt.Printf("DEBUG: Updating existing file: %s\n", req.FName)
        err = h.handler.Storage.UpdateFile(path, string(dataJSON))
    }

    if err != nil {
        fmt.Printf("DEBUG: Error saving file: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to save file: " + err.Error(),
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: File saved successfully: %s\n", req.FName)
    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
        "timestamp": getCurrentTimestamp(),
    })
}

func (h *WebAppHandler) handleGetFile(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.FName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or fname)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Getting file %s for user %s in app %s\n", req.FName, user, req.AppName)

    path := []string{"home", user, "securestore", req.AppName, req.FName}
    item, err := h.handler.Storage.GetFile(path)
    if err != nil {
        fmt.Printf("DEBUG: File not found: %s, error: %v\n", req.FName, err)
        c.JSON(http.StatusNotFound, gin.H{
            "data":   "file not found: " + req.FName,
            "result": "fail",
        })
        return
    }

    // Handle both old format (direct string) and new format (JSON with metadata)
    var fileContent string
    if dataStr, ok := item.Data.(string); ok {
        // Try to parse as JSON first
        var fileData map[string]interface{}
        if err := json.Unmarshal([]byte(dataStr), &fileData); err == nil {
            // New format with metadata
            if content, exists := fileData["content"]; exists {
                if contentStr, ok := content.(string); ok {
                    fileContent = contentStr
                } else {
                    fileContent = dataStr // Fallback to raw data
                }
            } else {
                fileContent = dataStr // No content field, use raw data
            }
        } else {
            // Old format, direct string
            fileContent = dataStr
        }
    } else {
        // Data is not a string, convert to JSON
        dataBytes, err := json.Marshal(item.Data)
        if err != nil {
            fmt.Printf("DEBUG: Error marshaling item data: %v\n", err)
            c.JSON(http.StatusInternalServerError, gin.H{
                "data":   "failed to read file data",
                "result": "fail",
            })
            return
        }
        fileContent = string(dataBytes)
    }

    fmt.Printf("DEBUG: File retrieved successfully: %s\n", req.FName)
    c.JSON(http.StatusOK, gin.H{
        "data":   fileContent,
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleDeleteFile(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.FName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or fname)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Deleting file %s for user %s in app %s\n", req.FName, user, req.AppName)

    path := []string{"home", user, "securestore", req.AppName, req.FName}
    err := h.handler.Storage.DeleteFile(path)
    if err != nil {
        fmt.Printf("DEBUG: Error deleting file: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to delete file: " + err.Error(),
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: File deleted successfully: %s\n", req.FName)
    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleListDir(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing app name",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Listing directory for user %s in app %s\n", user, req.AppName)

    path := []string{"home", user, "securestore", req.AppName}
    
    // Ensure directory exists
    item, err := h.handler.Storage.GetFile(path)
    if err != nil {
        // Directory doesn't exist, create it and return empty list
        err = h.ensureDirectoryStructure(user, req.AppName)
        if err != nil {
            fmt.Printf("DEBUG: Error creating directory: %v\n", err)
            c.JSON(http.StatusInternalServerError, gin.H{
                "data":   "failed to create directory: " + err.Error(),
                "result": "fail",
            })
            return
        }
        c.JSON(http.StatusOK, gin.H{
            "data":   []string{},
            "result": "ok",
            "storage_backend": h.handler.Config.StorageBackend,
        })
        return
    }

    // Extract file names from directory data
    var fileNames []string
    if data, ok := item.Data.([]interface{}); ok {
        for _, file := range data {
            if str, ok := file.(string); ok {
                fileNames = append(fileNames, str)
            }
        }
    }

    fmt.Printf("DEBUG: Directory listing successful, found %d files\n", len(fileNames))
    c.JSON(http.StatusOK, gin.H{
        "data":   fileNames,
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleSaveMultiple(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.Content == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or content)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Saving multiple files for user %s in app %s\n", user, req.AppName)

    // Parse the content as JSON
    var filesData map[string]interface{}
    err := json.Unmarshal([]byte(req.Content), &filesData)
    if err != nil {
        fmt.Printf("DEBUG: Error parsing content JSON: %v\n", err)
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "invalid JSON content: " + err.Error(),
            "result": "fail",
        })
        return
    }

    // Ensure directory structure exists
    err = h.ensureDirectoryStructure(user, req.AppName)
    if err != nil {
        fmt.Printf("DEBUG: Error ensuring directory structure: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to create directory: " + err.Error(),
            "result": "fail",
        })
        return
    }

    // Save each file
    savedFiles := []string{}
    for filename, content := range filesData {
        if content == nil {
            continue
        }

        path := []string{"home", user, "securestore", req.AppName, filename}
        
        // Create file data with metadata
        fileData := map[string]interface{}{
            "content": content,
            "user": user,
            "app": req.AppName,
            "filename": filename,
            "timestamp": fmt.Sprintf("%d", getCurrentTimestamp()),
            "storage_backend": h.handler.Config.StorageBackend,
        }

        contentStr, err := json.Marshal(fileData)
        if err != nil {
            fmt.Printf("DEBUG: Error marshaling file data for %s: %v\n", filename, err)
            continue
        }

        // Check if file exists
        _, err = h.handler.Storage.GetFile(path)
        if err != nil {
            // File doesn't exist, create it
            err = h.handler.Storage.CreateFile(path, string(contentStr))
        } else {
            // File exists, update it
            err = h.handler.Storage.UpdateFile(path, string(contentStr))
        }

        if err != nil {
            fmt.Printf("DEBUG: Error saving file %s: %v\n", filename, err)
            c.JSON(http.StatusInternalServerError, gin.H{
                "data":   "failed to save file: " + filename + " - " + err.Error(),
                "result": "fail",
            })
            return
        }
        
        savedFiles = append(savedFiles, filename)
    }

    fmt.Printf("DEBUG: Successfully saved %d files\n", len(savedFiles))
    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "saved_files": savedFiles,
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleGetData(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.Content == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or content)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Getting multiple files for user %s in app %s\n", user, req.AppName)

    // Parse the content as JSON array of filenames
    var filenames []string
    err := json.Unmarshal([]byte(req.Content), &filenames)
    if err != nil {
        fmt.Printf("DEBUG: Error parsing filenames JSON: %v\n", err)
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "invalid JSON content: " + err.Error(),
            "result": "fail",
        })
        return
    }

    data := make(map[string]interface{})
    retrievedCount := 0

    for _, filename := range filenames {
        path := []string{"home", user, "securestore", req.AppName, filename}
        item, err := h.handler.Storage.GetFile(path)
        if err == nil && item != nil {
            // Handle both old and new format
            if dataStr, ok := item.Data.(string); ok {
                var fileData map[string]interface{}
                if err := json.Unmarshal([]byte(dataStr), &fileData); err == nil {
                    // New format with metadata
                    if content, exists := fileData["content"]; exists {
                        data[filename] = content
                    } else {
                        data[filename] = dataStr
                    }
                } else {
                    // Old format, direct string
                    data[filename] = dataStr
                }
            } else {
                data[filename] = item.Data
            }
            retrievedCount++
        } else {
            fmt.Printf("DEBUG: File not found: %s\n", filename)
        }
    }

    fmt.Printf("DEBUG: Retrieved %d out of %d requested files\n", retrievedCount, len(filenames))
    c.JSON(http.StatusOK, gin.H{
        "data":   data,
        "result": "ok",
        "retrieved_count": retrievedCount,
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleBackup(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing app name",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Creating backup for user %s in app %s\n", user, req.AppName)

    // List all files in the app directory
    path := []string{"home", user, "securestore", req.AppName}
    item, err := h.handler.Storage.GetFile(path)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "data":   "app directory not found",
            "result": "fail",
        })
        return
    }

    // Get all file contents
    backup := make(map[string]interface{})
    if data, ok := item.Data.([]interface{}); ok {
        for _, file := range data {
            if filename, ok := file.(string); ok {
                filePath := []string{"home", user, "securestore", req.AppName, filename}
                fileItem, err := h.handler.Storage.GetFile(filePath)
                if err == nil && fileItem != nil {
                    backup[filename] = fileItem.Data
                }
            }
        }
    }

    // Save backup with timestamp
    backupFilename := fmt.Sprintf("backup_%d.json", getCurrentTimestamp())
    backupPath := []string{"home", user, "securestore", req.AppName, backupFilename}
    
    backupData, err := json.Marshal(backup)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to create backup data",
            "result": "fail",
        })
        return
    }

    err = h.handler.Storage.CreateFile(backupPath, string(backupData))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to save backup",
            "result": "fail",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "backup_file": backupFilename,
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) handleRestore(c *gin.Context, user string, req WebAppRequest) {
    if req.AppName == "" || req.FName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing parameters (appname or backup filename)",
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: Restoring backup %s for user %s in app %s\n", req.FName, user, req.AppName)

    // Get backup file
    backupPath := []string{"home", user, "securestore", req.AppName, req.FName}
    backupItem, err := h.handler.Storage.GetFile(backupPath)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "data":   "backup file not found",
            "result": "fail",
        })
        return
    }

    // Parse backup data
    var backupData map[string]interface{}
    if dataStr, ok := backupItem.Data.(string); ok {
        err = json.Unmarshal([]byte(dataStr), &backupData)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "data":   "invalid backup file format",
                "result": "fail",
            })
            return
        }
    } else {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "invalid backup file data",
            "result": "fail",
        })
        return
    }

    // Restore files
    restoredCount := 0
    for filename, content := range backupData {
        path := []string{"home", user, "securestore", req.AppName, filename}
        contentStr, _ := json.Marshal(content)
        
        err = h.handler.Storage.UpdateFile(path, string(contentStr))
        if err == nil {
            restoredCount++
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "result": "ok",
        "restored_files": restoredCount,
        "storage_backend": h.handler.Config.StorageBackend,
    })
}

func (h *WebAppHandler) ensureDirectoryStructure(user, appName string) error {
    // Create home directory
    homeDir := []string{"home"}
    _, err := h.handler.Storage.GetFile(homeDir)
    if err != nil {
        err = h.handler.Storage.CreateDir(homeDir)
        if err != nil {
            return fmt.Errorf("failed to create home directory: %w", err)
        }
    }

    // Create user directory
    userDir := []string{"home", user}
    _, err = h.handler.Storage.GetFile(userDir)
    if err != nil {
        err = h.handler.Storage.CreateDir(userDir)
        if err != nil {
            return fmt.Errorf("failed to create user directory: %w", err)
        }
    }

    // Create securestore directory
    secureDir := []string{"home", user, "securestore"}
    _, err = h.handler.Storage.GetFile(secureDir)
    if err != nil {
        err = h.handler.Storage.CreateDir(secureDir)
        if err != nil {
            return fmt.Errorf("failed to create securestore directory: %w", err)
        }
    }

    // Create app directory
    appDir := []string{"home", user, "securestore", appName}
    _, err = h.handler.Storage.GetFile(appDir)
    if err != nil {
        err = h.handler.Storage.CreateDir(appDir)
        if err != nil {
            return fmt.Errorf("failed to create app directory: %w", err)
        }
    }

    return nil
}

func getCurrentTimestamp() int64 {
    return 1691506800 // Mock timestamp for now
}

func (h *WebAppHandler) getCurrentUser(c *gin.Context) string {
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

// handleSocialCalcSave handles save requests from SocialCalc spreadsheet
func (h *WebAppHandler) handleSocialCalcSave(c *gin.Context, user string, req WebAppRequest) {
    // Get additional parameters that SocialCalc sends
    filename := c.PostForm("filename")
    content := c.PostForm("content")
    sessionid := c.PostForm("sessionid")
    
    // Use req fields as backup if form params are empty
    if filename == "" {
        filename = req.FName
    }
    if content == "" {
        content = req.Content
    }
    
    fmt.Printf("DEBUG: SocialCalc save - filename: %s, user: %s, sessionid: %s\n", 
        filename, user, sessionid)

    if filename == "" || content == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing filename or content",
            "result": "fail",
        })
        return
    }

    // Validate session if provided
    if sessionid != "" {
        session, exists := h.handler.Session.Get(sessionid)
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{
                "data":   "invalid session",
                "result": "fail",
            })
            return
        }
        
        // Double check user from session
        sessionUser, _ := session.GetString("user")
        if sessionUser != "" && sessionUser != user {
            c.JSON(http.StatusUnauthorized, gin.H{
                "data":   "session user mismatch",
                "result": "fail",
            })
            return
        }
    }

    // Use "touchcalc" as the app name for SocialCalc saves
    appName := "touchcalc"
    
    // Ensure directory structure exists
    err := h.ensureDirectoryStructure(user, appName)
    if err != nil {
        fmt.Printf("DEBUG: Error ensuring directory structure: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to create directory structure: " + err.Error(),
            "result": "fail",
        })
        return
    }

    // Create file path
    path := []string{"home", user, "securestore", appName, filename + ".msc"}
    
    // Create file data with metadata (compatible with your existing format)
    fileData := map[string]interface{}{
        "content": content,
        "user": user,
        "app": appName,
        "filename": filename,
        "timestamp": fmt.Sprintf("%d", getCurrentTimestamp()),
        "storage_backend": h.handler.Config.StorageBackend,
        "type": "socialcalc_spreadsheet",
    }

    dataJSON, err := json.Marshal(fileData)
    if err != nil {
        fmt.Printf("DEBUG: Error marshaling file data: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to encode file data",
            "result": "fail",
        })
        return
    }

    // Check if file exists and save accordingly
    _, err = h.handler.Storage.GetFile(path)
    if err != nil {
        // File doesn't exist, create it
        fmt.Printf("DEBUG: Creating new SocialCalc file: %s\n", filename)
        err = h.handler.Storage.CreateFile(path, string(dataJSON))
    } else {
        // File exists, update it
        fmt.Printf("DEBUG: Updating existing SocialCalc file: %s\n", filename)
        err = h.handler.Storage.UpdateFile(path, string(dataJSON))
    }

    if err != nil {
        fmt.Printf("DEBUG: Error saving SocialCalc file: %v\n", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "data":   "failed to save file: " + err.Error(),
            "result": "fail",
        })
        return
    }

    fmt.Printf("DEBUG: SocialCalc file saved successfully: %s\n", filename)
    
    // Return success response in format SocialCalc expects
    c.JSON(http.StatusOK, gin.H{
        "message": "File saved successfully",
        "filename": filename,
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
        "timestamp": getCurrentTimestamp(),
    })
}

// handleSocialCalcLoad handles load requests from SocialCalc spreadsheet  
func (h *WebAppHandler) handleSocialCalcLoad(c *gin.Context, user string, req WebAppRequest) {
    filename := c.PostForm("filename")
    if filename == "" {
        filename = req.FName
    }
    
    if filename == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "data":   "missing filename",
            "result": "fail",
        })
        return
    }

    appName := "touchcalc"
    path := []string{"home", user, "securestore", appName, filename + ".msc"}
    
    item, err := h.handler.Storage.GetFile(path)
    if err != nil {
        fmt.Printf("DEBUG: SocialCalc file not found: %s, error: %v\n", filename, err)
        c.JSON(http.StatusNotFound, gin.H{
            "data":   "file not found: " + filename,
            "result": "fail",
        })
        return
    }

    // Extract content from stored data
    var fileContent string
    if dataStr, ok := item.Data.(string); ok {
        var fileData map[string]interface{}
        if err := json.Unmarshal([]byte(dataStr), &fileData); err == nil {
            if content, exists := fileData["content"]; exists {
                if contentStr, ok := content.(string); ok {
                    fileContent = contentStr
                } else {
                    fileContent = dataStr
                }
            } else {
                fileContent = dataStr
            }
        } else {
            fileContent = dataStr
        }
    } else {
        dataBytes, _ := json.Marshal(item.Data)
        fileContent = string(dataBytes)
    }

    fmt.Printf("DEBUG: SocialCalc file loaded successfully: %s\n", filename)
    c.JSON(http.StatusOK, gin.H{
        "data":   fileContent,
        "filename": filename,
        "result": "ok",
        "storage_backend": h.handler.Config.StorageBackend,
    })
}
