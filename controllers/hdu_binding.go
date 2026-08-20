package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/casdoor/casdoor/idp"
	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/util"
)

const (
	hduCASLoginURL    = "https://sso.hdu.edu.cn/login"
	hduCASValidateURL = "https://sso.hdu.edu.cn/p3/serviceValidate"
)

type createHduBindingForm struct {
	Subject   string `json:"subject"`
	ReturnURL string `json:"returnUrl"`
}

type unlinkHduBindingForm struct {
	Subject        string `json:"subject"`
	BindingVersion string `json:"bindingVersion"`
}

type hduBindingAdminView struct {
	Subject           string `json:"subject"`
	UserName          string `json:"userName"`
	HduVerified       bool   `json:"hduVerified"`
	HduVerifiedAt     string `json:"hduVerifiedAt"`
	HduIdentityMasked string `json:"hduIdentityMasked"`
	BindingVersion    string `json:"bindingVersion"`
}

func getBasicApplication(request *http.Request) (*object.Application, error) {
	clientID, clientSecret, ok := request.BasicAuth()
	if !ok || clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("missing application credentials")
	}
	application, err := object.GetApplicationByClientId(clientID)
	if err != nil || application == nil {
		return nil, fmt.Errorf("invalid application credentials")
	}
	if subtle.ConstantTimeCompare([]byte(application.ClientSecret), []byte(clientSecret)) != 1 {
		return nil, fmt.Errorf("invalid application credentials")
	}
	return application, nil
}

func isHduBindingAdminClientAllowed(clientID, configured string) bool {
	for _, candidate := range strings.Split(configured, ",") {
		if strings.TrimSpace(candidate) == clientID && clientID != "" {
			return true
		}
	}
	return false
}

func getHduBindingAdminApplication(request *http.Request) (*object.Application, error) {
	application, err := getBasicApplication(request)
	if err != nil {
		return nil, err
	}
	if !isHduBindingAdminClientAllowed(application.ClientId, os.Getenv("HDU_BINDING_ADMIN_CLIENT_IDS")) {
		return nil, fmt.Errorf("application is not allowed to administer HDU bindings")
	}
	return application, nil
}

func maskHduIdentity(subject string) string {
	runes := []rune(strings.TrimSpace(subject))
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:2]) + strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-2:])
}

func hduBindingVersion(user *object.User, signingKey string) string {
	if user == nil || user.HduCAS == "" {
		return ""
	}
	value := strings.Join([]string{user.Owner, user.Id, user.HduCAS, user.HduVerifiedAt}, "\x00")
	digest := hmac.New(sha256.New, []byte(signingKey))
	_, _ = digest.Write([]byte(value))
	return hex.EncodeToString(digest.Sum(nil))
}

func newHduBindingAdminView(user *object.User, signingKey string) hduBindingAdminView {
	return hduBindingAdminView{
		Subject:           user.Id,
		UserName:          user.Name,
		HduVerified:       user.HduCAS != "",
		HduVerifiedAt:     user.HduVerifiedAt,
		HduIdentityMasked: maskHduIdentity(user.HduCAS),
		BindingVersion:    hduBindingVersion(user, signingKey),
	}
}

func validHduBindingVersion(version string) bool {
	decoded, err := hex.DecodeString(version)
	return err == nil && len(decoded) == sha256.Size
}

func urlOrigin(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", false
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), true
}

func isApplicationReturnURLAllowed(application *object.Application, returnURL string) bool {
	requestedOrigin, ok := urlOrigin(returnURL)
	if !ok {
		return false
	}
	allowed := append([]string{}, application.RedirectUris...)
	allowed = append(allowed, application.HomepageUrl)
	for _, candidate := range allowed {
		if origin, valid := urlOrigin(candidate); valid && origin == requestedOrigin {
			return true
		}
	}
	return false
}

func requestOrigin(request *http.Request) string {
	scheme := "https"
	if request.TLS == nil {
		hostname := strings.Split(request.Host, ":")[0]
		if hostname == "localhost" || hostname == "127.0.0.1" {
			scheme = "http"
		}
	}
	return fmt.Sprintf("%s://%s", scheme, request.Host)
}

func hduBindingService(record *object.HduBindingTicket, token string) string {
	return fmt.Sprintf("%s/api/hdu-binding/callback?token=%s", record.CallbackOrigin, url.QueryEscape(token))
}

func redirectWithHduStatus(c *ApiController, returnURL, status string) {
	target, err := url.Parse(returnURL)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Ctx.Output.Body([]byte("Invalid HDU binding return URL"))
		return
	}
	query := target.Query()
	query.Set("hdu", status)
	target.RawQuery = query.Encode()
	http.Redirect(c.Ctx.ResponseWriter, c.Ctx.Request, target.String(), http.StatusFound)
}

func (c *ApiController) CreateHduBinding() {
	application, err := getBasicApplication(c.Ctx.Request)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.ResponseError(err.Error())
		return
	}
	var form createHduBindingForm
	if err = json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil || strings.TrimSpace(form.Subject) == "" {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.ResponseError("invalid HDU binding request")
		return
	}
	if !isApplicationReturnURLAllowed(application, form.ReturnURL) {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.ResponseError("invalid HDU binding return URL")
		return
	}
	providerItem := application.GetProviderItemByType(object.HduCASProviderType)
	if providerItem == nil || providerItem.Provider == nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.ResponseError("HDU CAS provider is not configured")
		return
	}
	user, err := object.GetUserByField(application.Organization, "id", strings.TrimSpace(form.Subject))
	if err != nil || user == nil {
		c.Ctx.Output.SetStatus(http.StatusNotFound)
		c.ResponseError("user not found")
		return
	}
	if user.HduVerifiedAt != "" {
		c.Ctx.Output.SetStatus(http.StatusConflict)
		c.ResponseError("HDU identity is already verified")
		return
	}
	token, err := object.CreateHduBindingTicket(user, form.ReturnURL, requestOrigin(c.Ctx.Request))
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.ResponseError("failed to create HDU binding ticket")
		return
	}
	c.ResponseOk(map[string]string{"path": "/api/hdu-binding/start?token=" + url.QueryEscape(token)})
}

