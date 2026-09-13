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