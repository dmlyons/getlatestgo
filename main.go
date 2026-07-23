package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"

	"github.com/schollz/progressbar/v3"
)

const (
	defaultURL    = "https://golang.org/dl/?mode=json"
	downloadBase  = "https://dl.google.com/go/"
)

// GoRelease represents a single Go release from the API.
type GoRelease struct {
	Version string   `json:"version"`
	Stable  bool     `json:"stable"`
	Files   []GoFile `json:"files"`
}

// GoFile represents a downloadable file for a release.
type GoFile struct {
	Filename string `json:"filename"`
	Os       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Sha256   string `json:"sha256"`
	Size     int    `json:"size"`
	Kind     string `json:"kind"`
}

// FindRelease returns the release matching the given version string (e.g. "go1.22.5").
// If version is empty, the first (latest) release is returned.
func FindRelease(releases []GoRelease, version string) (*GoRelease, error) {
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}
	if version == "" {
		return &releases[0], nil
	}
	for i := range releases {
		if releases[i].Version == version {
			return &releases[i], nil
		}
	}
	return nil, fmt.Errorf("version %q not found", version)
}

// FindFile returns the file matching the given OS and architecture from a release.
func FindFile(release *GoRelease, goos, goarch string) (*GoFile, error) {
	for i := range release.Files {
		if release.Files[i].Os == goos && release.Files[i].Arch == goarch {
			return &release.Files[i], nil
		}
	}
	return nil, fmt.Errorf("no file found for %s/%s in %s", goos, goarch, release.Version)
}

// FetchReleases retrieves the release list from the given URL.
func FetchReleases(url string) ([]GoRelease, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var releases []GoRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("parsing releases: %w", err)
	}
	return releases, nil
}

// ParseReleases parses a JSON byte slice into a slice of GoRelease.
func ParseReleases(data []byte) ([]GoRelease, error) {
	var releases []GoRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, fmt.Errorf("parsing releases: %w", err)
	}
	return releases, nil
}

// VerifySHA256 checks that the file at path matches the expected SHA256 hex digest.
func VerifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file for verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hashing file: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("sha256 mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

// DownloadFile downloads a URL to a local file path, showing a progress bar.
func DownloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	bar := progressbar.DefaultBytes(resp.ContentLength, "downloading")
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	return err
}

// InstallGo removes the existing Go installation and extracts the tarball.
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

func main() {
	lg := log.New(os.Stderr, "glg: ", 0)

	execute := flag.Bool("execute", false, "download the file (implies verification)")
	install := flag.Bool("install", false, "download, verify, and install (implies -execute)")
	showVersion := flag.Bool("version", false, "show version and exit")
	targetVersion := flag.String("target", "", "download a specific Go version (e.g. go1.22.5)")
	list := flag.Bool("list", false, "list all available stable releases")
	flag.Parse()

	if *showVersion {
		i, _ := debug.ReadBuildInfo()
		lg.Printf("Version\n%s", i)
		os.Exit(0)
	}

	releases, err := FetchReleases(defaultURL)
	if err != nil {
		log.Fatal(err)
	}

	if *list {
		for _, r := range releases {
			if r.Stable {
				fmt.Println(r.Version)
			}
		}
		return
	}

	// --install implies --execute
	if *install {
		*execute = true
	}

	release, err := FindRelease(releases, *targetVersion)
	if err != nil {
		log.Fatal(err)
	}

	lg.Printf("%s arch=%s os=%s\n", release.Version, runtime.GOARCH, runtime.GOOS)

	f, err := FindFile(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		log.Fatal(err)
	}

	dlURL := downloadBase + f.Filename
	lg.Println("URL:", dlURL)

	if !*execute {
		return
	}

	localFile := filepath.Join(os.TempDir(), f.Filename)
	if err := DownloadFile(localFile, dlURL); err != nil {
		log.Fatalf("download failed: %v", err)
	}

	lg.Println("verifying SHA256...")
	if err := VerifySHA256(localFile, f.Sha256); err != nil {
		log.Fatalf("verification failed: %v", err)
	}
	lg.Println("SHA256 OK")

	if *install {
		lg.Println("installing...")
		if err := InstallGo(localFile); err != nil {
			log.Fatalf("install failed: %v", err)
		}
		lg.Println("installed to /usr/local/go")
	} else {
		fmt.Printf("sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf %s\n", localFile)
	}
}
