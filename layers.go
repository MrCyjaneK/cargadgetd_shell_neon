package main

import (
	"errors"
	"image"
	"image/draw"
	"os"
	"sort"
	"strings"

	"github.com/mrcyjanek/cargadgetd_sdk/cfg"
)

const (
	layerOff     = "Off"
	maxOverlays  = 8
	pluginDirAbs = "/usr/lib/cargadgetd/plugins"
)

var (
	errLayerRow = errors.New("no such row")
	errLayerCap = errors.New("too many overlays")
)

// pluginScanDir, when set, replaces the packaged plugin directory (tests).
var pluginScanDir string

// appLabels are the shipped plugin names. Anything else is shown as its id.
var appLabels = map[string]string{
	"maps":     "Maps",
	"music":    "Music",
	"area":     "Area",
	"sats":     "Satellites",
	"hello":    "Hello",
	"link":     "Link",
	"sample":   "Sample",
	"shells":   "Shells",
	"settings": "Settings",
}

const (
	layerBase    = "base"
	layerOverlay = "overlay"
)

// layerSpec is one drive layer. List order is bottom to top.
type layerSpec struct {
	ID   string
	Role string
}

type layerHit struct {
	app  *App
	base bool
}

type layerGesture struct {
	on  bool
	hit layerHit
}

func (s *Shell) layerReady(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID[id] == nil {
		return false
	}
	for _, l := range s.layers {
		if l.ID == id {
			return true
		}
	}
	return false
}

func (s *Shell) paintLayers(dst *image.RGBA, role string) {
	if dst == nil {
		return
	}
	s.mu.Lock()
	screen := s.screen
	var apps []*App
	for _, l := range s.layers {
		if l.Role != role {
			continue
		}
		a := s.byID[l.ID]
		if a == nil {
			continue
		}
		apps = append(apps, a)
	}
	s.mu.Unlock()
	if screen != viewDrive {
		return
	}
	b := dst.Bounds()
	bar := 0
	if role == layerBase {
		bar = s.barPx()
	}
	r := image.Rect(b.Min.X, b.Min.Y+bar, b.Max.X, b.Max.Y)
	if r.Dx() < 1 || r.Dy() < 1 {
		return
	}
	for _, a := range apps {
		if a.draw == nil {
			continue
		}
		s.ensureResumed(a)
		fr := a.paint(r.Dx(), r.Dy())
		if fr == nil {
			continue
		}
		draw.Draw(dst, r, fr, fr.Bounds().Min, draw.Over)
	}
}

// pauseLayers pauses every hosted layer except keep. The drive stack is the
// only place those apps are drawn; opening anything else stops them.
func (s *Shell) pauseLayers(keep *App) {
	s.mu.Lock()
	var apps []*App
	for _, l := range s.layers {
		a := s.byID[l.ID]
		if a == nil || a == keep {
			continue
		}
		apps = append(apps, a)
	}
	s.mu.Unlock()
	for _, a := range apps {
		if !a.resumed {
			continue
		}
		if a.pause != nil {
			a.pause()
		}
		a.resumed = false
	}
}

func (s *Shell) clearGesture() {
	s.mu.Lock()
	s.gesture = layerGesture{}
	s.mu.Unlock()
}

// driveTouch routes a drive-view finger. A finger-down locks the layer it
// hit until finger-up, including a drag that slides onto a clear pixel.
// A miss falls through to the bar and then to Fyne.
func (s *Shell) driveTouch(x, y, action int, sc float32) bool {
	s.mu.Lock()
	g := s.gesture
	s.mu.Unlock()
	var tgt layerHit
	if action != 1 && g.on {
		tgt = g.hit
	} else {
		tgt = s.hitLayer(x, y)
		if action == 1 {
			s.mu.Lock()
			s.gesture = layerGesture{on: true, hit: tgt}
			s.mu.Unlock()
		}
	}
	if action == 0 {
		s.clearGesture()
	}
	if tgt.app != nil && tgt.app.touch != nil {
		lx, ly := x, y
		if tgt.base {
			ly -= s.barPx()
		}
		tgt.app.touch(lx, ly, action)
		return true
	}
	s.ensureUI()
	if sc < 0.05 {
		sc = 1
	}
	s.kit.Touch(int(float32(x)/sc), int(float32(y)/sc), action)
	s.bump()
	return true
}

