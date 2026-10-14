// Package authcookie signs and verifies the value of the login cookie so that
// a client cannot choose which user it is.
//
// A value has the form
//
//	v1.<base64url(user)>.<expiry unix seconds>.<base64url(HMAC-SHA256)>
//
// where the MAC covers everything before the last dot.
package authcookie

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// Name is the name of the login cookie.
const Name = "user"

const version = "v1"

var enc = base64.RawURLEncoding

// Signer creates and checks signed cookie values.
type Signer struct {
	key []byte
	ttl time.Duration
	now func() time.Time
}

// NewSigner returns a Signer keyed with secret whose values are valid for ttl.
func NewSigner(secret string, ttl time.Duration) *Signer {
	return &Signer{key: []byte(secret), ttl: ttl, now: time.Now}
}

// Sign returns the cookie value for user.
func (s *Signer) Sign(user string) string {
	exp := s.now().Add(s.ttl).Unix()
	payload := version + "." + enc.EncodeToString([]byte(user)) + "." + strconv.FormatInt(exp, 10)
	return payload + "." + enc.EncodeToString(s.mac(payload))
}

// Verify returns the user in value, or false if the value is malformed, was not
// signed with this Signer's secret, or has expired.
func (s *Signer) Verify(value string) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != version {
		return "", false
	}
	got, err := enc.DecodeString(parts[3])
	if err != nil {
		return "", false
	}
	payload := strings.Join(parts[:3], ".")
	if !hmac.Equal(got, s.mac(payload)) {
		return "", false
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || !s.now().Before(time.Unix(exp, 0)) {
		return "", false
	}
	user, err := enc.DecodeString(parts[1])
	if err != nil || len(user) == 0 {
		return "", false
	}
	return string(user), true
}

func (s *Signer) mac(payload string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(payload))
	return m.Sum(nil)
}
