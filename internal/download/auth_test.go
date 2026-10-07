package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// AUTH-04: a configured token is sent as a Bearer credential.
func TestDownloadSendsBearerToken(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte("payload"))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "file.bin")
	if _, err := NewManager().Download(context.Background(), Options{
		URL:       server.URL,
		DestPath:  dest,
		AuthToken: "secret",
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}

	if want := "Bearer secret"; gotAuth != want {
		t.Errorf("Authorization = %q, 期望 %q", gotAuth, want)
	}
}

// AUTH-05: without a token no Authorization header is added.
func TestDownloadOmitsTokenWhenEmpty(t *testing.T) {
	sawAuth := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawAuth = r.Header["Authorization"]
		w.Write([]byte("payload"))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "file.bin")
	if _, err := NewManager().Download(context.Background(), Options{
		URL:      server.URL,
		DestPath: dest,
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}

	if sawAuth {
		t.Error("未配置令牌时不应发送 Authorization 头")
	}
}
