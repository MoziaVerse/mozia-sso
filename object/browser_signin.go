package object

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"
)

// BrowserSigninTicket carries a completed authentication, never a reusable token.
// Only the digest is stored. The originating product must POST it from an
// explicitly configured origin; accepting GET would allow login CSRF.
type BrowserSigninTicket struct {
	TokenHash   string `xorm:"varchar(64) pk"`
	Application string `xorm:"varchar(100) notnull"`
	UserId      string `xorm:"varchar(200) notnull"`
	Origin      string `xorm:"varchar(200) notnull"`
	ReturnUri   string `xorm:"varchar(1000) notnull"`
	ExpiresAt   int64  `xorm:"index notnull"`
}

func BrowserSigninOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Fragment != "" {
		return "", fmt.Errorf("invalid browser sign-in URL")
	}
	return u.Scheme + "://" + u.Host, nil
}

func ValidateBrowserSigninReturn(app *Application, raw string) (string, error) {
	origin, err := BrowserSigninOrigin(raw)
	if err != nil || len(raw) > 1000 {
		return "", fmt.Errorf("invalid browser sign-in return URL")
	}
	for _, allowed := range app.EmbeddedSigninOrigins {
		if allowed == origin {
			return origin, nil
		}
	}
	return "", fmt.Errorf("browser sign-in origin is not enabled for this application")
}

func browserSigninHash(token string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(token))) }

func CreateBrowserSigninTicket(app *Application, user *User, returnUri string) (string, error) {
	origin, err := ValidateBrowserSigninReturn(app, returnUri)
	if err != nil {
		return "", err
	}
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	row := &BrowserSigninTicket{TokenHash: browserSigninHash(token), Application: app.GetId(), UserId: user.GetId(), Origin: origin, ReturnUri: returnUri, ExpiresAt: time.Now().Add(60 * time.Second).Unix()}
	if _, err = ormer.Engine.Insert(row); err != nil {
		return "", err
	}
	_, _ = ormer.Engine.Where("expires_at < ?", time.Now().Add(-time.Hour).Unix()).Delete(&BrowserSigninTicket{})
	return token, nil
}

func ConsumeBrowserSigninTicket(token, origin string) (*BrowserSigninTicket, error) {
	if len(token) != 43 || origin == "" || origin == "null" {
		return nil, fmt.Errorf("invalid browser sign-in request")
	}
	row := &BrowserSigninTicket{TokenHash: browserSigninHash(token)}
	exists, err := ormer.Engine.Get(row)
	if err != nil {
		return nil, err
	}
	if !exists || row.Origin != origin || row.ExpiresAt <= time.Now().Unix() {
		return nil, fmt.Errorf("browser sign-in ticket is invalid or expired")
	}
	// Conditional deletion is the single-use boundary across concurrent instances.
	n, err := ormer.Engine.Where("token_hash = ? AND origin = ? AND expires_at > ?", row.TokenHash, origin, time.Now().Unix()).Delete(&BrowserSigninTicket{})
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, fmt.Errorf("browser sign-in ticket has already been used")
	}
	return row, nil
}
