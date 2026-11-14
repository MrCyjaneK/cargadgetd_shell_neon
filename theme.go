package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

var (
	colBG     = color.NRGBA{R: 8, G: 10, B: 12, A: 255}
	colCard   = color.NRGBA{R: 16, G: 20, B: 24, A: 255}
	colLime   = color.NRGBA{R: 212, G: 255, B: 0, A: 255}
	colFG     = color.NRGBA{R: 232, G: 236, B: 240, A: 255}
	colMuted  = color.NRGBA{R: 148, G: 156, B: 164, A: 255}
	colLog    = color.NRGBA{R: 160, G: 255, B: 48, A: 255}
	colBanner = color.NRGBA{R: 14, G: 40, B: 36, A: 255}
	colPin    = color.NRGBA{R: 70, G: 220, B: 230, A: 255}
	colInk    = color.NRGBA{R: 10, G: 14, B: 10, A: 255}
	// Plates over the map. The bar is the darkest; the entering card is
	// nearly as solid so the name reads.
	colTopBar = color.NRGBA{R: 4, G: 8, B: 10, A: 220}
	colAreaBG = color.NRGBA{R: 6, G: 10, B: 12, A: 210}
)

// designH is the logical canvas height. The software canvas scale is
// framebufferHeight/designH, so theme sizes and Fyne layout are in this space
// on every head unit.
const designH = 480

var darkBase fyne.Theme

func baseTheme() fyne.Theme {
	if darkBase == nil {
		darkBase = theme.DarkTheme()
	}
	return darkBase
}

type neonTheme struct{}

func (neonTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return colBG
	case theme.ColorNameButton:
		return colCard
	case theme.ColorNameForeground:
		return colFG
	case theme.ColorNamePrimary:
		return colLime
	case theme.ColorNameHover:
		return color.NRGBA{R: 36, G: 44, B: 32, A: 255}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 180, G: 220, B: 0, A: 255}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 90, G: 96, B: 90, A: 255}
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNameInputBackground:
		return colCard
	case theme.ColorNameFocus, theme.ColorNameSelection:
		return colLime
	default:
		return baseTheme().Color(n, v)
	}
}

func (neonTheme) Font(style fyne.TextStyle) fyne.Resource {
	return interResource(style.Bold || style.Monospace)
}

func (neonTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return baseTheme().Icon(n)
}

func (neonTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 16
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 18
	case theme.SizeNameCaptionText:
		return 13
	case theme.SizeNamePadding:
		return 10
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameInlineIcon:
		return 22
	default:
		return baseTheme().Size(n)
	}
}
