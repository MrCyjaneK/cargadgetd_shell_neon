package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/mrcyjanek/cargadgetd_sdk/cfg"
	"github.com/mrcyjanek/cargadgetd_shell_neon/gps"
)

func TestMain(m *testing.M) {
	enableCityNet = false
	openGPS = func(kind string) gps.Source {
		if gps.Normalize(kind) == "mock" {
			return gps.Open("mock")
		}
		return speedGPS{kmh: 42}
	}
	os.Exit(m.Run())
}

type speedGPS struct{ kmh int }

func (s speedGPS) Snapshot() gps.Fix {
	if s.kmh <= 0 {
		return gps.Fix{}
	}
	return gps.Fix{Kmh: s.kmh, HasSpeed: true}
}

func (speedGPS) Close() {}

func TestStopReleasesGPS(t *testing.T) {
	sh := NewShell()
	c := &closeGPS{}
	sh.mu.Lock()
	sh.gpsSrc = c
	sh.mu.Unlock()
	sh.Stop()
	if !c.closed {
		t.Fatal("gps still held after stop")
	}
	sh.mu.Lock()
	left := sh.gpsSrc
	sh.mu.Unlock()
	if left != nil {
		t.Fatal("gps source kept")
	}
}

type closeGPS struct{ closed bool }

func (closeGPS) Snapshot() gps.Fix { return gps.Fix{} }
func (c *closeGPS) Close()         { c.closed = true }

func TestGPSDefaultsToGeoclueAndCanMock(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("neon:\n  log_overlay: false\n  music: mpris\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prevS, prevU := configSearch, configUser
	configSearch = []string{p}
	configUser = p
	t.Cleanup(func() { configSearch, configUser = prevS, prevU })

	sh := NewShell()
	sh.logOn = false
	if sh.gpsKind != "geoclue" {
		t.Fatalf("kind %q", sh.gpsKind)
	}
	sh.Init(33, 800, 480)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 27, 21, 48, 0, 0, time.UTC)
	sh.Draw(dst, now)
	if sh.speedN == nil || !sh.speedN.Visible() || sh.speedN.Text != "42" {
		t.Fatalf("speed %v", sh.speedN)
	}
	if sh.speedWait == nil || sh.speedWait.Visible() {
		t.Fatal("establishing still visible")
	}

	sh.mu.Lock()
	sh.gpsSrc = speedGPS{}
	sh.gpsGen++
	sh.mu.Unlock()
	sh.Draw(dst, now)
	if sh.speedN.Visible() || sh.speedWait == nil || !sh.speedWait.Visible() || sh.speedWait.Text != gps.NoFix {
		t.Fatalf("wait %v number %v", sh.speedWait, sh.speedN)
	}

	if err := sh.setGPSSource("mock"); err != nil {
		t.Fatal(err)
	}
	sh.Draw(dst, now)
	if sh.speedN.Text != "42" || !sh.speedN.Visible() {
		t.Fatalf("mock speed %q vis %v", sh.speedN.Text, sh.speedN.Visible())
	}
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(string(body), "music: mpris", "log_overlay: false", "backend: mock") {
		t.Fatalf("yaml:\n%s", body)
	}
	if sh.gpsKind != "mock" {
		t.Fatalf("gps %q", sh.gpsKind)
	}
}

func TestBarPlateAndAppsText(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.Init(33, 800, 480)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	sh.Draw(dst, time.Date(2026, 10, 27, 21, 48, 0, 0, time.UTC))
	lime := countPix(dst, colLime, image.Rect(640, 0, 800, satCardH))
	if lime < 8 || lime > 400 {
		t.Fatalf("APPS lime pixels %d", lime)
	}
	// A gap in the bar, clear of the logo and the labels, should be the plate.
	i := dst.PixOffset(400, satCardH/2)
	if dst.Pix[i+3] != 255 || dst.Pix[i] > 40 {
		t.Fatalf("bar plate %d,%d,%d,%d", dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3])
	}
}

