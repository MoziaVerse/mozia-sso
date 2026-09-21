package object

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/beego/beego/v2/server/web"
	"github.com/xorm-io/xorm"
)

func phoneTestDatabase(t *testing.T) {
	t.Helper()
	engine, err := xorm.NewEngine("sqlite", filepath.Join(t.TempDir(), "phone.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.Sync2(new(VerificationRecord)); err != nil {
		t.Fatal(err)
	}
	old := ormer
	ormer = &Ormer{Engine: engine}
	previous, _ := web.AppConfig.String("verificationCodeTimeout")
	web.AppConfig.Set("verificationCodeTimeout", "5")
	t.Cleanup(func() { ormer = old; engine.Close(); web.AppConfig.Set("verificationCodeTimeout", previous) })
}

func TestPhoneChallengeIsOrganizationScopedAndSingleUse(t *testing.T) {
	phoneTestDatabase(t)
	now := time.Now().Unix()
	for _, r := range []VerificationRecord{
		{Owner: "org-a", Name: "old", Receiver: "+8613800138000", Code: "111111", Time: now - 10},
		{Owner: "org-a", Name: "new", Receiver: "+8613800138000", Code: "222222", Time: now},
		{Owner: "org-b", Name: "other", Receiver: "+8613800138000", Code: "333333", Time: now + 1},
	} {
		if _, err := ormer.Engine.Insert(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := ConsumePhoneSigninCode("org-a", "+8613800138000", "333333", "en", nil); err == nil {
		t.Fatal("accepted another organization code")
	}
	if err := ConsumePhoneSigninCode("org-a", "+8613800138000", "111111", "en", nil); err == nil {
		t.Fatal("accepted old challenge")
	}
	if err := ConsumePhoneSigninCode("org-a", "+8613800138000", "222222", "en", nil); err != nil {
		t.Fatal(err)
	}
	if err := ConsumePhoneSigninCode("org-a", "+8613800138000", "222222", "en", nil); err == nil {
		t.Fatal("replayed code")
	}
	if err := ConsumePhoneSigninCode("org-a", "+8613800138000", "111111", "en", nil); err == nil {
		t.Fatal("fell back to old unused challenge")
	}
}

func TestPhoneChallengeStopsUnknownUserGuesses(t *testing.T) {
	phoneTestDatabase(t)
	if _, err := ormer.Engine.Insert(&VerificationRecord{Owner: "org", Name: "code", Receiver: "+8613800138000", Code: "123456", Time: time.Now().Unix()}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := ConsumePhoneSigninCode("org", "+8613800138000", "999999", "en", nil); err == nil {
			t.Fatal("wrong code accepted")
		}
	}
	if err := ConsumePhoneSigninCode("org", "+8613800138000", "123456", "en", nil); err == nil {
		t.Fatal("accepted exhausted challenge")
	}
}

func TestPhoneSendLimitCountsAcrossApplicationsNotOtherNumbers(t *testing.T) {
	phoneTestDatabase(t)
	now := time.Now().Unix()
	if _, err := ormer.Engine.Insert(&VerificationRecord{Owner: "org", Name: "first", Receiver: "+8613800138000", Time: now}); err != nil {
		t.Fatal(err)
	}
	if err := CheckPhoneSendLimit("org", "+8613800138000"); err == nil {
		t.Fatal("cooldown missing")
	}
	if err := CheckPhoneSendLimit("org", "+8613900139000"); err != nil {
		t.Fatal("another number was throttled")
	}
	if err := CheckPhoneSendLimit("other", "+8613800138000"); err != nil {
		t.Fatal("another organization was throttled")
	}
}

// Set PHONE_AUTH_TEST_POSTGRES only to a disposable PostgreSQL database.
func TestPhonePostgresLockAcrossConnections(t *testing.T) {
	dsn := os.Getenv("PHONE_AUTH_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("disposable PostgreSQL not configured")
	}
	engine, err := xorm.NewEngine("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	old := ormer
	ormer = &Ormer{Engine: engine}
	defer func() { ormer = old; engine.Close() }()
	unlock, err := LockPhoneAuthentication("+8613800138000")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if duplicate, err := LockPhoneAuthentication("+8613800138000"); err == nil {
		duplicate()
		t.Fatal("concurrent request obtained lock")
	}
	different, err := LockPhoneAuthentication("+8613900139000")
	if err != nil {
		t.Fatal("unrelated phone blocked", err)
	}
	different()
	unlock()
	again, err := LockPhoneAuthentication("+8613800138000")
	if err != nil {
		t.Fatal("released lock remains held", err)
	}
	again()
}
