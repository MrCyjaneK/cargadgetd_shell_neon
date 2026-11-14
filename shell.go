package main

import (
	"fmt"
	"image"
	"image/draw"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/mrcyjanek/cargadgetd_sdk/cfg"
	"github.com/mrcyjanek/cargadgetd_sdk/fynekit"
	"github.com/mrcyjanek/cargadgetd_shell_neon/gps"
	xdraw "golang.org/x/image/draw"
)

type view int

const (
	viewDrive view = iota
	viewApps
	viewPlugin
)

// App is one host plugin the shell can open.
type App struct {
	ID, Name string
	Icon     *image.RGBA
	draw     func(*image.RGBA)
	touch    func(x, y, action int)
	pause    func()
	resume   func()
	resumed  bool
	buf      *image.RGBA
}

// Shell is the neon home chrome. Drive and Apps are Fyne, laid out
// in a designH-tall logical canvas and scaled to the framebuffer. An open
// plugin is blitted under that bar so its framebuffer is not captured again.
type Shell struct {
	mu sync.Mutex

	w, h    int
	frameMS int
	uiReady bool
	uiScale float32
	barDIP  float32
	fyneGen int
	fyneKey string
	fyneImg *image.RGBA

	apps    []*App
	byID    map[string]*App
	fg      *App
	layers  []layerSpec
	gesture layerGesture

	screen view

	clockTxt string
	logOn    bool
	logs     []logLine
	note     *notice

	gpsSrc     gps.Source
	gpsKind    string
	gpsShown   gps.Fix
	gpsHosted  bool
	gpsErr     string
	gpsGen     int
	gpsApplied int
	satStack   []satCard

	weatherPlace   *canvas.Text
	weatherTemp    *canvas.Text
	weatherDesc    *canvas.Text
	weatherIcon    *canvas.Image
	weatherCloud   fyne.CanvasObject
	weatherBit     fyne.CanvasObject
	weatherChecked time.Time
	weatherShown   metReport

	place        string
	cityLat      float64
	cityLon      float64
	cityQueried  bool
	cityBusy     bool
	cityAnswered bool
	cityHits     []placeHit
	cityFailed   time.Time
	speedHist    []speedSample
	speedKmh     float64
	areaName     string
	areaAt       time.Time

	kit        *fynekit.Kit
	top        fyne.CanvasObject
	clock      *canvas.Text
	action     *widget.Button
	actionTx   *canvas.Text
	speedN     *canvas.Text
	speedUnit  *canvas.Text
	speedWait  *canvas.Text
	appsBox    *fyne.Container
	appsScroll *container.Scroll
	drivePage  fyne.CanvasObject
	appsPage   fyne.CanvasObject
	stack      *fyne.Container
}

// openGPS is the location backend factory. Tests replace it.
var openGPS = gps.Open

func NewShell() *Shell {
	return &Shell{
		kit:        fynekit.NewTransparent(),
		byID:       map[string]*App{},
		screen:     viewDrive,
		logOn:      loadLogOverlay(),
		gpsKind:    gps.Normalize(loadGPS()),
		gpsApplied: -1,
		layers:     loadLayers(),
	}
}

var shell = NewShell()

func (s *Shell) Init(frameMS, w, h int) {
	s.mu.Lock()
	s.frameMS = frameMS
	if w > 0 {
		s.w = w
	}
	if h > 0 {
		s.h = h
	}
	s.mu.Unlock()
	s.logf("init %dx%d frame_ms=%d", w, h, frameMS)
}

func (s *Shell) Add(a *App) {
	if a == nil || a.ID == "" {
		return
	}
	s.mu.Lock()
	if s.byID == nil {
		s.byID = map[string]*App{}
	}
	if prev, ok := s.byID[a.ID]; ok {
		prev.Name = a.Name
		prev.Icon = a.Icon
		prev.draw = a.draw
		prev.touch = a.touch
		prev.pause = a.pause
		prev.resume = a.resume
		s.mu.Unlock()
	} else {
		s.apps = append(s.apps, a)
		s.byID[a.ID] = a
		s.mu.Unlock()
	}
	if s.appsBox != nil {
		s.rebuildApps()
	}
	s.logf("app %s", a.ID)
}

