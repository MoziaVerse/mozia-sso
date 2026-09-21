package controllers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/casdoor/casdoor/form"
	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/util"
)

func validatePhoneSignin(app *object.Application, input *form.AuthForm) error {
	if !app.EnablePhoneSigninSignup || app.DisableSignin || !app.IsCodeSigninViaSmsEnabled() || app.Organization == "built-in" || app.Organization != input.Organization {
		return fmt.Errorf("此应用未启用手机号一体登录")
	}
	if input.PhoneSigninSignup && phoneSigninNeedsAgreement(app) && !input.Agreement {
		return fmt.Errorf("请先同意用户协议和隐私政策")
	}
	return nil
}

// Registration requires consent; login-only organizations retain their existing
// sign-in agreement policy instead of gaining a registration checkbox.
func phoneSigninNeedsAgreement(app *object.Application) bool {
	if app.EnableSignUp {
		return true
	}
	for _, item := range app.SignupItems {
		if item.Name == "Agreement" && item.Required && item.Rule != "" && item.Rule != "None" {
			return true
		}
	}
	return false
}

func phoneSignupForm(input form.AuthForm) (form.AuthForm, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return input, err
	}
	input.Phone = util.GetSeperatedPhone(input.Username)
	input.PhoneCode = input.Code
	input.Username = "mozia_" + hex.EncodeToString(b[:9])
	input.Name = input.Username
	input.Password = "Mz" + base64.RawURLEncoding.EncodeToString(b) + "9@!"
	// Growth invitations stay in Matrix; the unified form does not accept a Casdoor invitation.
	input.InvitationCode = ""
	return input, nil
}

// Returns true when the new-user path has written the response. The caller holds
// the normalized phone lock through login or signup, including code consumption.
func (c *ApiController) preparePhoneSignin(input *form.AuthForm) (bool, func()) {
	noop := func() {}
	if input.Password != "" || (input.SigninMethod != "Verification code" && input.SigninMethod != "") || input.Username == "" || object.GetVerifyType(input.Username) != object.VerifyTypePhone {
		return false, noop
	}
	app, err := object.GetApplication("admin/" + input.Application)
	if err != nil {
		c.ResponseError(err.Error())
		return true, noop
	}
	if app == nil || !app.EnablePhoneSigninSignup {
		if input.PhoneSigninSignup {
			c.ResponseError("此应用未启用手机号一体登录")
			return true, noop
		}
		return false, noop
	}
	if err = validatePhoneSignin(app, input); err != nil {
		c.ResponseError(err.Error())
		return true, noop
	}
	// Validate the OAuth request before consuming a challenge or creating an
	// identity. The application's organization must not come from another client.
	if input.PhoneSigninSignup && input.Type == ResponseTypeCode {
		msg, oauthApp, oauthErr := object.CheckOAuthLogin(
			c.Ctx.Input.Query("clientId"), c.Ctx.Input.Query("responseType"),
			c.Ctx.Input.Query("redirectUri"), c.Ctx.Input.Query("scope"),
			c.Ctx.Input.Query("state"), c.GetAcceptLanguage())
		if oauthErr != nil {
			c.ResponseError(oauthErr.Error())
			return true, noop
		}
		if msg != "" || oauthApp == nil || oauthApp.GetId() != app.GetId() {
			c.ResponseError("授权应用或回调地址不匹配，请重新发起登录")
			return true, noop
		}
	}
	if input.CountryCode == "" {
		input.CountryCode = "CN"
	}
	phone, ok := util.GetE164Number(input.Username, input.CountryCode)
	if !ok {
		c.ResponseError("请输入有效的手机号")
		return true, noop
	}
	unlock, err := object.LockPhoneAuthentication(phone)
	if err != nil {
		c.ResponseError(err.Error())
		return true, noop
	}
	user, err := object.GetUserByPhone(app.Organization, phone)
	if err != nil {
		unlock()
		c.ResponseError(err.Error())
		return true, noop
	}
	if user != nil {
		userPhone, valid := util.GetE164Number(user.Phone, user.GetCountryCode(input.CountryCode))
		if !valid || userPhone != phone {
			unlock()
			c.ResponseError("手机号地区与账号不匹配")
			return true, noop
		}
		return false, unlock
	}
	if !input.PhoneSigninSignup {
		return false, unlock
	}
	if !app.EnableSignUp {
		unlock()
		c.ResponseError("此应用暂不允许注册新账号")
		return true, noop
	}
	input.Username = phone
	signup, err := phoneSignupForm(*input)
	if err != nil {
		unlock()
		c.ResponseError(err.Error())
		return true, noop
	}
	c.signup(signup, true)
	unlock()
	return true, noop
}
