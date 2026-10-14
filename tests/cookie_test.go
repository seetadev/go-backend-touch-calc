package tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/internal/auth"
	"github.com/c4gt/tornado-nginx-go-backend/internal/handlers"
	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	"github.com/c4gt/tornado-nginx-go-backend/internal/storage"
	"github.com/c4gt/tornado-nginx-go-backend/tests/testutils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	cookieUser = "alice@example.com"
	cookiePass = "s3cret-pass"
)

// userStore serves one registered user and delegates everything else.
type userStore struct {
	storage.Storage
	userPath string
	userJSON string
}

func (s *userStore) GetFile(path []string) (*models.StorageItem, error) {
	if strings.Join(path, "/") == s.userPath {
		return models.NewStorageItem(path, "file", s.userJSON), nil
	}
	return s.Storage.GetFile(path)
}

// cookieServer returns a router with login and a protected endpoint, and a
// registered user in storage.
func cookieServer(t *testing.T, environment string) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router, h := testutils.SetupTestServer(t)
	h.Config.Environment = environment
	h.Config.CookieSecret = "0123456789abcdef0123456789abcdef"

	u, err := models.NewUser(cookieUser, cookiePass)
	require.NoError(t, err)
	userJSON, err := u.ToJSON()
	require.NoError(t, err)
	store := &userStore{Storage: testutils.NewMockStorage(), userPath: "home/" + auth.UserDir + "/" + cookieUser, userJSON: userJSON}
	h.Storage = store
	h.Auth = handlers.NewAuthHandler(h, auth.NewService(store))

	router.POST("/login", h.Auth.HandleLogin)
	router.POST("/probe", h.WebApp.HandleWebApp)
	return router
}

func probe(router http.Handler, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/probe", strings.NewReader("action=listdir&appname=touchcalc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func login(t *testing.T, router http.Handler) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"`+cookieUser+`","password":"`+cookiePass+`"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	for _, c := range w.Result().Cookies() {
		if c.Name == "user" {
			return c
		}
	}
	t.Fatal("login did not set the user cookie")
	return nil
}

func TestPlainUserCookieIsRejected(t *testing.T) {
	router := cookieServer(t, "development")
	for _, value := range []string{cookieUser, `"` + cookieUser + `"`, "users"} {
		w := probe(router, &http.Cookie{Name: "user", Value: value})
		require.Equal(t, http.StatusUnauthorized, w.Code, "cookie %q: %s", value, w.Body.String())
	}
}

func TestTamperedCookieIsRejected(t *testing.T) {
	router := cookieServer(t, "development")
	c := login(t, router)

	// the genuine value with its last characters changed
	bad := c.Value[:len(c.Value)-2] + "AA"
	require.Equal(t, http.StatusUnauthorized, probe(router, &http.Cookie{Name: "user", Value: bad}).Code)
}

func TestLoginCookieIsSignedAndWorks(t *testing.T) {
	router := cookieServer(t, "development")
	c := login(t, router)

	raw, err := url.QueryUnescape(c.Value)
	require.NoError(t, err)
	require.NotEqual(t, cookieUser, raw, "cookie must not be the bare email address")
	require.True(t, c.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, c.SameSite)
	require.Greater(t, c.MaxAge, 0)
	require.NotEqual(t, http.StatusUnauthorized, probe(router, &http.Cookie{Name: "user", Value: c.Value}).Code)
}

func TestLoginCookieIsSecureOutsideDevelopment(t *testing.T) {
	require.True(t, login(t, cookieServer(t, "production")).Secure)
	require.False(t, login(t, cookieServer(t, "development")).Secure)
}
