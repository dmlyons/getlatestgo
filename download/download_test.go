package download

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testClient(httpClient *http.Client) *RetryClient {
	return &RetryClient{
		HTTPClient:  httpClient,
		MaxAttempts: defaultMaxAttempts,
		BackoffBase: defaultBackoffBase,
		RetrySleep:  func(time.Duration) {},
	}
}

func TestVerifySHA256OK(t *testing.T) {
	content := []byte("hello world")
	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	path := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := VerifySHA256(path, expected); err != nil {
		t.Fatalf("expected OK, got: %v", err)
	}
}

func TestVerifySHA256Mismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := VerifySHA256(path, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestVerifySHA256MissingFile(t *testing.T) {
	if err := VerifySHA256("/nonexistent/file", "abc"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestGetWithRetryNon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	client := testClient(server.Client())
	_, err := client.GetWithRetry(server.URL, "fetch releases")
	if err == nil {
		t.Fatal("expected non-2xx status error")
	}
	if !strings.Contains(err.Error(), "unexpected status 404") {
		t.Fatalf("expected 404 status in error, got: %v", err)
	}
}

func TestGetWithRetryRetryThenSuccess(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests < 3 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	client := testClient(server.Client())
	resp, err := client.GetWithRetry(server.URL, "fetch releases")
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	resp.Body.Close()

	if requests != 3 {
		t.Fatalf("expected 3 attempts, got %d", requests)
	}
}

func TestGetWithRetryRetryExhausted(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := testClient(server.Client())
	_, err := client.GetWithRetry(server.URL, "fetch releases")
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

func TestGetWithRetryTransportError(t *testing.T) {
	client := testClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp: i/o timeout")
		}),
	})

	_, err := client.GetWithRetry("https://example.invalid/releases", "fetch releases")
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

func TestDownloadFileNon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	client := testClient(server.Client())
	out := filepath.Join(t.TempDir(), "go.tar.gz")

	err := DownloadFile(client, out, server.URL)
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

func TestDownloadFileSuccess(t *testing.T) {
	const body = "the quick brown fox jumps over the lazy dog"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := testClient(server.Client())
	out := filepath.Join(t.TempDir(), "go.tar.gz")

	if err := DownloadFile(client, out, server.URL); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(got) != body {
		t.Fatalf("expected downloaded content %q, got %q", body, got)
	}
}

type failAfterReader struct {
	remaining []byte
	failErr   error
}

func (r *failAfterReader) Read(p []byte) (int, error) {
	if len(r.remaining) == 0 {
		return 0, r.failErr
	}
	n := copy(p, r.remaining)
	r.remaining = r.remaining[n:]
	return n, nil
}

func (r *failAfterReader) Close() error { return nil }

func TestDownloadFileCopyErrorCleansUpPartialFile(t *testing.T) {
	wantErr := errors.New("connection reset")
	client := testClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &failAfterReader{remaining: []byte("partial"), failErr: wantErr},
				Header:     make(http.Header),
			}, nil
		}),
	})
	out := filepath.Join(t.TempDir(), "go.tar.gz")

	err := DownloadFile(client, out, "http://example.invalid/go.tar.gz")
	if err == nil {
		t.Fatal("expected copy error")
	}
	if !strings.Contains(err.Error(), "writing data") {
		t.Fatalf("expected 'writing data' in error, got: %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("expected partial output file to be removed, stat error: %v", statErr)
	}
}
