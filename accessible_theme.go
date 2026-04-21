package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

const (
	fontScaleKey    = "io.patenaude.karmamanager.font_scale"
	highContrastKey = "io.patenaude.karmamanager.high_contrast"
)

// Font scale options
const (
	FontScaleSmall      = "Small"
	FontScaleDefault    = "Default"
	FontScaleLarge      = "Large"
	FontScaleExtraLarge = "Extra Large"
)

var fontScaleFactors = map[string]float32{
	FontScaleSmall:      0.85,
	FontScaleDefault:    1.0,
	FontScaleLarge:      1.3,
	FontScaleExtraLarge: 1.6,
}

var fontScaleOptions = []string{FontScaleSmall, FontScaleDefault, FontScaleLarge, FontScaleExtraLarge}

type accessibleTheme struct {
	base      fyne.Theme
	scale     float32
	highContr bool
}

func newAccessibleTheme(scale float32, highContrast bool) *accessibleTheme {
	return &accessibleTheme{
		base:      theme.DefaultTheme(),
		scale:     scale,
		highContr: highContrast,
	}
}

func (t *accessibleTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if !t.highContr {
		return t.base.Color(name, variant)
	}
	switch name {
	case theme.ColorNameForeground:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		}
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	case theme.ColorNameBackground:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
		}
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	case theme.ColorNameButton:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 80, G: 80, B: 80, A: 255}
		}
		return color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	case theme.ColorNameDisabled:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 140, G: 140, B: 140, A: 255}
		}
		return color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	case theme.ColorNamePlaceHolder:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 160, G: 160, B: 160, A: 255}
		}
		return color.NRGBA{R: 90, G: 90, B: 90, A: 255}
	}
	return t.base.Color(name, variant)
}

func (t *accessibleTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *accessibleTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *accessibleTheme) Size(name fyne.ThemeSizeName) float32 {
	base := t.base.Size(name)
	switch name {
	case theme.SizeNameText, theme.SizeNameHeadingText,
		theme.SizeNameSubHeadingText, theme.SizeNameCaptionText:
		return base * t.scale
	case theme.SizeNameInlineIcon:
		return base * t.scale
	case theme.SizeNamePadding, theme.SizeNameInnerPadding, theme.SizeNameLineSpacing:
		// Scale padding proportionally but less aggressively
		return base * (1.0 + (t.scale-1.0)*0.5)
	}
	return base
}
