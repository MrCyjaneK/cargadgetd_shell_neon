package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchGPSKeepsOtherKeys(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	in := "# header\nshell:\n  id: \"basic\"\n\nneon:\n  # overlay\n  log_overlay: false\n"
	if err := os.WriteFile(p, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchGPS(p, "mock"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !containsAll(got, "# header", `id: "basic"`, "# overlay", "log_overlay: false", "backend: mock") {
		t.Fatalf("yaml:\n%s", got)
	}
	prev := configSearch
	configSearch = []string{p}
	t.Cleanup(func() { configSearch = prev })
	if loadGPS() != "mock" {
		t.Fatalf("gps %q", loadGPS())
	}
	if err := patchGPS(p, "geoclue"); err != nil {
		t.Fatal(err)
	}
	if loadGPS() != "geoclue" {
		t.Fatalf("gps %q", loadGPS())
	}
}

func TestPatchAppendsGPSSection(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("shell:\n  id: \"basic\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchGPS(p, "mock"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !containsAll(got, `id: "basic"`, "gps:", "backend: mock") {
		t.Fatalf("yaml:\n%s", got)
	}
}

func TestLoadGPSDefaultsToGeoclue(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("neon:\n  gps: mock\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := configSearch
	configSearch = []string{p}
	t.Cleanup(func() { configSearch = prev })
	if got := loadGPS(); got != "geoclue" {
		t.Fatalf("gps %q", got)
	}
	if err := patchGPS(p, "mock"); err != nil {
		t.Fatal(err)
	}
	if got := loadGPS(); got != "mock" {
		t.Fatalf("gps %q", got)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(string(b), "gps: mock", "backend: mock") {
		t.Fatalf("yaml:\n%s", b)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}

func writeFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}