func (c *ApiController) StartHduBinding() {
	token := c.Ctx.Input.Query("token")
	record, err := object.GetActiveHduBindingTicket(token)
	if err != nil || record == nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Ctx.Output.Body([]byte("HDU binding link is invalid or expired"))
		return
	}
	service := hduBindingService(record, token)
	http.Redirect(c.Ctx.ResponseWriter, c.Ctx.Request, hduCASLoginURL+"?service="+url.QueryEscape(service), http.StatusFound)
}

func (c *ApiController) CompleteHduBinding() {
	token := c.Ctx.Input.Query("token")
	record, err := object.GetActiveHduBindingTicket(token)
	if err != nil || record == nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Ctx.Output.Body([]byte("HDU binding link is invalid or expired"))
		return
	}
	service := hduBindingService(record, token)
	provider := idp.NewHduCasIdProvider(hduCASValidateURL, record.CallbackOrigin+"/api/hdu-binding/callback")
	oauthToken, err := provider.GetToken(fmt.Sprintf(`{"ticket":%q,"service":%q}`, c.Ctx.Input.Query("ticket"), service))
	if err != nil {
		redirectWithHduStatus(c, record.ReturnURL, "failed")
		return
	}
	info, err := provider.GetUserInfo(oauthToken)
	if err != nil {
		redirectWithHduStatus(c, record.ReturnURL, "failed")
		return
	}
	consumed, err := object.ConsumeHduBindingTicket(token)
	if err != nil || !consumed {
		redirectWithHduStatus(c, record.ReturnURL, "failed")
		return
	}
	user, err := object.GetUser(fmt.Sprintf("%s/%s", record.UserOwner, record.UserName))
	if err != nil || user == nil {
		redirectWithHduStatus(c, record.ReturnURL, "failed")
		return
	}
	if user.HduCAS != "" && user.HduCAS != info.Id {
		redirectWithHduStatus(c, record.ReturnURL, "conflict")
		return
	}
	if user.HduCAS == "" {
		if _, err = object.LinkUserAccount(user, object.HduCASProviderType, info.Id); err != nil {
			if errors.Is(err, object.ErrHduIdentityAlreadyLinked) {
				redirectWithHduStatus(c, record.ReturnURL, "conflict")
				return
			}
			redirectWithHduStatus(c, record.ReturnURL, "failed")
			return
		}
	}
	redirectWithHduStatus(c, record.ReturnURL, "success")
}

func (c *ApiController) GetAdminHduBinding() {
	application, err := getHduBindingAdminApplication(c.Ctx.Request)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.ResponseError("invalid HDU binding administrator credentials")
		return
	}
	subject := strings.TrimSpace(c.Ctx.Input.Query("subject"))
	if subject == "" || utf8.RuneCountInString(subject) > 255 {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.ResponseError("invalid subject")
		return
	}
	user, err := object.GetUserByField(application.Organization, "id", subject)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.ResponseError("failed to read HDU binding")
		return
	}
	if user == nil {
		c.Ctx.Output.SetStatus(http.StatusNotFound)
		c.ResponseError("user not found")
		return
	}
	c.ResponseOk(newHduBindingAdminView(user, application.ClientSecret))
}

func (c *ApiController) UnlinkAdminHduBinding() {
	application, err := getHduBindingAdminApplication(c.Ctx.Request)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.ResponseError("invalid HDU binding administrator credentials")
		return
	}
	var form unlinkHduBindingForm
	if err = json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.ResponseError("invalid HDU unlink request")
		return
	}
	form.Subject = strings.TrimSpace(form.Subject)
	if form.Subject == "" || utf8.RuneCountInString(form.Subject) > 255 {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.ResponseError("invalid HDU unlink request")
		return
	}
	user, err := object.GetUserByField(application.Organization, "id", form.Subject)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.ResponseError("failed to read HDU binding")
		return
	}
	if user == nil {
		c.Ctx.Output.SetStatus(http.StatusNotFound)
		c.ResponseError("user not found")
		return
	}
	if user.HduCAS == "" {
		c.ResponseOk(map[string]bool{"unlinked": false, "alreadyUnlinked": true})
		return
	}
	if !validHduBindingVersion(form.BindingVersion) || subtle.ConstantTimeCompare([]byte(form.BindingVersion), []byte(hduBindingVersion(user, application.ClientSecret))) != 1 {
		c.Ctx.Output.SetStatus(http.StatusConflict)
		c.ResponseError("HDU identity binding changed")
		return
	}
	if _, err = object.UnlinkHduCAS(user, user.HduCAS); err != nil {
		if errors.Is(err, object.ErrHduIdentityBindingChanged) {
			c.Ctx.Output.SetStatus(http.StatusConflict)
			c.ResponseError("HDU identity binding changed")
			return
		}
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.ResponseError("failed to unlink HDU identity")
		return
	}
	util.LogInfo(c.Ctx, "API: application [%s] unlinked HDU identity from user [%s]", application.ClientId, user.Id)
	c.ResponseOk(map[string]bool{"unlinked": true, "alreadyUnlinked": false})
}
