# MFA (Session Login + Optional TOTP) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace stateless Basic Auth with a session-based login flow that supports optional TOTP two-step verification with self-service QR enrollment.

**Architecture:** Fiber session middleware (in-memory, 24h) + custom auth middleware protecting all routes except `/login`, `/verify`, `/static`. Two-step login (password → TOTP if enabled per user). TOTP secrets persisted in a new gitignored `2fa.json` via a mutex-protected `TotpStore`. New settings page for enrollment (QR PNG generated server-side, no CDN).

**Tech Stack:** Go 1.22.2, Fiber v2.52.2, pongo2 django templates (embedded in binary), viper env config, `github.com/pquerna/otp` (TOTP), `github.com/skip2/go-qrcode` (QR PNG).

## Global Constraints

- Go 1.22.2 (`.go-version`). Module name `go-mp4-server` — imports look like `go-mp4-server/pkg/...`.
- No new dependencies beyond `github.com/pquerna/otp` and `github.com/skip2/go-qrcode`.
- Verification for every task: `go build ./... && go vet ./...` must pass; tasks with tests also require `go test ./...` pass.
- Repo convention: conventional commits (`feat:`, `fix:`, `ci:`, `docs:`, `refactor:`, `chore:`, `test:`).
- Config keys follow viper dotted convention; env prefix `GOMP4_`, `.`→`_` in env names.
- New defaults (exact): `SERVER.TOTP.CONFIG.PATH` = `./2fa.json`, `SERVER.TOTP.ISSUER` = `go-mp4-server`.
- `auth.json` is committed with default creds `admin`/`admin` — never commit real credentials. `2fa.json` MUST be gitignored.
- Error messages are generic ("Invalid credentials" / "Invalid code"); never reveal whether a username exists.
- Session keys (constants, defined in `auth.go`): `sessionKeyUser = "user"`, `sessionKeyAuthenticated = "authenticated"`, `sessionKeyAwaitingOTP = "awaiting_otp"`, `sessionKeyPendingSecret = "pending_totp_secret"`.
- Views are embedded (`//go:embed views`) — template edits require `make local` rebuild to take effect.

## File Structure

- Modify `pkg/config/config.go` — two new viper defaults.
- Modify `main.go` — pass two new fields into `VideoServerCfg`.
- Modify `pkg/videoserver/videoserver.go` — add cfg fields; remove `configBaseAuth` + `User`/`Users` structs; reorder `NewVideoServer` (auth middleware before statics); add `ShowNav` to index render map; register new routes.
- Create `pkg/videoserver/totp.go` — `TotpStore` (read/write `2fa.json`, mutex-protected).
- Create `pkg/videoserver/auth.go` — session setup, auth middleware, session helpers, `loadUsers`, login/verify/logout handlers, `User`/`Users` structs (moved here).
- Create `pkg/videoserver/settings.go` — settings page + enable/confirm/cancel/disable handlers, QR generation.
- Create `pkg/videoserver/totp_test.go`, `pkg/videoserver/auth_test.go`, `pkg/videoserver/settings_test.go` + `pkg/videoserver/testdata/views/*.django` stubs.
- Create `views/login.django`, `views/verify.django`, `views/settings.django`; modify `views/partials/header.django` (nav conditional).
- Modify `.gitignore` (+`2fa.json`), `AGENTS.md`, `README.md`.

---

### Task 1: Dependencies and Config Plumbing

**Files:**
- Modify: `pkg/config/config.go` (after line 19, `viper.SetDefault("SERVER.AUTH.CONFIG.PATH", ...)`)
- Modify: `main.go:18-25` (cfg struct literal)
- Modify: `pkg/videoserver/videoserver.go:29-38` (`VideoServerCfg` struct)
- Modify: `.gitignore` (add `2fa.json`)

**Interfaces:**
- Produces: `VideoServerCfg` gains fields `TotpConfigPath string` and `TotpIssuer string`. Later tasks read them via `vs.Config.TotpConfigPath` / `vs.Config.TotpIssuer`.

- [ ] **Step 1: Add dependencies**

```bash
go get github.com/pquerna/otp@v1.4.0 github.com/skip2/go-qrcode@latest
```

- [ ] **Step 2: Add config defaults**

In `pkg/config/config.go` `setDefaults()`, after the `SERVER.AUTH.CONFIG.PATH` line add:

```go
viper.SetDefault("SERVER.TOTP.CONFIG.PATH", "./2fa.json") // 2fa.json
viper.SetDefault("SERVER.TOTP.ISSUER", "go-mp4-server")
```

- [ ] **Step 3: Extend `VideoServerCfg`**

In `pkg/videoserver/videoserver.go`, inside the `VideoServerCfg` struct (after `ViewsAssets embed.FS`) add:

```go
	// TOTP Config
	TotpConfigPath string
	TotpIssuer string
```

- [ ] **Step 4: Wire config in `main.go`**

In `main.go` inside the `VideoServerCfg{...}` literal, after `EnvConfig: viper.GetViper(),` add:

```go
		TotpConfigPath: viper.GetString("SERVER.TOTP.CONFIG.PATH"),
		TotpIssuer:     viper.GetString("SERVER.TOTP.ISSUER"),
```

- [ ] **Step 5: Gitignore `2fa.json`**

Append to `.gitignore`:

```
2fa.json
```

- [ ] **Step 6: Verify build**

