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

// postJSON sends a JSON request, replaying the cookies a browser would hold.
func postJSON(t *testing.T, router http.Handler, path string, body interface{}, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", path, bytes.NewBuffer(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestRegisterLoginSaveLoad(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := setupIntegration()
	creds := map[string]string{"email": "test@example.com", "password": "secret"}

	// Register
	w := postJSON(t, router, "/register", creds, nil)
	require.Equal(t, 200, w.Code, w.Body.String())

	// Login
	w = postJSON(t, router, "/login", creds, nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	cookies := w.Result().Cookies()
	require.NotEmpty(t, cookies, "login should set a session cookie")

	// Save a file
	w = postJSON(t, router, "/iwebapp", map[string]string{
		"action":  "savefile",
		"appname": "touchcalc",
		"fname":   "test1.json",
		"data":    `{"A1":"Hello"}`,
	}, cookies)
	require.Equal(t, 200, w.Code, w.Body.String())

	// Load the file
	w = postJSON(t, router, "/iwebapp", map[string]string{
		"action":  "getfile",
		"appname": "touchcalc",
		"fname":   "test1.json",
	}, cookies)
	require.Equal(t, 200, w.Code, w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, `{"A1":"Hello"}`, resp["data"])
}

func TestWebAppRequiresLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := setupIntegration()

	w := postJSON(t, router, "/iwebapp", map[string]string{"action": "listdir", "appname": "touchcalc"}, nil)
	require.Equal(t, 401, w.Code)
}
