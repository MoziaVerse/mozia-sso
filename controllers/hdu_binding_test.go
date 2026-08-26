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

func TestIsHduBindingAdminClientAllowed(t *testing.T) {
	if !isHduBindingAdminClientAllowed("mega-client", "matrix-client, mega-client") {
		t.Fatal("expected configured Mega client to be allowed")
	}
	if isHduBindingAdminClientAllowed("matrix-client", "mega-client") {
		t.Fatal("expected an unlisted client to be rejected")
	}
	if isHduBindingAdminClientAllowed("", "") {
		t.Fatal("expected an empty configuration to fail closed")
	}
}

func TestMaskHduIdentity(t *testing.T) {
	if actual := maskHduIdentity("0220261133"); actual != "02******33" {
		t.Fatalf("maskHduIdentity() = %q", actual)
	}
	if actual := maskHduIdentity("1234"); actual != "****" {
		t.Fatalf("short identity was not fully masked: %q", actual)
	}
}

func TestHasHduIdentityBindingUsesBindingValue(t *testing.T) {
	user := &object.User{HduVerifiedAt: "2026-08-20T12:00:00Z"}
	if hasHduIdentityBinding(user) {
		t.Fatal("stale verification time must not count as an HDU identity binding")
	}
	user.HduCAS = "0220261133"
	if !hasHduIdentityBinding(user) {
		t.Fatal("expected the HDU identity value to count as a binding")
	}
}

func TestHduBindingVersionChangesWithBinding(t *testing.T) {
	user := &object.User{Owner: "Mozia", Id: "subject-1", HduCAS: "0220261133", HduVerifiedAt: "2026-08-20T12:00:00Z"}
	first := hduBindingVersion(user, "test-signing-key")
	if !validHduBindingVersion(first) {
		t.Fatalf("invalid binding version: %q", first)
	}
	user.HduCAS = "0220261134"
	user.HduVerifiedAt = "2026-08-20T12:01:00Z"
	if second := hduBindingVersion(user, "test-signing-key"); second == first {
		t.Fatal("binding version did not change with the HDU identity")
	}
}

func TestGetHduBindingAdminOrganization(t *testing.T) {
	t.Setenv("HDU_BINDING_ADMIN_ORGANIZATION", " Mozia ")
	if actual := getHduBindingAdminOrganization(&object.Application{Organization: "mozia-internal"}); actual != "Mozia" {
		t.Fatalf("expected configured organization, got %q", actual)
	}
}