func TestDriveAppsAndClock(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.Init(33, 800, 480)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	t0 := time.Date(2026, 10, 27, 21, 48, 10, 0, time.UTC)
	sh.Draw(dst, t0)
	if sh.clock == nil || sh.clock.Text != "21:48" {
		t.Fatalf("clock %q", clockText(sh))
	}
	if sh.barPx() != satCardH {
		t.Fatalf("bar %d want %d", sh.barPx(), satCardH)
	}
	if sh.screen != viewDrive {
		t.Fatalf("screen %d", sh.screen)
	}
	if countPix(dst, colLime, dst.Bounds()) < 20 {
		t.Fatal("drive frame has no lime")
	}
	sh.Touch(300, 240, 1)
	sh.Touch(300, 240, 0)
	if sh.screen != viewDrive {
		t.Fatal("map tap changed screen")
	}

	tap(t, sh, sh.action)
	if sh.screen != viewApps {
		t.Fatalf("after APPS screen %d", sh.screen)
	}
	sh.Draw(dst, t0)
	tap(t, sh, sh.action)
	if sh.screen != viewDrive {
		t.Fatalf("after BACK screen %d", sh.screen)
	}

	sh.Draw(dst, t0.Add(time.Minute))
	if sh.clock.Text != "21:49" {
		t.Fatalf("clock %q", sh.clock.Text)
	}
	sh.HuInput("home", 1)
	if sh.screen != viewDrive {
		t.Fatal("huinput changed screen")
	}
}

func TestHuInputHomeBackAndMedia(t *testing.T) {
	sh := NewShell()
	sh.Init(33, 800, 480)
	sh.Add(&App{ID: "music", Name: "Music"})
	sh.Add(&App{ID: "maps", Name: "Maps"})
	sh.setScreen(viewApps)
	sh.HuInput("home", 0)
	if sh.screen != viewApps {
		t.Fatal("key release left Apps")
	}
	sh.HuInput("home", 1)
	if sh.screen != viewDrive || sh.fg != nil {
		t.Fatalf("home screen=%d fg=%v", sh.screen, sh.fg)
	}
	sh.setScreen(viewPlugin)
	sh.mu.Lock()
	sh.fg = sh.byID["music"]
	sh.mu.Unlock()
	sh.HuInput("back", 1)
	if sh.screen != viewApps || sh.fg != nil {
		t.Fatalf("back screen=%d fg=%v", sh.screen, sh.fg)
	}
	sh.HuInput("media", 1)
	if sh.screen != viewPlugin || sh.fg == nil || sh.fg.ID != "music" {
		t.Fatalf("media screen=%d fg=%v", sh.screen, sh.fg)
	}
	sh.HuInput("navigation", 1)
	if sh.screen != viewPlugin || sh.fg == nil || sh.fg.ID != "maps" {
		t.Fatalf("navigation screen=%d fg=%v", sh.screen, sh.fg)
	}
}

func TestAppsListScrolls(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.Init(33, 800, 480)
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"} {
		sh.Add(&App{ID: id, Name: id})
	}
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	sh.Draw(dst, now)
	tap(t, sh, sh.action)
	sh.Draw(dst, now)
	sc := sh.appsScroll
	if sc == nil || !sc.Visible() {
		t.Fatal("apps list is not on screen")
	}
	if sc.Content.MinSize().Height <= sc.Size().Height+1 {
		t.Fatalf("list fits the page, content %v view %v", sc.Content.MinSize(), sc.Size())
	}
	pos, ok := absPos(sh.kit.Canvas().Content(), sc, fyne.NewPos(0, 0))
	if !ok {
		t.Fatal("scroll not in canvas")
	}
	cw := sh.kit.Canvas().Size().Width
	if pos.X > 1 || sc.Size().Width < cw-1 {
		t.Fatalf("apps list at %v size %v, canvas width %v", pos, sc.Size(), cw)
	}
	x := int(pos.X + 40)
	y := int(pos.Y + sc.Size().Height/2)
	sh.Touch(x, y, 1)
	sh.Touch(x, y-120, 2)
	sh.Touch(x, y-120, 0)
	if sc.Offset.Y < 60 {
		t.Fatalf("offset %v", sc.Offset.Y)
	}
	if sh.screen != viewApps || sh.fg != nil {
		t.Fatalf("scroll opened screen %d fg %v", sh.screen, sh.fg)
	}
	sh.Touch(x, y, 1)
	sh.Touch(x, y+400, 2)
	sh.Touch(x, y+400, 0)
	if sc.Offset.Y != 0 {
		t.Fatalf("offset did not clamp, %v", sc.Offset.Y)
	}
}

