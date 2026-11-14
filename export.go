package main

/*
#cgo CFLAGS: -I${SRCDIR}/include
#include "cargadgetd/cg_shell.h"
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

static void cg_neon_draw(void *fn, uint32_t *rgba, int w, int h, int stride) {
	if (fn && rgba)
		((cargadgetd_app_draw_fn)fn)(rgba, w, h, stride);
}

static void cg_neon_touch(void *fn, int x, int y, int action) {
	if (fn)
		((cargadgetd_app_touch_fn)fn)(x, y, action);
}

static void cg_neon_ctrl(void *fn) {
	if (fn)
		((cargadgetd_app_ctrl_fn)fn)();
}
*/
import "C"

import (
	"image"
	"time"
	"unsafe"
)

var (
	neonName = C.CString("Neon")
	neonRetC *C.char
	neonErrC *C.char
)

func neonPublish(s string) *C.char {
	if neonRetC != nil {
		C.free(unsafe.Pointer(neonRetC))
		neonRetC = nil
	}
	if s == "" {
		return nil
	}
	neonRetC = C.CString(s)
	return neonRetC
}

func wrapDraw(fn unsafe.Pointer) func(*image.RGBA) {
	if fn == nil {
		return nil
	}
	return func(dst *image.RGBA) {
		if dst == nil || len(dst.Pix) == 0 {
			return
		}
		b := dst.Bounds()
		C.cg_neon_draw(fn, (*C.uint32_t)(unsafe.Pointer(&dst.Pix[0])), C.int(b.Dx()), C.int(b.Dy()), C.int(dst.Stride))
	}
}

func wrapTouch(fn unsafe.Pointer) func(int, int, int) {
	if fn == nil {
		return nil
	}
	return func(x, y, action int) {
		C.cg_neon_touch(fn, C.int(x), C.int(y), C.int(action))
	}
}

func wrapCtrl(fn unsafe.Pointer) func() {
	if fn == nil {
		return nil
	}
	return func() { C.cg_neon_ctrl(fn) }
}

func copyIcon(p *C.uint8_t, w, h, stride int) *image.RGBA {
	if p == nil || w < 1 || h < 1 {
		return nil
	}
	if stride < w*4 {
		stride = w * 4
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(p)), h*stride)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		copy(dst.Pix[y*dst.Stride:y*dst.Stride+w*4], src[y*stride:y*stride+w*4])
	}
	return dst
}

//export cargadgetd_shell_neon_init
func cargadgetd_shell_neon_init(frameMS, w, h C.int) {
	shell.Init(int(frameMS), int(w), int(h))
}

//export cargadgetd_shell_neon_app
func cargadgetd_shell_neon_app(id, name *C.char, icon *C.uint8_t, iconW, iconH, iconStride C.int, draw, touch, pause, resume unsafe.Pointer) {
	shell.Add(&App{
		ID:     C.GoString(id),
		Name:   C.GoString(name),
		Icon:   copyIcon(icon, int(iconW), int(iconH), int(iconStride)),
		draw:   wrapDraw(draw),
		touch:  wrapTouch(touch),
		pause:  wrapCtrl(pause),
		resume: wrapCtrl(resume),
	})
}

//export cargadgetd_shell_neon_notif
func cargadgetd_shell_neon_notif(title, body *C.char, icon *C.uint8_t, iconW, iconH, iconStride C.int) {
	shell.showNote(C.GoString(title), C.GoString(body), copyIcon(icon, int(iconW), int(iconH), int(iconStride)), time.Now())
}

//export cargadgetd_shell_neon_draw
func cargadgetd_shell_neon_draw(rgba *C.uint32_t, w, h, stride C.int, nowMS C.int64_t) {
	if rgba == nil || w < 1 || h < 1 || stride < w*4 {
		return
	}
	pix := unsafe.Slice((*byte)(unsafe.Pointer(rgba)), int(h)*int(stride))
	dst := &image.RGBA{
		Pix:    pix,
		Stride: int(stride),
		Rect:   image.Rect(0, 0, int(w), int(h)),
	}
	shell.Draw(dst, time.UnixMilli(int64(nowMS)))
}

//export cargadgetd_shell_neon_touch
func cargadgetd_shell_neon_touch(x, y, action C.int) {
	shell.Touch(int(x), int(y), int(action))
}

//export cargadgetd_shell_neon_huinput
func cargadgetd_shell_neon_huinput(event *C.char, pressed C.int) {
	shell.HuInput(C.GoString(event), int(pressed))
}

//export cargadgetd_shell_neon_gps_fix
func cargadgetd_shell_neon_gps_fix(hasPos C.int, lat, lon, acc C.double, hasSpeed, kmh, hasCourse C.int, course C.double, err *C.char) {
	shell.GPSFix(hasPos != 0, float64(lat), float64(lon), float64(acc), hasSpeed != 0, int(kmh), hasCourse != 0, float64(course), C.GoString(err))
}

