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