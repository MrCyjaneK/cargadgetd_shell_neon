package main

import (
	"image"
	"image/color"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/mrcyjanek/cargadgetd_shell_neon/gps"
)

// packRow places children at their own size, centered on the cross axis.
type packRow struct{ gap float32 }

func (p packRow) MinSize(objs []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	n := 0
	for _, o := range objs {
		if o == nil || !o.Visible() {
			continue
		}
		m := o.MinSize()
		w += m.Width
		if m.Height > h {
			h = m.Height
		}
		n++
	}
	if n > 1 {
		w += p.gap * float32(n-1)
	}
	return fyne.NewSize(w, h)
}

func (p packRow) Layout(objs []fyne.CanvasObject, sz fyne.Size) {
	x := float32(0)
	for _, o := range objs {
		if o == nil || !o.Visible() {
			continue
		}
		m := o.MinSize()
		y := (sz.Height - m.Height) / 2
		if y < 0 {
			y = 0
		}
		o.Move(fyne.NewPos(x, y))
		o.Resize(m)
		x += m.Width + p.gap
	}
}

// hInset keeps a fixed height and pads the child on the left and right.
type hInset struct{ n float32 }

func (in hInset) MinSize(objs []fyne.CanvasObject) fyne.Size {
	h := float32(satCardH)
	w := in.n * 2
	if len(objs) > 0 && objs[0] != nil {
		m := objs[0].MinSize()
		w += m.Width
		if m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(w, h)
}

func (in hInset) Layout(objs []fyne.CanvasObject, sz fyne.Size) {
	if len(objs) == 0 || objs[0] == nil {
		return
	}
	inner := sz.Width - in.n*2
	if inner < 1 {
		inner = 1
	}
	objs[0].Move(fyne.NewPos(in.n, 0))
	objs[0].Resize(fyne.NewSize(inner, sz.Height))
}

// hitLabel is a full-height tap target with a centered caption.
type hitLabel struct{ h float32 }

func (h hitLabel) MinSize(objs []fyne.CanvasObject) fyne.Size {
	w := float32(1)
	height := h.h
	for _, o := range objs {
		if o == nil {
			continue
		}
		m := o.MinSize()
		if m.Width > w {
			w = m.Width
		}
		if m.Height > height {
			height = m.Height
		}
	}
	return fyne.NewSize(w, height)
}

func (h hitLabel) Layout(objs []fyne.CanvasObject, sz fyne.Size) {
	if len(objs) == 0 {
		return
	}
	if objs[0] != nil {
		m := objs[0].MinSize()
		objs[0].Resize(m)
		objs[0].Move(fyne.NewPos((sz.Width-m.Width)/2, (sz.Height-m.Height)/2))
	}
	for _, o := range objs[1:] {
		if o == nil {
			continue
		}
		o.Move(fyne.NewPos(0, 0))
		o.Resize(sz)
	}
}

const (
	tileCols = 4
	tileH    = 132
	tileGap  = 12
)

// tileGrid places launcher tiles in columns and keeps them at the top.
// It does not stretch a tile to the leftover page height.
type tileGrid struct{ cols int }

func (g tileGrid) MinSize(objs []fyne.CanvasObject) fyne.Size {
	cols := g.cols
	if cols < 1 {
		cols = 1
	}
	rows := (len(objs) + cols - 1) / cols
	if rows < 1 {
		rows = 1
	}
	h := float32(rows)*tileH + float32(rows-1)*tileGap
	return fyne.NewSize(float32(cols)*160, h)
}

func (g tileGrid) Layout(objs []fyne.CanvasObject, sz fyne.Size) {
	cols := g.cols
	if cols < 1 {
		cols = 1
	}
	if len(objs) == 0 {
		return
	}
	gap := float32(tileGap)
	w := (sz.Width - gap*float32(cols-1)) / float32(cols)
	if w < 1 {
		w = 1
	}
	h := float32(tileH)
	for i, o := range objs {
		if o == nil {
			continue
		}
		col := i % cols
		row := i / cols
		o.Move(fyne.NewPos(float32(col)*(w+gap), float32(row)*(h+gap)))
		o.Resize(fyne.NewSize(w, h))
	}
}

type inset struct{ n float32 }

func (in inset) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) == 0 || objs[0] == nil {
		return fyne.NewSize(in.n*2, in.n*2)
	}
	return objs[0].MinSize().Add(fyne.NewSquareSize(in.n * 2))
}

func (in inset) Layout(objs []fyne.CanvasObject, sz fyne.Size) {
	if len(objs) == 0 || objs[0] == nil {
		return
	}
	objs[0].Move(fyne.NewPos(in.n, in.n))
	inner := sz.Subtract(fyne.NewSquareSize(in.n * 2))
	if inner.Width < 1 {
		inner.Width = 1
	}
	if inner.Height < 1 {
		inner.Height = 1
	}
	objs[0].Resize(inner)
}

