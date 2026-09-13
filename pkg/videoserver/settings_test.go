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