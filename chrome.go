package main

import (
	"image"
	"time"

	xdraw "golang.org/x/image/draw"
)

type notice struct {
	title string
	body  string
	icon  *image.RGBA
	until time.Time
}

func (s *Shell) showNote(title, body string, icon *image.RGBA, now time.Time) {
	s.mu.Lock()
	s.note = &notice{title: title, body: body, icon: icon, until: now.Add(notifyHold)}
	s.mu.Unlock()
	s.logf("notif %s", title)
}

func (s *Shell) paintBanner(dst *image.RGBA, now time.Time) {
	s.mu.Lock()
	n := s.note
	s.mu.Unlock()
	if n == nil || dst == nil || !now.Before(n.until) {
		return
	}
	b := dst.Bounds()
	bar := s.barPx()
	s.mu.Lock()
	sc := s.uiScale
	s.mu.Unlock()
	if sc < 0.25 {
		sc = 1
	}
	r := image.Rect(b.Min.X+int(8*sc), b.Min.Y+bar+int(4*sc), b.Max.X-int(8*sc), b.Min.Y+bar+int(48*sc))
	if r.Max.Y > b.Dy() {
		r.Max.Y = b.Dy()
	}
	fillRect(dst, r, colBanner)
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+4, r.Max.Y), colLime)
	textX := r.Min.X + 12
	if n.icon != nil {
		ir := image.Rect(textX, r.Min.Y+8, textX+24, r.Min.Y+32)
		xdraw.NearestNeighbor.Scale(dst, ir, n.icon, n.icon.Bounds(), xdraw.Over, nil)
		textX += 30
	}
	drawString(dst, n.title, textX, r.Min.Y+6, 1, colFG, false)
	drawString(dst, n.body, textX, r.Min.Y+22, 1, colMuted, false)
}
