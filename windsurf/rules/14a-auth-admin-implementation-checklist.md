---
trigger: always_on
---

Implementation checklist for admin authentication with chi, sessions/cookies, and CSRF.

# Dependencies (options)
- Router: `github.com/go-chi/chi/v5`
- CSRF: `github.com/gorilla/csrf` (token middleware)
- Password hashing: `golang.org/x/crypto/argon2` (preferred) or `golang.org/x/crypto/bcrypt`
- Sessions:
  - Option A (cookie only): signed cookies using `net/http` (no server store)
  - Option B (server store): `github.com/alexedwards/scs/v2` or `github.com/gorilla/sessions`

# Cookie & Session Settings
- Cookie name: `session`
- Attributes: `HttpOnly`, `Secure` (true in non-dev), `SameSite=Strict`, `Path=/`, `Max-Age` configurable
- Store only a minimal identifier in the cookie (e.g., `session_id`); user data is loaded server-side

# Password Hashing (Argon2id example)
```go
package auth

import (
    "crypto/rand"
    "crypto/subtle"
    "encoding/base64"
    "fmt"
    "golang.org/x/crypto/argon2"
)

type Argon2Params struct {
    Memory      uint32
    Iterations  uint32
    Parallelism uint8
    SaltLen     uint32
    KeyLen      uint32
}

var defaultParams = Argon2Params{Memory: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLen: 16, KeyLen: 32}

func HashPassword(pw string, p Argon2Params) (string, error) {
    salt := make([]byte, p.SaltLen)
    if _, err := rand.Read(salt); err != nil { return "", err }
    hash := argon2.IDKey([]byte(pw), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLen)
    // store as: argon2id$iter$mem$par$saltB64$hashB64
    return fmt.Sprintf("argon2id$%d$%d$%d$%s$%s", p.Iterations, p.Memory, p.Parallelism,
        base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(pw, encoded string) bool {
    var iter, mem, par uint32; var saltB64, hashB64 string
    if _, err := fmt.Sscanf(encoded, "argon2id$%d$%d$%d$%s$%s", &iter, &mem, &par, &saltB64, &hashB64); err != nil { return false }
    salt, err := base64.RawStdEncoding.DecodeString(saltB64); if err != nil { return false }
    expected, err := base64.RawStdEncoding.DecodeString(hashB64); if err != nil { return false }
    sum := argon2.IDKey([]byte(pw), salt, iter, mem, uint8(par), uint32(len(expected)))
    return subtle.ConstantTimeCompare(sum, expected) == 1
}
```

# CSRF Middleware (gorilla/csrf)
```go
csrfMw := csrf.Protect(
    []byte(cfg.CSRFKey),
    csrf.Secure(cfg.AppEnv == "prod"),
    csrf.HttpOnly(true),
    csrf.Path("/"),
)
r := chi.NewRouter()
r.Use(csrfMw)
// In handlers, read token via csrf.Token(r.Context()) and inject to forms as hidden input
```

# Session Middleware (cookie-based example)
```go
func SetSessionCookie(w http.ResponseWriter, sessionID string, secure bool) {
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    sessionID,
        Path:     "/",
        HttpOnly: true,
        Secure:   secure,
        SameSite: http.SameSiteStrictMode,
        MaxAge:   86400, // 1 day
    })
}

func ClearSessionCookie(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
}
```

# Auth Middleware
```go
// Inject user to context if session valid; otherwise 401/redirect
func AuthRequired(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        c, err := r.Cookie("session")
        if err != nil || c.Value == "" {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        // lookup session in store, load user
        // ctx := context.WithValue(r.Context(), userKey{}, user)
        next.ServeHTTP(w, r)
    })
}

func RoleRequired(role string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // user := r.Context().Value(userKey{}).(User)
        // if !user.HasRole(role) { http.Error(w, "forbidden", http.StatusForbidden); return }
        next.ServeHTTP(w, r)
    })
}
```

# Login/Logout Handlers (skeleton)
```go
func LoginHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        // render login form with csrf token
        return
    }
    // POST: parse username/password
    // user, err := repo.FindByUsername(ctx, username)
    // if err != nil || !VerifyPassword(password, user.PasswordHash) { http.Error(...); return }
    // sid, _ := sessions.Create(user.ID)
    // SetSessionCookie(w, sid, cfg.AppEnv != "dev")
    http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
    // sessions.Delete(sid)
    ClearSessionCookie(w)
    http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

# Chi Wiring (example)
```go
r := chi.NewRouter()
// middlewares: recover, request id, timeouts, cors, rate limit, tracing, metrics, security headers
r.Group(func(pub chi.Router) {
    pub.Post("/upload", uploadHandler)
    pub.Post("/evaluate", evaluateHandler)
    pub.Get("/result/{id}", resultHandler)
})

r.Group(func(admin chi.Router) {
    admin.Use(AuthRequired)
    admin.Get("/admin/", adminHome)
    admin.Get("/admin/upload", adminUploadForm)
    admin.Post("/admin/upload", adminUploadSubmit)
    admin.Get("/admin/evaluate", adminEvaluateForm)
    admin.Post("/admin/evaluate", adminEvaluateSubmit)
    admin.Get("/admin/result", adminResultForm)
})
```

# Tailwind Ie2e (UI)
- Use Tailwind for admin pages with responsive utility classes.
- Dev: CDN for quick prototyping; Prod: prebuild Tailwind CSS and serve static.
- Provide simple forms for upload/evaluate/result with CSRF hidden input and error/notice banners.

# Security Checklist
- Use Argon2id for password storage with strong parameters.
- Session cookies: HttpOnly, Secure, SameSite=Strict; rotate session on login.
- CSRF enabled on all form POSTs.
- Rate-limit login and audit failures.
- Restrict admin routes behind AuthRequired + RoleRequired.
