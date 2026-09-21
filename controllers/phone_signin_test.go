package controllers

import (
	"regexp"
	"testing"

	"github.com/casdoor/casdoor/form"
	"github.com/casdoor/casdoor/object"
)

func TestPhoneSignupKeepsOAuthContextAndGeneratesCredentials(t *testing.T) {
	original := form.AuthForm{Type: "code", Application: "test-app", Organization: "test-org", Username: "13800138000", Code: "123456", CountryCode: "CN", AutoSignin: true, Agreement: true, PhoneSigninSignup: true}
	got, err := phoneSignupForm(original)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "code" || got.Application != "test-app" || got.Organization != "test-org" || !got.AutoSignin {
		t.Fatal("lost authorization context")
	}
	if got.Phone != "13800138000" || got.PhoneCode != "123456" || got.Username == original.Username || len(got.Password) < 24 {
		t.Fatal("invalid signup identity")
	}
	if !regexp.MustCompile(`^mozia_[a-f0-9]{18}$`).MatchString(got.Username) {
		t.Fatal("generated username violates account rules")
	}
	if got.InvitationCode != "" {
		t.Fatal("business invitation must not become Casdoor invitation")
	}
}

func TestPhoneSigninPolicyDoesNotEnableOtherOrganizationsOrApplications(t *testing.T) {
	app := &object.Application{Organization: "test-org", EnablePhoneSigninSignup: true, EnableSignUp: true, SigninMethods: []*object.SigninMethod{{Name: "Verification code", Rule: "Phone only"}}}
	for _, tc := range []struct {
		org       string
		agreement bool
		ok        bool
	}{{"test-org", true, true}, {"other-org", true, false}, {"test-org", false, false}} {
		err := validatePhoneSignin(app, &form.AuthForm{Organization: tc.org, Agreement: tc.agreement, PhoneSigninSignup: true})
		if (err == nil) != tc.ok {
			t.Fatalf("org=%s agreement=%v err=%v", tc.org, tc.agreement, err)
		}
	}
	app.EnablePhoneSigninSignup = false
	if validatePhoneSignin(app, &form.AuthForm{Organization: "test-org", Agreement: true, PhoneSigninSignup: true}) == nil {
		t.Fatal("disabled feature accepted")
	}
}

func TestPhoneLoginOnlyDoesNotRequireRegistrationConsent(t *testing.T) {
	app := &object.Application{Organization: "internal", EnablePhoneSigninSignup: true, SigninMethods: []*object.SigninMethod{{Name: "Verification code", Rule: "Phone only"}}}
	input := &form.AuthForm{Organization: "internal", PhoneSigninSignup: true}
	if err := validatePhoneSignin(app, input); err != nil {
		t.Fatal(err)
	}
	app.EnableSignUp = true
	if err := validatePhoneSignin(app, input); err == nil {
		t.Fatal("registration accepted without consent")
	}
	app.EnableSignUp = false
	app.SignupItems = []*object.SignupItem{{Name: "Agreement", Required: true, Rule: "Signin (Default False)"}}
	if err := validatePhoneSignin(app, input); err == nil {
		t.Fatal("ignored configured sign-in agreement")
	}
}