func (s *Shell) Draw(dst *image.RGBA, now time.Time) {
	if dst == nil {
		return
	}
	b := dst.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return
	}
	s.mu.Lock()
	s.w, s.h = w, h
	s.expireLocked(now)
	screen := s.screen
	fg := s.fg
	s.mu.Unlock()

	s.syncClock(now)
	s.ensureUI()
	s.applyGPS(now)
	s.applyWeather(now)
	s.refreshFyne(w, h)
	fillRect(dst, b, colBG)
	if screen == viewDrive {
		s.paintLayers(dst, layerBase)
	}
	s.blitFyne(dst)
	if screen == viewPlugin && fg != nil {
		s.blitPlugin(dst, fg)
	}
	if screen == viewDrive {
		s.paintLayers(dst, layerOverlay)
	}
	s.paintBanner(dst, now)
	s.paintLogs(dst)
	if !s.layerReady("area") {
		s.paintArea(dst, now)
	}
	if !s.layerReady("sats") {
		s.paintSat(dst, now)
	}
}

func (s *Shell) blitPlugin(dst *image.RGBA, a *App) {
	b := dst.Bounds()
	bar := s.barPx()
	cw, ch := b.Dx(), b.Dy()-bar
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	s.ensureResumed(a)
	if fr := a.paint(cw, ch); fr != nil {
		draw.Draw(dst, image.Rect(b.Min.X, b.Min.Y+bar, b.Min.X+cw, b.Min.Y+bar+ch), fr, image.Point{}, draw.Over)
	}
}

// refreshFyne lays the UI out at logical height designH and caches the
// capture. The bar height is known after this returns, so the drive base
// can be painted in the rect under it before blitFyne composites the chrome.
func (s *Shell) refreshFyne(w, h int) {
	s.ensureUI()
	sc := float32(h) / float32(designH)
	if sc < 0.25 {
		sc = 0.25
	}
	s.mu.Lock()
	s.uiScale = sc
	clock := s.clockTxt
	gen := s.fyneGen
	screen := s.screen
	s.mu.Unlock()
	key := fmt.Sprintf("%d|%d|%d|%s|%d", w, h, screen, clock, gen)
	if s.fyneImg != nil && s.fyneKey == key && s.fyneImg.Bounds().Dx() == w && s.fyneImg.Bounds().Dy() == h {
		return
	}
	c := s.kit.Canvas()
	if c == nil {
		return
	}
	c.SetScale(sc)
	c.Resize(fyne.NewSize(float32(w)/sc, float32(h)/sc))
	if content := c.Content(); content != nil {
		sz := content.Size()
		content.Resize(fyne.NewSize(1, 1))
		content.Resize(sz)
	}
	if s.top != nil {
		s.mu.Lock()
		s.barDIP = s.top.Size().Height
		s.mu.Unlock()
	}
	img := c.Capture()
	cp := image.NewRGBA(image.Rect(0, 0, w, h))
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		xdraw.NearestNeighbor.Scale(cp, cp.Bounds(), img, img.Bounds(), draw.Src, nil)
	} else {
		draw.Draw(cp, cp.Bounds(), img, image.Point{}, draw.Src)
	}
	s.fyneImg = cp
	s.fyneKey = key
}

func (s *Shell) blitFyne(dst *image.RGBA) {
	if dst == nil || s.fyneImg == nil {
		return
	}
	draw.Draw(dst, dst.Bounds(), s.fyneImg, s.fyneImg.Bounds().Min, draw.Over)
}

func (s *Shell) barPx() int {
	s.mu.Lock()
	dip := s.barDIP
	sc := s.uiScale
	s.mu.Unlock()
	if sc < 0.05 {
		sc = 1
	}
	if dip < 1 {
		dip = 52
	}
	return int(dip*sc + 0.5)
}

