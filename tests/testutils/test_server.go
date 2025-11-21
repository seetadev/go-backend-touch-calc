package testutils

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/internal/auth"
	"github.com/c4gt/tornado-nginx-go-backend/internal/config"
	"github.com/c4gt/tornado-nginx-go-backend/internal/handlers"
	"github.com/c4gt/tornado-nginx-go-backend/internal/session"
	"github.com/c4gt/tornado-nginx-go-backend/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func SetupTestServer(t *testing.T) (*gin.Engine, *handlers.Handler) {
	cfg := &config.Config{
		Environment:    "test",
		Port:           "8080",
		CookieSecret:   "testsecret",
		StorageBackend: "mock",
	}

	router := gin.Default()
	router.Use(middleware.CORS(), middleware.Logger(), middleware.Recovery())

	// Provide minimal in-memory templates so handlers that render templates
	// (e.g., auth register/login) do not panic during tests. We don't care
	// about the actual HTML content in integration tests.
	tmpl := template.Must(template.New("base").Parse(`
{{define "login.html"}}login{{end}}
{{define "register.html"}}register{{end}}
`))
	router.SetHTMLTemplate(tmpl)

	// Use mock storage and an in-memory session manager
	h := &handlers.Handler{
		Config:  cfg,
		Storage: NewMockStorage(),
		Session: session.NewManager(),
	}

	// Initialize auth service with the same mock storage used by the handler
	authService := auth.NewService(h.Storage)
	h.Auth = handlers.NewAuthHandler(h, authService)
	h.WebApp = handlers.NewWebAppHandler(h)
	h.App = handlers.NewAppHandler(h)

	return router, h
}

func PerformRequest(r http.Handler, method, path string, body http.HandlerFunc) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func AssertStatusCode(t *testing.T, rr *httptest.ResponseRecorder, expected int) {
	require.Equal(t, expected, rr.Code)
}
