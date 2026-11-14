package main

import (
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mrcyjanek/cargadgetd_shell_neon/gps"
)

func TestPlaceTitleDropsParent(t *testing.T) {
	if got := placeTitle("Katowice, śląskie"); got != "Katowice" {
		t.Fatal(got)
	}
	if placeTitle("  ") != "" || placeTitle("") != "" {
		t.Fatal("blank title")
	}
}

func TestPickPlaceUsesTheNearestAndStepsUpWhenFast(t *testing.T) {
	admin := "Osiedle Paderewskiego, Osiedle Paderewskiego-Muchowiec, Katowice, Górnośląsko-Zagłębiowska Metropolia, województwo śląskie, Polska"
	hits := []placeHit{{
		name:  "Osiedle Paderewskiego",
		admin: admin,
		dist:  358,
		next:  1541,
		max:   1500,
	}, {
		name: "Milowice",
		dist: 5300,
		max:  3000,
	}}
	if got := pickPlace(hits, 30); got != "Osiedle Paderewskiego" {
		t.Fatalf("slow %q", got)
	}
	if got := pickPlace(hits, 0); got != "Osiedle Paderewskiego" {
		t.Fatalf("stopped %q", got)
	}
	// 1541m at 130 km/h is about 43s, still the quarter.
	if got := pickPlace(hits, 130); got != "Osiedle Paderewskiego" {
		t.Fatalf("highway gap %q", got)
	}
	tight := hits[0]
	tight.next = 400
	if got := pickPlace([]placeHit{tight}, 120); got != "Osiedle Paderewskiego-Muchowiec" {
		t.Fatalf("fast %q", got)
	}
	tight.next = 200
	if got := pickPlace([]placeHit{tight}, 130); got != "Katowice" {
		t.Fatalf("faster %q", got)
	}
	far := pickPlace([]placeHit{{name: "Kraków", dist: 40000, max: 30000}}, 30)
	if far != "" {
		t.Fatal(far)
	}
}

func TestAvgSpeedForgetsSamplesOlderThanTwoMinutes(t *testing.T) {
	sh := NewShell()
	t0 := time.Date(2026, 10, 28, 3, 0, 0, 0, time.UTC)
	sh.mu.Lock()
	for i := 0; i < 120; i++ {
		sh.pushSpeedLocked(t0.Add(time.Duration(i)*time.Second), 20)
	}
	sh.pushSpeedLocked(t0.Add(121*time.Second), 140)
	avg := sh.speedKmh
	sh.mu.Unlock()
	if avg < 20 || avg > 22 {
		t.Fatalf("avg %v", avg)
	}
	sh.mu.Lock()
	sh.pushSpeedLocked(t0.Add(speedWindow+2*time.Minute), 140)
	avg = sh.speedKmh
	sh.mu.Unlock()
	if avg != 140 {
		t.Fatalf("old samples kept %v", avg)
	}
}

func TestParseGuide(t *testing.T) {
	name, admin, dist, next, ok := parseGuide([]byte(`{"results":[{"title":"Osiedle Paderewskiego, Osiedle Paderewskiego-Muchowiec","admin_region":"Osiedle Paderewskiego, Katowice, Polska","distance":358},{"title":"Muchowiec","distance":1899}]}`))
	if !ok || name != "Osiedle Paderewskiego" || dist != 358 || next != 1899-358 || !strings.Contains(admin, "Katowice") {
		t.Fatalf("%q %q %v %v %v", name, admin, dist, next, ok)
	}
	name, _, _, _, ok = parseGuide([]byte(`{"results":[]}`))
	if !ok || name != "" {
		t.Fatalf("empty %q %v", name, ok)
	}
	if _, _, _, _, ok = parseGuide([]byte(`not json`)); ok {
		t.Fatal("bad json")
	}
}

func TestLookupCityScout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("poitype") != "place_city" {
			w.Write([]byte(`{"results":[]}`))
			return
		}
		w.Write([]byte(`{"results":[{"title":"Katowice, śląskie","distance":1800},{"title":"Sosnowiec","distance":2200}]}`))
	}))
	t.Cleanup(srv.Close)
	prev := scoutOrigin
	scoutOrigin = srv.URL
	t.Cleanup(func() { scoutOrigin = prev })

	hits, ok := lookupCityScout(50.26, 19.02)
	if !ok || len(hits) != 1 || hits[0].name != "Katowice" || hits[0].next != 2200-1800 {
		t.Fatalf("%v %v", hits, ok)
	}
}

