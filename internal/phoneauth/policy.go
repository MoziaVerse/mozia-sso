// Package phoneauth holds the policy shared by the opt-in phone signup/signin flow.
package phoneauth

import (
	"crypto/subtle"
	"errors"
	"fmt"
)

type RateLimitError struct {
	RetryAfterSeconds int `json:"retryAfterSeconds"`
}

func (e *RateLimitError) Error() string {
	if e.RetryAfterSeconds > 0 {
		return fmt.Sprintf("请 %d 秒后再获取验证码", e.RetryAfterSeconds)
	}
	return "24 小时内验证码获取次数已达上限，请稍后再试"
}

func CheckSendLimit(now, last, count int64) *RateLimitError {
	if count >= 10 {
		return &RateLimitError{}
	}
	if last > 0 && now-last < 60 {
		return &RateLimitError{RetryAfterSeconds: int(60 - (now - last))}
	}
	return nil
}

func CheckCode(used bool, attempts int, sent, now, ttl int64, code, input string) error {
	if used || attempts >= 5 || now-sent > ttl || input == "" || subtle.ConstantTimeCompare([]byte(code), []byte(input)) != 1 {
		return errors.New("验证码错误、已使用或已过期，请重新获取")
	}
	return nil
}
