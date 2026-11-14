package main

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func fillRect(dst *image.RGBA, r image.Rectangle, c color.Color) {
	if dst == nil {
		return
	}
	r = r.Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

func strokeRect(dst *image.RGBA, r image.Rectangle, c color.Color) {
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+2), c)
	fillRect(dst, image.Rect(r.Min.X, r.Max.Y-2, r.Max.X, r.Max.Y), c)
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+2, r.Max.Y), c)
	fillRect(dst, image.Rect(r.Max.X-2, r.Min.Y, r.Max.X, r.Max.Y), c)
}

func shadeRect(dst *image.RGBA, r image.Rectangle, c color.NRGBA) {
	if dst == nil {
		return
	}
	r = r.Intersect(dst.Bounds())
	a := int(c.A)
	if a <= 0 || r.Empty() {
		return
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := dst.PixOffset(x, y)
			dst.Pix[i+0] = uint8((int(c.R)*a + int(dst.Pix[i+0])*(255-a)) / 255)
			dst.Pix[i+1] = uint8((int(c.G)*a + int(dst.Pix[i+1])*(255-a)) / 255)
			dst.Pix[i+2] = uint8((int(c.B)*a + int(dst.Pix[i+2])*(255-a)) / 255)
			dst.Pix[i+3] = 255
		}
	}
}

// strokeOrtho draws a horizontal or vertical bar. Diagonal segments are skipped.
func strokeOrtho(dst *image.RGBA, a, b image.Point, thick int, c color.Color) {
	if thick < 1 {
		thick = 1
	}
	half := thick / 2
	if a.X == b.X {
		y0, y1 := a.Y, b.Y
		if y0 > y1 {
			y0, y1 = y1, y0
		}
		fillRect(dst, image.Rect(a.X-half, y0, a.X-half+thick, y1+thick), c)
		return
	}
	if a.Y == b.Y {
		x0, x1 := a.X, b.X
		if x0 > x1 {
			x0, x1 = x1, x0
		}
		fillRect(dst, image.Rect(x0, a.Y-half, x1+thick, a.Y-half+thick), c)
	}
}

func textWidth(s string, scale int) int {
	if scale < 1 {
		scale = 1
	}
	return font.MeasureString(basicfont.Face7x13, s).Ceil() * scale
}

// drawString blits a 7x13 bitmap, then nearest-neighbor scales it.
// cy is the top of the glyph box unless center is set.
func drawString(dst *image.RGBA, s string, cx, cy, scale int, col color.Color, center bool) {
	if dst == nil || s == "" {
		return
	}
	if scale < 1 {
		scale = 1
	}
	face := basicfont.Face7x13
	advance := font.MeasureString(face, s).Ceil() * scale
	height := face.Height * scale
	x := cx
	y := cy
	if center {
		x = cx - advance/2
		y = cy - height/2
	}
	tmp := image.NewRGBA(image.Rect(0, 0, advance/scale+2, face.Height+2))
	d := &font.Drawer{
		Dst:  tmp,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(1, face.Ascent),
	}
	d.DrawString(s)
	for py := 0; py < tmp.Bounds().Dy(); py++ {
		for px := 0; px < tmp.Bounds().Dx(); px++ {
			_, _, _, a := tmp.At(px, py).RGBA()
			if a == 0 {
				continue
			}
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					dst.Set(x+px*scale+sx, y+py*scale+sy, col)
				}
			}
		}
	}
}
