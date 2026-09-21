package object

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBrowserSigninReturnRequiresExactOrigin(t *testing.T) {
	app := &Application{EmbeddedSigninOrigins: []string{"https://matrix.test"}}
	for _, raw := range []string{"https://matrix.test.evil/login", "https://evil.test/login", "https://matrix.test:444/login", "http://matrix.test/login", "https://user@matrix.test/login", "https://matrix.test/login#secret", "//matrix.test/login", "javascript:alert(1)"} {
		if _, err := ValidateBrowserSigninReturn(app, raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if _, err := ValidateBrowserSigninReturn(app, "https://matrix.test/login?sso_return=test"); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserSigninTicketOriginExpiryAndSingleUse(t *testing.T) {
	phoneTestDatabase(t)
	ormer.Engine.SetMaxOpenConns(1)
	if err := ormer.Engine.Sync2(new(BrowserSigninTicket)); err != nil {
		t.Fatal(err)
	}
	app := &Application{Owner: "admin", Name: "matrix", EmbeddedSigninOrigins: []string{"https://matrix.test"}}
	user := &User{Owner: "org", Name: "test"}
	token, err := CreateBrowserSigninTicket(app, user, "https://matrix.test/login")
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"", "null", "https://evil.test"} {
		if _, err := ConsumeBrowserSigninTicket(token, origin); err == nil {
			t.Fatal("accepted invalid origin", origin)
		}
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := ConsumeBrowserSigninTicket(token, "https://matrix.test"); err == nil {
				successes.Add(1)
			} else {
				t.Log(err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("consumed %d times", successes.Load())
	}
	token, err = CreateBrowserSigninTicket(app, user, "https://matrix.test/login")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ormer.Engine.Where("token_hash = ?", browserSigninHash(token)).Cols("expires_at").Update(&BrowserSigninTicket{ExpiresAt: time.Now().Unix() - 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := ConsumeBrowserSigninTicket(token, "https://matrix.test"); err == nil {
		t.Fatal("accepted expired ticket")
	}
}
