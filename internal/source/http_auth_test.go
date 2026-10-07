package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// AUTH-01: a configured token is sent as a Bearer credential on every request.
func TestHTTPSourceSendsBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	src := NewHTTPSourceWithOptions(srv.URL, "", "secret-token")
	if _, _, err := src.fetchJSON(context.Background(), srv.URL+"/api/v1/manifest"); err != nil {
		t.Fatalf("fetchJSON: %v", err)
	}

	if want := "Bearer secret-token"; gotAuth != want {
		t.Errorf("Authorization = %q, 期望 %q", gotAuth, want)
	}
}

// AUTH-02: without a token no Authorization header is added.
func TestHTTPSourceOmitsTokenWhenEmpty(t *testing.T) {
	sawAuth := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawAuth = r.Header["Authorization"]
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	src := NewHTTPSource(srv.URL)
	if _, _, err := src.fetchJSON(context.Background(), srv.URL+"/api/v1/manifest"); err != nil {
		t.Fatalf("fetchJSON: %v", err)
	}

	if sawAuth {
		t.Error("未配置令牌时不应发送 Authorization 头")
	}
}
