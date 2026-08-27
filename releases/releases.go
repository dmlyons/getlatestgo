package releases

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dmlyons/getlatestgo/download"
)

const DefaultURL = "https://golang.org/dl/?mode=json"

type GoRelease struct {
	Version string   `json:"version"`
	Stable  bool     `json:"stable"`
	Files   []GoFile `json:"files"`
}

type GoFile struct {
	Filename string `json:"filename"`
	Os       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Sha256   string `json:"sha256"`
	Size     int    `json:"size"`
	Kind     string `json:"kind"`
}

func Parse(data []byte) ([]GoRelease, error) {
	var releaseList []GoRelease
	if err := json.Unmarshal(data, &releaseList); err != nil {
		return nil, fmt.Errorf("parsing releases: %w", err)
	}
	return releaseList, nil
}

func Fetch(client *download.RetryClient, url string) ([]GoRelease, error) {
	resp, err := client.GetWithRetry(url, "fetch releases")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: reading response: %w", err)
	}

	releaseList, err := Parse(body)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}

	return releaseList, nil
}

func FindRelease(releaseList []GoRelease, version string) (*GoRelease, error) {
	if len(releaseList) == 0 {
		return nil, fmt.Errorf("no releases found")
	}
	if version == "" {
		for i := range releaseList {
			if releaseList[i].Stable {
				return &releaseList[i], nil
			}
		}
		return nil, fmt.Errorf("no stable releases found")
	}

	for i := range releaseList {
		if releaseList[i].Version == version {
			return &releaseList[i], nil
		}
	}

	return nil, fmt.Errorf("version %q not found", version)
}

func FindFile(release *GoRelease, goos, goarch string) (*GoFile, error) {
	for i := range release.Files {
		if release.Files[i].Os == goos && release.Files[i].Arch == goarch {
			return &release.Files[i], nil
		}
	}

	return nil, fmt.Errorf("no file found for %s/%s in %s", goos, goarch, release.Version)
}
