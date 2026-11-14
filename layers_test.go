package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrcyjanek/cargadgetd_sdk/cfg"
)

func solid(c color.RGBA) func(*image.RGBA) {
	return func(dst *image.RGBA) {
		if dst == nil {
			return
		}
		for i := 0; i < len(dst.Pix); i += 4 {
			dst.Pix[i] = c.R
			dst.Pix[i+1] = c.G
			dst.Pix[i+2] = c.B
			dst.Pix[i+3] = c.A
		}
	}
}

func corner(c color.RGBA, n int) func(*image.RGBA) {
	return func(dst *image.RGBA) {
		if dst == nil {
			return
		}
		for i := range dst.Pix {
			dst.Pix[i] = 0
		}
		b := dst.Bounds()
		for y := b.Min.Y; y < b.Min.Y+n && y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Min.X+n && x < b.Max.X; x++ {
				dst.SetRGBA(x, y, c)
			}
		}
	}
}

func TestLayersPaintOrderAndDriveOnly(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.layers = []layerSpec{
		{ID: "ground", Role: layerBase},
		{ID: "low", Role: layerOverlay},
		{ID: "high", Role: layerOverlay},
		{ID: "nope", Role: "float"},
	}
	red := color.RGBA{R: 200, G: 10, B: 10, A: 255}
	blue := color.RGBA{R: 10, G: 20, B: 200, A: 255}
	green := color.RGBA{R: 10, G: 200, B: 40, A: 255}
	var resumed, paused int
	ground := &App{ID: "ground", draw: solid(red), resume: func() { resumed++ }, pause: func() { paused++ }}
	low := &App{ID: "low", draw: corner(blue, 12), resume: func() {}, pause: func() { paused++ }}
	high := &App{ID: "high", draw: corner(green, 4), resume: func() {}, pause: func() {}}
	sh.Add(ground)
	sh.Add(low)
	sh.Add(high)
	sh.Init(33, 800, 480)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	sh.Draw(dst, now)
	if resumed != 1 {
		t.Fatalf("base resume %d", resumed)
	}
	if dst.RGBAAt(1, 1) != green {
		t.Fatalf("top overlay %+v", dst.RGBAAt(1, 1))
	}
	if dst.RGBAAt(8, 1) != blue {
		t.Fatalf("lower overlay %+v", dst.RGBAAt(8, 1))
	}
	bar := sh.barPx()
	if bar < 8 {
		t.Fatalf("bar %d", bar)
	}
	if dst.RGBAAt(40, bar+30) != red {
		t.Fatalf("base under bar %+v", dst.RGBAAt(40, bar+30))
	}
	// The top of the bar, outside the corner overlays, stays the plate.
	if dst.RGBAAt(400, 4).A == 0 {
		t.Fatal("overlay cleared the bar")
	}

	sh.setScreen(viewApps)
	sh.Draw(dst, now)
	if paused < 2 {
		t.Fatalf("paused %d", paused)
	}
	if dst.RGBAAt(40, bar+30) == red {
		t.Fatal("base still painted off the drive")
	}
	if ground.resumed || low.resumed {
		t.Fatal("layer stayed resumed off the drive")
	}
}

func TestLayerAlphaFallsThroughAndGestureLocks(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.layers = []layerSpec{
		{ID: "ground", Role: layerBase},
		{ID: "toast", Role: layerOverlay},
	}
	red := color.RGBA{R: 200, G: 10, B: 10, A: 255}
	blue := color.RGBA{R: 10, G: 20, B: 200, A: 255}
	var got []string
	touch := func(id string) func(int, int, int) {
		return func(x, y, action int) {
			got = append(got, id)
			_ = x
			_ = y
			_ = action
		}
	}
	sh.Add(&App{ID: "ground", draw: solid(red), touch: touch("ground"), resume: func() {}})
	sh.Add(&App{ID: "toast", draw: corner(blue, 16), touch: touch("toast"), resume: func() {}})
	sh.Init(33, 800, 480)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	sh.Draw(dst, now)
	bar := sh.barPx()

	sh.Touch(4, 4, 1)
	sh.Touch(80, bar+40, 2)
	sh.Touch(80, bar+40, 0)
	if len(got) != 3 || got[0] != "toast" || got[1] != "toast" || got[2] != "toast" {
		t.Fatalf("lock %v", got)
	}
	got = nil
	sh.Touch(80, bar+40, 1)
	sh.Touch(4, 4, 2)
	sh.Touch(4, 4, 0)
	if len(got) != 3 || got[0] != "ground" || got[1] != "ground" || got[2] != "ground" {
		t.Fatalf("base %v", got)
	}
}

func TestLayersIgnoreDaemonYAML(t *testing.T) {
	dir := t.TempDir()
	cfg.SetOwnRootForTest(t, dir)
	p := filepath.Join(dir, "config.yaml")
	body := "neon:\n  layers:\n    - { id: music, role: overlay }\n    - { id: maps, role: base }\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := configSearch
	configSearch = []string{p}
	t.Cleanup(func() { configSearch = prev })
	got := loadLayers()
	if len(got) != 4 || got[0].ID != "maps" || got[0].Role != layerBase || got[3].ID != "sats" {
		t.Fatalf("%+v", got)
	}
}

