package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// SSMSDarkTheme provides a dark palette inspired by SQL Server Management Studio
type SSMSDarkTheme struct{}

var _ fyne.Theme = (*SSMSDarkTheme)(nil)

func (t *SSMSDarkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x1e, G: 0x1e, B: 0x2e, A: 0xff} // Catppuccin Base / SSMS Slate
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x18, G: 0x18, B: 0x25, A: 0xff} // Editor Dark
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x24, G: 0x27, B: 0x3a, A: 0xff}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x31, G: 0x32, B: 0x44, A: 0xff}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x89, G: 0xb4, B: 0xfa, A: 0xff} // SQL Blue
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xcd, G: 0xd6, B: 0xf4, A: 0xff} // Text Light
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x6c, G: 0x70, B: 0x86, A: 0xff}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x45, G: 0x47, B: 0x5a, A: 0xff}
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0xa6, G: 0xe3, B: 0xa1, A: 0xff} // Green
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xf9, G: 0xe2, B: 0xaf, A: 0xff} // Yellow
	case theme.ColorNameError:
		return color.NRGBA{R: 0xf3, G: 0x8b, B: 0xa8, A: 0xff} // Red
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0x45, G: 0x47, B: 0x5a, A: 0xff}
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (t *SSMSDarkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *SSMSDarkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *SSMSDarkTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
