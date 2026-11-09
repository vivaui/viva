package ui

import "github.com/vivaui/viva/gfx"

// The words of package gfx a page uses, so a page imports ui alone. Each is the gfx
// name itself, not a copy: a ui.Color is a gfx.Color. Painting, paths, input events,
// keys and identities stay in gfx; reaching for them is writing code below a page.

// Color is an RGBA color.
type Color = gfx.Color

// Rect is a rectangle in logical pixels: a position and a size.
type Rect = gfx.Rect

// Size is a width and a height in logical pixels.
type Size = gfx.Size

// Point is a position in logical pixels.
type Point = gfx.Point

// Theme is the colors and metrics a page is painted with.
type Theme = gfx.Theme

// LightTheme is the light theme.
func LightTheme() Theme { return gfx.LightTheme() }

// DarkTheme is the dark theme.
func DarkTheme() Theme { return gfx.DarkTheme() }

// RGB is an opaque color from 8-bit channels.
func RGB(red, green, blue uint8) Color { return gfx.RGB(red, green, blue) }

// RGBA is a color from 8-bit channels and an opacity from 0 to 1.
func RGBA(red, green, blue uint8, alpha float32) Color { return gfx.RGBA(red, green, blue, alpha) }
