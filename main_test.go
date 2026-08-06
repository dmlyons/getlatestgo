package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var sampleJSON = `[
  {
    "version": "go1.22.5",
    "stable": true,
    "files": [
      {"filename":"go1.22.5.linux-amd64.tar.gz","os":"linux","arch":"amd64","version":"go1.22.5","sha256":"aaaa","size":100,"kind":"archive"},
      {"filename":"go1.22.5.darwin-arm64.tar.gz","os":"darwin","arch":"arm64","version":"go1.22.5","sha256":"bbbb","size":100,"kind":"archive"},
      {"filename":"go1.22.5.windows-amd64.zip","os":"windows","arch":"amd64","version":"go1.22.5","sha256":"cccc","size":100,"kind":"archive"}
    ]
  },
  {
    "version": "go1.22.4",
    "stable": true,
    "files": [
      {"filename":"go1.22.4.linux-amd64.tar.gz","os":"linux","arch":"amd64","version":"go1.22.4","sha256":"dddd","size":100,"kind":"archive"}
    ]
  },
  {
    "version": "go1.23rc1",
    "stable": false,
    "files": []
  }
]`

func mustParseReleases(t *testing.T) []GoRelease {
	t.Helper()
	releases, err := ParseReleases([]byte(sampleJSON))
	if err != nil {
		t.Fatalf("ParseReleases: %v", err)
	}
	return releases
}

func TestParseReleases(t *testing.T) {
	releases := mustParseReleases(t)

	if len(releases) != 3 {
		t.Fatalf("expected 3 releases, got %d", len(releases))
	}
	if releases[0].Version != "go1.22.5" {
		t.Errorf("expected go1.22.5, got %s", releases[0].Version)
	}
	if !releases[0].Stable {
		t.Error("expected first release to be stable")
	}
	if releases[2].Stable {
		t.Error("expected rc release to not be stable")
	}
}

func TestParseReleasesInvalidJSON(t *testing.T) {
	_, err := ParseReleases([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFindRelease_Latest(t *testing.T) {
	releases := mustParseReleases(t)

	r, err := FindRelease(releases, "")
	if err != nil {
		t.Fatalf("FindRelease: %v", err)
	}
	if r.Version != "go1.22.5" {
		t.Errorf("expected go1.22.5, got %s", r.Version)
	}
}

func TestFindRelease_Specific(t *testing.T) {
	releases := mustParseReleases(t)

	r, err := FindRelease(releases, "go1.22.4")
	if err != nil {
		t.Fatalf("FindRelease: %v", err)
	}
	if r.Version != "go1.22.4" {
		t.Errorf("expected go1.22.4, got %s", r.Version)
	}
}

func TestFindRelease_NotFound(t *testing.T) {
	releases := mustParseReleases(t)

	_, err := FindRelease(releases, "go1.99.0")
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestFindRelease_Empty(t *testing.T) {
	_, err := FindRelease(nil, "")
	if err == nil {
		t.Fatal("expected error for empty releases")
	}
}

func TestFindFile(t *testing.T) {
	releases := mustParseReleases(t)

	tests := []struct {
		goos, goarch string
		wantFile     string
	}{
		{"linux", "amd64", "go1.22.5.linux-amd64.tar.gz"},
		{"darwin", "arm64", "go1.22.5.darwin-arm64.tar.gz"},
		{"windows", "amd64", "go1.22.5.windows-amd64.zip"},
	}

	for _, tc := range tests {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			f, err := FindFile(&releases[0], tc.goos, tc.goarch)
			if err != nil {
				t.Fatalf("FindFile: %v", err)
			}
			if f.Filename != tc.wantFile {
				t.Errorf("expected %s, got %s", tc.wantFile, f.Filename)
			}
		})
	}
}

func TestFindFile_NotFound(t *testing.T) {
	releases := mustParseReleases(t)

	_, err := FindFile(&releases[0], "plan9", "mips")
	if err == nil {
		t.Fatal("expected error for missing os/arch")
	}
}

func TestVerifySHA256_OK(t *testing.T) {
	content := []byte("hello world")
	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	path := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := VerifySHA256(path, expected); err != nil {
		t.Fatalf("expected OK, got: %v", err)
	}
}

func TestVerifySHA256_Mismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	err := VerifySHA256(path, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestVerifySHA256_MissingFile(t *testing.T) {
	err := VerifySHA256("/nonexistent/file", "abc")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestGoReleaseJSON_RoundTrip(t *testing.T) {
	releases := mustParseReleases(t)

	data, err := json.Marshal(releases)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var parsed []GoRelease
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(parsed) != len(releases) {
		t.Fatalf("expected %d releases after round-trip, got %d", len(releases), len(parsed))
	}
	if parsed[0].Files[0].Sha256 != releases[0].Files[0].Sha256 {
		t.Error("sha256 mismatch after round-trip")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func setHTTPTestHooks(t *testing.T, client *http.Client) {
	t.Helper()

	originalClient := httpClient
	originalSleep := retrySleep

	httpClient = client
	retrySleep = func(time.Duration) {}

	t.Cleanup(func() {
		httpClient = originalClient
		retrySleep = originalSleep
	})
}

func TestFetchReleases_Non2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	setHTTPTestHooks(t, server.Client())

	_, err := FetchReleases(server.URL)
	if err == nil {
		t.Fatal("expected non-2xx status error")
	}
	if !strings.Contains(err.Error(), "unexpected status 404") {
		t.Fatalf("expected 404 status in error, got: %v", err)
	}
}

func TestDownloadFile_Non2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	setHTTPTestHooks(t, server.Client())

	out := filepath.Join(t.TempDir(), "go.tar.gz")
	err := DownloadFile(out, server.URL)
	if err == nil {
		t.Fatal("expected non-2xx status error")
	}
	if !strings.Contains(err.Error(), "unexpected status 403") {
		t.Fatalf("expected 403 status in error, got: %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected output file to not exist, stat error: %v", statErr)
	}
}

func TestFetchReleases_RetryThenSuccess(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests < 3 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleJSON))
	}))
	defer server.Close()

	setHTTPTestHooks(t, server.Client())

	releases, err := FetchReleases(server.URL)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if len(releases) == 0 {
		t.Fatal("expected parsed releases")
	}
	if requests != 3 {
		t.Fatalf("expected 3 attempts, got %d", requests)
	}
}

func TestFetchReleases_RetryExhausted(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	setHTTPTestHooks(t, server.Client())

	_, err := FetchReleases(server.URL)
	if err == nil {
		t.Fatal("expected retry exhaustion error")
	}
	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Fatalf("expected retry exhaustion in error, got: %v", err)
	}
	if requests != 3 {
		t.Fatalf("expected 3 attempts, got %d", requests)
	}
}

func TestFetchReleases_TransportError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp: i/o timeout")
		}),
	}
	setHTTPTestHooks(t, client)

	_, err := FetchReleases("https://example.invalid/releases")
	if err == nil {
		t.Fatal("expected transport error")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("expected request failure in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Fatalf("expected retry exhaustion in error, got: %v", err)
	}
}
