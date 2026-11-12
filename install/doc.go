// Package install detects and replaces the Go installation in [GoRoot]
// (/usr/local/go). It imports no other getlatestgo package and shells out via
// os/exec for privileged work.
//
// [Installed] reads <goroot>/VERSION: the version on the first line, then
// optional "key value" lines, of which "time <RFC3339>" (Go 1.21+) fills
// [InstalledGo.Time]. It returns nil, nil when no VERSION file exists, and an
// error when the file is empty. The GUI compares the result against the newest
// stable release with go/version.Compare.
//
// There are two installers, both removing [GoRoot] and extracting the tarball
// into /usr/local:
//
//   - [InstallGo] (CLI) runs "sudo -n rm -rf" and then "sudo -n tar -xzf". It
//     never prompts, so credentials must already be cached ("sudo -v"); its
//     errors say so.
//   - [InstallGoGraphical] (GUI) runs both steps as one privileged shell
//     command, so the user is asked once and no terminal is needed: through
//     pkexec /bin/sh -c on Linux and through osascript "with administrator
//     privileges" on macOS. The tarball path is passed as an argument, never
//     interpolated into the script. [GraphicalSupported] is false on other
//     systems (Windows), where the GUI disables Install.
//
// Importers alias this package as installpkg so local install variables do
// not shadow it.
package install
