# getlatestgo-gui

Fyne desktop front end for getlatestgo. User-facing features and build prerequisites are in the
[root README](../README.md#gui); this file covers the module's internals and its build pipeline.

## Module

- Separate module `github.com/dmlyons/getlatestgo/gui` with `replace github.com/dmlyons/getlatestgo => ../`.
  Fyne needs cgo plus OpenGL/X11/Wayland headers; as a nested module the root `go build ./...` never
  sees it, so the CLI stays CGO-free and GoReleaser is unaffected.
- Single `main.go` over the same `releases` / `download` / `install` calls as the CLI. Install
  uses `install.InstallGoGraphical` (one desktop auth prompt) and is disabled when
  `install.GraphicalSupported()` is false.
- App ID `io.github.dmlyons.getlatestgo` (`app.NewWithID`).
- `icon.png` is the Go gopher (Renée French, CC BY 4.0), embedded with `//go:embed` and set via
  `App.SetIcon`.

## Concurrency

This is the only place in the repo with goroutines. All `gui` struct fields belong to the Fyne UI
goroutine. Network, disk and install work runs in background goroutines, and every widget update
goes back through `fyne.Do`. Never touch widgets directly from a background goroutine.

## Docker build (`Dockerfile`)

```sh
docker build -f gui/Dockerfile -o gui .   # from the repo root → gui/getlatestgo-gui
```

- The build context must be the repo root because of the `../` replace. `.dockerignore` keeps
  `dist/`, `dist-gui/` and built binaries out of it.
- Built on `golang:1.25-bookworm` with the Fyne headers; bump the tag together with `go.mod`.
  Bookworm keeps the binary's glibc requirement at 2.34.
- Runs `go vet` and `go build -trimpath -ldflags="-s -w"`, and the final `scratch` stage holds only
  the binary, so `-o` exports just that file.
- BuildKit cache mounts hold the module and build caches: the first build takes minutes because cgo
  compiles the GL/GLFW bindings, and rebuilds take seconds.

## CI and releases

- `.github/workflows/go.yml`, job `gui`: builds and vets through the Dockerfile with Buildx and the
  GitHub Actions layer cache (scope `gui-linux-amd64`, shared with the release job). Cache mounts are
  not persisted on hosted runners, so the cgo compile runs on every CI build.
- `.github/workflows/release.yml`, job `gui`: runs per arch on native runners (`ubuntu-latest` for amd64,
  `ubuntu-24.04-arm` for arm64), because the cgo build is too slow under QEMU. It packages
  `getlatestgo-gui_{Version}_linux_{Arch}.tar.gz` (binary, `LICENSE`, `README.md`) and uploads it as
  an artifact, which GoReleaser attaches to the release (see [Releasing](../README.md#releasing)).
  Releases contain the GUI for Linux only.
