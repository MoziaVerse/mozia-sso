export function unifiedPhoneMethod(application) {
  if (!application?.enablePhoneSigninSignup) {
    return null;
  }
  const method = application.signinMethods?.find(item => item.name === "Verification code" && ["Phone only", "All"].includes(item.rule));
  return method ? (method.rule === "Phone only" ? "verificationCodePhone" : "verificationCode") : null;
}

export function isUnifiedPhoneLogin(application, method, validEmail) {
  return Boolean(unifiedPhoneMethod(application) && method?.startsWith("verificationCode") && method !== "verificationCodeEmail" && !validEmail);
}

export function phoneSigninValues(values, unified) {
  // Ant Design preserves unmounted fields. Do not let a password from the other
  // tab make the server treat a phone-code submission as password authentication.
  if (!unified) {
    return values;
  }
  const {password: _password, ...phoneValues} = values;
  return {...phoneValues, phoneSigninSignup: true};
}
