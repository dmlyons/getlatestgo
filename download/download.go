package download

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

const (
	requestTimeout     = 30 * time.Second
	defaultMaxAttempts = 3
	defaultBackoffBase = 300 * time.Millisecond
	errorBodyMaxLength = 512
)

type RetryClient struct {
	HTTPClient  *http.Client
	MaxAttempts int
	BackoffBase time.Duration
	RetrySleep  func(time.Duration)
}

func NewRetryClient() *RetryClient {
	return &RetryClient{
		HTTPClient:  &http.Client{Timeout: requestTimeout},
		MaxAttempts: defaultMaxAttempts,
		BackoffBase: defaultBackoffBase,
		RetrySleep:  time.Sleep,
	}
}

func (c *RetryClient) GetWithRetry(url, operation string) (*http.Response, error) {
	client, maxAttempts, backoffBase, sleepFn := c.normalized()
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: creating request: %w", operation, err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s: request failed: %w", operation, err)
		} else {
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				return resp, nil
			}

			statusErr := buildHTTPStatusError(operation, resp)
			if !isRetryableStatus(resp.StatusCode) {
				return nil, statusErr
			}
			lastErr = statusErr
		}

		if attempt < maxAttempts {
			sleepFn(backoffForAttempt(backoffBase, attempt))
		}
	}

	return nil, fmt.Errorf("%s: failed after %d attempts: %w", operation, maxAttempts, lastErr)
}

func (c *RetryClient) normalized() (*http.Client, int, time.Duration, func(time.Duration)) {
	if c == nil {
		defaults := NewRetryClient()
		return defaults.HTTPClient, defaults.MaxAttempts, defaults.BackoffBase, defaults.RetrySleep
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}

	maxAttempts := c.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}

	backoffBase := c.BackoffBase
	if backoffBase <= 0 {
		backoffBase = defaultBackoffBase
	}

	sleepFn := c.RetrySleep
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	return client, maxAttempts, backoffBase, sleepFn
}

func DownloadFile(client *RetryClient, localPath, url string) error {
	resp, err := client.GetWithRetry(url, "download file")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("download file: creating local file: %w", err)
	}
	defer out.Close()

	bar := progressbar.DefaultBytes(resp.ContentLength, "downloading")
	if _, err := io.Copy(io.MultiWriter(out, bar), resp.Body); err != nil {
		return fmt.Errorf("download file: writing data: %w", err)
	}

	return nil
}

func VerifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file for verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hashing file: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("sha256 mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

func buildHTTPStatusError(operation string, resp *http.Response) error {
	bodyPreview := readErrorBodyPreview(resp.Body)
	closeErr := resp.Body.Close()

	base := fmt.Sprintf("%s: unexpected status %d %s", operation, resp.StatusCode, http.StatusText(resp.StatusCode))
	if bodyPreview != "" {
		base = fmt.Sprintf("%s: %s", base, bodyPreview)
	}
	if closeErr != nil {
		base = fmt.Sprintf("%s (closing response body: %v)", base, closeErr)
	}

	return errors.New(base)
}

func readErrorBodyPreview(r io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(r, errorBodyMaxLength))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(body))
}

func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func backoffForAttempt(base time.Duration, attempt int) time.Duration {
	if attempt < 1 {
		return base
	}
	return base * time.Duration(1<<(attempt-1))
}
