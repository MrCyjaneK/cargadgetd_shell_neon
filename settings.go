package main

import (
	"fmt"
	"strings"
	"sync"
)

const (
	neonSetLog = iota
	neonSetMap
)

var (
	neonSettingsMu  sync.Mutex
	neonSettingsErr string
)

func neonSettingsCount() int {
	_, overlays := shell.slots()
	n := len(overlays)
	if n < maxOverlays {
		n++
	}
	return neonSetMap + 1 + n
}

func neonSettingsLabel(i int) string {
	switch {
	case i == neonSetLog:
		return "On-screen log"
	case i == neonSetMap:
		return "Map"
	case i > neonSetMap && i < neonSettingsCount():
		return fmt.Sprintf("Overlay %d", i-neonSetMap)
	default:
		return ""
	}
}

func neonSettingsType(i int) int {
	if i == neonSetLog {
		return 0
	}
	if i > neonSetLog && i < neonSettingsCount() {
		return 1
	}
	return -1
}

func neonSettingsEnums(i int) string {
	if neonSettingsType(i) != 1 {
		return ""
	}
	labels, _ := layerChoices(layerExtra())
	return strings.Join(labels, ",")
}

func layerExtra() []string {
	base, overlays := shell.slots()
	extra := append([]string{}, overlays...)
	if base != "" {
		extra = append(extra, base)
	}
	return extra
}

func neonSettingsGetBool(i int) int {
	if i != neonSetLog {
		return 0
	}
	shell.mu.Lock()
	on := shell.logOn
	shell.mu.Unlock()
	if on {
		return 1
	}
	return 0
}

func neonSettingsGetEnum(i int) string {
	if neonSettingsType(i) != 1 {
		return ""
	}
	base, overlays := shell.slots()
	id := ""
	if i == neonSetMap {
		id = base
	} else {
		idx := i - (neonSetMap + 1)
		if idx >= 0 && idx < len(overlays) {
			id = overlays[idx]
		}
	}
	_, by := layerChoices(layerExtra())
	return labelForID(id, by)
}

func neonSettingsGetInt(i int) int { return 0 }

func neonSettingsSetBool(i, v int) int {
	neonClearErr()
	if i != neonSetLog {
		return neonFail("not a bool")
	}
	if err := shell.setLogOverlay(v != 0); err != nil {
		return neonFail(err.Error())
	}
	return 0
}

func neonSettingsSetEnum(i int, v string) int {
	neonClearErr()
	if neonSettingsType(i) != 1 {
		return neonFail("not an enum")
	}
	labels, by := layerChoices(layerExtra())
	if !enumHas(labels, v) {
		return neonFail("unknown value")
	}
	id := ""
	if v != layerOff {
		var ok bool
		id, ok = by[v]
		if !ok || id == "" {
			return neonFail("unknown value")
		}
	}
	base, overlays := shell.slots()
	nextBase, nextOver, err := assignSlots(base, overlays, i, id)
	if err != nil {
		return neonFail(err.Error())
	}
	if err := shell.setSlots(nextBase, nextOver); err != nil {
		return neonFail(err.Error())
	}
	return 0
}

func enumHas(labels []string, v string) bool {
	for _, lab := range labels {
		if lab == v {
			return true
		}
	}
	return false
}

func neonSettingsSetInt(i, v int) int {
	neonClearErr()
	return neonFail("not an int")
}

func neonSettingsError() string {
	neonSettingsMu.Lock()
	defer neonSettingsMu.Unlock()
	return neonSettingsErr
}

func neonClearErr() {
	neonSettingsMu.Lock()
	neonSettingsErr = ""
	neonSettingsMu.Unlock()
}

func neonFail(msg string) int {
	neonSettingsMu.Lock()
	neonSettingsErr = msg
	neonSettingsMu.Unlock()
	return 1
}