func card(inner fyne.CanvasObject) fyne.CanvasObject {
	rim := canvas.NewRectangle(color.NRGBA{R: 46, G: 56, B: 54, A: 210})
	bg := canvas.NewRectangle(color.NRGBA{R: 16, G: 20, B: 24, A: 168})
	return container.NewStack(rim, container.New(&inset{n: 1}, container.NewStack(bg, container.NewPadded(inner))))
}

func (s *Shell) ensureUI() {
	s.kit.Ensure(func() fyne.CanvasObject {
		fyne.CurrentApp().Settings().SetTheme(neonTheme{})
		return s.buildUI()
	})
	if c := s.kit.Canvas(); c != nil {
		c.SetPadded(false)
	}
	s.uiReady = true
}

func (s *Shell) buildUI() fyne.CanvasObject {
	badge := canvas.NewRectangle(colLime)
	badge.SetMinSize(fyne.NewSize(22, 22))
	letter := canvas.NewText("N", colInk)
	letter.Alignment = fyne.TextAlignCenter
	letter.TextStyle = fyne.TextStyle{Bold: true}
	logo := container.NewStack(badge, letter)

	title := canvas.NewText("NEON / DRIVE", colFG)
	title.TextStyle = fyne.TextStyle{Bold: true}

	s.clock = canvas.NewText("--:--", colFG)
	s.clock.TextSize = 18
	s.clock.TextStyle = fyne.TextStyle{Bold: true}
	s.mu.Lock()
	if s.clockTxt != "" {
		s.clock.Text = s.clockTxt
	}
	s.mu.Unlock()

	s.actionTx = canvas.NewText("APPS", colLime)
	s.actionTx.TextSize = 18
	s.actionTx.TextStyle = fyne.TextStyle{Bold: true}
	s.actionTx.Alignment = fyne.TextAlignCenter
	s.action = widget.NewButton("", s.onPrimary)
	s.action.Importance = widget.LowImportance
	action := container.New(&hitLabel{h: float32(satCardH)}, s.actionTx, s.action)

	s.speedN = canvas.NewText("", colFG)
	s.speedN.TextSize = 22
	s.speedN.TextStyle = fyne.TextStyle{Bold: true}
	s.speedN.Hide()
	s.speedUnit = canvas.NewText("km/h", colMuted)
	s.speedUnit.TextSize = 14
	s.speedUnit.Hide()
	s.speedWait = canvas.NewText(gps.NoFix, colFG)
	s.speedWait.TextSize = 14
	s.speedWait.TextStyle = fyne.TextStyle{Bold: true}
	speedBit := container.New(&packRow{gap: 4}, s.speedN, s.speedUnit, s.speedWait)

	s.weatherPlace = canvas.NewText("", colFG)
	s.weatherTemp = canvas.NewText("", colFG)
	s.weatherTemp.TextSize = 20
	s.weatherTemp.TextStyle = fyne.TextStyle{Bold: true}
	s.weatherDesc = canvas.NewText("", colMuted)
	s.weatherCloud = newCloud()
	s.weatherIcon = canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	s.weatherIcon.FillMode = canvas.ImageFillContain
	s.weatherIcon.SetMinSize(fyne.NewSize(26, 26))
	s.weatherIcon.Hide()
	s.weatherBit = container.New(&packRow{gap: 4}, container.NewStack(s.weatherCloud, s.weatherIcon), s.weatherTemp)
	s.weatherBit.Hide()

	left := container.NewCenter(container.New(&packRow{gap: 8}, logo, title))
	right := container.NewCenter(container.New(&packRow{gap: 10}, s.clock, s.weatherBit, speedBit, action))
	row := container.NewBorder(nil, nil, left, right, nil)
	bg := canvas.NewRectangle(colTopBar)
	bg.SetMinSize(fyne.NewSize(1, float32(satCardH)))
	s.top = container.NewStack(bg, container.New(&hInset{n: 12}, row))

	s.drivePage = s.buildDrive()
	s.appsPage = s.buildApps()
	s.stack = container.NewStack(s.drivePage, s.appsPage)
	s.applyChrome()
	return container.NewBorder(s.top, nil, nil, nil, s.stack)
}

type fracBar struct{ frac float32 }

func (f fracBar) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(180, 10) }

func (f fracBar) Layout(objs []fyne.CanvasObject, sz fyne.Size) {
	if len(objs) < 2 {
		return
	}
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(sz)
	w := sz.Width * f.frac
	if w < 1 {
		w = 1
	}
	objs[1].Move(fyne.NewPos(0, 0))
	objs[1].Resize(fyne.NewSize(w, sz.Height))
}

func (s *Shell) buildDrive() fyne.CanvasObject {
	return container.NewStack()
}

func (s *Shell) buildApps() fyne.CanvasObject {
	s.appsBox = container.New(&tileGrid{cols: tileCols})
	s.rebuildApps()
	s.appsScroll = container.NewVScroll(s.appsBox)
	// The scroll is the page. Border's fourth argument is the right edge,
	// which held the grid at its minimum width and left the rest black.
	return s.appsScroll
}