// hitLayer walks overlays from the top. Alpha 0 falls through. The bar is
// next, then bases from the top with y shifted by the bar height.
func (s *Shell) hitLayer(x, y int) layerHit {
	s.mu.Lock()
	layers := append([]layerSpec(nil), s.layers...)
	by := s.byID
	w, h := s.w, s.h
	s.mu.Unlock()
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	bar := s.barPx()
	for i := len(layers) - 1; i >= 0; i-- {
		l := layers[i]
		if l.Role != layerOverlay {
			continue
		}
		a := by[l.ID]
		if a == nil {
			continue
		}
		if opaqueAt(a.buf, x, y, w, h) {
			return layerHit{app: a}
		}
	}
	if y < bar {
		return layerHit{}
	}
	for i := len(layers) - 1; i >= 0; i-- {
		l := layers[i]
		if l.Role != layerBase {
			continue
		}
		a := by[l.ID]
		if a == nil {
			continue
		}
		bw, bh := w, h-bar
		if bh < 1 {
			bh = 1
		}
		if opaqueAt(a.buf, x, y-bar, bw, bh) {
			return layerHit{app: a, base: true}
		}
	}
	return layerHit{}
}

func opaqueAt(img *image.RGBA, x, y, w, h int) bool {
	if img == nil || x < 0 || y < 0 || x >= w || y >= h {
		return false
	}
	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		return false
	}
	return img.RGBAAt(b.Min.X+x, b.Min.Y+y).A != 0
}

func loadLayers() []layerSpec {
	if base, overlays, ok := pluginSlots(); ok {
		return slotsToSpecs(base, overlays)
	}
	return defaultLayers()
}

// pluginSlots reads the neon plugin file. ok is false when neither key has
// been written, so the built-in stack applies.
func pluginSlots() (base string, overlays []string, ok bool) {
	b, bok := cfg.OwnGet("neon", "layer_base")
	o, ook := cfg.OwnGet("neon", "overlays")
	if !bok && !ook {
		return "", nil, false
	}
	if bok {
		base = strings.TrimSpace(b)
	}
	if ook {
		overlays = splitIDs(o)
	}
	return base, overlays, true
}

