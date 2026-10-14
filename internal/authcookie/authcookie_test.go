package authcookie

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestSignVerifyRoundTrip(t *testing.T) {
	s := NewSigner(secret, time.Hour)
	for _, user := range []string{"alice@example.com", "a.b+c@sub.example.org", "रमेश@example.in"} {
		got, ok := s.Verify(s.Sign(user))
		require.True(t, ok, user)
		require.Equal(t, user, got)
	}
}

func TestVerifyRejectsPlainAndMalformedValues(t *testing.T) {
	s := NewSigner(secret, time.Hour)
	for _, v := range []string{
		"",
		"alice@example.com",
		`"alice@example.com"`,
		"v1.x.y",
		"v1..1.2",
		"v2." + strings.Repeat("a", 10) + ".1.2",
		"....",
	} {
		_, ok := s.Verify(v)
		require.False(t, ok, v)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	s := NewSigner(secret, time.Hour)
	parts := strings.Split(s.Sign("alice@example.com"), ".")

	// swap in another user, keep the old MAC
	forged := strings.Join([]string{parts[0], enc.EncodeToString([]byte("bob@example.com")), parts[2], parts[3]}, ".")
	_, ok := s.Verify(forged)
	require.False(t, ok, "different user, same MAC")

	// extend the expiry, keep the old MAC
	forged = strings.Join([]string{parts[0], parts[1], "99999999999", parts[3]}, ".")
	_, ok = s.Verify(forged)
	require.False(t, ok, "different expiry, same MAC")

	// corrupt the MAC
	forged = strings.Join([]string{parts[0], parts[1], parts[2], enc.EncodeToString([]byte("nope"))}, ".")
	_, ok = s.Verify(forged)
	require.False(t, ok, "wrong MAC")
}

func TestVerifyRejectsOtherSecret(t *testing.T) {
	value := NewSigner(secret, time.Hour).Sign("alice@example.com")
	_, ok := NewSigner("another-secret-another-secret-0000", time.Hour).Verify(value)
	require.False(t, ok)
}

func TestVerifyRejectsExpired(t *testing.T) {
	s := NewSigner(secret, time.Hour)
	value := s.Sign("alice@example.com")

	s.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	_, ok := s.Verify(value)
	require.False(t, ok)
}
