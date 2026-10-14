package handlers

import (
    "crypto/rand"
    "encoding/base64"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/c4gt/tornado-nginx-go-backend/internal/auth"
    "github.com/c4gt/tornado-nginx-go-backend/internal/authcookie"
    "github.com/c4gt/tornado-nginx-go-backend/internal/config"
    "github.com/c4gt/tornado-nginx-go-backend/internal/email"
    "github.com/c4gt/tornado-nginx-go-backend/internal/session"
    "github.com/c4gt/tornado-nginx-go-backend/internal/storage"
    "github.com/gin-gonic/gin"
)

// loginTTL is how long a login cookie stays valid.
const loginTTL = 24 * time.Hour

type Handler struct {
    Config  *config.Config
    Storage storage.Storage
    Session *session.Manager
    Auth    *AuthHandler
    WebApp  *WebAppHandler
    Email   *EmailHandler
    App     *AppHandler
    Dropbox *DropboxHandler

    signerOnce sync.Once
    signer     *authcookie.Signer
}

// Signer returns the signer for login cookies, keyed with Config.CookieSecret.
// Without a configured secret (only allowed outside production) it uses a random
// per-process key, so logins do not survive a restart.
func (h *Handler) Signer() *authcookie.Signer {
    h.signerOnce.Do(func() {
        secret := h.Config.CookieSecret
        if secret == "" {
            buf := make([]byte, 32)
            if _, err := rand.Read(buf); err != nil {
                log.Fatalf("cannot generate a cookie secret: %v", err)
            }
            secret = base64.StdEncoding.EncodeToString(buf)
            log.Println("COOKIE_SECRET is not set; using a temporary secret, logins reset on restart")
        }
        h.signer = authcookie.NewSigner(secret, loginTTL)
    })
    return h.signer
}

// CurrentUser returns the logged-in user from the signed login cookie, or ""
// if there is none or it is invalid or expired.
func (h *Handler) CurrentUser(c *gin.Context) string {
    value, err := c.Cookie(authcookie.Name)
    if err != nil {
        return ""
    }
    user, ok := h.Signer().Verify(value)
    if !ok {
        return ""
    }
    return user
}

// SetUserCookie logs user in by setting the signed login cookie.
func (h *Handler) SetUserCookie(c *gin.Context, user string) {
    c.SetSameSite(http.SameSiteLaxMode)
    c.SetCookie(authcookie.Name, h.Signer().Sign(user), int(loginTTL.Seconds()), "/", "", h.Config.SecureCookies(), true)
}

// ClearUserCookie removes the login cookie.
func (h *Handler) ClearUserCookie(c *gin.Context) {
    c.SetSameSite(http.SameSiteLaxMode)
    c.SetCookie(authcookie.Name, "", -1, "/", "", h.Config.SecureCookies(), true)
}

func NewHandler(cfg *config.Config) *Handler {
    // Initialize storage with proper error handling
    storageBackend, err := storage.NewStorage(cfg)
    if err != nil {
        log.Fatalf("Failed to initialize storage backend (%s): %v", cfg.StorageBackend, err)
    }

    // Initialize session manager
    sessionManager := session.NewManager()

    // Initialize auth service
    authService := auth.NewService(storageBackend)

    // Initialize email service (with fallback if AWS not configured)
    var emailService *email.SESService
    if cfg.AWSAccessKey != "" && cfg.AWSSecretKey != "" && 
       cfg.AWSAccessKey != "your_aws_access_key" && cfg.AWSSecretKey != "your_aws_secret_key" {
        emailService, err = email.NewSESService()
        if err != nil {
            log.Printf("Failed to initialize SES email service: %v", err)
            log.Println("Email functionality will be disabled")
        } else {
            log.Println("Email service initialized successfully")
        }
    } else {
        log.Println("AWS credentials not provided or using placeholder values, email functionality disabled")
    }

    h := &Handler{
        Config:  cfg,
        Storage: storageBackend,
        Session: sessionManager,
    }

    // Initialize sub-handlers
    h.Auth = NewAuthHandler(h, authService)
    h.WebApp = NewWebAppHandler(h)
    h.Email = NewEmailHandler(h, emailService)
    h.App = NewAppHandler(h)
    h.Dropbox = NewDropboxHandler(h)

    return h
}
