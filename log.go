package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"strings"
	"time"
)

const (
	logHold     = 10 * time.Second
	notifyHold  = 8 * time.Second
	maxLogLines = 5
)

type logLine struct {
	text string
	at   time.Time
}

func (s *Shell) logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("neon: %s", msg)
	s.addLog(msg, time.Now())
}

func (s *Shell) addLog(text string, at time.Time) {
	text = strings.TrimSpace(text)
	if text == "" || s == nil {
		return
	}
	if len(text) > 160 {
		text = text[:160]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, logLine{text: text, at: at})
	if len(s.logs) > maxLogLines {
		s.logs = append([]logLine(nil), s.logs[len(s.logs)-maxLogLines:]...)
	}
}

func (s *Shell) expireLocked(now time.Time) {
	cut := now.Add(-logHold)
	kept := s.logs[:0]
	for _, ln := range s.logs {
		if ln.at.After(cut) {
			kept = append(kept, ln)
		}
	}
	s.logs = kept
	if s.note != nil && !now.Before(s.note.until) {
		s.note = nil
	}
}

// paintLogs draws the shell's own lines. It is applied after the Fyne capture
// so a log line does not force a full widget raster.
func (s *Shell) paintLogs(dst *image.RGBA) {
	s.mu.Lock()
	on := s.logOn
	lines := append([]logLine(nil), s.logs...)
	s.mu.Unlock()
	if !on || len(lines) == 0 || dst == nil {
		return
	}
	b := dst.Bounds()
	const lineH = 14
	top := b.Max.Y - lineH*len(lines) - 6
	if top < b.Min.Y {
		top = b.Min.Y
	}
	shadeRect(dst, image.Rect(b.Min.X, top, b.Max.X, b.Max.Y), color.NRGBA{A: 170})
	for i, ln := range lines {
		drawString(dst, ln.text, b.Min.X+8, top+4+i*lineH, 1, colLog, false)
	}
}
