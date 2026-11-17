package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type amoledTheme struct{}

var _ fyne.Theme = (*amoledTheme)(nil)

// NewAMOLEDTheme cria o tema preto puro.
func NewAMOLEDTheme() fyne.Theme {
	return &amoledTheme{}
}

func (t *amoledTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255} // Preto puro (#000000)
	case theme.ColorNameButton, theme.ColorNameInputBackground:
		return color.NRGBA{R: 18, G: 18, B: 18, A: 255} // #121212
	case theme.ColorNameForeground:
		return color.White
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 88, G: 101, B: 242, A: 255} // #5865F2
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 150, G: 150, B: 150, A: 255}
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (t *amoledTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *amoledTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *amoledTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}