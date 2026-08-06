package releases

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dmlyons/getlatestgo/download"
)

var sampleJSON = `[
  {
    "version": "go1.22.5",
    "stable": true,
    "files": [
      {"filename":"go1.22.5.linux-amd64.tar.gz","os":"linux","arch":"amd64","version":"go1.22.5","sha256":"aaaa","size":100,"kind":"archive"},
      {"filename":"go1.22.5.darwin-arm64.tar.gz","os":"darwin","arch":"arm64","version":"go1.22.5","sha256":"bbbb","size":100,"kind":"archive"},
      {"filename":"go1.22.5.windows-amd64.zip","os":"windows","arch":"amd64","version":"go1.22.5","sha256":"cccc","size":100,"kind":"archive"}
    ]
  },
  {
    "version": "go1.22.4",
    "stable": true,
    "files": [
      {"filename":"go1.22.4.linux-amd64.tar.gz","os":"linux","arch":"amd64","version":"go1.22.4","sha256":"dddd","size":100,"kind":"archive"}
    ]
  },
  {
    "version": "go1.23rc1",
    "stable": false,
    "files": []
  }
]`

func mustParseReleases(t *testing.T) []GoRelease {
	t.Helper()
	releaseList, err := Parse([]byte(sampleJSON))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return releaseList
}

func TestParse(t *testing.T) {
	releaseList := mustParseReleases(t)
	if len(releaseList) != 3 {
		t.Fatalf("expected 3 releases, got %d", len(releaseList))
	}
	if releaseList[0].Version != "go1.22.5" {
		t.Errorf("expected go1.22.5, got %s", releaseList[0].Version)
	}
	if !releaseList[0].Stable {
		t.Error("expected first release to be stable")
	}
	if releaseList[2].Stable {
		t.Error("expected rc release to not be stable")
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFindReleaseLatest(t *testing.T) {
	releaseList := mustParseReleases(t)
	r, err := FindRelease(releaseList, "")
	if err != nil {
		t.Fatalf("FindRelease: %v", err)
	}
	if r.Version != "go1.22.5" {
		t.Errorf("expected go1.22.5, got %s", r.Version)
	}
}

func TestFindReleaseSpecific(t *testing.T) {
	releaseList := mustParseReleases(t)
	r, err := FindRelease(releaseList, "go1.22.4")
	if err != nil {
		t.Fatalf("FindRelease: %v", err)
	}
	if r.Version != "go1.22.4" {
		t.Errorf("expected go1.22.4, got %s", r.Version)
	}
}

func TestFindReleaseNotFound(t *testing.T) {
	releaseList := mustParseReleases(t)
	if _, err := FindRelease(releaseList, "go1.99.0"); err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestFindReleaseEmpty(t *testing.T) {
	if _, err := FindRelease(nil, ""); err == nil {
		t.Fatal("expected error for empty releases")
	}
}

func TestFindFile(t *testing.T) {
	releaseList := mustParseReleases(t)
	tests := []struct {
		goos, goarch string
		wantFile     string
	}{
		{"linux", "amd64", "go1.22.5.linux-amd64.tar.gz"},
		{"darwin", "arm64", "go1.22.5.darwin-arm64.tar.gz"},
		{"windows", "amd64", "go1.22.5.windows-amd64.zip"},
	}

	for _, tc := range tests {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			f, err := FindFile(&releaseList[0], tc.goos, tc.goarch)
			if err != nil {
				t.Fatalf("FindFile: %v", err)
			}
			if f.Filename != tc.wantFile {
				t.Errorf("expected %s, got %s", tc.wantFile, f.Filename)
			}
		})
	}
}

func TestFindFileNotFound(t *testing.T) {
	releaseList := mustParseReleases(t)
	if _, err := FindFile(&releaseList[0], "plan9", "mips"); err == nil {
		t.Fatal("expected error for missing os/arch")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	releaseList := mustParseReleases(t)

	data, err := json.Marshal(releaseList)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var parsed []GoRelease
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(parsed) != len(releaseList) {
		t.Fatalf("expected %d releases after round-trip, got %d", len(releaseList), len(parsed))
	}
	if parsed[0].Files[0].Sha256 != releaseList[0].Files[0].Sha256 {
		t.Error("sha256 mismatch after round-trip")
	}
}

func TestFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleJSON))
	}))
	defer server.Close()

	client := download.NewRetryClient()
	client.HTTPClient = server.Client()
	client.RetrySleep = func(_ time.Duration) {}

	releaseList, err := Fetch(client, server.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(releaseList) != 3 {
		t.Fatalf("expected 3 releases, got %d", len(releaseList))
	}
}
