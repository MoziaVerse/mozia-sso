package object

import (
	"crypto/sha256"
	"fmt"

	"github.com/casdoor/casdoor/util"
	"github.com/xorm-io/core"
)

const HduCASProviderType = "HduCAS"

type ExternalIdentityBinding struct {
	SubjectKey string `xorm:"varchar(64) pk" json:"-"`
	UserKey    string `xorm:"varchar(64) unique" json:"-"`
	Owner      string `xorm:"varchar(100) index" json:"owner"`
	Provider   string `xorm:"varchar(100) index" json:"provider"`
	Subject    string `xorm:"varchar(255)" json:"subject"`
	UserName   string `xorm:"varchar(100)" json:"userName"`
	VerifiedAt string `xorm:"varchar(100)" json:"verifiedAt"`
}

func hduIdentityKey(owner, subject string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprint([]string{owner, HduCASProviderType, subject}))))
}

func getHduIdentityUser(owner, subject string) (*User, error) {
	binding := ExternalIdentityBinding{SubjectKey: hduIdentityKey(owner, subject)}
	exists, err := ormer.Engine.Get(&binding)
	if err != nil || !exists {
		return nil, err
	}
	return getUser(binding.Owner, binding.UserName)
}

func linkHduCAS(user *User, subject string) (bool, error) {
	session := ormer.Engine.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return false, err
	}
	defer session.Rollback()

	verifiedAt := ""
	if subject == "" {
		if _, err := session.Where("user_key = ?", hduIdentityKey(user.Owner, user.Name)).Delete(&ExternalIdentityBinding{}); err != nil {
			return false, err
		}
		if _, err := session.Table(user).ID(core.PK{user.Owner, user.Name}).Update(map[string]interface{}{"hdu_cas": "", "hdu_verified_at": ""}); err != nil {
			return false, err
		}
	} else {
		verifiedAt = util.GetCurrentTime()
		binding := ExternalIdentityBinding{
			SubjectKey: hduIdentityKey(user.Owner, subject),
			UserKey:    hduIdentityKey(user.Owner, user.Name),
			Owner:      user.Owner, Provider: HduCASProviderType, Subject: subject, UserName: user.Name, VerifiedAt: verifiedAt,
		}
		if _, err := session.Insert(&binding); err != nil {
			return false, fmt.Errorf("HDU identity is already linked: %w", err)
		}
		if _, err := session.Table(user).ID(core.PK{user.Owner, user.Name}).Update(map[string]interface{}{"hdu_cas": subject, "hdu_verified_at": verifiedAt}); err != nil {
			return false, err
		}
	}
	if err := session.Commit(); err != nil {
		return false, err
	}
	user.HduCAS = subject
	user.HduVerifiedAt = verifiedAt
	updated, err := getUser(user.Owner, user.Name)
	if err != nil {
		return false, err
	}
	return true, updated.UpdateUserHash()
}