func TestLookupCityScoutDown(t *testing.T) {
	prev := scoutOrigin
	scoutOrigin = "http://127.0.0.1:1"
	t.Cleanup(func() { scoutOrigin = prev })
	hits, ok := lookupCityScout(50.26, 19.02)
	if ok || len(hits) != 0 {
		t.Fatalf("%v %v", hits, ok)
	}
}

type posGPS struct {
	lat, lon float64
	kmh      int
}

func (p posGPS) Snapshot() gps.Fix {
	return gps.Fix{HasPos: true, Lat: p.lat, Lon: p.lon, HasSpeed: true, Kmh: p.kmh}
}
func (posGPS) Close() {}

func TestDriveHeadingFromScout(t *testing.T) {
	prevNet, prevLookup := enableCityNet, lookupCityFn
	t.Cleanup(func() {
		enableCityNet = prevNet
		lookupCityFn = prevLookup
	})
	enableCityNet = true
	done := make(chan struct{})
	lookupCityFn = func(lat, lon float64) ([]placeHit, bool) {
		defer close(done)
		if lat < 50 || lon < 19 {
			t.Errorf("pos %v %v", lat, lon)
		}
		return []placeHit{{name: "Katowice, śląskie", dist: 1800, max: 30000}}, true
	}

	sh := NewShell()
	sh.mu.Lock()
	sh.gpsSrc = posGPS{lat: 50.26, lon: 19.02}
	sh.mu.Unlock()
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC)
	sh.Draw(dst, now)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lookup did not run")
	}
	deadline := time.Now().Add(2 * time.Second)
	for sh.place != "Katowice" && time.Now().Before(deadline) {
		now = now.Add(time.Second)
		sh.Draw(dst, now)
		time.Sleep(5 * time.Millisecond)
	}
	if sh.place != "Katowice" {
		t.Fatalf("place %q", sh.place)
	}

	moved := make(chan struct{})
	lookupCityFn = func(float64, float64) ([]placeHit, bool) {
		defer close(moved)
		return nil, true
	}
	sh.mu.Lock()
	sh.gpsSrc = posGPS{lat: 50.40, lon: 19.02}
	sh.mu.Unlock()
	sh.Draw(dst, now.Add(2*time.Second))
	select {
	case <-moved:
	case <-time.After(2 * time.Second):
		t.Fatal("moved lookup did not run")
	}
	sh.Draw(dst, now.Add(3*time.Second))
	if sh.place != "Katowice" {
		t.Fatalf("place dropped %q", sh.place)
	}
}

func TestDriveHeadingStepsUpAfterTheSpeedWindow(t *testing.T) {
	prevNet, prevLookup := enableCityNet, lookupCityFn
	t.Cleanup(func() {
		enableCityNet = prevNet
		lookupCityFn = prevLookup
	})
	enableCityNet = true
	admin := "Osiedle Paderewskiego, Osiedle Paderewskiego-Muchowiec, Katowice, Górnośląsko-Zagłębiowska Metropolia, województwo śląskie, Polska"
	hit := placeHit{
		name: "Osiedle Paderewskiego", admin: admin,
		dist: 358, next: 400, max: 1500,
	}
	lookupCityFn = func(float64, float64) ([]placeHit, bool) {
		return []placeHit{hit}, true
	}
	sh := NewShell()
	gps := &posGPS{lat: 50.247, lon: 19.040, kmh: 20}
	sh.mu.Lock()
	sh.gpsSrc = gps
	sh.mu.Unlock()
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 28, 3, 0, 0, 0, time.UTC)
	sh.Draw(dst, now)
	deadline := time.Now().Add(2 * time.Second)
	for sh.place != "Osiedle Paderewskiego" && time.Now().Before(deadline) {
		now = now.Add(time.Second)
		sh.Draw(dst, now)
		time.Sleep(5 * time.Millisecond)
	}
	if sh.place != "Osiedle Paderewskiego" {
		t.Fatalf("slow %q", sh.place)
	}
	for i := 0; i < 120; i++ {
		now = now.Add(time.Second)
		sh.Draw(dst, now)
	}
	gps.kmh = 130
	now = now.Add(time.Second)
	sh.Draw(dst, now)
	if sh.place != "Osiedle Paderewskiego" {
		t.Fatalf("one fast sample renamed to %q", sh.place)
	}
	for i := 0; i < 130; i++ {
		now = now.Add(time.Second)
		sh.Draw(dst, now)
	}
	if sh.place != "Osiedle Paderewskiego-Muchowiec" {
		t.Fatalf("fast %q", sh.place)
	}
}

func TestDriveHeadingStaysBlankWithoutScout(t *testing.T) {
	sh := NewShell()
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	sh.Draw(dst, time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC))
	if sh.place != "" {
		t.Fatalf("place still shown %q", sh.place)
	}
}
