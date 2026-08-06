package install

import (
	"fmt"
	"os/exec"
)

func InstallGo(tarball string) error {
	rmCmd := exec.Command("sudo", "rm", "-rf", "/usr/local/go")
	if out, err := rmCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("removing old go: %s: %w", string(out), err)
	}

	tarCmd := exec.Command("sudo", "tar", "-C", "/usr/local", "-xzf", tarball)
	if out, err := tarCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extracting tarball: %s: %w", string(out), err)
	}

	return nil
}
