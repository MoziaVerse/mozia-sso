package object

import "testing"

func TestStrictApplicationRedirect(t *testing.T) {
	app := &Application{EnableStrictRedirectUri: true, RedirectUris: []string{"https://matrix.example/api/external/oidc/zeo-canvas/callback"}}
	for _, uri := range []string{
		"https://evil.example/?next=https://matrix.example/api/external/oidc/zeo-canvas/callback",
		"https://matrix.example/api/external/oidc/zeo-canvas/callback/extra",
		"https://matrix.example/api/external/oidc/zeo-canvas/callback?next=evil",
		"http://localhost/callback", "http://127.0.0.1/callback", "",
	} {
		if app.IsRedirectUriValid(uri) {
			t.Errorf("strict redirect accepted %q", uri)
		}
	}
	if !app.IsRedirectUriValid(app.RedirectUris[0]) {
		t.Fatal("exact callback rejected")
	}
	// Existing applications retain their pre-existing policy until explicitly opted in.
	app.EnableStrictRedirectUri = false
	if !app.IsRedirectUriValid("http://localhost/callback") {
		t.Fatal("legacy behavior changed")
	}
}
