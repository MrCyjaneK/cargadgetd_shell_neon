package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Sailfish Weather writes the conditions it shows into this file. The icons
// are the same theme PNGs the app loads as icon-m-weather-<code>, mode 0644.
const weatherHold = 30 * time.Second

var weatherFile = func() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", "org.sailfishos", "weather", "met_norway.json")
}

var weatherIconDirs = []string{
	"/usr/share/themes/sailfish-default/silica/z2.0/icons",
	"/usr/share/themes/sailfish-default/silica/z1.75/icons",
	"/usr/share/themes/sailfish-default/silica/z1.5/icons",
	"/usr/share/themes/sailfish-default/silica/z1.25/icons",
	"/usr/share/themes/sailfish-default/silica/z1.0/icons",
}

type metReport struct {
	City string
	Temp int
	Desc string
	Type string
}

func (s *Shell) applyWeather(now time.Time) {
	if s.weatherPlace == nil {
		return
	}
	if !s.weatherChecked.IsZero() && now.Sub(s.weatherChecked) < weatherHold {
		return
	}
	s.weatherChecked = now
	rep, ok := readMetNorway(weatherFile())
	if !ok || rep == s.weatherShown {
		return
	}
	s.weatherShown = rep
	s.weatherPlace.Text = rep.City
	s.weatherTemp.Text = fmt.Sprintf("%d°", rep.Temp)
	s.weatherDesc.Text = rep.Desc
	s.weatherPlace.Refresh()
	s.weatherTemp.Refresh()
	s.weatherDesc.Refresh()
	if img := loadWeatherIcon(rep.Type); img != nil && s.weatherIcon != nil {
		s.weatherIcon.Image = img
		s.weatherIcon.Show()
		if s.weatherCloud != nil {
			s.weatherCloud.Hide()
		}
		s.weatherIcon.Refresh()
	}
	if s.weatherBit != nil {
		s.weatherBit.Show()
	}
	s.bump()
}

func readMetNorway(path string) (metReport, bool) {
	if path == "" {
		return metReport{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return metReport{}, false
	}
	var doc struct {
		CurrentLocation struct {
			City    string `json:"city"`
			Weather struct {
				Description string  `json:"description"`
				Temperature float64 `json:"temperature"`
				WeatherType string  `json:"weatherType"`
				Populated   bool    `json:"populated"`
			} `json:"weather"`
		} `json:"currentLocation"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return metReport{}, false
	}
	w := doc.CurrentLocation.Weather
	if !w.Populated || doc.CurrentLocation.City == "" {
		return metReport{}, false
	}
	return metReport{
		City: doc.CurrentLocation.City,
		Temp: int(w.Temperature),
		Desc: w.Description,
		Type: w.WeatherType,
	}, true
}

func loadWeatherIcon(code string) image.Image {
	path := weatherIconPath(code)
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

func weatherIconPath(code string) string {
	if !weatherSymbol(code) {
		return ""
	}
	name := "icon-m-weather-" + code + "-dark.png"
	for _, dir := range weatherIconDirs {
		p := filepath.Join(dir, name)
		st, err := os.Stat(p)
		if err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

func weatherSymbol(code string) bool {
	if len(code) < 2 || len(code) > 8 {
		return false
	}
	for _, r := range code {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return !strings.Contains(code, "..")
}