Run: `go build ./... && go vet ./...`
Expected: PASS (no output, exit 0)

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum pkg/config/config.go main.go pkg/videoserver/videoserver.go .gitignore
git commit -m "chore: add TOTP dependencies and config plumbing"
```

---

### Task 2: TotpStore with Tests (TDD)

**Files:**
- Create: `pkg/videoserver/totp.go`
- Test: `pkg/videoserver/totp_test.go`

**Interfaces:**
- Produces:
  - `type TotpStore struct` (unexported fields: `mu sync.Mutex`, `path string`)
  - `func NewTotpStore(path string) *TotpStore`
  - `func (s *TotpStore) Enabled(username string) (bool, error)` — false if file missing
  - `func (s *TotpStore) Secret(username string) (secret string, ok bool, err error)` — ok=false if user has no entry
  - `func (s *TotpStore) Save(username, secret string) error` — creates/updates entry, sets `enabled: true`
  - `func (s *TotpStore) Disable(username string) error` — removes the user's entry entirely
- Later tasks use `vs.TotpStore` (field on `VideoServer`) with exactly these methods.

- [ ] **Step 1: Write the failing test**

Create `pkg/videoserver/totp_test.go`:

```go
package videoserver

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestTotpStore(t *testing.T) *TotpStore {
	t.Helper()
	return NewTotpStore(filepath.Join(t.TempDir(), "2fa.json"))
}

func TestTotpStoreEnabledMissingFile(t *testing.T) {
	store := newTestTotpStore(t)
	enabled, err := store.Enabled("admin")
	if err != nil {
		t.Fatalf("Enabled() error: %v", err)
	}
	if enabled {
		t.Fatal("expected disabled when no 2fa.json exists")
	}
}

func TestTotpStoreSaveSecretRoundtrip(t *testing.T) {
	store := newTestTotpStore(t)
	if err := store.Save("admin", "SECRET123"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	enabled, err := store.Enabled("admin")
	if err != nil || !enabled {
		t.Fatalf("expected enabled after Save, enabled=%v err=%v", enabled, err)
	}
	secret, ok, err := store.Secret("admin")
	if err != nil || !ok || secret != "SECRET123" {
		t.Fatalf("expected secret roundtrip, secret=%q ok=%v err=%v", secret, ok, err)
	}
}

func TestTotpStorePersistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "2fa.json")
	if err := NewTotpStore(path).Save("admin", "SECRET123"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	store := NewTotpStore(path)
	enabled, err := store.Enabled("admin")
	if err != nil || !enabled {
		t.Fatalf("expected enabled from disk, enabled=%v err=%v", enabled, err)
	}
}

func TestTotpStoreDisableRemovesEntry(t *testing.T) {
	store := newTestTotpStore(t)
	if err := store.Save("admin", "SECRET123"); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if err := store.Disable("admin"); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	enabled, err := store.Enabled("admin")
	if err != nil || enabled {
		t.Fatalf("expected disabled after Disable, enabled=%v err=%v", enabled, err)
	}
	_, ok, err := store.Secret("admin")
	if err != nil || ok {
		t.Fatalf("expected no secret after Disable, ok=%v err=%v", ok, err)
	}
}

