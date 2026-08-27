package controllers

import (
	"strings"
	"testing"

	"github.com/casdoor/casdoor/object"
)

func TestGenerateHduCasSignupIdentity(t *testing.T) {
	userID, username, err := generateHduCasSignupIdentity(&object.Application{})
	if err != nil {
		t.Fatal(err)
	}
	if userID == "" || userID == "student-number" {
		t.Fatalf("expected a generated platform ID, got %q", userID)
	}
	if !strings.HasPrefix(username, "mozia_") || len(username) != len("mozia_")+8 {
		t.Fatalf("expected Matrix-style generated username, got %q", username)
	}
}

func TestHduCasSignupDoesNotMatchExistingUsersByProfile(t *testing.T) {
	user, err := getExistUserByBindingRule(object.HduCASProviderType, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if user != nil {
		t.Fatal("HDU CAS signup must not match an existing user by email, phone, or name")
	}
}
