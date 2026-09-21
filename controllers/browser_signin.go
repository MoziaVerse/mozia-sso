package controllers

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/casdoor/casdoor/form"
	"github.com/casdoor/casdoor/object"
)

// Validate before consuming credentials. The application secret is carried in a
// server-to-server header, not in query parameters or the recorded request body.
func (c *ApiController) validateEmbeddedSignin(input *form.AuthForm) error {
	if input.BrowserReturnUri == "" {
		return nil
	}
	if input.Type != ResponseTypeLogin {
		return fmt.Errorf("embedded sign-in requires login response type")
	}
	app, err := object.GetApplication("admin/" + input.Application)
	if err != nil {
		return err
	}
	if app == nil || app.Organization != input.Organization || app.DisableSignin || !app.EnableSigninSession {
		return fmt.Errorf("embedded sign-in is not enabled")
	}
	header := c.Ctx.Request.Header.Get("X-Casdoor-Embedded-Client")
	encoded, ok := strings.CutPrefix(header, "Basic ")
	if !ok {
		return fmt.Errorf("invalid embedded client credentials")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("invalid embedded client credentials")
	}
	id, secret, ok := strings.Cut(string(raw), ":")
	if !ok || id != app.ClientId || secret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(app.ClientSecret)) != 1 {
		return fmt.Errorf("invalid embedded client credentials")
	}
	_, err = object.ValidateBrowserSigninReturn(app, input.BrowserReturnUri)
	return err
}

// BrowserSignin is a top-level cross-origin form POST. Origin validation is
// mandatory even when global CORS settings are permissive. No GET/query ticket.
func (c *ApiController) BrowserSignin() {
	c.Ctx.Output.Header("Cache-Control", "no-store")
	c.Ctx.Output.Header("Referrer-Policy", "no-referrer")
	if c.Ctx.Request.Method != http.MethodPost || !strings.HasPrefix(c.Ctx.Request.Header.Get("Content-Type"), "application/x-www-form-urlencoded") || c.Ctx.Request.ContentLength > 4096 {
		c.CustomAbort(http.StatusBadRequest, "Invalid sign-in request")
		return
	}
	if err := c.Ctx.Request.ParseForm(); err != nil {
		c.CustomAbort(http.StatusBadRequest, "Invalid sign-in request")
		return
	}
	ticket, err := object.ConsumeBrowserSigninTicket(c.Ctx.Request.PostForm.Get("ticket"), c.Ctx.Request.Header.Get("Origin"))
	if err != nil {
		c.CustomAbort(http.StatusBadRequest, "Sign-in handoff expired or invalid. Return to the application and sign in again.")
		return
	}
	app, err := object.GetApplication(ticket.Application)
	if err != nil || app == nil || app.DisableSignin || !app.EnableSigninSession {
		c.CustomAbort(http.StatusForbidden, "Sign-in unavailable")
		return
	}
	if _, err = object.ValidateBrowserSigninReturn(app, ticket.ReturnUri); err != nil {
		c.CustomAbort(http.StatusForbidden, "Sign-in unavailable")
		return
	}
	user, err := object.GetUser(ticket.UserId)
	if err != nil || user == nil || user.NeedUpdatePassword || user.Owner != app.Organization {
		c.CustomAbort(http.StatusForbidden, "Sign-in unavailable")
		return
	}
	if err = c.SessionRegenerateID(); err != nil {
		c.CustomAbort(http.StatusInternalServerError, "Sign-in unavailable")
		return
	}
	// Recheck current account status, organization, IP, tag and application policy.
	resp := c.HandleLoggedIn(app, user, &form.AuthForm{Type: ResponseTypeLogin})
	if resp == nil || resp.Status != "ok" || resp.Data != user.GetId() {
		return
	}
	c.Ctx.Redirect(http.StatusSeeOther, ticket.ReturnUri)
}
