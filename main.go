package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"

	"github.com/schollz/progressbar/v3"
)

const goURL = "https://golang.org/dl/?mode=json"

type golangOrgResp []struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Files   []struct {
		Filename string `json:"filename"`
		Os       string `json:"os"`
		Arch     string `json:"arch"`
		Version  string `json:"version"`
		Sha256   string `json:"sha256"`
		Size     int    `json:"size"`
		Kind     string `json:"kind"`
	} `json:"files"`
}

func main() {
	lg := log.New(os.Stderr, "glg:", 0)
	execute := flag.Bool("execute", false, "actually download the file")
	version := flag.Bool("version", false, "show version and exit")
	flag.Parse()

	if *version {
		i, _ := debug.ReadBuildInfo()
		lg.Printf(" Version\n%s", i)
		os.Exit(0)
	}

	resp, err := http.Get(goURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var g golangOrgResp

	err = json.Unmarshal(body, &g)
	if err != nil {
		log.Fatal(err)
	}

	lg.Printf("%v a: %s o: %s\n", g[0].Version, runtime.GOARCH, runtime.GOOS)

	// https://dl.google.com/go/go1.14.linux-amd64.tar.gz
	// find right one
	for _, f := range g[0].Files {
		if f.Arch == runtime.GOARCH && f.Os == runtime.GOOS {
			fn := fmt.Sprintf("https://dl.google.com/go/%s", f.Filename)
			lg.Println("DL: ", fn)
			if *execute {
				localFile := filepath.Join(os.TempDir(), f.Filename)
				DownloadFile(localFile, fn)
				fmt.Printf("sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzvf %s\n", localFile)
			}
			break
		}
	}
}

func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"downloading",
	)

	// Write the body to file
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	return err
}
