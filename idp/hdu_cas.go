package idp

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const hduCasValidateURL = "https://sso.hdu.edu.cn/p3/serviceValidate"

type HduCasIdProvider struct {
	Client      *http.Client
	ValidateURL string
	RedirectURL string
}

type hduCasCode struct {
	Ticket  string `json:"ticket"`
	Service string `json:"service"`
}

type hduCasResponse struct {
	Success *struct {
		User       string `xml:"user"`
		Attributes struct {
			Name  string `xml:"name"`
			Email string `xml:"email"`
		} `xml:"attributes"`
	} `xml:"authenticationSuccess"`
	Failure *struct {
		Message string `xml:",chardata"`
	} `xml:"authenticationFailure"`
}

func NewHduCasIdProvider(validateURL, redirectURL string) *HduCasIdProvider {
	return &HduCasIdProvider{ValidateURL: validateURL, RedirectURL: redirectURL}
}

func (p *HduCasIdProvider) SetHttpClient(client *http.Client) { p.Client = client }

func (p *HduCasIdProvider) GetToken(code string) (*oauth2.Token, error) {
	var payload hduCasCode
	if err := json.Unmarshal([]byte(code), &payload); err != nil || strings.TrimSpace(payload.Ticket) == "" || strings.TrimSpace(payload.Service) == "" {
		return nil, fmt.Errorf("invalid HDU CAS callback")
	}
	callback, err := url.Parse(payload.Service)
	if err != nil || callback.Scheme == "" || callback.Host == "" || !strings.HasPrefix(payload.Service, p.RedirectURL+"?") {
		return nil, fmt.Errorf("invalid HDU CAS service")
	}
	return (&oauth2.Token{AccessToken: payload.Ticket, Expiry: time.Now().Add(time.Minute)}).WithExtra(map[string]interface{}{"service": payload.Service}), nil
}

func (p *HduCasIdProvider) GetUserInfo(token *oauth2.Token) (*UserInfo, error) {
	service, _ := token.Extra("service").(string)
	validateURL, err := url.Parse(p.ValidateURL)
	if err != nil || validateURL.Scheme != "https" || validateURL.Host == "" || service == "" {
		return nil, fmt.Errorf("invalid HDU CAS validation request")
	}
	query := validateURL.Query()
	query.Set("ticket", token.AccessToken)
	query.Set("service", service)
	validateURL.RawQuery = query.Encode()
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Get(validateURL.String())
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HDU CAS validation returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var result hduCasResponse
	if err = xml.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid HDU CAS response: %w", err)
	}
	if result.Success == nil || strings.TrimSpace(result.Success.User) == "" {
		if result.Failure != nil {
			return nil, fmt.Errorf("HDU CAS rejected ticket: %s", strings.TrimSpace(result.Failure.Message))
		}
		return nil, fmt.Errorf("HDU CAS response has no authenticated user")
	}
	username := strings.TrimSpace(result.Success.User)
	displayName := strings.TrimSpace(result.Success.Attributes.Name)
	if displayName == "" {
		displayName = username
	}
	return &UserInfo{Id: username, Username: username, DisplayName: displayName, Email: strings.TrimSpace(result.Success.Attributes.Email)}, nil
}
