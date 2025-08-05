package handlers

import (
	"encoding/json"
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
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "invalid action",
			"result": "fail",
		})
	}
}

func (h *WebAppHandler) handleSaveFile(c *gin.Context, user string, req WebAppRequest) {
	if req.AppName == "" || req.FName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "missing parameters",
			"result": "fail",
		})
		return
	}

	path := []string{"home", user, "securestore", req.AppName, req.FName}
	dirPath := []string{"home", user, "securestore", req.AppName}

	// Ensure directory exists
	_, err := h.handler.Storage.GetFile(dirPath)
	if err != nil {
		err = h.handler.Storage.CreateDir(dirPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"data":   "failed to create directory",
				"result": "fail",
			})
			return
		}
	}

	// Check if file exists
	_, err = h.handler.Storage.GetFile(path)
	if err != nil {
		// File doesn't exist, create it
		err = h.handler.Storage.CreateFile(path, req.Data)
	} else {
		// File exists, update it
		err = h.handler.Storage.UpdateFile(path, req.Data)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"data":   "failed to save file",
			"result": "fail",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": "ok",
	})
}

func (h *WebAppHandler) handleGetFile(c *gin.Context, user string, req WebAppRequest) {
	if req.AppName == "" || req.FName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "missing parameters",
			"result": "fail",
		})
		return
	}

	path := []string{"home", user, "securestore", req.AppName, req.FName}
	item, err := h.handler.Storage.GetFile(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"data":   "file not found",
			"result": "fail",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   item.Data,
		"result": "ok",
	})
}

func (h *WebAppHandler) handleDeleteFile(c *gin.Context, user string, req WebAppRequest) {
	if req.AppName == "" || req.FName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "missing parameters",
			"result": "fail",
		})
		return
	}

	path := []string{"home", user, "securestore", req.AppName, req.FName}
	err := h.handler.Storage.DeleteFile(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"data":   "failed to delete file",
			"result": "fail",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": "ok",
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

	path := []string{"home", user, "securestore", req.AppName}
	
	// Ensure directory exists
	item, err := h.handler.Storage.GetFile(path)
	if err != nil {
		err = h.handler.Storage.CreateDir(path)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"data":   "failed to create directory",
				"result": "fail",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"data":   []string{},
			"result": "ok",
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

	c.JSON(http.StatusOK, gin.H{
		"data":   fileNames,
		"result": "ok",
	})
}

func (h *WebAppHandler) handleSaveMultiple(c *gin.Context, user string, req WebAppRequest) {
	if req.AppName == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "missing parameters",
			"result": "fail",
		})
		return
	}

	// Parse the content as JSON
	var filesData map[string]interface{}
	err := json.Unmarshal([]byte(req.Content), &filesData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "invalid JSON content",
			"result": "fail",
		})
		return
	}

	dirPath := []string{"home", user, "securestore", req.AppName}
	
	// Ensure directory exists
	_, err = h.handler.Storage.GetFile(dirPath)
	if err != nil {
		err = h.handler.Storage.CreateDir(dirPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"data":   "failed to create directory",
				"result": "fail",
			})
			return
		}
	}

	// Save each file
	for filename, content := range filesData {
		if content == nil {
			continue
		}

		path := []string{"home", user, "securestore", req.AppName, filename}
		contentStr, _ := json.Marshal(content)

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
			c.JSON(http.StatusInternalServerError, gin.H{
				"data":   "failed to save file: " + filename,
				"result": "fail",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"result": "ok",
	})
}

func (h *WebAppHandler) handleGetData(c *gin.Context, user string, req WebAppRequest) {
	if req.AppName == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "missing parameters",
			"result": "fail",
		})
		return
	}

	// Parse the content as JSON array of filenames
	var filenames []string
	err := json.Unmarshal([]byte(req.Content), &filenames)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"data":   "invalid JSON content",
			"result": "fail",
		})
		return
	}

	data := make(map[string]interface{})

	for _, filename := range filenames {
		path := []string{"home", user, "securestore", req.AppName, filename}
		item, err := h.handler.Storage.GetFile(path)
		if err == nil && item != nil {
			data[filename] = item.Data
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   data,
		"result": "ok",
	})
}

func (h *WebAppHandler) getCurrentUser(c *gin.Context) string {
	userCookie, err := c.Cookie("user")
	if err != nil {
		return ""
	}

	var user string
	err = json.Unmarshal([]byte(userCookie), &user)
	if err != nil {
		return ""
	}

	return user
}