func splitIDs(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		id := strings.TrimSpace(p)
		if id == "" || seen[id] || len(out) >= maxOverlays {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func slotsToSpecs(base string, overlays []string) []layerSpec {
	var out []layerSpec
	if id := strings.TrimSpace(base); id != "" {
		out = append(out, layerSpec{ID: id, Role: layerBase})
	}
	n := 0
	for _, id := range overlays {
		id = strings.TrimSpace(id)
		if id == "" || id == base || n >= maxOverlays {
			continue
		}
		n++
		out = append(out, layerSpec{ID: id, Role: layerOverlay})
	}
	return out
}

func (s *Shell) slots() (string, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slotsFrom(s.layers)
}

func slotsFrom(layers []layerSpec) (string, []string) {
	base := ""
	var overlays []string
	for _, l := range layers {
		switch l.Role {
		case layerBase:
			if base == "" {
				base = l.ID
			}
		case layerOverlay:
			if len(overlays) < maxOverlays {
				overlays = append(overlays, l.ID)
			}
		}
	}
	return base, overlays
}

// setSlots stores the stack and pauses apps that are no longer in it.
func (s *Shell) setSlots(base string, overlays []string) error {
	base = strings.TrimSpace(base)
	overlays = splitIDs(strings.Join(overlays, ","))
	if err := cfg.OwnSet("neon", "layer_base", base); err != nil {
		s.logf("settings: %v", err)
		return err
	}
	if err := cfg.OwnSet("neon", "overlays", strings.Join(overlays, ",")); err != nil {
		s.logf("settings: %v", err)
		return err
	}
	next := slotsToSpecs(base, overlays)
	s.mu.Lock()
	prev := s.layers
	s.layers = next
	s.gesture = layerGesture{}
	keep := map[string]bool{}
	for _, l := range next {
		keep[l.ID] = true
	}
	var drop []*App
	seen := map[*App]bool{}
	for _, l := range prev {
		if keep[l.ID] {
			continue
		}
		a := s.byID[l.ID]
		if a == nil || seen[a] {
			continue
		}
		seen[a] = true
		drop = append(drop, a)
	}
	s.mu.Unlock()
	for _, a := range drop {
		if !a.resumed {
			continue
		}
		if a.pause != nil {
			a.pause()
		}
		a.resumed = false
	}
	s.bump()
	return nil
}

// assignSlots puts id on the settings row. An empty id clears that row.
// Map is row neonSetMap. Overlay rows follow, and the last one is Off while
// the list is under the cap. An app already on the stack moves.
func assignSlots(base string, overlays []string, row int, id string) (string, []string, error) {
	overlays = append([]string{}, overlays...)
	if row == neonSetMap {
		if id == "" {
			return "", overlays, nil
		}
		return id, dropID(overlays, id), nil
	}
	idx := row - (neonSetMap + 1)
	n := len(overlays)
	if idx == n && n >= maxOverlays {
		return base, overlays, errLayerCap
	}
	rows := n + 1
	if n >= maxOverlays {
		rows = maxOverlays
	}
	if idx < 0 || idx >= rows {
		return base, overlays, errLayerRow
	}
	if id == "" {
		if idx >= n {
			return base, overlays, nil
		}
		next := append([]string{}, overlays[:idx]...)
		return base, append(next, overlays[idx+1:]...), nil
	}
	if idx < n && overlays[idx] == id {
		if base == id {
			base = ""
		}
		return base, overlays, nil
	}
	trailing := idx == n
	if base == id {
		base = ""
	}
	removedBefore := 0
	kept := make([]string, 0, len(overlays))
	for i, cur := range overlays {
		if cur == id {
			if i < idx {
				removedBefore++
			}
			continue
		}
		kept = append(kept, cur)
	}
	if trailing {
		if len(kept) >= maxOverlays {
			return base, overlays, errLayerCap
		}
		return base, append(kept, id), nil
	}
	idx -= removedBefore
	if idx < 0 {
		idx = 0
	}
	if idx >= len(kept) {
		if len(kept) >= maxOverlays {
			return base, overlays, errLayerCap
		}
		return base, append(kept, id), nil
	}
	kept[idx] = id
	return base, kept, nil
}

func dropID(overlays []string, id string) []string {
	out := make([]string, 0, len(overlays))
	for _, cur := range overlays {
		if cur != id {
			out = append(out, cur)
		}
	}
	return out
}

func installedAppIDs() []string {
	dir := pluginScanDir
	if dir == "" {
		dir = pluginDirAbs
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := appIDFromBase(e.Name())
		if ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func appIDFromBase(name string) (string, bool) {
	const prefix = "libcargadgetd_app_"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".so") {
		return "", false
	}
	id := strings.TrimSuffix(name[len(prefix):], ".so")
	if !validLayerID(id) {
		return "", false
	}
	return id, true
}

func validLayerID(id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if i == 0 {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_') {
				return false
			}
			continue
		}
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// layerChoices is Off plus one label per installed app. extra ids stay on
// the list when their plugin file is gone. The map is label to id.
func layerChoices(extra []string) ([]string, map[string]string) {
	ids := installedAppIDs()
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	for _, id := range extra {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] || strings.Contains(id, ",") {
			continue
		}
		ids = append(ids, id)
		seen[id] = true
	}
	type pair struct{ id, label string }
	var pairs []pair
	tally := map[string]int{}
	for _, id := range ids {
		lab := appLabels[id]
		if lab == "" || lab == layerOff || strings.Contains(lab, ",") {
			lab = id
		}
		if lab == layerOff || strings.Contains(lab, ",") {
			continue
		}
		pairs = append(pairs, pair{id, lab})
		tally[lab]++
	}
	for i := range pairs {
		if tally[pairs[i].label] > 1 {
			pairs[i].label = pairs[i].id
		}
	}
	tally = map[string]int{}
	for _, p := range pairs {
		tally[p.label]++
	}
	var clean []pair
	for _, p := range pairs {
		if tally[p.label] > 1 || p.label == layerOff || strings.Contains(p.label, ",") {
			continue
		}
		clean = append(clean, p)
	}
	sort.Slice(clean, func(i, j int) bool {
		if clean[i].label == clean[j].label {
			return clean[i].id < clean[j].id
		}
		return clean[i].label < clean[j].label
	})
	labels := []string{layerOff}
	by := map[string]string{layerOff: ""}
	for _, p := range clean {
		labels = append(labels, p.label)
		by[p.label] = p.id
	}
	return labels, by
}

func labelForID(id string, by map[string]string) string {
	if id == "" {
		return layerOff
	}
	for lab, got := range by {
		if lab != layerOff && got == id {
			return lab
		}
	}
	return id
}

func defaultLayers() []layerSpec {
	return []layerSpec{
		{ID: "maps", Role: layerBase},
		{ID: "music", Role: layerOverlay},
		{ID: "area", Role: layerOverlay},
		{ID: "sats", Role: layerOverlay},
	}
}
