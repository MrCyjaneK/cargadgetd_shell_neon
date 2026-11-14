package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrcyjanek/cargadgetd_sdk/cfg"
)

func TestNeonSettingsABI(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(user, []byte("neon:\n  log_overlay: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prevS, prevU := configSearch, configUser
	configSearch = []string{user}
	configUser = user
	cfg.SetOwnRootForTest(t, dir)
	shell.mu.Lock()
	oldLog := shell.logOn
	oldLayers := append([]layerSpec(nil), shell.layers...)
	shell.logOn = false
	shell.layers = defaultLayers()
	shell.mu.Unlock()
	t.Cleanup(func() {
		shell.mu.Lock()
		shell.logOn = oldLog
		shell.layers = oldLayers
		shell.mu.Unlock()
		configSearch, configUser = prevS, prevU
	})

	if abiNeonCount() != 6 || abiNeonLabel(neonSetLog) != "On-screen log" || abiNeonType(neonSetLog) != 0 {
		t.Fatalf("count %d label %q type %d", abiNeonCount(), abiNeonLabel(neonSetLog), abiNeonType(neonSetLog))
	}
	if abiNeonSetEnum(0, "Mock") == 0 || abiNeonError() == "" {
		t.Fatal("enum accepted")
	}
	if abiNeonSetBool(neonSetLog, 1) != 0 || abiNeonGetBool(neonSetLog) != 1 {
		t.Fatalf("log err %s", abiNeonError())
	}
	body, err := os.ReadFile(user)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "log_overlay: false") {
		t.Fatalf("daemon yaml changed:\n%s", body)
	}
	plug, err := os.ReadFile(filepath.Join(dir, "neon.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plug), "log_overlay: true") && !strings.Contains(string(plug), "log_overlay: \"true\"") {
		t.Fatalf("plugin:\n%s", plug)
	}
}

func TestNeonLayerSettings(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "config.yaml")
	orig := "neon:\n  log_overlay: false\n  layers:\n    - { id: link, role: base }\n"
	if err := os.WriteFile(user, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	plug := t.TempDir()
	for _, id := range []string{"maps", "music", "area", "sats", "hello", "link", "sample", "shells", "settings"} {
		if err := os.WriteFile(filepath.Join(plug, "libcargadgetd_app_"+id+".so"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prevS, prevU := configSearch, configUser
	prevScan := pluginScanDir
	configSearch = []string{user}
	configUser = user
	pluginScanDir = plug
	cfg.SetOwnRootForTest(t, dir)
	shell.mu.Lock()
	oldLayers := append([]layerSpec(nil), shell.layers...)
	shell.layers = defaultLayers()
	shell.mu.Unlock()
	t.Cleanup(func() {
		configSearch, configUser = prevS, prevU
		pluginScanDir = prevScan
		shell.mu.Lock()
		shell.layers = oldLayers
		shell.mu.Unlock()
	})

	if abiNeonCount() != 6 || abiNeonLabel(neonSetMap) != "Map" || abiNeonType(neonSetMap) != 1 {
		t.Fatalf("count %d map %q type %d", abiNeonCount(), abiNeonLabel(neonSetMap), abiNeonType(neonSetMap))
	}
	if abiNeonGetEnum(neonSetMap) != "Maps" || abiNeonGetEnum(neonSetMap+1) != "Music" {
		t.Fatalf("values %s %s", abiNeonGetEnum(neonSetMap), abiNeonGetEnum(neonSetMap+1))
	}
	if abiNeonLabel(neonSetMap+4) != "Overlay 4" || abiNeonGetEnum(neonSetMap+4) != "Off" {
		t.Fatalf("trailing %s %s", abiNeonLabel(neonSetMap+4), abiNeonGetEnum(neonSetMap+4))
	}
	if abiNeonSetEnum(neonSetMap, "Nope") == 0 || abiNeonError() == "" {
		t.Fatal("unknown accepted")
	}
	if abiNeonSetEnum(neonSetMap+4, "Hello") != 0 {
		t.Fatal(abiNeonError())
	}
	if abiNeonCount() != 7 || abiNeonGetEnum(neonSetMap+4) != "Hello" || abiNeonGetEnum(neonSetMap+5) != "Off" {
		t.Fatalf("grew count %d %s %s", abiNeonCount(), abiNeonGetEnum(neonSetMap+4), abiNeonGetEnum(neonSetMap+5))
	}
	if abiNeonSetEnum(neonSetMap+2, "Off") != 0 {
		t.Fatal(abiNeonError())
	}
	if abiNeonGetEnum(neonSetMap+1) != "Music" || abiNeonGetEnum(neonSetMap+2) != "Satellites" || abiNeonGetEnum(neonSetMap+3) != "Hello" {
		t.Fatalf("compact %s %s %s", abiNeonGetEnum(neonSetMap+1), abiNeonGetEnum(neonSetMap+2), abiNeonGetEnum(neonSetMap+3))
	}
	if abiNeonCount() != 6 {
		t.Fatalf("compact count %d", abiNeonCount())
	}
	if abiNeonSetEnum(neonSetMap, "Music") != 0 {
		t.Fatal(abiNeonError())
	}
	if abiNeonGetEnum(neonSetMap) != "Music" || abiNeonGetEnum(neonSetMap+1) != "Satellites" || abiNeonGetEnum(neonSetMap+2) != "Hello" {
		t.Fatalf("moved %s %s %s", abiNeonGetEnum(neonSetMap), abiNeonGetEnum(neonSetMap+1), abiNeonGetEnum(neonSetMap+2))
	}
	body, err := os.ReadFile(user)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != orig {
		t.Fatalf("daemon yaml changed:\n%s", body)
	}
	base, ok := cfg.OwnGet("neon", "layer_base")
	if !ok || base != "music" {
		t.Fatalf("base %q ok %v", base, ok)
	}
	over, ok := cfg.OwnGet("neon", "overlays")
	if !ok || over != "sats,hello" {
		t.Fatalf("overlays %q", over)
	}

	ids := []string{"music", "area", "sats", "hello", "link", "sample", "shells", "extra"}
	eight := []layerSpec{{ID: "maps", Role: layerBase}}
	for _, id := range ids {
		eight = append(eight, layerSpec{ID: id, Role: layerOverlay})
	}
	shell.mu.Lock()
	shell.layers = eight
	shell.mu.Unlock()
	if abiNeonCount() != neonSetMap+1+maxOverlays || abiNeonGetEnum(neonSetMap+maxOverlays) != "extra" {
		t.Fatalf("full count %d last %s", abiNeonCount(), abiNeonGetEnum(neonSetMap+maxOverlays))
	}
	if abiNeonSetEnum(abiNeonCount(), "Settings") == 0 {
		t.Fatal("9th row accepted")
	}
	if _, _, err := assignSlots("maps", ids, neonSetMap+1+len(ids), "settings"); err != errLayerCap {
		t.Fatalf("cap %v", err)
	}
}
