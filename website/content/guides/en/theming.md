---
title: Theming and fonts
summary: Light and dark themes, overriding tokens, control density, reduced motion and fixed-pitch fonts.
order: 8
group: guides
---

## Themes are values

A `ui.Theme` is a struct of semantic color tokens and size metrics. There is no stylesheet and nothing is parsed at run time. Every component takes the theme you give it as an argument, so changing the appearance means painting with a different value:

<!-- example: theming/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = ui.Tree{}
	page = tree.Feature("theming")

	toggle = tree.ID(page, "toggle")
)

type model struct {
	dark bool
	view flex.View
}

// theme is the active theme: a value, so changing appearance is changing which
// value Paint reads.
func (m *model) theme() ui.Theme {
	theme := ui.LightTheme()
	if m.dark {
		theme = ui.DarkTheme()
	}
	// Tokens are plain fields. Override one to brand the application.
	theme.Primary = ui.Color{Red: 0.9, Green: 0.45, Blue: 0.1, Alpha: 1}
	theme.PrimaryForeground = ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	return theme
}

func newConfig(m *model) app.Config {
	config := app.Config{Tree: &tree, Title: "Theme", Width: 480, Height: 240}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		theme := m.theme()
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Column().Fill().Pad(24).Gap(12).Items(flex.Start).Children(func() {
				if f.Button(toggle, "Toggle theme").Clicked() {
					m.dark = !m.dark
				}
				f.Text("Hello")
			})
		})
	}
	return config
}

func main() {
	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

`ui.LightTheme()` and `ui.DarkTheme()` return complete themes. The tokens are plain fields: `Background` and `Foreground` for the page, `Card` and `CardForeground` for surfaces, `Popover` for menus and dialogs, `Primary`, `Secondary`, `Accent` and `Muted` with matching foregrounds, `Selection` and `SelectionForeground` for text selection, `Border`, `Input`, `Ring` for focus rings, `Success` and `Danger`, and five `Chart` colors. Override a field to brand the application, as the example does with `Primary`.

Keep one function that returns the active theme, like the `theme` method above, and read it at the top of `Paint`. Nothing else needs to know which theme is active.

## Following the system

Viva does not currently expose the operating system's light or dark setting, and it never picks a theme for you. The application stores the user's choice in its own state, as the example's toggle does. The one appearance preference the runtime does report is reduced motion.

## Density

`Theme.Metrics` carries the size tokens components consume: control height, corner radii, font sizes and padding. `Metrics.WithDensity(ui.DensityCompact)` (or `DensityLarge`) derives a set at another density without touching the colors:

```go
theme := ui.DarkTheme()
theme.Metrics = theme.Metrics.WithDensity(ui.DensityCompact)
```

Components read geometry from the metrics rather than hardcoding it, so every control in the window follows.

## Motion

`Theme.Transition` is the default state-change motion for controls; a zero value disables animation, which is what a hand-built theme gets. `app.ReduceMotion()` reports the operating system's reduced-motion preference on macOS, Windows and Linux and `false` elsewhere. `Canvas.Animate` already honors it, returning the target value unchanged, so components that animate through the canvas need no extra handling.

## Text

`ui.TextStyle` takes a `Size`, a `Weight` and a `Color`. Text is drawn in the system font, so each platform shows its own face and CJK text falls back to the system's CJK font without any setup. Viva does not bundle fonts.

For code and terminals, set `Mono: true`. By default that is the system's fixed-pitch font. `MonoFace` selects another: `ui.MonoClassic` asks for a traditional terminal face, and `ui.MonoFamily(name)` selects any installed family. `app.MonospaceFamilies()` lists them for a settings page; it reads the system each time and can take tens of milliseconds, so call it when the page opens, not while painting. Every ASCII character in a fixed-pitch line advances by `Canvas.CellSize(size)`, which is what a character grid needs.