func TestAppsGridFillsWideCanvas(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.Init(33, 1280, 720)
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		sh.Add(&App{ID: id, Name: id})
	}
	dst := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	now := time.Date(2026, 10, 30, 23, 39, 0, 0, time.UTC)
	sh.Draw(dst, now)
	sh.onPrimary()
	sh.Draw(dst, now)
	if sh.screen != viewApps {
		t.Fatalf("screen %d", sh.screen)
	}
	sc := sh.appsScroll
	cw := sh.kit.Canvas().Size().Width
	pos, ok := absPos(sh.kit.Canvas().Content(), sc, fyne.NewPos(0, 0))
	if !ok || pos.X > 1 || sc.Size().Width < cw-1 {
		t.Fatalf("apps list at %v size %v, canvas width %v", pos, sc.Size(), cw)
	}
	bar := sh.barPx()
	rim := color.NRGBA{R: 46, G: 56, B: 54, A: 255}
	if countPix(dst, rim, image.Rect(0, bar, 24, bar+tileH)) < 8 {
		t.Fatal("first column is not on the left edge")
	}
	if countPix(dst, rim, image.Rect(dst.Bounds().Dx()-24, bar, dst.Bounds().Dx(), bar+tileH)) < 8 {
		t.Fatal("last column is not on the right edge")
	}
}

func TestSatCardFloatsOverPlugin(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.Init(33, 800, 480)
	app := &App{
		ID:   "maps",
		Name: "Maps",
		draw: func(dst *image.RGBA) {
			fill(dst, color.NRGBA{R: 220, G: 20, B: 40, A: 255})
		},
		touch:  func(int, int, int) {},
		resume: func() {},
	}
	sh.Add(app)
	src := &barGPS{fix: gps.Fix{
		Notes: []gps.SatNote{{
			Gain:   true,
			Kicker: "SAT LOCK",
			Title:  "E26",
			Sub:    "Galileo · European Union",
			Detail: "EL 37    AZ 67    SNR 34",
			Flag:   "eu",
		}},
	}}
	sh.mu.Lock()
	sh.gpsSrc = src
	sh.gpsGen++
	sh.mu.Unlock()
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 27, 21, 48, 0, 0, time.UTC)
	sh.Draw(dst, now)
	tap(t, sh, sh.action)
	sh.Draw(dst, now)
	label := findText(sh.kit.Canvas().Content(), "Maps")
	if label == nil {
		t.Fatal("Maps tile missing")
	}
	tap(t, sh, label)
	sh.Draw(dst, now)
	first := gps.SatNote{
		Kicker: "SAT LOCK", Title: "E26", Sub: "Galileo · European Union", Detail: "EL 37    AZ 67    SNR 34",
	}
	box := satToastBox(dst.Bounds(), sh.uiScale, first, 0)
	if box.Min.X != 0 || box.Min.Y != 0 {
		t.Fatalf("card at %v, want 0,0", box)
	}
	if box.Dy() > 56 || box.Dx() > 280 {
		t.Fatalf("card too large %dx%d", box.Dx(), box.Dy())
	}
	if countPix(dst, colSatBG, box) < 200 {
		t.Fatal("satellite card missing")
	}
	eu := color.NRGBA{R: 0, G: 51, B: 153, A: 255}
	if countPix(dst, eu, box) < 20 {
		t.Fatal("flag missing")
	}
	sh.queueSatNotes([]gps.SatNote{{
		Gain: true, Kicker: "SAT LOCK", Title: "G27", Sub: "GPS · United States", Detail: "EL 1    AZ 2    SNR 3", Flag: "us",
	}})
	sh.Draw(dst, now)
	top := satToastBox(dst.Bounds(), sh.uiScale, gps.SatNote{
		Kicker: "SAT LOCK", Title: "G27", Sub: "GPS · United States", Detail: "EL 1    AZ 2    SNR 3",
	}, 0)
	us := color.NRGBA{R: 20, G: 40, B: 140, A: 255}
	if countPix(dst, us, top) < 4 {
		t.Fatal("new card did not take the top")
	}
	below := image.Rect(0, top.Max.Y, dst.Bounds().Max.X, top.Max.Y+box.Dy())
	if countPix(dst, colSatBG, below) != 0 {
		t.Fatal("satellite cards stepped down")
	}
	if countPix(dst, eu, top) != 0 {
		t.Fatal("previous card was not covered")
	}
	red := color.NRGBA{R: 220, G: 20, B: 40, A: 255}
	if !pixAt(dst, dst.Bounds().Dx()-20, sh.barPx()+20, red) {
		t.Fatal("card covered the rest of the open app")
	}
	sh.Draw(dst, now.Add(satHold))
	if countPix(dst, colSatBG, box) != 0 {
		t.Fatal("satellite card stuck")
	}
}

type barGPS struct {
	fix  gps.Fix
	once bool
}

func (b *barGPS) Snapshot() gps.Fix {
	f := b.fix
	if b.once {
		f.Notes = nil
	}
	b.once = true
	return f
}

func (barGPS) Close() {}

