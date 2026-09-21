/* eslint-env jest */
import {isUnifiedPhoneLogin, phoneSigninValues, unifiedPhoneMethod} from "./phoneSignin";

describe("手机号一体入口", () => {
  it("仅启用应用的手机号验证码模式进入一体注册", () => {
    expect(isUnifiedPhoneLogin({enablePhoneSigninSignup: true, signinMethods: [{name: "Verification code", rule: "Phone only"}]}, "verificationCodePhone", false)).toBe(true);
    expect(isUnifiedPhoneLogin({enablePhoneSigninSignup: true, signinMethods: [{name: "Verification code", rule: "Phone only"}]}, "verificationCodeEmail", true)).toBe(false);
    expect(isUnifiedPhoneLogin({enablePhoneSigninSignup: true, signinMethods: [{name: "Verification code", rule: "Phone only"}]}, "password", false)).toBe(false);
    expect(isUnifiedPhoneLogin({}, "verificationCodePhone", false)).toBe(false);
  });
  it("默认选择已配置的手机方式，关闭开关或仅邮箱应用保持旧行为", () => {
    expect(unifiedPhoneMethod({enablePhoneSigninSignup: true, signinMethods: [{name: "Password", rule: "All"}, {name: "Verification code", rule: "Phone only"}]})).toBe("verificationCodePhone");
    expect(unifiedPhoneMethod({enablePhoneSigninSignup: true, signinMethods: [{name: "Verification code", rule: "All"}]})).toBe("verificationCode");
    expect(unifiedPhoneMethod({enablePhoneSigninSignup: true, signinMethods: [{name: "Verification code", rule: "Email only"}]})).toBeNull();
    expect(unifiedPhoneMethod({enablePhoneSigninSignup: false})).toBeNull();
  });
  it("账号密码切回手机时丢弃保留的密码，不改变其他授权参数", () => {
    const values = {password: "old-password", code: "123456", state: "state", nonce: "nonce", code_challenge: "challenge"};
    expect(phoneSigninValues(values, true)).toEqual({code: "123456", state: "state", nonce: "nonce", code_challenge: "challenge", phoneSigninSignup: true});
    expect(phoneSigninValues(values, false).password).toBe("old-password");
  });
  it("保留授权表单字段并显式标记入口，旧方式不增加标记", () => {
    const values = {username: "13800138000", agreement: true, application: "app", organization: "org"};
    expect(phoneSigninValues(values, true)).toEqual({...values, phoneSigninSignup: true});
    expect(phoneSigninValues(values, false)).toEqual(values);
    expect(values.phoneSigninSignup).toBeUndefined();
  });
});
