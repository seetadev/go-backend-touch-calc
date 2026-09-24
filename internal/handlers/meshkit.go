package handlers

import (
	"fmt"
	"net/http"

	"github.com/c4gt/tornado-nginx-go-backend/pkg/meshkit"
	"github.com/gin-gonic/gin"
)

// MeshkitHandler exposes meshkit service endpoints via the Go backend.
type MeshkitHandler struct {
	handler *Handler
	client  *meshkit.Client
}

func NewMeshkitHandler(h *Handler, client *meshkit.Client) *MeshkitHandler {
	return &MeshkitHandler{
		handler: h,
		client:  client,
	}
}

type MeshkitUploadRequest struct {
	Path    string `json:"path" form:"path"`
	Content string `json:"content" form:"content"`
}

// HandleUpload implements POST /api/storage/upload. It accepts JSON
// ({path, content}) or form data (path, content) and uploads the content
// to Meshkit, returning the resulting CID.
func (h *MeshkitHandler) HandleUpload(c *gin.Context) {
	var req MeshkitUploadRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Path == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path and content are required"})
		return
	}

	cid, err := h.client.UploadFile(c.Request.Context(), req.Path, req.Content)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Failed to upload to Meshkit: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"cid":    cid,
		"path":   req.Path,
	})
}