func (s *Shell) bump() {
	s.mu.Lock()
	s.fyneGen++
	s.mu.Unlock()
}

func (a *App) paint(w, h int) *image.RGBA {
	if a == nil {
		return nil
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if a.buf == nil || a.buf.Bounds().Dx() != w || a.buf.Bounds().Dy() != h {
		a.buf = image.NewRGBA(image.Rect(0, 0, w, h))
	}
	if a.draw != nil {
		a.draw(a.buf)
	}
	return a.buf
}

func (s *Shell) ensureResumed(a *App) {
	if a == nil || a.resumed {
		return
	}
	if a.resume != nil {
		a.resume()
	}
	a.resumed = true
}

// Touch takes framebuffer pixels. Fyne hit testing is in logical units, so
// coordinates are divided by the same scale paintFyne applied.
func (s *Shell) Touch(x, y, action int) {
	s.mu.Lock()
	screen := s.screen
	fg := s.fg
	sc := s.uiScale
	s.mu.Unlock()
	if sc < 0.05 {
		sc = 1
	}
	if screen == viewPlugin && fg != nil && y >= s.barPx() {
		s.clearGesture()
		if fg.touch != nil {
			fg.touch(x, y-s.barPx(), action)
		}
		return
	}
	if screen == viewDrive && s.driveTouch(x, y, action, sc) {
		return
	}
	s.ensureUI()
	s.kit.Touch(int(float32(x)/sc), int(float32(y)/sc), action)
	s.bump()
}

func (s *Shell) onPrimary() {
	s.mu.Lock()
	v := s.screen
	s.mu.Unlock()
	if v == viewDrive {
		s.setScreen(viewApps)
		return
	}
	s.back()
}

func (s *Shell) openApp(id string) {
	s.mu.Lock()
	a := s.byID[id]
	prev := s.fg
	s.mu.Unlock()
	if a == nil {
		return
	}
	if prev != nil && prev != a && prev.resumed && prev.pause != nil {
		prev.pause()
		prev.resumed = false
	}
	s.mu.Lock()
	s.fg = a
	s.mu.Unlock()
	s.setScreen(viewPlugin)
	s.logf("open %s", id)
}

func (s *Shell) back() {
	s.mu.Lock()
	screen := s.screen
	s.mu.Unlock()
	switch screen {
	case viewApps:
		s.setScreen(viewDrive)
	case viewPlugin:
		s.setScreen(viewApps)
		s.mu.Lock()
		s.fg = nil
		s.mu.Unlock()
	default:
		s.setScreen(viewDrive)
	}
}

func (s *Shell) setScreen(v view) {
	s.mu.Lock()
	prev := s.screen
	fg := s.fg
	s.screen = v
	s.mu.Unlock()
	if prev == viewPlugin && v != viewPlugin && fg != nil && fg.resumed && fg.pause != nil {
		fg.pause()
		fg.resumed = false
	}
	if v != viewDrive {
		keep := fg
		if v != viewPlugin {
			keep = nil
		}
		s.pauseLayers(keep)
		s.clearGesture()
	}
	s.applyChrome()
	s.kit.MarkDirty()
}

// GPSFix is the daemon broadcast. It replaces any client this shell opened.
func (s *Shell) GPSFix(hasPos bool, lat, lon, acc float64, hasSpeed bool, kmh int, hasCourse bool, course float64, err string) {
	fix := gps.Fix{
		HasPos: hasPos, Lat: lat, Lon: lon, AccM: acc,
		HasSpeed: hasSpeed, Kmh: kmh, HasCourse: hasCourse, Course: course, Err: err,
	}
	s.mu.Lock()
	old := s.gpsSrc
	s.gpsSrc = nil
	s.gpsHosted = true
	s.gpsShown = fix
	s.gpsGen++
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}
}

