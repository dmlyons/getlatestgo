# Repository Guidelines

## Project Overview

`getlatestgo` is a small Go CLI (module `github.com/dmlyons/getlatestgo`) that looks up the latest
Go release for the host OS/arch from the official `golang.org/dl` JSON feed, optionally downloads
it with SHA256 verification, and can install it to `/usr/local/go` via `sudo tar`. Network requests
use explicit timeouts and bounded retries for transient failures.

## Architecture & Data Flow

Layered, unidirectional dependency flow, no circular imports, no goroutines/channels — everything is
synchronous/sequential:

```
main.go → cli.Run() → releases.Fetch/FindRelease/FindFile → download.DownloadFile/VerifySHA256 → install.InstallGo
                                    ↑                                    ↑
                                    └──────── both depend on ───────────┘
                                          download.RetryClient
```

- `main.go` — thin entry point: `main()` calls `cli.Run(os.Args[1:], os.Stdout, os.Stderr)`, `log.Fatal`s on error. No logic here.
- `cli.Run(args, stdout, stderr) error` — parses flags, builds a `download.NewRetryClient()`, fetches the release list via `releases.Fetch`, resolves the target release/file via `releases.FindRelease`/`releases.FindFile`, and (if `-execute`) downloads + verifies + optionally installs. Errors from lower layers bubble up wrapped with `fmt.Errorf("<context>: %w", err)` at every boundary; `Run` never calls `log.Fatal` itself (that's `main`'s job) — this keeps `Run` testable via injected `io.Writer`s.
- `releases` package — domain layer: parses the `golang.org/dl` JSON schema (`GoRelease`, `GoFile`), picks the right release/file. Depends on `download` only for its `*RetryClient` type (to fetch the JSON feed).
- `download` package — transport layer: `RetryClient` (injectable `HTTPClient`, `MaxAttempts`, `BackoffBase`, `RetrySleep`), `GetWithRetry`, `DownloadFile` (streams with a progress bar via `schollz/progressbar/v3`), `VerifySHA256`. Zero dependency on other internal packages — leaf of the domain graph.
- `install` package — leaf package, shells out via `os/exec`: `sudo -n rm -rf /usr/local/go` then `sudo -n tar -C /usr/local -xzf <tarball>`. No internal deps.

Dry-run behavior: without `-execute`, `cli.Run` only resolves and prints the download URL, it never
downloads. `-install` implies `-execute`.

## Key Directories

| Path | Purpose |
|---|---|
| `cli/` | Flag parsing (`flag.NewFlagSet`) and command orchestration (`run.go`) |
| `releases/` | Go release metadata parsing, lookup (`FindRelease`/`FindFile`), and fetch logic |
| `download/` | HTTP retry client (`RetryClient`), file download with progress bar, SHA256 verification |
| `install/` | Installs the downloaded tarball via `sudo rm`/`sudo tar` |

## Development Commands

No Makefile/shell scripts exist — plain `go` tooling only (mirrors `.github/workflows/go.yml`):

```sh
go build -v ./...     # build
go test -v ./...      # test
go vet ./...          # vet (no separate linter configured)
```

Run locally:

```sh
go run . -list                 # list all available stable versions
go run . -target go1.22.5      # dry-run: print the download URL for a specific version
go run . -execute               # download + verify the latest release
go run . -install                # download, verify, and install (implies -execute)
go run . -version                # print build version info
```

Releases are cut by pushing a `v*` tag (or manual `workflow_dispatch` with a `tag` input), which runs
GoReleaser v2 (`.goreleaser.yml`): `go mod tidy` → cross-compile CGO-disabled binaries for
linux/windows/darwin × amd64/arm64 → archive (`tar.gz`, `zip` on Windows) as
`{ProjectName}_{Version}_{Os}_{Arch}` → `checksums.txt` → GitHub release. Changelog excludes commits
prefixed `docs:` or `test:` — follow that convention in commit messages touching only docs/tests.

## Git Workflow

All work happens on feature branches; `main` only receives changes via pull request (see the
`dmlyons-*` branch/PR history for the existing convention). Never commit directly to `main`.

```sh
git checkout -b <branch-name> main
# ...commit work...
git push -u origin <branch-name>
gh pr create --fill
```

## Code Conventions & Common Patterns

