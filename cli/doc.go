// Package cli implements the getlatestgo command line. main calls
// [Run](os.Args[1:], os.Stdout, os.Stderr) and turns a returned error into
// log.Fatal; Run itself never exits and writes to the given writers (except
// the download progress bar, which goes to os.Stderr), so it can be driven
// with in-memory buffers.
//
// Run parses flags (-execute, -install, -target, -list, -version), fetches the
// release feed with a download.NewRetryClient, resolves the release with
// releases.FindRelease (the newest stable release unless -target is set) and
// the file for runtime.GOOS/GOARCH with releases.FindFile, then:
//
//   - by default (dry run): logs the download URL and stops; nothing is
//     downloaded.
//   - with -execute: calls download.EnsureFile on GoFile.LocalPath, reusing a
//     verified cached file ("using cached local download (SHA256 OK)") or
//     downloading with a terminal progress bar (schollz/progressbar), then
//     prints the sudo rm/tar command that would install it.
//   - with -install (implies -execute): installs through install.InstallGo.
//
// Results meant for piping (the -list output and the suggested install
// command) go to stdout; log lines, prefixed "glg: ", go to stderr. Lower-layer
// errors are returned with context added via fmt.Errorf("...: %w").
package cli
