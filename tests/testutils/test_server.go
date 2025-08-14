package testutils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/internal/config"
	"github.com/c4gt/tornado-nginx-go-backend/internal/handlers"
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

	// Use mock storage
	h := &handlers.Handler{
		Config:  cfg,
		Storage: NewMockStorage(),
	}

	h.Auth = handlers.NewAuthHandler(h, nil)
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
