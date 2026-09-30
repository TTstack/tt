package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKeysFailClosedWithoutEchoingSecrets(t *testing.T) {
	for _, key := range []string{"", " ", "secret\nheader", "non-ascii-密钥"} {
		_, err := ParseAPIKey(key)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal(err)
		}
	}
	if key, err := ParseAPIKey("tt-valid-key"); err != nil || key != "tt-valid-key" {
		t.Fatal(key, err)
	}
}
func TestEqualStrings(t *testing.T) {
	for _, s := range []string{"", "abc", "tt-0123456789abcdef"} {
		if !ConstantTimeEqual(s, s) {
			t.Fatal(s)
		}
	}
	for _, p := range [][2]string{{"abc", "abd"}, {"abc", "ab"}, {"", "a"}} {
		if ConstantTimeEqual(p[0], p[1]) {
			t.Fatal(p)
		}
	}
}
func TestBearerWhitespaceIsNotNormalized(t *testing.T) {
	key := "valid-key"
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	for header, want := range map[string]int{"Bearer valid-key": 204, "bEaReR valid-key": 204, "Bearer  valid-key": 401, "Bearer valid-key ": 401, "Bearer\tvalid-key": 401, "": 401} {
		r := httptest.NewRequest("GET", "/api/status", nil)
		r.Header.Set("Authorization", header)
		w := httptest.NewRecorder()
		Middleware(&key, next).ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("%q: %d", header, w.Code)
		}
		if want == 401 && w.Body.String() != `{"ok":false,"error":"invalid or missing API key"}` {
			t.Error(w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	Middleware(nil, next).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
}
