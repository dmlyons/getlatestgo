# Repository Guidelines

## Project Overview

`getlatestgo` is a small Go tool (module `github.com/dmlyons/getlatestgo`) that looks up the latest
Go release for the host OS/arch from the official `golang.org/dl` JSON feed, optionally downloads
it with SHA256 verification, and can install it to `/usr/local/go`. It ships as a CLI (root module)
and a Fyne desktop GUI (separate module in `gui/`). Network requests use explicit timeouts and
bounded retries for transient failures.

## Architecture & Data Flow

Layered, unidirectional dependency flow, no circular imports. The core packages (`releases`,
`download`, `install`) and the CLI are synchronous/sequential with no goroutines/channels:

```
main.go → cli.Run() ──┐
                      ├→ releases.Fetch/FindRelease/FindFile → download.EnsureFile → install.InstallGo (CLI)
gui/main.go (Fyne) ───┘                    ↑                           ↑            install.InstallGoGraphical (GUI)
                                           └────── both depend on ─────┘
                                                download.RetryClient
```

- `main.go` — thin CLI entry point: `main()` calls `cli.Run(os.Args[1:], os.Stdout, os.Stderr)`, `log.Fatal`s on error. No logic here.
- `gui/` — separate module `github.com/dmlyons/getlatestgo/gui` (`replace github.com/dmlyons/getlatestgo => ../`), single `main.go` building a Fyne window over the same `releases`/`download`/`install` calls. Kept out of the root module because Fyne needs CGO plus OpenGL/X11/Wayland headers; root `./...` never includes it (nested module), so the CLI stays CGO-free and goreleaser is unaffected. It is the only place with goroutines: blocking calls run off the UI goroutine and every widget update goes through `fyne.Do`.
- `cli.Run(args, stdout, stderr) error` — parses flags, builds a `download.NewRetryClient()`, fetches the release list via `releases.Fetch`, resolves the target release/file via `releases.FindRelease`/`releases.FindFile`, and (if `-execute`) calls `download.EnsureFile` (reuses a SHA256-verified local file if one already exists at the target path, otherwise downloads + verifies) + optionally installs. Errors from lower layers bubble up wrapped with `fmt.Errorf("<context>: %w", err)` at every boundary; `Run` never calls `log.Fatal` itself (that's `main`'s job) — this keeps `Run` testable via injected `io.Writer`s.
- `releases` package — domain layer: parses the `golang.org/dl` JSON schema (`GoRelease`, `GoFile`), picks the right release/file. Depends on `download` only for its `*RetryClient` type (to fetch the JSON feed).
- `download` package — transport layer: `RetryClient` (injectable `HTTPClient`, `MaxAttempts`, `BackoffBase`, `RetrySleep`), `GetWithRetry`, `DownloadFile` (streams to disk, reporting through an optional `Progress func(downloaded, total int64)` callback — the CLI renders it with `schollz/progressbar/v3`, the GUI with a Fyne progress bar), `VerifySHA256`, `EnsureFile` (checks for an existing local file matching the expected SHA256 before falling back to `DownloadFile`+verify, removing the file on post-download verification failure). Zero dependency on other internal packages and no UI code — leaf of the domain graph.
- `install` package — leaf package. `GoRoot = "/usr/local/go"`; `Installed(goroot) (*InstalledGo, error)` parses `<goroot>/VERSION` (version line + optional `time <RFC3339>` line; nil when absent) — the GUI shows it and compares against the newest stable via `go/version.Compare`. Shells out via `os/exec`: `InstallGo` (CLI): `sudo -n rm -rf /usr/local/go` then `sudo -n tar -C /usr/local -xzf <tarball>`. `InstallGoGraphical` (GUI): one privileged `rm && tar` via `pkexec /bin/sh -c` on Linux or `osascript ... with administrator privileges` on macOS (one auth prompt, no terminal); `GraphicalSupported()` is false elsewhere (Windows). No internal deps.

Dry-run behavior: without `-execute`, `cli.Run` only resolves and prints the download URL, it never
downloads. `-install` implies `-execute`.

