package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/routerctl/internal/backend"
)

func TestResolveReleaseAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/repos/acme/router/releases/tags/v1.0.0"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer secret"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"assets":[{"name":"ax23v.bin","browser_download_url":"https://downloads.example/ax23v.bin","digest":"sha256:abc"}]}`))
	}))
	defer server.Close()

	b, err := New(Config{Repository: "acme/router", Tag: "v1.0.0", Token: "secret", APIURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := b.Resolve(context.Background(), backend.Request{Device: "ax23v", Target: "ax23v.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Path != "https://downloads.example/ax23v.bin" || artifact.Digest != "sha256:abc" {
		t.Fatalf("artifact = %#v", artifact)
	}
}

func TestResolveUsesLatestReleaseAndReportsMissingAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/repos/acme/router/releases/latest"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"assets":[{"name":"other.bin","browser_download_url":"https://downloads.example/other.bin"}]}`))
	}))
	defer server.Close()

	b, err := New(Config{Repository: "acme/router", APIURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Resolve(context.Background(), backend.Request{Target: "ax23v.bin"})
	if err == nil || err.Error() != `github backend: release asset "ax23v.bin" not found in acme/router` {
		t.Fatalf("Resolve error = %v", err)
	}
}

func TestNewRejectsInvalidRepository(t *testing.T) {
	if _, err := New(Config{Repository: "not-a-repository"}); err == nil {
		t.Fatal("New succeeded for an invalid repository")
	}
}
