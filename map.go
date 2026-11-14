package main

import (
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

func fillCircle(dst *image.RGBA, cx, cy, r int, c color.Color) {
	if r < 1 || dst == nil {
		return
	}
	rr := r * r
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= rr {
				if image.Pt(x, y).In(dst.Bounds()) {
					dst.Set(x, y, c)
				}
			}
		}
	}
}

func fillRoundRect(dst *image.RGBA, r image.Rectangle, rad int, c color.Color) {
	if rad < 1 {
		fillRect(dst, r, c)
		return
	}
	if rad*2 > r.Dx() {
		rad = r.Dx() / 2
	}
	if rad*2 > r.Dy() {
		rad = r.Dy() / 2
	}
	fillRect(dst, image.Rect(r.Min.X+rad, r.Min.Y, r.Max.X-rad, r.Max.Y), c)
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y+rad, r.Max.X, r.Max.Y-rad), c)
	fillCircle(dst, r.Min.X+rad, r.Min.Y+rad, rad, c)
	fillCircle(dst, r.Max.X-rad-1, r.Min.Y+rad, rad, c)
	fillCircle(dst, r.Min.X+rad, r.Max.Y-rad-1, rad, c)
	fillCircle(dst, r.Max.X-rad-1, r.Max.Y-rad-1, rad, c)
}

func strokeRoundRect(dst *image.RGBA, r image.Rectangle, rad, thick int, c color.Color) {
	if thick < 1 {
		thick = 1
	}
	for i := 0; i < thick; i++ {
		rr := image.Rect(r.Min.X+i, r.Min.Y+i, r.Max.X-i, r.Max.Y-i)
		if rr.Dx() < 2 || rr.Dy() < 2 {
			break
		}
		strokeRect(dst, rr, c)
	}
	_ = rad
}

func paintCloud(dst *image.RGBA, cx, cy, rad int) {
	c := color.NRGBA{R: 120, G: 210, B: 230, A: 255}
	fillCircle(dst, cx-rad/2, cy, rad, c)
	fillCircle(dst, cx+rad/2, cy+rad/5, rad-1, c)
	fillCircle(dst, cx, cy-rad/2, rad-1, c)
}

func newCloud() fyne.CanvasObject {
	r := canvas.NewRaster(func(w, h int) image.Image {
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		rad := h / 5
		if rad < 4 {
			rad = 4
		}
		paintCloud(dst, w/2, h/2, rad)
		return dst
	})
	r.ScaleMode = canvas.ImageScalePixels
	r.SetMinSize(fyne.NewSize(26, 26))
	return r
}
