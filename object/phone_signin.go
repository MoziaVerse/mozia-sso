package object

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/internal/phoneauth"
	"github.com/xorm-io/core"
)

// LockPhoneAuthentication serializes requests across processes using the production
// PostgreSQL database. It deliberately fails closed on unsupported databases.
// A transaction keeps the advisory lock on one connection; closing it releases the
// lock even on error. Actual signup writes use existing Casdoor services.
func LockPhoneAuthentication(phone string) (func(), error) {
	if ormer.Engine.DriverName() != "postgres" {
		return nil, fmt.Errorf("手机号一体登录目前需要 PostgreSQL")
	}
	sum := sha256.Sum256([]byte("mozia-phone-auth:" + phone))
	key := int64(binary.BigEndian.Uint64(sum[:8]))
	session := ormer.Engine.NewSession()
	if err := session.Begin(); err != nil {
		session.Close()
		return nil, err
	}
	rows, err := session.QueryString("SELECT pg_try_advisory_xact_lock(?) AS locked", key)
	if err != nil || len(rows) != 1 || (rows[0]["locked"] != "true" && rows[0]["locked"] != "t") {
		session.Rollback()
		session.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("该手机号正在处理中，请稍后重试")
	}
	return func() { session.Rollback(); session.Close() }, nil
}

// Call with the phone lock held. Limits span applications in the same organization.
func CheckPhoneSendLimit(owner, phone string) error {
	now := time.Now().Unix()
	var records []VerificationRecord
	err := ormer.Engine.Where("owner = ? AND receiver = ? AND time > ?", owner, phone, now-86400).Desc("time").Find(&records)
	if err != nil {
		return err
	}
	var last int64
	if len(records) > 0 {
		last = records[0].Time
	}
	if limit := phoneauth.CheckSendLimit(now, last, int64(len(records))); limit != nil {
		return limit
	}
	return nil
}

// Consume exactly the latest challenge for this organization and number. Never
// fall back to an older unused code. Failed guesses are durable across processes.
func ConsumePhoneSigninCode(owner, phone, code, lang string, user *User) error {
	if user != nil {
		if err := checkSigninErrorTimes(user, lang); err != nil {
			return err
		}
	}
	record := &VerificationRecord{}
	found, err := ormer.Engine.Where("owner = ? AND receiver = ?", owner, phone).Desc("time").Desc("created_time").Get(record)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("请先获取验证码")
	}
	ttl, err := conf.GetConfigInt64("verificationCodeTimeout")
	if err != nil {
		return err
	}
	if err = phoneauth.CheckCode(record.IsUsed, record.FailedAttempts, record.Time, time.Now().Unix(), ttl*60, record.Code, code); err != nil {
		if !record.IsUsed && record.FailedAttempts < 5 {
			if _, writeErr := ormer.Engine.ID(core.PK{record.Owner, record.Name}).Incr("failed_attempts", 1).Update(&VerificationRecord{}); writeErr != nil {
				return writeErr
			}
		}
		if user != nil && record.Code != code {
			return recordSigninErrorInfo(user, lang)
		}
		return err
	}
	n, err := ormer.Engine.ID(core.PK{record.Owner, record.Name}).Where("is_used = ?", false).Cols("is_used").Update(&VerificationRecord{IsUsed: true})
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("验证码已使用，请重新获取")
	}
	if user != nil {
		return resetUserSigninErrorTimes(user)
	}
	return nil
}
