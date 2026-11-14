package main

import (
	"image"
	"testing"
	"time"
)

func TestAreaOffsetEasesInFromTheLeft(t *testing.T) {
	if areaOffset(0, 200) != -200 {
		t.Fatal(areaOffset(0, 200))
	}
	if areaOffset(areaSlide, 200) != 0 || areaOffset(areaSlide+time.Second, 200) != 0 {
		t.Fatal("rest")
	}
	mid := areaOffset(areaSlide/2, 200)
	if mid <= -100 || mid >= 0 {
		t.Fatalf("ease %d", mid)
	}
}

func TestAreaBannerSlidesInForANewPlace(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 28, 3, 10, 0, 0, time.UTC)
	sh.mu.Lock()
	sh.cityHits = []placeHit{{name: "Katowice", dist: 100, max: 30000}}
	sh.mu.Unlock()

	sh.Draw(dst, now)
	if countPix(dst, colAreaKicker, dst.Bounds()) != 0 {
		t.Fatal("banner started on screen")
	}
	sh.Draw(dst, now.Add(areaSlide))
	if countPix(dst, colAreaKicker, dst.Bounds()) < 40 {
		t.Fatal("banner did not arrive")
	}
	w, h := measureArea(1, "Katowice")
	x, y := areaOrigin(dst.Bounds(), areaSlide, w, h)
	if x != dst.Bounds().Min.X {
		t.Fatalf("banner x %d", x)
	}
	mid := dst.Bounds().Dy() / 2
	if y+h/2 < mid-2 || y+h/2 > mid+2 {
		t.Fatalf("banner center %d, screen mid %d", y+h/2, mid)
	}
	if countPix(dst, colAreaKicker, image.Rect(0, 0, dst.Bounds().Dx(), y)) != 0 {
		t.Fatal("banner still under the bar")
	}
	dark := 0
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w && xx < dst.Bounds().Max.X; xx++ {
			p := dst.RGBAAt(xx, yy)
			if p.A == 255 && p.R < 40 && p.G < 40 && p.B < 50 {
				dark++
			}
		}
	}
	if dark < w*h/2 {
		t.Fatalf("entering plate covers %d px", dark)
	}
	if countPix(dst, colPin, dst.Bounds()) < 20 {
		t.Fatal("place name missing")
	}
	rest := countPix(dst, colAreaKicker, dst.Bounds())

	sh.mu.Lock()
	sh.cityHits = []placeHit{{name: "Katowice", dist: 100, max: 30000}}
	sh.mu.Unlock()
	sh.Draw(dst, now.Add(areaHold-80*time.Millisecond))
	if countPix(dst, colAreaKicker, dst.Bounds()) < rest/2 {
		t.Fatal("same place restarted the slide")
	}

	sh.Draw(dst, now.Add(areaHold))
	if countPix(dst, colAreaKicker, dst.Bounds()) != 0 {
		t.Fatal("banner stuck")
	}

	sh.mu.Lock()
	sh.cityHits = []placeHit{{name: "Kraków", dist: 100, max: 30000}}
	sh.mu.Unlock()
	left := now.Add(areaHold + time.Second)
	sh.Draw(dst, left)
	if countPix(dst, colAreaKicker, dst.Bounds()) != 0 {
		t.Fatal("next area started on screen")
	}
	sh.Draw(dst, left.Add(areaSlide))
	if countPix(dst, colAreaKicker, dst.Bounds()) < 40 || countPix(dst, colPin, dst.Bounds()) < 20 {
		t.Fatal("next area did not arrive")
	}
}
