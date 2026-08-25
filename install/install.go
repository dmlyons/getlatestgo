package install

import (
	"fmt"
	"os/exec"
)

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
