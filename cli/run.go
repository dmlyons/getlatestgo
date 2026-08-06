package cli

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"

	"github.com/dmlyons/getlatestgo/download"
	installpkg "github.com/dmlyons/getlatestgo/install"
	"github.com/dmlyons/getlatestgo/releases"
)

const downloadBase = "https://dl.google.com/go/"

func Run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("getlatestgo", flag.ContinueOnError)
	flags.SetOutput(stderr)

	execute := flags.Bool("execute", false, "download the file (implies verification)")
	install := flags.Bool("install", false, "download, verify, and install (implies -execute)")
	showVersion := flags.Bool("version", false, "show version and exit")
	targetVersion := flags.String("target", "", "download a specific Go version (e.g. go1.22.5)")
	list := flags.Bool("list", false, "list all available stable releases")

	if err := flags.Parse(args); err != nil {
		return err
	}

	lg := log.New(stderr, "glg: ", 0)
	if *showVersion {
		info, _ := debug.ReadBuildInfo()
		lg.Printf("Version\n%s", info)
		return nil
	}

	client := download.NewRetryClient()
	releaseList, err := releases.Fetch(client, releases.DefaultURL)
	if err != nil {
		return err
	}

	if *list {
		for _, r := range releaseList {
			if r.Stable {
				fmt.Fprintln(stdout, r.Version)
			}
		}
		return nil
	}

	// --install implies --execute.
	if *install {
		*execute = true
	}

	release, err := releases.FindRelease(releaseList, *targetVersion)
	if err != nil {
		return err
	}

	lg.Printf("%s arch=%s os=%s\n", release.Version, runtime.GOARCH, runtime.GOOS)

	f, err := releases.FindFile(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	dlURL := downloadBase + f.Filename
	lg.Println("URL:", dlURL)

	if !*execute {
		return nil
	}

	localFile := filepath.Join(os.TempDir(), f.Filename)
	if err := download.DownloadFile(client, localFile, dlURL); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	lg.Println("verifying SHA256...")
	if err := download.VerifySHA256(localFile, f.Sha256); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}
	lg.Println("SHA256 OK")

	if *install {
		lg.Println("installing...")
		if err := installpkg.InstallGo(localFile); err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
		lg.Println("installed to /usr/local/go")
		return nil
	}

	fmt.Fprintf(stdout, "sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf %s\n", localFile)
	return nil
}
