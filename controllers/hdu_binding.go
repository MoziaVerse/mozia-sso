package controllers

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/casdoor/casdoor/idp"
	"github.com/casdoor/casdoor/object"
)

const (
	hduCASLoginURL    = "https://sso.hdu.edu.cn/login"
	hduCASValidateURL = "https://sso.hdu.edu.cn/p3/serviceValidate"
)

type createHduBindingForm struct {
	Subject   string `json:"subject"`
	ReturnURL string `json:"returnUrl"`
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
	if err != nil || user == nil || (user.HduCAS != "" && user.HduCAS != info.Id) {
		redirectWithHduStatus(c, record.ReturnURL, "failed")
		return
	}
	if user.HduCAS == "" {
		if _, err = object.LinkUserAccount(user, object.HduCASProviderType, info.Id); err != nil {
			redirectWithHduStatus(c, record.ReturnURL, "failed")
			return
		}
	}
	redirectWithHduStatus(c, record.ReturnURL, "success")
}