// GPSSat is one satellite lock or drop from the daemon broadcast.
func (s *Shell) GPSSat(gain bool, kicker, title, sub, detail, flag string) {
	s.queueSatNotes([]gps.SatNote{{
		Gain: gain, Kicker: kicker, Title: title, Sub: sub, Detail: detail, Flag: flag,
	}})
}

func (s *Shell) gpsSnapshot() gps.Fix {
	s.mu.Lock()
	if s.gpsHosted {
		f := s.gpsShown
		s.mu.Unlock()
		return f
	}
	if s.gpsSrc == nil {
		s.gpsSrc = openGPS(s.gpsKind)
	}
	g := s.gpsSrc
	s.mu.Unlock()
	if g == nil {
		return gps.Fix{}
	}
	return g.Snapshot()
}

func (s *Shell) applyGPS(now time.Time) {
	fix := s.gpsSnapshot()
	if len(fix.Notes) > 0 {
		s.queueSatNotes(fix.Notes)
	}
	s.noteCity(now, fix)
	s.applyCity(now)
	if s.speedN == nil {
		return
	}
	s.mu.Lock()
	gen := s.gpsGen
	s.mu.Unlock()
	if gen == s.gpsApplied && sameFix(fix, s.gpsShown) {
		return
	}
	s.gpsApplied = gen
	if fix.Err != "" && fix.Err != s.gpsErr {
		s.gpsErr = fix.Err
		s.logf("gps: %s", fix.Err)
	}
	s.gpsShown = fix
	s.renderSpeed(fix)
}

func sameFix(a, b gps.Fix) bool {
	return a.HasSpeed == b.HasSpeed && a.Kmh == b.Kmh && a.Err == b.Err
}

func (s *Shell) setGPSSource(kind string) error {
	kind = gps.Normalize(kind)
	s.mu.Lock()
	same := s.gpsKind == kind && s.gpsSrc != nil
	s.mu.Unlock()
	if same {
		return nil
	}
	if err := patchGPS(userConfigPath(), kind); err != nil {
		s.logf("gps: %v", err)
		return err
	}
	next := openGPS(kind)
	s.mu.Lock()
	old := s.gpsSrc
	s.gpsSrc = next
	s.gpsKind = kind
	s.gpsGen++
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

func (s *Shell) setLogOverlay(on bool) error {
	s.mu.Lock()
	same := s.logOn == on
	s.mu.Unlock()
	if same {
		return nil
	}
	val := "false"
	if on {
		val = "true"
	}
	if err := cfg.OwnSet("neon", "log_overlay", val); err != nil {
		s.logf("settings: %v", err)
		return err
	}
	s.mu.Lock()
	s.logOn = on
	s.mu.Unlock()
	state := "off"
	if on {
		state = "on"
	}
	s.logf("log overlay %s", state)
	return nil
}

// HuInput is a head-unit key. Home returns to the drive, back walks the
// stack, and the media and navigation keys open those apps.
func (s *Shell) HuInput(event string, pressed int) {
	if pressed == 0 || event == "" {
		return
	}
	switch event {
	case "home":
		s.goDrive()
	case "back":
		s.back()
	case "media":
		s.openApp("music")
	case "navigation":
		s.openApp("maps")
	}
}

func (s *Shell) goDrive() {
	s.mu.Lock()
	fg := s.fg
	s.fg = nil
	s.mu.Unlock()
	if fg != nil && fg.resumed && fg.pause != nil {
		fg.pause()
		fg.resumed = false
	}
	s.setScreen(viewDrive)
}

func (s *Shell) Stop() {
	s.mu.Lock()
	fg := s.fg
	s.fg = nil
	s.screen = viewDrive
	src := s.gpsSrc
	s.mu.Unlock()
	if fg != nil && fg.resumed && fg.pause != nil {
		fg.pause()
		fg.resumed = false
	}
	if src != nil {
		src.Close()
	}
	s.mu.Lock()
	if s.gpsSrc == src {
		s.gpsSrc = nil
	}
	s.mu.Unlock()
	s.applyChrome()
	s.logf("stop")
}