func TestOpenPluginUnderBar(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.Init(33, 800, 480)
	var seq []string
	var gotY int
	app := &App{
		ID:   "plug",
		Name: "Maps",
		draw: func(dst *image.RGBA) {
			seq = append(seq, "draw")
			fill(dst, color.NRGBA{R: 220, G: 20, B: 40, A: 255})
		},
		touch: func(x, y, action int) {
			if action == 1 {
				gotY = y
			}
		},
		resume: func() { seq = append(seq, "resume") },
		pause:  func() { seq = append(seq, "pause") },
	}
	sh.Add(app)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 27, 21, 48, 0, 0, time.UTC)
	sh.Draw(dst, now)
	tap(t, sh, sh.action)
	sh.Draw(dst, now)
	root := sh.kit.Canvas().Content()
	label := findText(root, "Maps")
	if label == nil {
		t.Fatal("Maps tile missing")
	}
	tap(t, sh, label)
	if sh.screen != viewPlugin || sh.fg == nil || sh.fg.ID != "plug" {
		t.Fatalf("screen %d fg %v", sh.screen, sh.fg)
	}
	sh.Draw(dst, now)
	if len(seq) < 2 || seq[0] != "resume" || seq[1] != "draw" {
		t.Fatalf("seq %v", seq)
	}
	red := color.NRGBA{R: 220, G: 20, B: 40, A: 255}
	bar := sh.barPx()
	if !pixAt(dst, 20, bar+20, red) {
		t.Fatalf("plugin pixels missing at y=%d", bar+20)
	}
	if pixAt(dst, 12, 8, red) {
		t.Fatal("plugin painted over the top bar")
	}
	sh.Touch(30, bar+50, 1)
	if gotY != 50 {
		t.Fatalf("touch y %d", gotY)
	}
	tap(t, sh, sh.action)
	if sh.screen != viewApps || sh.fg != nil {
		t.Fatalf("after back screen %d fg %v", sh.screen, sh.fg)
	}
	if len(seq) != 3 || seq[2] != "pause" {
		t.Fatalf("seq %v", seq)
	}
}

func TestSettingsWritesLogOverlay(t *testing.T) {
	dir := t.TempDir()
	user := dir + "/config.yaml"
	body := "# keep\nshell:\n  id: \"basic\"\n\nneon:\n  # overlay\n  log_overlay: false\n"
	if err := writeFile(user, body); err != nil {
		t.Fatal(err)
	}
	prevS, prevU := configSearch, configUser
	configSearch = []string{user}
	configUser = user
	cfg.SetOwnRootForTest(t, dir)
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
	})

	sh := NewShell()
	if sh.logOn {
		t.Fatal("expected overlay off")
	}
	if err := sh.setLogOverlay(true); err != nil {
		t.Fatal(err)
	}
	if !sh.logOn {
		t.Fatal("toggle did not stick")
	}
	got, err := readFile(user)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(got, `id: "basic"`, "log_overlay: false", "# keep", "# overlay") {
		t.Fatalf("daemon yaml:\n%s", got)
	}
	plug, err := readFile(dir + "/neon.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plug, "log_overlay") || !strings.Contains(plug, "true") {
		t.Fatalf("plugin:\n%s", plug)
	}
}

func TestLogLineExpires(t *testing.T) {
	sh := NewShell()
	sh.logOn = true
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2000, 1, 2, 3, 4, 0, 0, time.UTC)
	sh.addLog("NEONLOG", now)
	sh.Draw(dst, now)
	if countPix(dst, colLog, image.Rect(0, 400, 800, 480)) < 8 {
		t.Fatal("log line not visible")
	}
	sh.Draw(dst, now.Add(9*time.Second))
	if countPix(dst, colLog, image.Rect(0, 400, 800, 480)) < 8 {
		t.Fatal("log line expired early")
	}
	sh.Draw(dst, now.Add(10*time.Second))
	if countPix(dst, colLog, image.Rect(0, 400, 800, 480)) != 0 {
		t.Fatal("log line still visible")
	}
}

func TestNotifBanner(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.Init(33, 800, 480)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 27, 21, 48, 0, 0, time.UTC)
	sh.showNote("Ping", "Kabuki", nil, now)
	sh.Draw(dst, now)
	bar := sh.barPx()
	strip := image.Rect(0, bar, 800, bar+int(52))
	if countPix(dst, colBanner, strip) < 100 {
		t.Fatal("banner missing")
	}
	sh.Draw(dst, now.Add(8*time.Second))
	if countPix(dst, colBanner, strip) != 0 {
		t.Fatal("banner stuck")
	}
}

