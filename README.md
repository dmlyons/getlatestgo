# getlatestgo

A tool that looks up the latest Go release for your architecture, optionally downloads it with SHA256 verification, and can install it automatically. It comes as a CLI (`getlatestgo`) and a desktop GUI (`getlatestgo-gui`, built with [Fyne](https://fyne.io)). Network requests use explicit timeouts and bounded retries for transient failures.

## Usage

```
getlatestgo                    # show the latest release URL
getlatestgo -list              # list all available stable versions
getlatestgo -execute           # download and verify the latest release
getlatestgo -target go1.22.5   # target a specific version
getlatestgo -install           # download, verify, and install (implies -execute)
getlatestgo -version           # show build version info
```

## Flags

| Flag | Description |
|------|-------------|
| `-execute` | Download the file and verify its SHA256 checksum (with retry + timeout handling) |
| `-install` | Download, verify, and install to `/usr/local/go` (implies `-execute`) |
| `-target`  | Download a specific Go version (e.g. `go1.22.5`) |
| `-list`    | List all available stable releases |
| `-version` | Show version and exit |

## GUI

`getlatestgo-gui` shows the Go currently installed in `/usr/local/go` (version, release date, and whether it is up to date with the newest stable release), lists the stable releases (newest selected by default), shows the file, URL and SHA256 for your OS/arch, and offers **Download** (download + verify) and **Download & Install** (replaces `/usr/local/go`). Both reuse a previously downloaded file in the temp directory when its SHA256 still matches, the same cache the CLI uses.

Installing asks for an administrator password through the desktop's own dialog (`pkexec` on Linux, the administrator prompt on macOS), so no terminal or `sudo -v` is needed. Install is unavailable on Windows; Download still works there.

The GUI is a separate Go module in `gui/` so the CLI stays CGO-free. Building it needs a C compiler and, on Linux, the OpenGL/X11/Wayland headers:

```sh
# Debian/Ubuntu/Mint
sudo apt-get install gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev wayland-protocols

cd gui
go build -o getlatestgo-gui .
```

macOS needs the Xcode command line tools; Windows needs a GCC toolchain such as MSYS2 MinGW-w64. See the [Fyne prerequisites](https://docs.fyne.io/started/).

The app icon is the Go gopher, designed by [Renée French](https://reneefrench.blogspot.com/) and licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) (see [the Go blog](https://go.dev/blog/gopher)); `gui/icon.png` is a 512×512 downscale of `gophercolor.png` from [golang/website](https://github.com/golang/website/tree/master/_content/doc/gopher).

## Code layout

- `cli`: flag parsing and command orchestration (terminal progress bar)
- `gui`: Fyne desktop app (separate module, uses the packages below)
- `releases`: Go release metadata parsing, lookup, and fetch logic
- `download`: HTTP retry client, file download with progress callback, and SHA256 verification
- `install`: installation via `sudo` (CLI) or the desktop authentication dialog (GUI)
