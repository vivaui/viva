package ui

import "github.com/vivaui/viva/base"

// The words of package gfx a page uses, so a page imports ui alone. They are
// package base's names, the behavior layer's vocabulary; ui re-exports them as
// the permanent application facade.
type Color = base.Color
type Rect = base.Rect
type Size = base.Size
type Point = base.Point
type Theme = base.Theme

// LightTheme is the light theme.
func LightTheme() Theme { return base.LightTheme() }

// DarkTheme is the dark theme.
func DarkTheme() Theme { return base.DarkTheme() }

// RGB is an opaque color from 8-bit channels.
func RGB(red, green, blue uint8) Color { return base.RGB(red, green, blue) }

// RGBA is a color from 8-bit channels and an opacity from 0 to 1.
func RGBA(red, green, blue uint8, alpha float32) Color { return base.RGBA(red, green, blue, alpha) }
