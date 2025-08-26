package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/c4gt/tornado-nginx-go-backend/internal/email"
    "github.com/gin-gonic/gin"
)

type EmailHandler struct {
    handler *Handler
    service *email.SESService // Changed from *email.Service to *email.SESService
}

func NewEmailHandler(h *Handler, service *email.SESService) *EmailHandler { // Changed parameter type
    return &EmailHandler{
        handler: h,
        service: service,
    }
}

type EmailRequest struct {
    To      string `json:"to" form:"to"`
    Data    string `json:"data" form:"data"`
    Subject string `json:"subject" form:"subject"`
    Text    string `json:"text" form:"text"`
    AppName string `json:"appname" form:"appname"`
}

func (h *EmailHandler) HandleRunAsEmail(c *gin.Context) {
    // If email service is not available, return graceful error
    if h.service == nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "data":   "Email service not configured (AWS SES credentials not provided)",
            "result": "fail",
        })
        return
    }

    var req EmailRequest
    if err := c.ShouldBind(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid request",
        })
        return
    }

    // Get current user from cookie
    user := h.getCurrentUser(c)
    
    // Prepare email message
    message := email.NewMessage()
    
    // Set subject
    if req.Subject != "" {
        message.Subject = req.Subject
    } else if user != "" {
        message.Subject = user + " has shared " + req.AppName
    } else {
        message.Subject = req.AppName
    }

    // Set body
    if req.Text != "" {
        message.BodyHTML = "<div><p>" + req.Text + "</p></div>" + req.Data
    } else {
        message.BodyHTML = req.Data
    }

	// Send email
	fromEmail := h.handler.Config.FromEmail
	err := h.service.SendEmail(fromEmail, req.To, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to send email",
		})
		return
	}

    c.JSON(http.StatusOK, gin.H{
        "data": req.To,
    })
}

func (h *EmailHandler) getCurrentUser(c *gin.Context) string {
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
