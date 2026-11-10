package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeVersionFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInstalledWithTime(t *testing.T) {
	dir := writeVersionFile(t, "go1.22.5\ntime 2024-06-27T20:11:12Z\n")

	inst, err := Installed(dir)
	if err != nil {
		t.Fatalf("Installed: %v", err)
	}
	if inst == nil || inst.Version != "go1.22.5" {
		t.Fatalf("expected go1.22.5, got %+v", inst)
	}
	want := time.Date(2024, 6, 27, 20, 11, 12, 0, time.UTC)
	if !inst.Time.Equal(want) {
		t.Fatalf("expected time %v, got %v", want, inst.Time)
	}
}

func TestInstalledLegacyVersionOnly(t *testing.T) {
	dir := writeVersionFile(t, "go1.20.14")

	inst, err := Installed(dir)
	if err != nil {
		t.Fatalf("Installed: %v", err)
	}
	if inst == nil || inst.Version != "go1.20.14" || !inst.Time.IsZero() {
		t.Fatalf("expected go1.20.14 with zero time, got %+v", inst)
	}
}

func TestInstalledNotInstalled(t *testing.T) {
	inst, err := Installed(filepath.Join(t.TempDir(), "go"))
	if err != nil {
		t.Fatalf("Installed: %v", err)
	}
	if inst != nil {
		t.Fatalf("expected nil for missing installation, got %+v", inst)
	}
}

func TestInstalledEmptyVersionFile(t *testing.T) {
	dir := writeVersionFile(t, "\n")

	_, err := Installed(dir)
	if err == nil {
		t.Fatal("expected error for empty VERSION file")
	}
	if !strings.Contains(err.Error(), "empty VERSION file") {
		t.Fatalf("expected 'empty VERSION file' in error, got: %v", err)
	}
}
