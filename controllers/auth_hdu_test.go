package controllers

import (
	"strings"
	"testing"

	"github.com/casdoor/casdoor/idp"
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

func TestRepairGeneratedHduCasPlatformName(t *testing.T) {
	userInfo := &idp.UserInfo{Id: "student-number", Username: "student-number", DisplayName: "student-number"}
	user := &object.User{Name: "mozia_12345678", DisplayName: "student-number", HduCAS: "student-number"}
	repairGeneratedHduCasPlatformName(userInfo, user)

	if userInfo.Id != "student-number" {
		t.Fatalf("HDU binding ID must be preserved, got %q", userInfo.Id)
	}
	if user.DisplayName != user.Name || userInfo.Username != user.Name || userInfo.DisplayName != user.Name {
		t.Fatalf("expected generated HDU account names to be repaired: user=%q username=%q displayName=%q", user.DisplayName, userInfo.Username, userInfo.DisplayName)
	}

	custom := &object.User{Name: "mozia_87654321", DisplayName: "Custom Name", HduCAS: "student-number"}
	customInfo := &idp.UserInfo{Id: "student-number", Username: "student-number", DisplayName: "student-number"}
	repairGeneratedHduCasPlatformName(customInfo, custom)
	if custom.DisplayName != "Custom Name" || customInfo.DisplayName != "student-number" {
		t.Fatal("custom display names must be preserved")
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