func TestTotpStoreDisableMissingFile(t *testing.T) {
	store := newTestTotpStore(t)
	if err := store.Disable("admin"); err != nil {
		t.Fatalf("Disable() on missing file should not error, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/videoserver/ -run TestTotpStore -v`
Expected: FAIL — `undefined: TotpStore` / `NewTotpStore`

- [ ] **Step 3: Write minimal implementation**

Create `pkg/videoserver/totp.go`:

```go
package videoserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type totpUser struct {
	Name       string `json:"name"`
	TotpSecret string `json:"totp_secret"`
	Enabled    bool   `json:"enabled"`
}

type totpFile struct {
	Users []totpUser `json:"users"`
}

// TotpStore persists TOTP enrollment state in a JSON file.
type TotpStore struct {
	mu   sync.Mutex
	path string
}

// NewTotpStore returns a store backed by the file at path.
func NewTotpStore(path string) *TotpStore {
	return &TotpStore{path: path}
}

func (s *TotpStore) read() (totpFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return totpFile{}, nil
		}
		return totpFile{}, err
	}
	var f totpFile
	if err := json.Unmarshal(data, &f); err != nil {
		return totpFile{}, err
	}
	return f, nil
}

func (s *TotpStore) write(f totpFile) error {
	data, err := json.MarshalIndent(f, "", "    ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Enabled reports whether username has 2FA enabled. A missing file means nobody does.
func (s *TotpStore) Enabled(username string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return false, err
	}
	for _, u := range f.Users {
		if u.Name == username && u.Enabled {
			return true, nil
		}
	}
	return false, nil
}

// Secret returns the TOTP secret for username, ok=false if not enrolled.
func (s *TotpStore) Secret(username string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return "", false, err
	}
	for _, u := range f.Users {
		if u.Name == username && u.Enabled {
			return u.TotpSecret, true, nil
		}
	}
	return "", false, nil
}

// Save enrolls username with the given secret (enabled=true).
func (s *TotpStore) Save(username, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	replaced := false
	for i := range f.Users {
		if f.Users[i].Name == username {
			f.Users[i].TotpSecret = secret
			f.Users[i].Enabled = true
			replaced = true
			break
		}
	}
	if !replaced {
		f.Users = append(f.Users, totpUser{Name: username, TotpSecret: secret, Enabled: true})
	}
	return s.write(f)
}

// Disable removes username's entry entirely.
func (s *TotpStore) Disable(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	out := f.Users[:0]
	for _, u := range f.Users {
		if u.Name != username {
			out = append(out, u)
		}
	}
	f.Users = out
	return s.write(f)
}

// ensure filepath import is used (dir resolution in tests)
var _ = filepath.Join
```

Note: drop the last two lines (`var _ = filepath.Join`) if `filepath` ends up unused — it should be. The implementation above does not use `filepath`; remove the import in that case.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/videoserver/ -run TestTotpStore -v`
Expected: PASS (all 5 tests)

- [ ] **Step 5: Wire store into VideoServer**

In `pkg/videoserver/videoserver.go`:

1. Add field to the `VideoServer` struct (after `Config *VideoServerCfg`):

```go
	TotpStore *TotpStore
```

2. In `NewVideoServer`, after `videoServer := &VideoServer{...}` add:

```go
	videoServer.TotpStore = NewTotpStore(cfg.TotpConfigPath)
```

- [ ] **Step 6: Full verification**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add pkg/videoserver/totp.go pkg/videoserver/totp_test.go pkg/videoserver/videoserver.go
git commit -m "feat: add TotpStore for 2fa.json persistence"
```

---

### Task 3: Session Auth Middleware and Login Flow (TDD)

**Files:**
- Create: `pkg/videoserver/auth.go`
- Create: `pkg/videoserver/testdata/views/login.django`, `testdata/views/verify.django`, `testdata/views/settings.django`, `testdata/views/index.django`, `testdata/views/404.django` (test stubs)
- Test: `pkg/videoserver/auth_test.go`
- Modify: `pkg/videoserver/videoserver.go` (remove `configBaseAuth` + `User`/`Users` structs + unused imports; call `configureAuth()` before statics; register login/verify/logout routes)

**Interfaces:**
- Produces (all on `*VideoServer`):
  - Field `SessionStore *session.Store`
  - `func (vs *VideoServer) configureAuth()` — creates session store (24h) and registers `app.Use(vs.authMiddleware)`
  - `func (vs *VideoServer) authMiddleware(c *fiber.Ctx) error` — skips `/login`, `/verify`, `/static*`; redirects to `/login` when `authenticated` is not true
  - `func (vs *VideoServer) sessionValue(c *fiber.Ctx, key string) any`
  - `func (vs *VideoServer) sessionSet(c *fiber.Ctx, key string, val any) error`
  - `func (vs *VideoServer) currentUser(c *fiber.Ctx) string` — reads `sessionKeyUser`
  - `func (vs *VideoServer) handleLoginPage(c *fiber.Ctx) error` — GET `/login`
  - `func (vs *VideoServer) handleLogin(c *fiber.Ctx) error` — POST `/login`
  - `func (vs *VideoServer) handleVerifyPage(c *fiber.Ctx) error` — GET `/verify`
  - `func (vs *VideoServer) handleVerify(c *fiber.Ctx) error` — POST `/verify`
  - `func (vs *VideoServer) handleLogout(c *fiber.Ctx) error` — GET `/logout`
  - `func loadUsers(path string) (map[string]string, error)` — package-level; `User`/`Users` structs moved here from videoserver.go
- Task 4 consumes: `vs.SessionStore`, `sessionValue`, `sessionSet`, `currentUser`, session key constants.

- [ ] **Step 1: Create test template stubs**

Create `pkg/videoserver/testdata/views/login.django`:

```
{{ Error }}
```

Create `pkg/videoserver/testdata/views/verify.django`:

```
{{ Error }}
```

Create `pkg/videoserver/testdata/views/settings.django`:

```
{% if Enabled %}ENABLED{% else %}{% if PendingSecret %}{{ PendingSecret }}{% else %}DISABLED{% endif %}{% endif %}
```

Create `pkg/videoserver/testdata/views/index.django`:

```
OK
```

Create `pkg/videoserver/testdata/views/404.django`:

```
404
```

- [ ] **Step 2: Write the failing test**

Create `pkg/videoserver/auth_test.go`:

```go
package videoserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/django/v3"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/viper"
)

// newTestVideoServer builds a server with the same wiring as production
// (configureAuth + configureRoutes), using stub templates and temp files.
func newTestVideoServer(t *testing.T, with2FA bool) (*VideoServer, *fiber.App, string) {
	t.Helper()
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"users":[{"name":"admin","password":"admin"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	totpPath := filepath.Join(dir, "2fa.json")
	cfg := &VideoServerCfg{
		Debug:              false,
		Reload:             false,
		EnableBaseAuth:     true,
		BaseAuthConfigPath: authPath,
		TotpConfigPath:     totpPath,
		TotpIssuer:         "test-issuer",
		EnvConfig:          viper.New(),
	}
	engine := django.NewFileSystem(http.Dir("testdata/views"), ".django")
	app := fiber.New(fiber.Config{Views: engine})
	vs := &VideoServer{App: app, Config: cfg, TotpStore: NewTotpStore(totpPath)}
	vs.configureAuth()
	vs.configureRoutes()
	secret := ""
	if with2FA {
		key, err := totp.Generate(totp.GenerateOpts{Issuer: "test-issuer", AccountName: "admin"})
		if err != nil {
			t.Fatal(err)
		}
		secret = key.Secret()
		if err := vs.TotpStore.Save("admin", secret); err != nil {
			t.Fatal(err)
		}
	}
	return vs, app, secret
}

func doRequest(t *testing.T, app *fiber.App, method, path string, form map[string]string, cookie string) (*http.Response, string) {
	t.Helper()
	var body io.Reader
	if form != nil {
		vals := url.Values{}
		for k, v := range form {
			vals.Set(k, v)
		}
		body = strings.NewReader(vals.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(respBody)
}

func extractCookie(resp *http.Response) string {
	if len(resp.Cookies()) == 0 {
		return ""
	}
	parts := make([]string, 0, len(resp.Cookies()))
	for _, c := range resp.Cookies() {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

func TestUnauthenticatedRedirectsToLogin(t *testing.T) {
	_, app, _ := newTestVideoServer(t, false)
	resp, _ := doRequest(t, app, fiber.MethodGet, "/settings", nil, "")
	if resp.StatusCode != fiber.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("expected Location /login, got %q", loc)
	}
}

func TestLoginWithout2FA(t *testing.T) {
	_, app, _ := newTestVideoServer(t, false)
	resp, _ := doRequest(t, app, fiber.MethodPost, "/login",
		map[string]string{"username": "admin", "password": "admin"}, "")
	if resp.StatusCode != fiber.StatusFound || resp.Header.Get("Location") != "/" {
		t.Fatalf("expected 302 to /, got %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	cookie := extractCookie(resp)
	resp2, _ := doRequest(t, app, fiber.MethodGet, "/settings", nil, cookie)
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected authenticated /settings 200, got %d", resp2.StatusCode)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	_, app, _ := newTestVideoServer(t, false)
	resp, body := doRequest(t, app, fiber.MethodPost, "/login",
		map[string]string{"username": "admin", "password": "wrong"}, "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 with error page, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Invalid credentials") {
		t.Fatalf("expected error message, got %q", body)
	}
}

func TestLoginWith2FARequiresVerify(t *testing.T) {
	_, app, _ := newTestVideoServer(t, true)
	resp, _ := doRequest(t, app, fiber.MethodPost, "/login",
		map[string]string{"username": "admin", "password": "admin"}, "")
	if resp.StatusCode != fiber.StatusFound || resp.Header.Get("Location") != "/verify" {
		t.Fatalf("expected 302 to /verify, got %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	cookie := extractCookie(resp)
	resp2, _ := doRequest(t, app, fiber.MethodGet, "/settings", nil, cookie)
	if resp2.StatusCode != fiber.StatusFound || resp2.Header.Get("Location") != "/login" {
		t.Fatalf("expected /settings blocked before verify, got %d %q", resp2.StatusCode, resp2.Header.Get("Location"))
	}
}

func TestVerifyValidCodeAuthenticates(t *testing.T) {
	_, app, secret := newTestVideoServer(t, true)
	resp, _ := doRequest(t, app, fiber.MethodPost, "/login",
		map[string]string{"username": "admin", "password": "admin"}, "")
	cookie := extractCookie(resp)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	resp2, _ := doRequest(t, app, fiber.MethodPost, "/verify", map[string]string{"code": code}, cookie)
	if resp2.StatusCode != fiber.StatusFound || resp2.Header.Get("Location") != "/" {
		t.Fatalf("expected 302 to / after verify, got %d %q", resp2.StatusCode, resp2.Header.Get("Location"))
	}
	cookie2 := extractCookie(resp2)
	resp3, _ := doRequest(t, app, fiber.MethodGet, "/settings", nil, cookie2)
	if resp3.StatusCode != fiber.StatusOK {
		t.Fatalf("expected authenticated /settings 200, got %d", resp3.StatusCode)
	}
}

func TestVerifyInvalidCodeShowsError(t *testing.T) {
	_, app, _ := newTestVideoServer(t, true)
	resp, _ := doRequest(t, app, fiber.MethodPost, "/login",
		map[string]string{"username": "admin", "password": "admin"}, "")
	cookie := extractCookie(resp)
	resp2, body := doRequest(t, app, fiber.MethodPost, "/verify", map[string]string{"code": "000000"}, cookie)
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 with error page, got %d", resp2.StatusCode)
	}
	if !strings.Contains(body, "Invalid code") {
		t.Fatalf("expected error message, got %q", body)
	}
}

func TestLogoutDestroysSession(t *testing.T) {
	_, app, _ := newTestVideoServer(t, false)
	resp, _ := doRequest(t, app, fiber.MethodPost, "/login",
		map[string]string{"username": "admin", "password": "admin"}, "")
	cookie := extractCookie(resp)
	resp2, _ := doRequest(t, app, fiber.MethodGet, "/logout", nil, cookie)
	if resp2.StatusCode != fiber.StatusFound || resp2.Header.Get("Location") != "/login" {
		t.Fatalf("expected 302 to /login after logout, got %d %q", resp2.StatusCode, resp2.Header.Get("Location"))
	}
	resp3, _ := doRequest(t, app, fiber.MethodGet, "/settings", nil, cookie)
	if resp3.StatusCode != fiber.StatusFound {
		t.Fatalf("expected session destroyed, /settings should redirect, got %d", resp3.StatusCode)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./pkg/videoserver/ -run TestUnauthenticatedRedirectsToLogin -v`
Expected: FAIL — `undefined: configureAuth`

- [ ] **Step 4: Write auth.go implementation**

Create `pkg/videoserver/auth.go`:

```go
package videoserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/pquerna/otp/totp"
)

const (
	sessionKeyUser          = "user"
	sessionKeyAuthenticated = "authenticated"
	sessionKeyAwaitingOTP   = "awaiting_otp"
	sessionKeyPendingSecret = "pending_totp_secret"
)

// User is a single entry of the auth config file.
type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// Users is the root structure of the auth config file.
type Users struct {
	Users []User `json:"users"`
}

// configureAuth sets up the session store and the auth middleware.
// Must be called before any route/static registration so every request passes through it.
func (vs *VideoServer) configureAuth() {
	vs.SessionStore = session.New(session.Config{
		Expiration: 24 * time.Hour,
	})
	vs.App.Use(vs.authMiddleware)
}

// authMiddleware allows /login, /verify and /static through; everything
// else requires an authenticated session, otherwise redirects to /login.
func (vs *VideoServer) authMiddleware(c *fiber.Ctx) error {
	path := c.Path()
	if path == "/login" || path == "/verify" || strings.HasPrefix(path, "/static") {
		return c.Next()
	}
	if vs.sessionValue(c, sessionKeyAuthenticated) != true {
		return c.Redirect("/login")
	}
	return c.Next()
}

// sessionValue returns a value from the current session, or nil.
func (vs *VideoServer) sessionValue(c *fiber.Ctx, key string) any {
	sess, err := vs.SessionStore.Get(c)
	if err != nil {
		return nil
	}
	return sess.Get(key)
}

// sessionSet stores a value in the current session and saves it.
func (vs *VideoServer) sessionSet(c *fiber.Ctx, key string, val any) error {
	sess, err := vs.SessionStore.Get(c)
	if err != nil {
		return err
	}
	sess.Set(key, val)
	return sess.Save()
}

// currentUser returns the logged-in username from the session.
func (vs *VideoServer) currentUser(c *fiber.Ctx) string {
	name, _ := vs.sessionValue(c, sessionKeyUser).(string)
	return name
}

// handleLoginPage renders the first login step (username/password).
func (vs *VideoServer) handleLoginPage(c *fiber.Ctx) error {
	return c.Render("login", fiber.Map{"Title": "Login", "Error": ""})
}

// handleLogin validates credentials, then either authenticates the session
// or moves to the TOTP step when the user has 2FA enabled.
func (vs *VideoServer) handleLogin(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")
	users, err := loadUsers(vs.Config.BaseAuthConfigPath)
	if err != nil {
		users = map[string]string{}
	}
	if users[username] != password {
		return c.Render("login", fiber.Map{"Title": "Login", "Error": "Invalid credentials"})
	}
	if err := vs.sessionSet(c, sessionKeyUser, username); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	enabled, err := vs.TotpStore.Enabled(username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	if enabled {
		if err := vs.sessionSet(c, sessionKeyAwaitingOTP, true); err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
		}
		return c.Redirect("/verify")
	}
	if err := vs.sessionSet(c, sessionKeyAuthenticated, true); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	return c.Redirect("/")
}

// handleVerifyPage renders the second login step (TOTP code).
func (vs *VideoServer) handleVerifyPage(c *fiber.Ctx) error {
	if vs.sessionValue(c, sessionKeyAwaitingOTP) != true {
		return c.Redirect("/login")
	}
	return c.Render("verify", fiber.Map{"Title": "Verify", "Error": ""})
}

// handleVerify validates the TOTP code and completes authentication.
func (vs *VideoServer) handleVerify(c *fiber.Ctx) error {
	if vs.sessionValue(c, sessionKeyAwaitingOTP) != true {
		return c.Redirect("/login")
	}
	username := vs.currentUser(c)
	secret, ok, err := vs.TotpStore.Secret(username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	if !ok {
		return c.Redirect("/login")
	}
	code := c.FormValue("code")
	if !totp.Validate(code, secret) {
		return c.Render("verify", fiber.Map{"Title": "Verify", "Error": "Invalid code"})
	}
	if err := vs.sessionSet(c, sessionKeyAuthenticated, true); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	if err := vs.sessionSet(c, sessionKeyAwaitingOTP, ""); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	return c.Redirect("/")
}

// handleLogout destroys the session.
func (vs *VideoServer) handleLogout(c *fiber.Ctx) error {
	sess, err := vs.SessionStore.Get(c)
	if err == nil {
		sess.Destroy()
	}
	return c.Redirect("/login")
}

// loadUsers reads the auth config file into a username -> password map.
func loadUsers(path string) (map[string]string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	var users Users
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	m := make(map[string]string, len(users.Users))
	for _, u := range users.Users {
		m[u.Name] = u.Password
	}
	return m, nil
}
```

- [ ] **Step 5: Rewire videoserver.go**

In `pkg/videoserver/videoserver.go`:

1. Add field to `VideoServer` struct (after `TotpStore *TotpStore`):

```go
	SessionStore *session.Store
```

2. Add import `"github.com/gofiber/fiber/v2/middleware/session"`.

3. Remove imports that become unused: `basicauth`, `encoding/json`, `fmt`, `io/ioutil` (check: `os`, `path/filepath`, `strconv`, `strings` are still used by `handleVideo`/`getMp4Files`).

4. Remove the `User` struct, `Users` struct, and the whole `configBaseAuth` method (they moved to auth.go).

5. In `NewVideoServer`, after the `videoServer := &VideoServer{...}` line (which currently contains `App: app, Config: cfg, TotpStore: NewTotpStore(cfg.TotpConfigPath)`), insert BEFORE `videoServer.configureStaticFiles()`:

```go
	// Configure auth (session + middleware) before any routes/statics
	videoServer.configureAuth()
```

6. In `configureRoutes`, remove the line `vs.configBaseAuth()` and add the auth routes at the top (before `vs.App.Get("/", vs.handleVideo)`):

```go
	vs.App.Get("/login", vs.handleLoginPage)
	vs.App.Post("/login", vs.handleLogin)
	vs.App.Get("/verify", vs.handleVerifyPage)
	vs.App.Post("/verify", vs.handleVerify)
	vs.App.Get("/logout", vs.handleLogout)
	vs.App.Get("/settings", vs.handleSettings)
	vs.App.Post("/settings/2fa/enable", vs.handleEnable2FA)
	vs.App.Post("/settings/2fa/confirm", vs.handleConfirm2FA)
	vs.App.Post("/settings/2fa/cancel", vs.handleCancel2FA)
	vs.App.Post("/settings/2fa/disable", vs.handleDisable2FA)
```

Note: `handleSettings` and the 2FA handlers don't exist yet — the build will fail until Task 4. This is expected; keep going (the test in Step 6 only targets the auth flow).

- [ ] **Step 6: Run the auth tests**

Run: `go test ./pkg/videoserver/ -run 'TestUnauthenticated|TestLogin|TestVerify|TestLogout' -v`
Expected: PASS (7 tests). If the build fails on undefined settings handlers, that's the known Task 4 dependency — proceed to Task 4 and re-run.

- [ ] **Step 7: Commit**

```bash
git add pkg/videoserver/auth.go pkg/videoserver/auth_test.go pkg/videoserver/testdata pkg/videoserver/videoserver.go
git commit -m "feat: add session-based login flow with TOTP verify step"
```

---

### Task 4: Settings Page and 2FA Enrollment (TDD)

**Files:**
- Create: `pkg/videoserver/settings.go`
- Test: `pkg/videoserver/settings_test.go`
- Modify: `pkg/videoserver/testdata/views/settings.django` (already created in Task 3)

**Interfaces:**
- Consumes: `vs.SessionStore`, `vs.sessionValue`, `vs.sessionSet`, `vs.currentUser`, `vs.TotpStore`, `vs.Config.TotpIssuer`, `sessionKeyPendingSecret`.
- Produces (all on `*VideoServer`):
  - `func (vs *VideoServer) handleSettings(c *fiber.Ctx) error` — GET `/settings`
  - `func (vs *VideoServer) handleEnable2FA(c *fiber.Ctx) error` — POST `/settings/2fa/enable`
  - `func (vs *VideoServer) handleConfirm2FA(c *fiber.Ctx) error` — POST `/settings/2fa/confirm`
  - `func (vs *VideoServer) handleCancel2FA(c *fiber.Ctx) error` — POST `/settings/2fa/cancel`
  - `func (vs *VideoServer) handleDisable2FA(c *fiber.Ctx) error` — POST `/settings/2fa/disable`
  - `func (vs *VideoServer) renderSettings(c *fiber.Ctx, errMsg string) error` — shared page render
- Render map keys (consumed by stub AND real templates): `Title`, `ShowNav`, `Enabled` (bool), `PendingSecret` (string), `QRDataURI` (string), `Error` (string).

- [ ] **Step 1: Write the failing test**

Create `pkg/videoserver/settings_test.go`:

```go
package videoserver

import (
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
)

func loginAsAdmin(t *testing.T, app *fiber.App) string {
	t.Helper()
	resp, _ := doRequest(t, app, fiber.MethodPost, "/login",
		map[string]string{"username": "admin", "password": "admin"}, "")
	if resp.StatusCode != fiber.StatusFound {
		t.Fatalf("login failed, got %d", resp.StatusCode)
	}
	cookie := extractCookie(resp)
	if cookie == "" {
		t.Fatal("login did not set a cookie")
	}
	return cookie
}

func TestEnableConfirmDisableFlow(t *testing.T) {
	vs, app, _ := newTestVideoServer(t, false)
	cookie := loginAsAdmin(t, app)

	resp, _ := doRequest(t, app, fiber.MethodPost, "/settings/2fa/enable", nil, cookie)
	if resp.StatusCode != fiber.StatusFound || resp.Header.Get("Location") != "/settings" {
		t.Fatalf("enable: expected 302 to /settings, got %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	resp2, body := doRequest(t, app, fiber.MethodGet, "/settings", nil, cookie)
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("settings page: got %d", resp2.StatusCode)
	}
	secret := body // stub template renders only the pending secret
	if secret == "" {
		t.Fatal("expected pending secret on settings page")
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	resp3, _ := doRequest(t, app, fiber.MethodPost, "/settings/2fa/confirm",
		map[string]string{"code": code}, cookie)
	if resp3.StatusCode != fiber.StatusFound || resp3.Header.Get("Location") != "/settings" {
		t.Fatalf("confirm: expected 302 to /settings, got %d %q", resp3.StatusCode, resp3.Header.Get("Location"))
	}
	enabled, err := vs.TotpStore.Enabled("admin")
	if err != nil || !enabled {
		t.Fatalf("expected 2FA enabled after confirm, enabled=%v err=%v", enabled, err)
	}

	code2, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	resp4, _ := doRequest(t, app, fiber.MethodPost, "/settings/2fa/disable",
		map[string]string{"code": code2}, cookie)
	if resp4.StatusCode != fiber.StatusFound {
		t.Fatalf("disable: expected 302, got %d", resp4.StatusCode)
	}
	enabled, err = vs.TotpStore.Enabled("admin")
	if err != nil || enabled {
		t.Fatalf("expected 2FA disabled after disable, enabled=%v err=%v", enabled, err)
	}
}

func TestConfirmWrongCodeKeepsPending(t *testing.T) {
	_, app, _ := newTestVideoServer(t, false)
	cookie := loginAsAdmin(t, app)

	doRequest(t, app, fiber.MethodPost, "/settings/2fa/enable", nil, cookie)
	_, body := doRequest(t, app, fiber.MethodGet, "/settings", nil, cookie)
	secret := body
	if secret == "" {
		t.Fatal("expected pending secret")
	}

	resp, body2 := doRequest(t, app, fiber.MethodPost, "/settings/2fa/confirm",
		map[string]string{"code": "000000"}, cookie)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("wrong code should re-render settings, got %d", resp.StatusCode)
	}
	if !strings.Contains(body2, secret) {
		t.Fatalf("expected pending secret to survive wrong code, body=%q", body2)
	}

	resp2, _ := doRequest(t, app, fiber.MethodPost, "/settings/2fa/cancel", nil, cookie)
	if resp2.StatusCode != fiber.StatusFound {
		t.Fatalf("cancel: expected 302, got %d", resp2.StatusCode)
	}
	resp3, body3 := doRequest(t, app, fiber.MethodGet, "/settings", nil, cookie)
	if resp3.StatusCode != fiber.StatusOK || strings.Contains(body3, secret) {
		t.Fatalf("expected pending cleared after cancel, body=%q", body3)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/videoserver/ -run 'TestEnableConfirmDisable|TestConfirmWrongCode' -v`
Expected: FAIL — `undefined: handleSettings` (build error)

- [ ] **Step 3: Write settings.go implementation**

Create `pkg/videoserver/settings.go`:

```go
package videoserver

import (
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
)

// handleSettings renders the 2FA management page.
func (vs *VideoServer) handleSettings(c *fiber.Ctx) error {
	return vs.renderSettings(c, "")
}

// renderSettings renders the settings page; errMsg is shown to the user.
func (vs *VideoServer) renderSettings(c *fiber.Ctx, errMsg string) error {
	username := vs.currentUser(c)
	enabled, err := vs.TotpStore.Enabled(username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	pending, _ := vs.sessionValue(c, sessionKeyPendingSecret).(string)
	renderMap := fiber.Map{
		"Title":        "Settings",
		"ShowNav":      true,
		"Enabled":      enabled,
		"PendingSecret": pending,
		"Error":        errMsg,
	}
	if pending != "" {
		uri := fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s",
			url.QueryEscape(vs.Config.TotpIssuer),
			url.QueryEscape(username),
			pending,
			url.QueryEscape(vs.Config.TotpIssuer))
		png, err := qrcode.Encode(uri, qrcode.Medium, 256)
		if err == nil {
			renderMap["QRDataURI"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
		}
	}
	return c.Render("settings", renderMap)
}

// handleEnable2FA generates a new TOTP secret and stores it pending in the
// session until the user confirms it with a valid code.
func (vs *VideoServer) handleEnable2FA(c *fiber.Ctx) error {
	username := vs.currentUser(c)
	enabled, err := vs.TotpStore.Enabled(username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	if enabled {
		return c.Redirect("/settings")
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      vs.Config.TotpIssuer,
		AccountName: username,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	if err := vs.sessionSet(c, sessionKeyPendingSecret, key.Secret()); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	return c.Redirect("/settings")
}

// handleConfirm2FA validates the pending secret with the submitted code and
// persists the enrollment.
func (vs *VideoServer) handleConfirm2FA(c *fiber.Ctx) error {
	username := vs.currentUser(c)
	pending, _ := vs.sessionValue(c, sessionKeyPendingSecret).(string)
	if pending == "" {
		return c.Redirect("/settings")
	}
	code := c.FormValue("code")
	if !totp.Validate(code, pending) {
		return vs.renderSettings(c, "Invalid code")
	}
	if err := vs.TotpStore.Save(username, pending); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	if err := vs.sessionSet(c, sessionKeyPendingSecret, ""); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	return c.Redirect("/settings")
}

// handleCancel2FA discards the pending enrollment.
func (vs *VideoServer) handleCancel2FA(c *fiber.Ctx) error {
	if err := vs.sessionSet(c, sessionKeyPendingSecret, ""); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	return c.Redirect("/settings")
}

// handleDisable2FA turns 2FA off; a valid current TOTP code is required.
func (vs *VideoServer) handleDisable2FA(c *fiber.Ctx) error {
	username := vs.currentUser(c)
	secret, ok, err := vs.TotpStore.Secret(username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	if !ok {
		return c.Redirect("/settings")
	}
	code := c.FormValue("code")
	if !totp.Validate(code, secret) {
		return vs.renderSettings(c, "Invalid code")
	}
	if err := vs.TotpStore.Disable(username); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	if err := vs.sessionSet(c, sessionKeyPendingSecret, ""); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	return c.Redirect("/settings")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/videoserver/ -v`
Expected: PASS (all auth + totp + settings tests)

- [ ] **Step 5: Full verification**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/videoserver/settings.go pkg/videoserver/settings_test.go
git commit -m "feat: add 2FA enrollment settings page"
```

---

### Task 5: Real Templates and Nav

**Files:**
- Create: `views/login.django`, `views/verify.django`, `views/settings.django`
- Modify: `views/partials/header.django` (nav conditional)
- Modify: `pkg/videoserver/videoserver.go` (`handleVideo` render map — add `ShowNav: true`)

**Interfaces:**
- Consumes: render map keys `Title`, `ShowNav`, `Error`, `Enabled`, `PendingSecret`, `QRDataURI` (produced by Task 4 handlers) and `Error` for login/verify.
- Produces: `ShowNav` render key must be added to the `index` render map in `handleVideo`.

- [ ] **Step 1: Add nav to header partial**

In `views/partials/header.django`, after the `<h1> {{ Title }} </h1>` line add:

```django
{% if ShowNav %}
<nav>
    <a href="/">Home</a> | <a href="/settings">Settings</a> | <a href="/logout">Logout</a>
</nav>
{% endif %}
```

- [ ] **Step 2: Add ShowNav to the index render map**

In `pkg/videoserver/videoserver.go`, in `handleVideo`, add to the `renderMap` literal (after `"Title":            "go-mp4-server",`):

```go
		"ShowNav":          true,
```

- [ ] **Step 3: Create login template**

Create `views/login.django`:

```django
{% include "partials/header.django" %}

<body>
  <div class="container">
    <h2>Login</h2>
    {% if Error %}
    <p class="error">{{ Error }}</p>
    {% endif %}
    <form method="POST" action="/login">
      <label>Username: <input type="text" name="username" required></label>
      <br>
      <label>Password: <input type="password" name="password" required></label>
      <br>
      <button type="submit">Login</button>
    </form>
  </div>

  {% include "partials/footer.django" %}
</body>

</html>
```

- [ ] **Step 4: Create verify template**

Create `views/verify.django`:

```django
{% include "partials/header.django" %}

<body>
  <div class="container">
    <h2>Two-Factor Verification</h2>
    {% if Error %}
    <p class="error">{{ Error }}</p>
    {% endif %}
    <p>Enter the 6-digit code from your authenticator app.</p>
    <form method="POST" action="/verify">
      <label>Code: <input type="text" name="code" inputmode="numeric" pattern="[0-9]{6}" maxlength="6" required></label>
      <br>
      <button type="submit">Verify</button>
    </form>
  </div>

  {% include "partials/footer.django" %}
</body>

</html>
```

- [ ] **Step 5: Create settings template**

Create `views/settings.django`:

```django
{% include "partials/header.django" %}

<body>
  <div class="container">
    <h2>Settings</h2>
    {% if Error %}
    <p class="error">{{ Error }}</p>
    {% endif %}

    {% if Enabled %}
    <p>Two-factor authentication (TOTP) is <strong>enabled</strong>.</p>
    <form method="POST" action="/settings/2fa/disable">
      <label>Enter your current 6-digit code to disable: <input type="text" name="code" inputmode="numeric" maxlength="6" required></label>
      <br>
      <button type="submit">Disable 2FA</button>
    </form>
    {% elif PendingSecret %}
    <p>Scan this QR code with your authenticator app (Google Authenticator, 1Password, ...):</p>
    <img src="{{ QRDataURI }}" alt="TOTP QR code">
    <p>Or enter the secret manually: <code>{{ PendingSecret }}</code></p>
    <form method="POST" action="/settings/2fa/confirm">
      <label>Enter the 6-digit code to confirm: <input type="text" name="code" inputmode="numeric" maxlength="6" required></label>
      <br>
      <button type="submit">Confirm</button>
    </form>
    <form method="POST" action="/settings/2fa/cancel">
      <button type="submit">Cancel</button>
    </form>
    {% else %}
    <p>Two-factor authentication is <strong>disabled</strong>.</p>
    <form method="POST" action="/settings/2fa/enable">
      <button type="submit">Enable 2FA</button>
    </form>
    {% endif %}
  </div>

  {% include "partials/footer.django" %}
</body>

</html>
```

- [ ] **Step 6: Verify build and templates render**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS (test stubs unaffected; real templates are embedded — compile check only)

- [ ] **Step 7: Commit**

```bash
git add views/login.django views/verify.django views/settings.django views/partials/header.django pkg/videoserver/videoserver.go
git commit -m "feat: add login, verify and settings templates with nav"
```

---

### Task 6: Docs and Final Verification

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`

- [ ] **Step 1: Update AGENTS.md**

In `AGENTS.md`:

1. In the Build & run section, replace the "Run locally" comment about auth with nothing (auth section covered below).
2. In the Config section, after the `GOMP4_SERVER_AUTH_CONFIG_PATH` bullet add:

```markdown
- `GOMP4_SERVER_TOTP_CONFIG_PATH` — TOTP (2FA) enrollment file, default `./2fa.json`. Gitignored — secrets must never be committed.
- `GOMP4_SERVER_TOTP_ISSUER` — label shown in authenticator apps, default `go-mp4-server`.
```

3. Replace the "Basic auth is always enabled..." paragraph with:

```markdown
Auth is session-based: a login page (`/login`) validates against `auth.json` (read on **every** login, so password edits apply without restart), then an optional TOTP step (`/verify`) when that user has 2FA enabled. `auth.json` is committed with default creds `admin`/`admin` — don't commit real credentials. Users self-enroll via `/settings` (QR code; `2fa.json` stores secrets). Sessions are in-memory (24h) — a restart logs everyone out. Lost authenticator: manually delete the user's entry from `2fa.json`.
```

4. In the Architecture section, replace the `pkg/videoserver/videoserver.go` bullet with:

```markdown
- `pkg/videoserver/` — `videoserver.go` (app wiring, video routes, recursive `getMp4Files` mapping to `/videos/...` served with `ByteRange: true`), `auth.go` (session middleware + login/verify/logout), `totp.go` (`TotpStore`, mutex-protected `2fa.json` access), `settings.go` (2FA enrollment UI + server-side QR PNG). `getMp4Files` walks `VIDEO.DIR` **recursively**, keeps only `*.mp4` (skips dotfiles).
```

5. Replace the "No tests exist; verification = ..." line in the first paragraph with:

```markdown
Verification = `go build ./...` / `go vet ./...` / `go test ./...` (all pass). Unit tests live in `pkg/videoserver/*_test.go` (TotpStore, auth flow, enrollment) using fiber `app.Test()` + stub templates in `pkg/videoserver/testdata/views/` — mirror production wiring, don't embed the real views.
```

- [ ] **Step 2: Update README.md**

In `README.md` Parameters section, after the `GOMP4_VIDEO_DIR` line add:

```markdown
GOMP4_SERVER_AUTH_CONFIG_PATH: path to the user config JSON (default ./auth.json)
GOMP4_SERVER_TOTP_CONFIG_PATH: path to the 2FA enrollment file (default ./2fa.json, gitignored)
GOMP4_SERVER_TOTP_ISSUER: issuer label shown in authenticator apps (default go-mp4-server)
```

- [ ] **Step 3: Full verification**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS

- [ ] **Step 4: Manual smoke test**

```bash
make local
GOMP4_VIDEO_DIR=$PWD/videos GOMP4_SERVER_PORT=30080 ./bin/go-mp4-server
```

In a browser:
1. Visit `http://localhost:30080` → redirected to `/login`.
2. Login with `admin`/`admin` → lands on `/` (2FA not enabled yet).
3. Open `/settings` → "Enable 2FA" → scan QR with an authenticator app → enter code → "enabled".
4. `/logout` → login again → `/verify` step appears → wrong code shows error, right code logs in.
5. Disable 2FA from `/settings` (needs current code).
6. Play a video and seek (confirms `/videos` still works behind the session).

- [ ] **Step 5: Commit**

```bash
git add AGENTS.md README.md
git commit -m "docs: document session login and TOTP 2FA setup"
```