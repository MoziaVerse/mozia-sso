package controllers

import (
	"testing"

	"github.com/casdoor/casdoor/object"
)

func TestIsApplicationReturnURLAllowed(t *testing.T) {
	application := &object.Application{
		HomepageUrl: "https://matrix.example.com",
		RedirectUris: []string{
			"https://matrix.example.com/api/auth/callback/oidc",
			"http://localhost:5173/api/auth/callback/oidc",
		},
	}
	tests := []struct {
		url     string
		allowed bool
	}{
		{"https://matrix.example.com/api/me/hdu-binding/complete", true},
		{"http://localhost:5173/api/me/hdu-binding/complete", true},
		{"https://evil.example.com/api/me/hdu-binding/complete", false},
		{"https://matrix.example.com@evil.example.com/callback", false},
		{"javascript:alert(1)", false},
		{"https://matrix.example.com/callback#token", false},
	}
	for _, test := range tests {
		if actual := isApplicationReturnURLAllowed(application, test.url); actual != test.allowed {
			t.Fatalf("isApplicationReturnURLAllowed(%q) = %v, want %v", test.url, actual, test.allowed)
		}
	}
}
