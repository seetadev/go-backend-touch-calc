package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/c4gt/tornado-nginx-go-backend/internal/authcookie"
	"github.com/stretchr/testify/require"
)

func TestExpiredAndForeignCookiesAreRejected(t *testing.T) {
	router := cookieServer(t, "development")

	expired := authcookie.NewSigner("0123456789abcdef0123456789abcdef", -time.Minute).Sign(cookieUser)
	require.Equal(t, http.StatusUnauthorized, probe(router, &http.Cookie{Name: "user", Value: expired}).Code)

	foreign := authcookie.NewSigner("a-different-secret-a-different-secret", time.Hour).Sign(cookieUser)
	require.Equal(t, http.StatusUnauthorized, probe(router, &http.Cookie{Name: "user", Value: foreign}).Code)

	valid := authcookie.NewSigner("0123456789abcdef0123456789abcdef", time.Hour).Sign(cookieUser)
	require.NotEqual(t, http.StatusUnauthorized, probe(router, &http.Cookie{Name: "user", Value: valid}).Code)
}
