---
title: Theming and fonts
summary: Light and dark themes, overriding tokens, control density, reduced motion and fixed-pitch fonts.
order: 8
group: guides
---

## Themes are values

A `gfx.Theme` is a struct of semantic color tokens and size metrics. There is no stylesheet and nothing is parsed at run time. Every component takes the theme you give it as an argument, so changing the appearance means painting with a different value:

<!-- example: theming/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type model struct {
	dark  bool
	theme gfx.Theme
}

// applyTheme recomputes the active theme: a value, so changing appearance is
// changing the value the page paints with.
func (m *model) applyTheme() {
	theme := gfx.LightTheme()
	if m.dark {
		theme = gfx.DarkTheme()
	}
	// Tokens are plain fields. Override one to brand the application.
	theme.Primary = gfx.Color{Red: 0.9, Green: 0.45, Blue: 0.1, Alpha: 1}
	theme.PrimaryForeground = gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	m.theme = theme
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		if ui.Button(c, "Toggle theme").Clicked() {
			m.dark = !m.dark
			m.applyTheme()
		}
		ui.Text(c, "Hello")
	})
}

func newConfig(m *model) app.Config {
	m.applyTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &m.theme})
	return page.Config(app.Config{Title: "Theme", Width: 480, Height: 240})
}

func main() {
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

`gfx.LightTheme()` and `gfx.DarkTheme()` return complete themes. The tokens are plain fields: `Background` and `Foreground` for the page, `Card` and `CardForeground` for surfaces, `Popover` for menus and dialogs, `Primary`, `Secondary`, `Accent` and `Muted` with matching foregrounds, `Selection` and `SelectionForeground` for text selection, `Border`, `Input`, `Ring` for focus rings, `Success` and `Danger`, and five `Chart` colors. Override a field to brand the application, as the example does with `Primary`.

Keep one function that returns the active theme, like the `theme` method above, and read it at the top of `Paint`. Nothing else needs to know which theme is active.

## Following the system

Viva does not currently expose the operating system's light or dark setting, and it never picks a theme for you. The application stores the user's choice in its own state, as the example's toggle does. The one appearance preference the runtime does report is reduced motion.

## Density

`Theme.Metrics` carries the size tokens components consume: control height, corner radii, font sizes and padding. `Metrics.WithDensity(gfx.DensityCompact)` (or `DensityLarge`) derives a set at another density without touching the colors:

```go
theme := gfx.DarkTheme()
theme.Metrics = theme.Metrics.WithDensity(gfx.DensityCompact)
```

Components read geometry from the metrics rather than hardcoding it, so every control in the window follows.

## One control with its own radius or font size

Density changes every control in the window. To change one control, give that control a theme of its own: a theme is a value, so the copy affects only the call it is passed to.

```go
// A pill-shaped search field with larger text; the fields around it are untouched.
components.PaintInput(canvas, theme.WithRadius(gfx.RadiusFull).WithFontSize(16), bounds, snapshot)
```

`WithRadius` sets the corner radius of controls (`Metrics.RadiusControl`; what is derived from it, such as the rows of a menu, follows, while cards and dialogs keep `RadiusSurface`). `WithFontSize` sets the body size and keeps the small and large steps in proportion. `WithControlHeight` sets `Metrics.ControlHeight`, which the height of a button and the geometry scaled with it follow. `gfx.RadiusFull` is a radius larger than any control, which rounds to a pill. The caret, selection and composition underline of a text field follow its font size, and the text starts where the density's padding puts it (`Metrics.TextInsetX`).

In a page the same two overrides are `Radius` and `FontSize` on the control's element. The control is measured and painted with them and its neighbours are not affected:

```go
ui.TextField(c, &m.search, "Search", "").Radius(gfx.RadiusFull).FontSize(16).Height(40)
ui.Select(c, &m.size, sizes, nil).FontSize(15)
```

`Height` still decides how tall a field is; the font size does not resize it.

## Motion

`Theme.Transition` is the default state-change motion for controls; a zero value disables animation, which is what a hand-built theme gets. `app.ReduceMotion()` reports the operating system's reduced-motion preference on macOS, Windows and Linux and `false` elsewhere. `Canvas.Animate` already honors it, returning the target value unchanged, so components that animate through the canvas need no extra handling.

`Theme.TooltipDelay` is how long the pointer rests on a control before a `ui.Tooltip` appears; zero, the default and what a hand-built theme gets, shows it with the pointer. Half a second reads like a desktop.

## Text

`gfx.TextStyle` takes a `Size`, a `Weight` and a `Color`. Text is drawn in the system font, so each platform shows its own face and CJK text falls back to the system's CJK font without any setup. Viva does not bundle fonts.

For code and terminals, set `Mono: true`. By default that is the system's fixed-pitch font. `MonoFace` selects another: `gfx.MonoClassic` asks for a traditional terminal face, and `gfx.MonoFamily(name)` selects any installed family. `app.MonospaceFamilies()` lists them for a settings page; it reads the system each time and can take tens of milliseconds, so call it when the page opens, not while painting. A fixed-pitch `Label` is set on a character grid: every character stands on a cell `Canvas.MonoCellSize(size, face)` wide, a wide character, such as a CJK ideograph, on two, and a combining mark on the cell of the character it follows. `Canvas.TextWidthStyle` counts the same cells, so the line ends where it was measured to end and its columns line up with a terminal row drawn with `LabelRuns` in the same face. In an installed family (`gfx.MonoFamily`) each stretch of ASCII is shaped as one line and fitted to its cells, so the family's programming ligatures still appear; the built-in faces have none and draw one glyph on each cell.

An application can carry its own fonts. `app.RegisterFont(data)` takes the bytes of a `.ttf`, `.otf` or `.ttc` (embed them with `//go:embed`), makes the font available to this process without installing it, and returns the family it names, which goes straight to `gfx.MonoFamily` or `app.SetInterfaceFontFamily`. Each weight or style of a family is its own file and its own call. Register before the first paint that uses the family; the function it returns takes the font away again. It works on macOS, Windows (10 version 1607 or later) and Linux, and returns `app.ErrUnsupported` elsewhere.
