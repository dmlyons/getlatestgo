package install

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// GoRoot is where InstallGo and InstallGoGraphical install Go.
const GoRoot = "/usr/local/go"

// InstalledGo describes a Go installation, as recorded in its VERSION file.
type InstalledGo struct {
	Version string    // e.g. "go1.22.5"
	Time    time.Time // release build time; zero if the VERSION file has none
}

// Installed reports the Go installation at goroot, or nil if goroot holds no
// Go installation.
func Installed(goroot string) (*InstalledGo, error) {
	data, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading installed go version: %w", err)
	}

	// Format: version on the first line, then optional "key value" lines such
	// as "time 2024-06-27T20:11:12Z" (Go 1.21+; older releases have only the
	// version, without a trailing newline).
	sc := bufio.NewScanner(bytes.NewReader(data))
	if !sc.Scan() || strings.TrimSpace(sc.Text()) == "" {
		return nil, fmt.Errorf("reading installed go version: empty VERSION file in %s", goroot)
	}
	inst := &InstalledGo{Version: strings.TrimSpace(sc.Text())}
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "time "); ok {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(v)); err == nil {
				inst.Time = t
			}
		}
	}
	return inst, nil
}

// InstallGo replaces /usr/local/go with the contents of tarball using
// non-interactive sudo, so credentials must already be cached (`sudo -v`).
func InstallGo(tarball string) error {
	rmCmd := exec.Command("sudo", "-n", "rm", "-rf", "/usr/local/go")
	if out, err := rmCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("removing old go (run 'sudo -v' to pre-authenticate): %s: %w", string(out), err)
	}

	tarCmd := exec.Command("sudo", "-n", "tar", "-C", "/usr/local", "-xzf", tarball)
	if out, err := tarCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extracting tarball (run 'sudo -v' to pre-authenticate): %s: %w", string(out), err)
	}

	return nil
}

// GraphicalSupported reports whether InstallGoGraphical works on this OS.
func GraphicalSupported() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "darwin"
}

// InstallGoGraphical replaces /usr/local/go with the contents of tarball,
// obtaining root through the desktop's authentication dialog (pkexec on Linux,
// the administrator prompt on macOS) so it works without a terminal. Removal and
// extraction run as one privileged command, so the user is prompted once.
func InstallGoGraphical(tarball string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("pkexec", "/bin/sh", "-c",
			`rm -rf /usr/local/go && tar -C /usr/local -xzf "$1"`, "sh", tarball)
	case "darwin":
		// The tarball path is passed via argv so it never needs AppleScript escaping.
		cmd = exec.Command("osascript",
			"-e", "on run argv",
			"-e", `do shell script ("rm -rf /usr/local/go && tar -C /usr/local -xzf " & quoted form of (item 1 of argv)) with administrator privileges`,
			"-e", "end run",
			tarball)
	default:
		return fmt.Errorf("installing is not supported on %s", runtime.GOOS)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("installing go: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
