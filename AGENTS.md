# Repository Guidelines

`getlatestgo` (module `github.com/dmlyons/getlatestgo`, Go 1.25.0) looks up the latest Go release
for the host OS/arch from the `golang.org/dl` JSON feed, optionally downloads it with SHA256
verification, and installs it to `/usr/local/go`. It ships as a CLI (root module) and a Fyne
desktop GUI (separate module in `gui/`).

Design details live next to the code. Read the relevant doc before changing a package, and update
it when behavior changes. This file covers only repo-wide rules.

| Path | Docs | Role |
|---|---|---|
| `main.go` | — | Entry point: `cli.Run(os.Args[1:], os.Stdout, os.Stderr)`, `log.Fatal` on error; no logic |
| `cli/` | `cli/doc.go` | Flags, orchestration, dry-run vs `-execute`/`-install`, terminal progress bar |
| `releases/` | `releases/doc.go` | Feed schema, release/file lookup, download URL, local cache path |
| `download/` | `download/doc.go` | Retry client (statuses, backoff, defaults), streaming download, SHA256, cache reuse |
| `install/` | `install/doc.go` | `VERSION` parsing; install via `sudo -n` (CLI) or `pkexec`/`osascript` (GUI) |
| `gui/` | `gui/README.md` | Fyne app, `fyne.Do` threading, `Dockerfile`, CI/release GUI jobs |
| `README.md` | — | User docs, GUI build prerequisites, release pipeline (`## Releasing`) |

## Architecture

```
main.go → cli.Run() ──┐
                      ├→ releases.Fetch/FindRelease/FindFile → download.EnsureFile → install.InstallGo (CLI)
gui/main.go (Fyne) ───┘                    ↑                           ↑            install.InstallGoGraphical (GUI)
                                           └────── both depend on ─────┘
                                                download.RetryClient
```

- Dependencies flow one way with no cycles. `download` and `install` import no other internal
  package; `releases` imports `download` only for `*RetryClient`; `cli` and `gui` sit on top.
- No goroutines or channels outside `gui/`; core packages and the CLI are synchronous.
- `gui/` is a nested module (`replace … => ../`), so root `./...` never builds it and the CLI stays
  CGO-free.
- CLI and GUI share one download cache (`GoFile.LocalPath` + `download.EnsureFile`).
- Only third-party dependencies: `schollz/progressbar/v3` (root) and `fyne.io/fyne/v2` (`gui/`).

## Commands

Plain `go` tooling; no Makefile, no linter config. CI (`.github/workflows/go.yml`) runs the root
module on Go `stable` and `oldstable`, and the GUI through `gui/Dockerfile`.

```sh
go build -v ./... && go test -v ./... && go vet ./...   # root module, as CI
go run . -list                       # also: -target go1.22.5 (dry run), -execute, -install, -version
docker build -f gui/Dockerfile -o gui .   # GUI → gui/getlatestgo-gui; run from repo root, no host toolchain
```

Native GUI builds need cgo and the Fyne headers; see `README.md` (`## GUI`).

## Releasing

Push an annotated `v*` tag on `main` (semver: minor bump for features, patch for fixes). The release
workflow builds Linux GUI archives, then GoReleaser builds the CLI archives and publishes both with
one `checksums.txt`; details in `README.md` (`## Releasing`). The changelog skips `docs:`/`test:`
commits, so use those prefixes for docs-only or test-only commits.

## Git Workflow

Work on a feature branch; `main` changes only through pull requests, merged with a merge commit.
Never commit directly to `main`.

```sh
git checkout -b <branch-name> main
git push -u origin <branch-name>
gh pr create --fill
```

## Code Conventions

- **Errors**: wrap at every layer boundary with `fmt.Errorf("<context>: %w", err)` to build a
  breadcrumb chain. No panics; only `main` calls `log.Fatal`.
- **Testability by injection**: `cli.Run` takes `stdout`/`stderr` writers instead of globals.
  `download.RetryClient` exposes exported fields (`HTTPClient`, `MaxAttempts`, `BackoffBase`,
  `RetrySleep`) rather than constructor options; nil or zero fields fall back to defaults.
- **Naming**: Go-idiomatic exported PascalCase, unexported camelCase helpers. Package names are
  short single lowercase words matching the directory. Import `install` as `installpkg`.
- **Package docs**: design notes go in the package comment in `doc.go` (GUI: `gui/README.md`), not here.

## Testing

- Stdlib `testing` only: no testify, no mocking libraries, no coverage threshold.
- HTTP: `httptest.NewServer` with `http.HandlerFunc` closures (`defer server.Close()`). For
  transport-level errors such as dial timeouts, use the inline `roundTripFunc` adapter in
  `download/download_test.go`.
- Helpers: `mustParseReleases(t)` (`releases_test.go`) and `testClient(httpClient)`
  (`download_test.go`). Both set a no-op `RetrySleep` so tests are fast and deterministic.
- Names: `Test<FunctionName><Scenario>` (e.g. `TestFindReleaseNotFound`). Use table-driven `t.Run`
  where inputs vary over a matrix (`TestFindFile`); otherwise write one test function per scenario.
- Assertions: `t.Fatalf`/`t.Errorf` with `strings.Contains(err.Error(), "...")`. No sentinel errors.
- Filesystem tests use `t.TempDir()`.
