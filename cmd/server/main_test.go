package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/tests/testutils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router, handler := testutils.SetupTestServer(t)
	setupRoutes(router, handler)
	return router
}

func post(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("action=listdir&appname=touchcalc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// The webapp endpoint is served at /webapp, as the Tornado backend and its
// clients expect. Without a login it answers 401, not 404.
func TestWebAppRouteIsServedAtWebapp(t *testing.T) {
	w := post(newTestRouter(t), "/webapp")
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "usererror")
}

// Callers of the former /iwebapp path are sent to /webapp with the POST intact.
func TestLegacyIWebAppRedirectsToWebapp(t *testing.T) {
	w := post(newTestRouter(t), "/iwebapp")
	require.Equal(t, http.StatusTemporaryRedirect, w.Code)
	require.Equal(t, "/webapp", w.Header().Get("Location"))
}
