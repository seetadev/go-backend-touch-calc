package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const goodSecret = "0123456789abcdef0123456789abcdef"

func TestValidateProductionRequiresStrongCookieSecret(t *testing.T) {
	cases := []struct {
		name    string
		secret  string
		wantErr string
	}{
		{"missing", "", "must be set"},
		{"leaked example value", leakedCookieSecret, "publicly known"},
		{"too short", "short", "at least"},
		{"good", goodSecret, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := (&Config{Environment: "production", CookieSecret: tc.secret}).Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.True(t, strings.Contains(err.Error(), tc.wantErr), err.Error())
		})
	}
}

func TestValidateDevelopmentDoesNotRequireSecret(t *testing.T) {
	require.NoError(t, (&Config{Environment: "development"}).Validate())
}

func TestLoadHasNoDefaultCookieSecret(t *testing.T) {
	t.Setenv("COOKIE_SECRET", "")
	require.Empty(t, Load().CookieSecret)
}

func TestSecureCookies(t *testing.T) {
	require.False(t, (&Config{Environment: "development"}).SecureCookies())
	require.False(t, (&Config{Environment: "test"}).SecureCookies())
	require.True(t, (&Config{Environment: "production"}).SecureCookies())
	require.True(t, (&Config{Environment: "staging"}).SecureCookies())
}
