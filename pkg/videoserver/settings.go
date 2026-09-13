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
		"Title":         "Settings",
		"ShowNav":       true,
		"Enabled":       enabled,
		"PendingSecret": pending,
		"Error":         errMsg,
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