// Copyright 2026 The Casdoor Authors. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/casdoor/casdoor/proxy"
)

type smsTestTransport func(*http.Request) (*http.Response, error)

func (f smsTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSendSmsHidesSignedRequest(t *testing.T) {
	var logs bytes.Buffer
	originalLogOutput := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(originalLogOutput) })
	original := proxy.DefaultHttpClient
	t.Cleanup(func() { proxy.DefaultHttpClient = original })
	proxy.DefaultHttpClient = &http.Client{Transport: smsTestTransport(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.RawQuery, "TEST_ACCESS_ID") {
			t.Fatal("test must exercise an HTTP request carrying sensitive query parameters")
		}
		return nil, &net.DNSError{Err: "i/o timeout", Name: "sms.invalid", IsTimeout: true}
	})}
	err := SendSms(&Provider{
		Type: "Custom HTTP SMS", Method: "POST", Title: "code",
		Endpoint: "https://sms.invalid/?AccessKeyId=TEST_ACCESS_ID&PhoneNumbers=TEST_PHONE&TemplateParam=TEST_CODE&Signature=TEST_SIGNATURE",
	}, "TEST_CODE", "TEST_PHONE")
	if err == nil || err.Error() != "Failed to send SMS, please try again later" {
		t.Fatalf("expected safe public SMS error, got %v", err)
	}
	if errors.Unwrap(err) != nil {
		t.Fatal("public error must not wrap sensitive cause")
	}
	if !strings.Contains(logs.String(), "category=dns_error") {
		t.Fatal("logs should retain a safe diagnostic category")
	}
	for _, sensitive := range []string{"TEST_ACCESS_ID", "TEST_PHONE", "TEST_CODE", "TEST_SIGNATURE", "sms.invalid"} {
		if strings.Contains(logs.String(), sensitive) {
			t.Fatalf("logs leaked %s", sensitive)
		}
	}
}

func TestSendSmsHidesClientInitializationError(t *testing.T) {
	err := SendSms(&Provider{Type: "unsupported-TEST_ACCESS_ID"}, "TEST_CODE", "TEST_PHONE")
	if err == nil || err.Error() != "Failed to send SMS, please try again later" {
		t.Fatalf("expected safe public SMS error, got %v", err)
	}
}

func TestSendSmsSuccessUnchanged(t *testing.T) {
	original := proxy.DefaultHttpClient
	t.Cleanup(func() { proxy.DefaultHttpClient = original })
	proxy.DefaultHttpClient = &http.Client{Transport: smsTestTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	err := SendSms(&Provider{Type: "Custom HTTP SMS", Method: "POST", Title: "code", Endpoint: "https://sms.invalid/"}, "TEST_CODE", "TEST_PHONE")
	if err != nil {
		t.Fatalf("successful send must stay successful: %v", err)
	}
}