- **Error wrapping**: every error is wrapped with `fmt.Errorf("<context>: %w", err)` at each layer
  boundary, building a breadcrumb chain (e.g. `"install failed: removing old go (run 'sudo -v' to
  pre-authenticate): <output>: <exec error>"`). No panics except the top-level `log.Fatal` in `main`.
- **Dependency injection for testability**: `cli.Run` takes explicit `stdout, stderr io.Writer`
  instead of using globals; `download.RetryClient` exposes exported, test-overridable fields
  (`HTTPClient`, `MaxAttempts`, `BackoffBase`, `RetrySleep`) instead of constructor options — tests
  set `RetrySleep: func(time.Duration){}` to skip real backoff delays.
- **Nil-safety**: `RetryClient.normalized()` is a private method handling nil receiver / nil-or-zero
  fields, falling back to package defaults (`defaultMaxAttempts`, `defaultBackoffBase`, `time.Sleep`).
- **Retry/backoff**: `isRetryableStatus` treats 408/425/429/500/502/503/504 as retryable;
  `backoffForAttempt` is capped exponential backoff (`base * 2^(attempt-1)`).
- **Naming**: exported Go-idiomatic PascalCase (`RetryClient`, `GoRelease`, `FindRelease`,
  `DownloadFile`, `VerifySHA256`); unexported camelCase helpers (`normalized`,
  `buildHTTPStatusError`, `isRetryableStatus`). Package names are short, lowercase, single words
  matching their directory (`cli`, `install`, `releases`, `download`). The `install` package is
  import-aliased as `installpkg` in `cli/run.go` to avoid colliding with the local `-install` bool
  flag variable.
- **No concurrency**: single retry loop with real `time.Sleep`-based backoff; no goroutines/channels
  anywhere.

## Important Files

- `main.go` — entry point.
- `cli/run.go` — flag definitions (`-execute`, `-install`, `-target`, `-list`, `-version`) and the
  full orchestration flow.
- `releases/releases.go` — `DefaultURL = "https://golang.org/dl/?mode=json"`, `Parse`, `Fetch`,
  `FindRelease`, `FindFile`.
- `download/download.go` — `RetryClient`, `NewRetryClient`, `GetWithRetry`, `DownloadFile`,
  `VerifySHA256`; `downloadBase = "https://dl.google.com/go/"` lives in `cli/run.go`.
- `install/install.go` — `InstallGo(tarball string) error`.
- `go.mod` — module `github.com/dmlyons/getlatestgo`, Go 1.25.0; one direct dependency
  (`schollz/progressbar/v3`).
- `.goreleaser.yml`, `.github/workflows/go.yml`, `.github/workflows/release.yml` — CI/CD config.

## Runtime/Tooling Preferences

- Go 1.25.0 (see `go.mod`); CI matrix tests against `stable` and `oldstable` Go versions.
- Standard `go` toolchain / module system — no other package manager.
- Only direct third-party dependency: `github.com/schollz/progressbar/v3` (CLI progress bar during
  download). Everything else is stdlib.
- No linter config (no `golangci-lint`, etc.) — CI relies on `go build`, `go test`, `go vet` only.

## Testing & QA

- Framework: stdlib `testing` only — no testify, no mocking libraries.
- HTTP stubbing: `net/http/httptest.NewServer` with `http.HandlerFunc` closures (paired with
  `defer server.Close()`) for realistic response simulation; a minimal inline `roundTripFunc`
  (`http.RoundTripper` function adapter) in `download/download_test.go` for pure transport-level
  errors (e.g. simulated dial timeouts).
- Shared fixture/client helpers: `mustParseReleases(t)` in `releases/releases_test.go` and
  `testClient(httpClient)` in `download/download_test.go`, both neutralizing `RetrySleep` so tests
  run fast and deterministically.
- Naming: `Test<FunctionName><Scenario>` (e.g. `TestFindReleaseNotFound`,
  `TestGetWithRetryRetryExhausted`). Table-driven subtests via `t.Run` appear in
  `releases_test.go`'s `TestFindFile` (iterating OS/arch combos); `download_test.go` uses one
  top-level `Test` function per scenario instead.
- Assertions: `t.Fatalf`/`t.Errorf` with `strings.Contains(err.Error(), "...")` checks — no sentinel
  errors (`errors.Is`) compared, no assertion library.
- Filesystem tests use `t.TempDir()` for isolation (SHA256 verification, download-failure cleanup).
- Run all tests: `go test -v ./...`. No coverage threshold is enforced in CI.