func TestPluginSlotsWinAndCap(t *testing.T) {
	dir := t.TempDir()
	cfg.SetOwnRootForTest(t, dir)
	yamlPath := filepath.Join(dir, "config.yaml")
	var lines []string
	lines = append(lines, "neon:", "  layers:", "    - { id: link, role: base }", "    - { id: maps, role: base }")
	for i := 0; i < 9; i++ {
		lines = append(lines, "    - { id: o"+string(rune('a'+i))+", role: overlay }")
	}
	if err := os.WriteFile(yamlPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := configSearch
	configSearch = []string{yamlPath}
	t.Cleanup(func() { configSearch = prev })

	got := loadLayers()
	if len(got) != 4 || got[0].ID != "maps" || got[3].ID != "sats" {
		t.Fatalf("daemon yaml %+v", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "neon.yaml"), []byte("log_overlay: \"true\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = loadLayers()
	if len(got) != 4 || got[0].ID != "maps" {
		t.Fatalf("log-only file hid defaults %+v", got)
	}

	if err := cfg.OwnSet("neon", "layer_base", "hello"); err != nil {
		t.Fatal(err)
	}
	got = loadLayers()
	if len(got) != 1 || got[0].ID != "hello" || got[0].Role != layerBase {
		t.Fatalf("base only %+v", got)
	}
	if err := cfg.OwnSet("neon", "overlays", "link,area,link"); err != nil {
		t.Fatal(err)
	}
	got = loadLayers()
	if len(got) != 3 || got[0].ID != "hello" || got[1].ID != "link" || got[2].ID != "area" {
		t.Fatalf("plugin %+v", got)
	}
	if err := cfg.OwnSet("neon", "layer_base", ""); err != nil {
		t.Fatal(err)
	}
	got = loadLayers()
	if len(got) != 2 || got[0].ID != "link" || got[0].Role != layerOverlay {
		t.Fatalf("base off %+v", got)
	}
}

func TestLayerChoiceKeepsSavedID(t *testing.T) {
	prev := pluginScanDir
	pluginScanDir = t.TempDir()
	t.Cleanup(func() { pluginScanDir = prev })
	labels, by := layerChoices([]string{"maps", "ghost"})
	if labels[0] != layerOff || by["Maps"] != "maps" || by["ghost"] != "ghost" {
		t.Fatalf("%v %v", labels, by)
	}
}

func TestSetSlotsPausesRemoved(t *testing.T) {
	cfg.SetOwnRootForTest(t, t.TempDir())
	sh := NewShell()
	sh.logOn = false
	var paused bool
	music := &App{ID: "music", resume: func() {}, pause: func() { paused = true }, resumed: true}
	sh.Add(music)
	sh.layers = defaultLayers()
	if err := sh.setSlots("maps", []string{"area", "sats"}); err != nil {
		t.Fatal(err)
	}
	if !paused || sh.layerReady("music") {
		t.Fatalf("paused %v ready %v", paused, sh.layerReady("music"))
	}
	base, overlays := sh.slots()
	if base != "maps" || len(overlays) != 2 || overlays[0] != "area" || overlays[1] != "sats" {
		t.Fatalf("%s %v", base, overlays)
	}
}

func TestOpenLayerUsesPluginRect(t *testing.T) {
	sh := NewShell()
	sh.logOn = false
	sh.layers = []layerSpec{{ID: "maps", Role: layerBase}, {ID: "sats", Role: layerOverlay}}
	var paints int
	maps := &App{
		ID:     "maps",
		Name:   "Maps",
		draw:   func(*image.RGBA) { paints++ },
		touch:  func(int, int, int) {},
		resume: func() {},
		pause:  func() {},
	}
	var satPaused bool
	sats := &App{
		ID:     "sats",
		draw:   solid(color.RGBA{R: 1, G: 2, B: 3, A: 255}),
		resume: func() {},
		pause:  func() { satPaused = true },
	}
	sh.Add(maps)
	sh.Add(sats)
	sh.Init(33, 800, 480)
	dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	sh.Draw(dst, now)
	if paints < 1 || !sats.resumed {
		t.Fatalf("drive paints %d sat %v", paints, sats.resumed)
	}
	sh.openApp("maps")
	sh.Draw(dst, now)
	if sh.screen != viewPlugin {
		t.Fatalf("screen %d", sh.screen)
	}
	if !satPaused || sats.resumed {
		t.Fatal("overlay stayed up off the drive")
	}
	if !maps.resumed {
		t.Fatal("opened layer was paused")
	}
	if dst.RGBAAt(10, 10).R == 1 {
		t.Fatal("overlay painted on the plugin view")
	}
}
