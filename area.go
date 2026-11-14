package main

import (
	"image"
	"image/color"
	"time"
)

const areaKicker = "ENTERING A NEW AREA"

const (
	areaSlide = 420 * time.Millisecond
	areaHold  = 4 * time.Second
)

var (
	colAreaKicker = color.NRGBA{R: 240, G: 186, B: 40, A: 255}
	colGlitchRed  = color.NRGBA{R: 255, G: 32, B: 64, A: 255}
)

// areaEase is 0 at the start of the slide and 1 once the card is at rest.
func areaEase(elapsed time.Duration) float64 {
	if elapsed >= areaSlide {
		return 1
	}
	if elapsed <= 0 {
		return 0
	}
	t := float64(elapsed) / float64(areaSlide)
	return 1 - (1-t)*(1-t)
}

// areaOffset is how far left of the rest position the card still is.
// 0 is at rest. The motion eases out over areaSlide.
func areaOffset(elapsed time.Duration, width int) int {
	if width < 1 {
		width = 1
	}
	return -width + int(float64(width)*areaEase(elapsed))
}

// areaOrigin is the card's top-left. It eases in from off the left edge
// and rests against that edge, halfway down the screen.
func areaOrigin(b image.Rectangle, elapsed time.Duration, w, h int) (x, y int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	x = b.Min.X + areaOffset(elapsed, w)
	y = b.Min.Y + (b.Dy()-h)/2
	if y < b.Min.Y {
		y = b.Min.Y
	}
	return x, y
}

func (s *Shell) paintArea(dst *image.RGBA, now time.Time) {
	if dst == nil || s.areaName == "" || s.areaAt.IsZero() {
		return
	}
	elapsed := now.Sub(s.areaAt)
	if elapsed < 0 || elapsed >= areaHold {
		return
	}
	s.mu.Lock()
	sc := s.uiScale
	s.mu.Unlock()
	if sc < 0.25 {
		sc = 1
	}
	w, h := measureArea(sc, s.areaName)
	x, y := areaOrigin(dst.Bounds(), elapsed, w, h)
	paintAreaCard(dst, x, y, sc, s.areaName)
}

func measureArea(sc float32, name string) (w, h int) {
	kicker := textWidth("+  "+areaKicker, 1)
	row := kicker + px(16, sc) + px(26, sc)
	if nw := textWidth(name, 2); nw > row {
		row = nw
	}
	w = px(12, sc) + row
	h = px(18, sc) + 13*2 + px(6, sc)
	return w, h
}

func paintAreaCard(dst *image.RGBA, x, y int, sc float32, name string) {
	w, h := measureArea(sc, name)
	shadeRect(dst, image.Rect(x, y, x+w, y+h), colAreaBG)
	textX := x + px(12, sc)
	drawString(dst, "+", textX, y+px(2, sc), 1, colAreaKicker, false)
	drawString(dst, areaKicker, textX+textWidth("+  ", 1), y+px(2, sc), 1, colAreaKicker, false)
	dashX := textX + textWidth("+  "+areaKicker, 1) + px(16, sc)
	dashY := y + px(6, sc)
	fillRect(dst, image.Rect(dashX, dashY, dashX+px(26, sc), dashY+px(3, sc)), colGlitchRed)
	drawString(dst, name, textX, y+px(18, sc), 2, colPin, false)
}
