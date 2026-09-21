package phoneauth

import (
	"testing"
)

func TestSendLimits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		now, last int64
		count     int64
		wantRetry int
		wantErr   bool
	}{
		{"first send", 1000, 0, 0, 0, false},
		{"cooldown", 1000, 970, 1, 30, true},
		{"cooldown boundary", 1000, 940, 1, 0, false},
		{"daily cap", 1000, 900, 10, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckSendLimit(tc.now, tc.last, tc.count)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v", err)
			}
			if err != nil && err.RetryAfterSeconds != tc.wantRetry {
				t.Fatalf("retry=%d", err.RetryAfterSeconds)
			}
		})
	}
}

func TestCodeCannotBeReusedOrGuessedIndefinitely(t *testing.T) {
	for _, tc := range []struct {
		name        string
		used        bool
		attempts    int
		sent, now   int64
		code, input string
		ok          bool
	}{
		{"valid", false, 0, 1000, 1100, "123456", "123456", true},
		{"used", true, 0, 1000, 1100, "123456", "123456", false},
		{"expired", false, 0, 1000, 1301, "123456", "123456", false},
		{"guesses exhausted", false, 5, 1000, 1100, "123456", "123456", false},
		{"wrong code", false, 0, 1000, 1100, "123456", "999999", false},
		{"empty code", false, 0, 1000, 1100, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckCode(tc.used, tc.attempts, tc.sent, tc.now, 300, tc.code, tc.input)
			if (err == nil) != tc.ok {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