func (s *Shell) renderSpeed(fix gps.Fix) {
	if s.speedN == nil {
		return
	}
	if !fix.HasSpeed {
		s.speedN.Hide()
		if s.speedUnit != nil {
			s.speedUnit.Hide()
		}
		if s.speedWait != nil {
			s.speedWait.Show()
		}
	} else {
		s.speedN.Text = strconv.Itoa(fix.Kmh)
		s.speedN.Show()
		if s.speedUnit != nil {
			s.speedUnit.Show()
		}
		if s.speedWait != nil {
			s.speedWait.Hide()
		}
	}
	s.speedN.Refresh()
	if s.speedUnit != nil {
		s.speedUnit.Refresh()
	}
	if s.speedWait != nil {
		s.speedWait.Refresh()
	}
	s.bump()
}

func fitLine(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 2 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func (s *Shell) rebuildApps() {
	s.mu.Lock()
	apps := append([]*App(nil), s.apps...)
	box := s.appsBox
	s.mu.Unlock()
	if box == nil {
		return
	}
	var objs []fyne.CanvasObject
	if len(apps) == 0 {
		objs = []fyne.CanvasObject{widget.NewLabel("No apps loaded")}
	} else {
		objs = make([]fyne.CanvasObject, 0, len(apps))
		for _, a := range apps {
			a := a
			name := a.Name
			if name == "" {
				name = a.ID
			}
			objs = append(objs, s.appTile(a, name))
		}
	}
	box.Objects = objs
	box.Refresh()
	if s.appsScroll != nil {
		s.appsScroll.Refresh()
	}
	s.bump()
}

func (s *Shell) appTile(a *App, name string) fyne.CanvasObject {
	label := canvas.NewText(name, colFG)
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle = fyne.TextStyle{Bold: true}
	body := container.NewVBox(container.NewCenter(appGlyph(a)), label)
	bg := canvas.NewRectangle(colCard)
	rim := canvas.NewRectangle(color.NRGBA{R: 46, G: 56, B: 54, A: 255})
	id := ""
	if a != nil {
		id = a.ID
	}
	hit := widget.NewButton("", func() { s.openApp(id) })
	hit.Importance = widget.LowImportance
	return container.NewStack(
		rim,
		container.New(&inset{n: 1}, bg),
		container.NewPadded(container.NewCenter(body)),
		hit,
	)
}

func appGlyph(a *App) fyne.CanvasObject {
	if a != nil && a.Icon != nil {
		img := canvas.NewImageFromImage(a.Icon)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(64, 64))
		return img
	}
	id := ""
	label := "?"
	if a != nil {
		id = a.ID
		if a.Name != "" {
			label = a.Name[:1]
		} else if a.ID != "" {
			label = a.ID[:1]
		}
	}
	bg := canvas.NewRectangle(accent(id))
	bg.SetMinSize(fyne.NewSize(64, 64))
	tx := canvas.NewText(label, colInk)
	tx.Alignment = fyne.TextAlignCenter
	tx.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewStack(bg, tx)
}

func accent(id string) color.Color {
	pal := []color.NRGBA{
		{R: 140, G: 220, B: 60, A: 255},
		{R: 255, G: 120, B: 70, A: 255},
		{R: 70, G: 210, B: 220, A: 255},
		{R: 230, G: 200, B: 60, A: 255},
		{R: 170, G: 130, B: 255, A: 255},
		{R: 255, G: 150, B: 40, A: 255},
		{R: 80, G: 170, B: 255, A: 255},
	}
	h := 0
	for _, c := range id {
		h += int(c)
	}
	return pal[h%len(pal)]
}

func (s *Shell) applyChrome() {
	if s.actionTx == nil {
		return
	}
	s.mu.Lock()
	v := s.screen
	s.mu.Unlock()
	if v == viewDrive {
		s.actionTx.Text = "APPS"
		s.actionTx.Color = colLime
	} else {
		s.actionTx.Text = "BACK"
		s.actionTx.Color = colLime
	}
	s.actionTx.Refresh()
	show := func(o fyne.CanvasObject, on bool) {
		if o == nil {
			return
		}
		if on {
			o.Show()
		} else {
			o.Hide()
		}
	}
	// Plugin mode paints its own bar; the Fyne pages stay hidden underneath.
	show(s.drivePage, v == viewDrive)
	show(s.appsPage, v == viewApps)
	if s.stack != nil {
		s.stack.Refresh()
	}
}

func (s *Shell) syncClock(now time.Time) {
	txt := now.Format("15:04")
	s.mu.Lock()
	changed := txt != s.clockTxt
	s.clockTxt = txt
	lbl := s.clock
	s.mu.Unlock()
	if !changed || lbl == nil {
		return
	}
	lbl.Text = txt
	lbl.Refresh()
	s.kit.MarkDirty()
}