//export cargadgetd_shell_neon_gps_sat
func cargadgetd_shell_neon_gps_sat(gain C.int, kicker, title, sub, detail, flag *C.char) {
	shell.GPSSat(gain != 0, C.GoString(kicker), C.GoString(title), C.GoString(sub), C.GoString(detail), C.GoString(flag))
}

//export cargadgetd_shell_neon_stop
func cargadgetd_shell_neon_stop() {
	shell.Stop()
}

//export cargadgetd_shell_neon_name
func cargadgetd_shell_neon_name() *C.char { return neonName }

//export cargadgetd_shell_neon_settings_count
func cargadgetd_shell_neon_settings_count() C.int { return C.int(neonSettingsCount()) }

//export cargadgetd_shell_neon_settings_label
func cargadgetd_shell_neon_settings_label(i C.int) *C.char {
	return neonPublish(neonSettingsLabel(int(i)))
}

//export cargadgetd_shell_neon_settings_type
func cargadgetd_shell_neon_settings_type(i C.int) C.int { return C.int(neonSettingsType(int(i))) }

//export cargadgetd_shell_neon_settings_enum_values
func cargadgetd_shell_neon_settings_enum_values(i C.int) *C.char {
	return neonPublish(neonSettingsEnums(int(i)))
}

//export cargadgetd_shell_neon_settings_get_bool
func cargadgetd_shell_neon_settings_get_bool(i C.int) C.int {
	return C.int(neonSettingsGetBool(int(i)))
}

//export cargadgetd_shell_neon_settings_get_enum
func cargadgetd_shell_neon_settings_get_enum(i C.int) *C.char {
	return neonPublish(neonSettingsGetEnum(int(i)))
}

//export cargadgetd_shell_neon_settings_get_int
func cargadgetd_shell_neon_settings_get_int(i C.int) C.int {
	return C.int(neonSettingsGetInt(int(i)))
}

//export cargadgetd_shell_neon_settings_set_bool
func cargadgetd_shell_neon_settings_set_bool(i, v C.int) C.int {
	return C.int(neonSettingsSetBool(int(i), int(v)))
}

//export cargadgetd_shell_neon_settings_set_enum
func cargadgetd_shell_neon_settings_set_enum(i C.int, v *C.char) C.int {
	return C.int(neonSettingsSetEnum(int(i), C.GoString(v)))
}

//export cargadgetd_shell_neon_settings_set_int
func cargadgetd_shell_neon_settings_set_int(i, v C.int) C.int {
	return C.int(neonSettingsSetInt(int(i), int(v)))
}

//export cargadgetd_shell_neon_settings_error
func cargadgetd_shell_neon_settings_error() *C.char {
	neonSettingsMu.Lock()
	defer neonSettingsMu.Unlock()
	if neonErrC != nil {
		C.free(unsafe.Pointer(neonErrC))
		neonErrC = nil
	}
	if neonSettingsErr == "" {
		return nil
	}
	neonErrC = C.CString(neonSettingsErr)
	return neonErrC
}

func abiNeonCount() int { return int(cargadgetd_shell_neon_settings_count()) }

func abiNeonLabel(i int) string {
	return C.GoString(cargadgetd_shell_neon_settings_label(C.int(i)))
}

func abiNeonType(i int) int { return int(cargadgetd_shell_neon_settings_type(C.int(i))) }

func abiNeonEnums(i int) string {
	return C.GoString(cargadgetd_shell_neon_settings_enum_values(C.int(i)))
}

func abiNeonGetBool(i int) int { return int(cargadgetd_shell_neon_settings_get_bool(C.int(i))) }

func abiNeonGetEnum(i int) string {
	return C.GoString(cargadgetd_shell_neon_settings_get_enum(C.int(i)))
}

func abiNeonGetInt(i int) int { return int(cargadgetd_shell_neon_settings_get_int(C.int(i))) }

func abiNeonSetBool(i, v int) int {
	return int(cargadgetd_shell_neon_settings_set_bool(C.int(i), C.int(v)))
}

func abiNeonSetEnum(i int, v string) int {
	cs := C.CString(v)
	defer C.free(unsafe.Pointer(cs))
	return int(cargadgetd_shell_neon_settings_set_enum(C.int(i), cs))
}

func abiNeonSetInt(i, v int) int {
	return int(cargadgetd_shell_neon_settings_set_int(C.int(i), C.int(v)))
}

func abiNeonError() string {
	p := cargadgetd_shell_neon_settings_error()
	if p == nil {
		return ""
	}
	return C.GoString(p)
}

func main() {}
