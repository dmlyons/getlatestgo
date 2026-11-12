// Package download is the transport layer of getlatestgo: an HTTP GET client
// with bounded retries, streaming file downloads with progress reporting, and
// SHA256 verification. It imports no other getlatestgo package and contains no
// UI code; the CLI and the GUI render [Progress] themselves.
//
// # Retries
//
// [RetryClient] fields are exported so callers and tests can override them
// directly instead of going through constructor options:
//
//   - HTTPClient: the underlying client ([NewRetryClient] sets a 30s timeout).
//   - MaxAttempts: total attempts, including the first (default 3).
//   - BackoffBase: delay before the second attempt (default 300ms).
//   - RetrySleep: the sleep function (default [time.Sleep]); tests pass a
//     no-op so retries run instantly.
//
// A nil *RetryClient, and nil or non-positive fields, fall back to these
// defaults. [RetryClient.GetWithRetry] retries transport errors and the
// statuses 408, 425, 429, 500, 502, 503 and 504; any other non-2xx status
// fails immediately. The delay before attempt n+1 is BackoffBase * 2^(n-1),
// uncapped (MaxAttempts bounds it). Status errors include up to 512 bytes of
// the response body. Every error is prefixed with the caller's operation name,
// and exhausted retries report "failed after N attempts" wrapping the last error.
//
// # Downloads and the local cache
//
// [DownloadFile] streams to disk and removes the partial file if writing or
// closing fails. [EnsureFile] makes download-and-verify idempotent: if a
// regular file already exists at the target path and matches the expected
// SHA256, it is reused (cached=true) and progress is never called. Otherwise
// the file is downloaded and verified; a file that fails verification after a
// fresh download is removed. Callers pass releases.GoFile.LocalPath as the
// target, whose name encodes version, OS and arch, so the CLI and the GUI
// share one cache.
package download
