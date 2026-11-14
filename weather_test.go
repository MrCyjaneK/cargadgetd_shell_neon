package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
)

func TestWeatherCardReadsMetFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "met_norway.json")
	body := []byte(`{"currentLocation":{"city":"Katowice","weather":{"description":"Clear","temperature":5.2,"weatherType":"n000","populated":true}}}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	iconDir := t.TempDir()
	f, err := os.Create(filepath.Join(iconDir, "icon-m-weather-n000-dark.png"))
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	prevFile, prevDirs := weatherFile, weatherIconDirs
	weatherFile = func() string { return path }
	weatherIconDirs = []string{iconDir}
	t.Cleanup(func() {
		weatherFile = prevFile
		weatherIconDirs = prevDirs
	})

	sh := NewShell()
	sh.logOn = false
	sh.Init(33, 800, 480)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 28, 1, 44, 0, 0, time.UTC)
	sh.Draw(dst, now)
	if sh.weatherPlace.Text != "Katowice" || sh.weatherTemp.Text != "5°" || sh.weatherDesc.Text != "Clear" {
		t.Fatalf("%q %q %q", sh.weatherPlace.Text, sh.weatherTemp.Text, sh.weatherDesc.Text)
	}
	if !sh.weatherIcon.Visible() || sh.weatherIcon.Image == nil {
		t.Fatal("theme icon not shown")
	}
	cpos, cok := absPos(sh.kit.Canvas().Content(), sh.clock, fyne.NewPos(0, 0))
	wpos, wok := absPos(sh.kit.Canvas().Content(), sh.weatherTemp, fyne.NewPos(0, 0))
	spos, sok := absPos(sh.kit.Canvas().Content(), sh.speedN, fyne.NewPos(0, 0))
	if !cok || !wok || !sok || !(cpos.X < wpos.X && wpos.X < spos.X) {
		t.Fatalf("clock %v weather %v speed %v", cpos, wpos, spos)
	}
	sh.Draw(dst, now.Add(time.Second))
	if sh.weatherPlace.Text != "Katowice" {
		t.Fatal("weather reread inside the hold")
	}
}

func TestWeatherIconRejectsOddNames(t *testing.T) {
	if weatherIconPath("../n000") != "" || weatherIconPath("n000/x") != "" {
		t.Fatal("symbol not confined to the theme directory")
	}
}
