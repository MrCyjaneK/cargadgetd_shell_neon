package main

import (
	"image"
	"image/color"
	"time"

	"github.com/mrcyjanek/cargadgetd_shell_neon/gps"
)

const satHold = 4 * time.Second

// satCardH is the toast height in logical pixels: three text rows plus the
// padding around them. The top bar uses the same height.
const satCardH = 3 + 14 + 14 + 13 + 3

// Newest card is painted at the top-left as soon as it arrives. Older cards
// stay in that same rectangle, underneath, until their own hold ends.
const satQueueMax = 8

type satCard struct {
	note  gps.SatNote
	until time.Time
	shown bool
}

var (
	colSatBG   = color.NRGBA{R: 8, G: 16, B: 14, A: 255}
	colSatLoss = color.NRGBA{R: 255, G: 48, B: 128, A: 255}
)

func (s *Shell) queueSatNotes(notes []gps.SatNote) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range notes {
		s.satStack = append([]satCard{{note: n}}, s.satStack...)
	}
	if len(s.satStack) > satQueueMax {
		s.satStack = s.satStack[:satQueueMax]
	}
}

func (s *Shell) paintSat(dst *image.RGBA, now time.Time) {
	if dst == nil {
		return
	}
	s.mu.Lock()
	live := s.satStack[:0:0]
	for _, c := range s.satStack {
		if c.shown && !now.Before(c.until) {
			continue
		}
		if !c.shown {
			c.shown = true
			c.until = now.Add(satHold)
		}
		live = append(live, c)
	}
	if len(live) > satQueueMax {
		live = live[:satQueueMax]
	}
	s.satStack = append([]satCard(nil), live...)
	sc := s.uiScale
	s.mu.Unlock()
	if sc < 0.25 {
		sc = 1
	}
	y := dst.Bounds().Min.Y
	for i := len(live) - 1; i >= 0; i-- {
		c := live[i]
		box := satToastBox(dst.Bounds(), sc, c.note, y)
		paintSatCard(dst, box, sc, c.note)
	}
}

func paintSatCard(dst *image.RGBA, box image.Rectangle, sc float32, n gps.SatNote) {
	fillRect(dst, box, colSatBG)
	rail := colLime
	if !n.Gain {
		rail = colSatLoss
	}
	fillRect(dst, image.Rect(box.Min.X, box.Min.Y, box.Min.X+px(3, sc), box.Max.Y), rail)
	flagW, flagH := px(20, sc), px(12, sc)
	flagX := box.Min.X + px(7, sc)
	flagY := box.Min.Y + (box.Dy()-flagH)/2
	paintFlag(dst, image.Rect(flagX, flagY, flagX+flagW, flagY+flagH), n.Flag)
	textX := flagX + flagW + px(6, sc)
	y0 := box.Min.Y + px(3, sc)
	step := px(14, sc)
	drawString(dst, n.Kicker, textX, y0, 1, colMuted, false)
	title := colFG
	if n.Gain {
		title = colLime
	} else {
		title = colSatLoss
	}
	drawString(dst, n.Title, textX+textWidth(n.Kicker+"  ", 1), y0, 1, title, false)
	drawString(dst, n.Sub, textX, y0+step, 1, colFG, false)
	drawString(dst, n.Detail, textX, y0+step*2, 1, colMuted, false)
}

func satToastBox(b image.Rectangle, sc float32, n gps.SatNote, y int) image.Rectangle {
	tw := textWidth(n.Kicker+"  "+n.Title, 1)
	for _, line := range []string{n.Sub, n.Detail} {
		if w := textWidth(line, 1); w > tw {
			tw = w
		}
	}
	w := px(7+20+6, sc) + tw + px(6, sc)
	h := px(satCardH, sc)
	if w > b.Dx() {
		w = b.Dx()
	}
	if h > b.Dy() {
		h = b.Dy()
	}
	return image.Rect(b.Min.X, y, b.Min.X+w, y+h)
}

func px(n int, sc float32) int {
	v := int(float32(n)*sc + 0.5)
	if v < 1 {
		return 1
	}
	return v
}

func paintFlag(dst *image.RGBA, r image.Rectangle, code string) {
	if r.Dx() < 4 || r.Dy() < 4 {
		return
	}
	switch code {
	case "us":
		fillRect(dst, r, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		band := r.Dy() / 5
		if band < 1 {
			band = 1
		}
		red := color.NRGBA{R: 180, G: 20, B: 40, A: 255}
		for i := 0; i < 5; i += 2 {
			fillRect(dst, image.Rect(r.Min.X, r.Min.Y+i*band, r.Max.X, r.Min.Y+(i+1)*band), red)
		}
		fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+r.Dx()*2/5, r.Min.Y+r.Dy()*3/5), color.NRGBA{R: 20, G: 40, B: 140, A: 255})
	case "ru":
		h := r.Dy() / 3
		fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+h), color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		fillRect(dst, image.Rect(r.Min.X, r.Min.Y+h, r.Max.X, r.Min.Y+2*h), color.NRGBA{R: 20, G: 60, B: 180, A: 255})
		fillRect(dst, image.Rect(r.Min.X, r.Min.Y+2*h, r.Max.X, r.Max.Y), color.NRGBA{R: 200, G: 30, B: 40, A: 255})
	case "eu":
		fillRect(dst, r, color.NRGBA{R: 0, G: 51, B: 153, A: 255})
		gold := color.NRGBA{R: 255, G: 204, B: 0, A: 255}
		cx := (r.Min.X + r.Max.X) / 2
		cy := (r.Min.Y + r.Max.Y) / 2
		rad := r.Dy() / 3
		if rad < 2 {
			rad = 2
		}
		for i := 0; i < 8; i++ {
			// Eight points on a circle, stepped in eighths of a turn.
			dx, dy := circle8(i, rad)
			fillRect(dst, image.Rect(cx+dx-1, cy+dy-1, cx+dx+1, cy+dy+1), gold)
		}
	case "jp":
		fillRect(dst, r, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		cx := (r.Min.X + r.Max.X) / 2
		cy := (r.Min.Y + r.Max.Y) / 2
		rad := r.Dy() / 4
		if rad < 2 {
			rad = 2
		}
		fillCircle(dst, cx, cy, rad, color.NRGBA{R: 188, G: 0, B: 45, A: 255})
	case "cn":
		fillRect(dst, r, color.NRGBA{R: 222, G: 41, B: 16, A: 255})
		fillRect(dst, image.Rect(r.Min.X+2, r.Min.Y+2, r.Min.X+r.Dx()/3, r.Min.Y+r.Dy()/2), color.NRGBA{R: 255, G: 222, B: 0, A: 255})
	default:
		fillRect(dst, r, color.NRGBA{R: 70, G: 220, B: 230, A: 255})
	}
}

func circle8(i, rad int) (int, int) {
	// cos/sin for k*45° without importing math into the paint path's hot
	// values: 1, 0.7, 0, -0.7, -1.
	s := []int{rad, rad * 7 / 10, 0, -rad * 7 / 10, -rad, -rad * 7 / 10, 0, rad * 7 / 10}
	c := []int{0, rad * 7 / 10, rad, rad * 7 / 10, 0, -rad * 7 / 10, -rad, -rad * 7 / 10}
	return c[i%8], s[i%8]
}