Local cache reuse: `GoFile.LocalPath()` (`filepath.Join(os.TempDir(), f.Filename)`) encodes
version/os/arch in its name, so `download.EnsureFile` checks that path first — if it already exists
and its SHA256 matches the feed's `f.Sha256`, it's reused and `cli.Run` logs `"using cached local
download (SHA256 OK)"` instead of re-downloading. A mismatch (or no file) falls back to
`DownloadFile` + verify, with the file removed on post-download verification failure. CLI and GUI
share this path, so a file downloaded by one is reused by the other.

## Key Directories

| Path | Purpose |
|---|---|
| `cli/` | Flag parsing (`flag.NewFlagSet`), command orchestration, terminal progress bar (`run.go`) |
| `gui/` | Fyne desktop app, separate Go module (`main.go`, `go.mod`, `icon.png` — the Go gopher, CC BY 4.0, embedded and set via `App.SetIcon`) |
| `releases/` | Go release metadata parsing, lookup (`FindRelease`/`FindFile`/`StableVersions`), download URL/local path, and fetch logic |
| `download/` | HTTP retry client (`RetryClient`), file download with progress callback, SHA256 verification |
| `install/` | Installs the downloaded tarball via `sudo` (CLI) or `pkexec`/`osascript` (GUI) |

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

GUI (from `gui/`; needs a C compiler and on Debian/Ubuntu `gcc libgl1-mesa-dev xorg-dev
libwayland-dev libxkbcommon-dev wayland-protocols`, the same packages the CI `gui` job installs):

```sh
cd gui && go build -o getlatestgo-gui . && go vet ./...
go run .                         # launch the GUI
```

Releases are cut by pushing a `v*` tag (or manual `workflow_dispatch` with a `tag` input), which runs
GoReleaser v2 (`.goreleaser.yml`): `go mod tidy` → cross-compile CGO-disabled binaries for
linux/windows/darwin × amd64/arm64 → archive (`tar.gz`, `zip` on Windows) as
`{ProjectName}_{Version}_{Os}_{Arch}` → `checksums.txt` → GitHub release. This covers the CLI only;
the GUI is built and vetted in CI but not yet released. Changelog excludes commits
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
  import-aliased as `installpkg` in `cli/run.go` and `gui/main.go` to avoid colliding with local
  `install` variables.
- **No concurrency in core/CLI**: single retry loop with real `time.Sleep`-based backoff; no
  goroutines/channels outside `gui/`.

## Important Files

- `main.go` — entry point.
- `cli/run.go` — flag definitions (`-execute`, `-install`, `-target`, `-list`, `-version`) and the
  full orchestration flow.
- `releases/releases.go` — `DefaultURL = "https://golang.org/dl/?mode=json"`, `DownloadBase =
  "https://dl.google.com/go/"`, `Parse`, `Fetch`, `FindRelease`, `FindFile`, `StableVersions`,
  `GoFile.DownloadURL`, `GoFile.LocalPath`.
- `download/download.go` — `RetryClient`, `NewRetryClient`, `GetWithRetry`, `Progress`,
  `DownloadFile`, `VerifySHA256`, `EnsureFile` (cache-aware wrapper combining the two).
- `install/install.go` — `GoRoot`, `Installed`, `InstallGo`, `InstallGoGraphical`, `GraphicalSupported`.
- `gui/main.go` — Fyne app (`app.NewWithID("io.github.dmlyons.getlatestgo")`).
- `go.mod` — module `github.com/dmlyons/getlatestgo`, Go 1.25.0; one direct dependency
  (`schollz/progressbar/v3`). `gui/go.mod` adds `fyne.io/fyne/v2`.
- `.goreleaser.yml`, `.github/workflows/go.yml`, `.github/workflows/release.yml` — CI/CD config.

## Runtime/Tooling Preferences

- Go 1.25.0 (see `go.mod`); CI matrix tests the root module against `stable` and `oldstable` Go
  versions, plus a separate `gui` job that builds and vets `gui/` on `stable`.
- Standard `go` toolchain / module system — no other package manager.
- Only direct third-party dependency of the root module: `github.com/schollz/progressbar/v3` (CLI
  progress bar during download). Everything else is stdlib. The GUI module adds `fyne.io/fyne/v2`.
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
