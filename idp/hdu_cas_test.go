package idp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHduCasIdProviderValidatesTicketAndService(t *testing.T) {
	service := "https://auth.example.com/callback?state=state-123"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ticket") != "ST-123" || r.URL.Query().Get("service") != service {
			t.Fatalf("unexpected validation query: %s", r.URL.RawQuery)
		}
		_, _ = fmt.Fprint(w, `<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas"><cas:authenticationSuccess><cas:user>student-1</cas:user><cas:attributes><cas:name>Student One</cas:name><cas:email>student@example.com</cas:email></cas:attributes></cas:authenticationSuccess></cas:serviceResponse>`)
	}))
	defer server.Close()

	provider := NewHduCasIdProvider(server.URL, "https://auth.example.com/callback")
	provider.SetHttpClient(server.Client())
	code := fmt.Sprintf(`{"ticket":"ST-123","service":%q}`, service)
	token, err := provider.GetToken(code)
	if err != nil {
		t.Fatal(err)
	}
	user, err := provider.GetUserInfo(token)
	if err != nil {
		t.Fatal(err)
	}
	if user.Id != "student-1" || user.DisplayName != "Student One" || user.Email != "student@example.com" {
		t.Fatalf("unexpected user: %#v", user)
	}
	if _, err = provider.GetToken(fmt.Sprintf(`{"ticket":"ST-123","service":%q}`, (&url.URL{Scheme: "https", Host: "evil.example", Path: "/callback"}).String())); err == nil {
		t.Fatal("expected foreign callback to be rejected")
	}
}