func TestDriveScalesWithCanvas(t *testing.T) {
	small := drawDrive(t, 800, 480)
	large := drawDrive(t, 1280, 720)
	band := func(img *image.RGBA) image.Rectangle {
		h := img.Bounds().Dy() * satCardH / designH
		if h < 1 {
			h = 1
		}
		return image.Rect(img.Bounds().Min.X, img.Bounds().Min.Y, img.Bounds().Max.X, img.Bounds().Min.Y+h)
	}
	hs := whiteSpan(small, band(small))
	hl := whiteSpan(large, band(large))
	if hs < 12 {
		t.Fatalf("speed glyph on 480p is %dpx", hs)
	}
	if hl*10 < hs*13 || hl*10 > hs*18 {
		t.Fatalf("speed glyph did not scale: 480=%d 720=%d", hs, hl)
	}
}

func drawDrive(t *testing.T, w, h int) *image.RGBA {
	t.Helper()
	sh := NewShell()
	sh.logOn = false
	sh.Init(33, w, h)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sh.Draw(dst, time.Date(2026, 10, 27, 21, 48, 0, 0, time.UTC))
	return dst
}

func whiteSpan(img *image.RGBA, r image.Rectangle) int {
	r = r.Intersect(img.Bounds())
	minY, maxY := r.Max.Y, r.Min.Y
	found := false
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := img.PixOffset(x, y)
			if img.Pix[i] > 210 && img.Pix[i+1] > 210 && img.Pix[i+2] > 210 {
				found = true
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if !found {
		return 0
	}
	return maxY - minY + 1
}

func clockText(sh *Shell) string {
	if sh == nil || sh.clock == nil {
		return ""
	}
	return sh.clock.Text
}

func tap(t *testing.T, sh *Shell, obj fyne.CanvasObject) {
	t.Helper()
	if sh.kit.Canvas() == nil {
		t.Fatal("no canvas")
	}
	pos, ok := absPos(sh.kit.Canvas().Content(), obj, fyne.NewPos(0, 0))
	if !ok {
		t.Fatalf("object not in canvas: %T", obj)
	}
	sz := obj.Size()
	if sz.Width < 1 || sz.Height < 1 {
		t.Fatalf("zero size %v at %v", sz, pos)
	}
	x := int(pos.X + sz.Width/2)
	y := int(pos.Y + sz.Height/2)
	sh.Touch(x, y, 1)
	sh.Touch(x, y, 0)
}

func absPos(o fyne.CanvasObject, target fyne.CanvasObject, acc fyne.Position) (fyne.Position, bool) {
	if o == nil {
		return fyne.Position{}, false
	}
	p := acc.Add(o.Position())
	if o == target {
		return p, true
	}
	if c, ok := o.(*fyne.Container); ok {
		for _, ch := range c.Objects {
			if pos, found := absPos(ch, target, p); found {
				return pos, true
			}
		}
	}
	if sc, ok := o.(*container.Scroll); ok {
		if pos, found := absPos(sc.Content, target, p); found {
			return pos, true
		}
	}
	return fyne.Position{}, false
}

func findText(root fyne.CanvasObject, text string) *canvas.Text {
	var found *canvas.Text
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || found != nil {
			return
		}
		if tx, ok := o.(*canvas.Text); ok && tx.Text == text {
			found = tx
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
		}
		if sc, ok := o.(*container.Scroll); ok {
			walk(sc.Content)
		}
	}
	walk(root)
	return found
}

func findButton(root fyne.CanvasObject, text string) *widget.Button {
	var found *widget.Button
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || found != nil {
			return
		}
		if b, ok := o.(*widget.Button); ok && b.Text == text {
			found = b
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
		}
		if sc, ok := o.(*container.Scroll); ok {
			walk(sc.Content)
		}
	}
	walk(root)
	return found
}

func fill(dst *image.RGBA, c color.NRGBA) {
	for i := 0; i < len(dst.Pix); i += 4 {
		dst.Pix[i] = c.R
		dst.Pix[i+1] = c.G
		dst.Pix[i+2] = c.B
		dst.Pix[i+3] = c.A
	}
}

func pixAt(dst *image.RGBA, x, y int, c color.NRGBA) bool {
	if !image.Pt(x, y).In(dst.Bounds()) {
		return false
	}
	i := dst.PixOffset(x, y)
	return dst.Pix[i] == c.R && dst.Pix[i+1] == c.G && dst.Pix[i+2] == c.B && dst.Pix[i+3] == c.A
}

func countPix(dst *image.RGBA, c color.NRGBA, r image.Rectangle) int {
	r = r.Intersect(dst.Bounds())
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if pixAt(dst, x, y, c) {
				n++
			}
		}
	}
	return n
}
