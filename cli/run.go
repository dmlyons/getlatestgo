package cli

import (
	"flag"
	"fmt"
	"io"
	"log"
	"runtime"
	"runtime/debug"

	"github.com/schollz/progressbar/v3"

	"github.com/dmlyons/getlatestgo/download"
	installpkg "github.com/dmlyons/getlatestgo/install"
	"github.com/dmlyons/getlatestgo/releases"
)

// terminalProgress renders download progress as a terminal progress bar.
func terminalProgress() download.Progress {
	var bar *progressbar.ProgressBar
	return func(downloaded, total int64) {
		if bar == nil {
			bar = progressbar.DefaultBytes(total, "downloading")
		}
		_ = bar.Set64(downloaded)
	}
}

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
		for _, v := range releases.StableVersions(releaseList) {
			fmt.Fprintln(stdout, v)
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

	dlURL := f.DownloadURL()
	lg.Println("URL:", dlURL)

	if !*execute {
		return nil
	}

	localFile := f.LocalPath()
	cached, err := download.EnsureFile(client, localFile, dlURL, f.Sha256, terminalProgress())
	if err != nil {
		return err
	}
	if cached {
		lg.Println("using cached local download (SHA256 OK)")
	} else {
		lg.Println("SHA256 OK")
	}

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
