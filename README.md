# getlatestgo

A CLI tool that looks up the latest Go release for your architecture, optionally downloads it with SHA256 verification, and can install it automatically. Network requests use explicit timeouts and bounded retries for transient failures.

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
