package object

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

const hduBindingTicketTTL = 5 * time.Minute

type HduBindingTicket struct {
	TokenHash        string `xorm:"varchar(64) pk" json:"-"`
	ApplicationOwner string `xorm:"varchar(100) index" json:"applicationOwner"`
	ApplicationName  string `xorm:"varchar(100) index" json:"applicationName"`
	UserOwner        string `xorm:"varchar(100) index" json:"userOwner"`
	UserName         string `xorm:"varchar(100) index" json:"userName"`
	ReturnURL        string `xorm:"varchar(500)" json:"returnUrl"`
	CallbackOrigin   string `xorm:"varchar(200)" json:"callbackOrigin"`
	ExpiresAt        int64  `xorm:"index" json:"expiresAt"`
	Used             bool   `xorm:"index" json:"used"`
}

func hduBindingTokenHash(token string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
}

func CreateHduBindingTicket(application *Application, user *User, returnURL, callbackOrigin string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	record := &HduBindingTicket{
		TokenHash:        hduBindingTokenHash(token),
		ApplicationOwner: application.Owner,
		ApplicationName:  application.Name,
		UserOwner:        user.Owner,
		UserName:         user.Name,
		ReturnURL:        returnURL,
		CallbackOrigin:   callbackOrigin,
		ExpiresAt:        time.Now().Add(hduBindingTicketTTL).Unix(),
	}
	if _, err := ormer.Engine.Insert(record); err != nil {
		return "", err
	}
	_, _ = ormer.Engine.Where("expires_at < ?", time.Now().Add(-time.Hour).Unix()).Delete(&HduBindingTicket{})
	return token, nil
}

func GetActiveHduBindingTicket(token string) (*HduBindingTicket, error) {
	record := &HduBindingTicket{TokenHash: hduBindingTokenHash(token)}
	exists, err := ormer.Engine.Get(record)
	if err != nil || !exists || record.Used || record.ExpiresAt < time.Now().Unix() {
		return nil, err
	}
	return record, nil
}

func ConsumeHduBindingTicket(token string) (bool, error) {
	updated, err := ormer.Engine.
		Where("token_hash = ? AND used = ? AND expires_at >= ?", hduBindingTokenHash(token), false, time.Now().Unix()).
		Cols("used").
		Update(&HduBindingTicket{Used: true})
	return updated == 1, err
}
