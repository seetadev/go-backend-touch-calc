package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/internal/handlers"
	"github.com/c4gt/tornado-nginx-go-backend/tests/testutils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupIntegration() (*gin.Engine, *handlers.Handler) {
	router, handler := testutils.SetupTestServer(nil)

	api := router.Group("/")
	{
		api.POST("/register", handler.Auth.HandleRegister)
		api.POST("/login", handler.Auth.HandleLogin)
		api.POST("/iwebapp", handler.WebApp.HandleWebApp)
	}
	return router, handler
}

func TestRegisterLoginSaveLoad(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := setupIntegration()

	// Register
	w := httptest.NewRecorder()
	body := bytes.NewBufferString("email=test@example.com&password=secret")
	req, _ := http.NewRequest("POST", "/register", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)

	// Login
	w = httptest.NewRecorder()
	body = bytes.NewBufferString("email=test@example.com&password=secret")
	req, _ = http.NewRequest("POST", "/login", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)

	// Save a file
	saveReq := map[string]string{
		"action":  "savefile",
		"appname": "touchcalc",
		"fname":   "test1.json",
		"data":    `{"A1":"Hello"}`,
	}
	saveJSON, _ := json.Marshal(saveReq)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/iwebapp", bytes.NewBuffer(saveJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)

	// Load the file
	loadReq := map[string]string{
		"action":  "getfile",
		"appname": "touchcalc",
		"fname":   "test1.json",
	}
	loadJSON, _ := json.Marshal(loadReq)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/iwebapp", bytes.NewBuffer(loadJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
}
