package app

import (
	"math"
	"os"
	"runtime"
	"strings"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// WindowControlWidth is the width in logical pixels of each of the minimize,
// maximize and close buttons Viva draws in a custom titlebar. They are as tall as
// Config.TitlebarHeight.
const WindowControlWidth = platform.WindowControlWidth

// windowControlsLayer is above every layer an application uses, so the buttons
// stay in view and clickable under a dialog: a window can be closed or minimized
// whatever it is showing.
const windowControlsLayer = math.MaxUint16

// The buttons, in the order they are painted. Their identities are names in a
// feature of their own in the canvas's identity tree, so no application
// identity can be one of them, and its Clicked never sees them.
const (
	windowControlMinimize = iota
	windowControlMaximize
	windowControlClose
)

type desktopTheme uint8

const (
	desktopDefault desktopTheme = iota
	desktopBreeze
	desktopAdwaita
)

func detectDesktopTheme() desktopTheme {
	if runtime.GOOS != "linux" {
		return desktopDefault
	}
	desktop := strings.ToUpper(os.Getenv("XDG_CURRENT_DESKTOP"))
	if strings.Contains(desktop, "KDE") || strings.Contains(desktop, "PLASMA") {
		return desktopBreeze
	}
	if strings.Contains(desktop, "GNOME") || strings.Contains(desktop, "UNITY") || strings.Contains(desktop, "PANTHEON") {
		return desktopAdwaita
	}
	return desktopDefault
}

// WindowControlsStyle is how the window buttons look. A zero field takes a
// default that suits Config.ClearColor.
type WindowControlsStyle struct {
	// Foreground is the glyph's color, Hover the button's background under the
	// pointer, and Pressed while it is held down.
	Foreground, Hover, Pressed ui.Color
	// Close is the close button's background under the pointer, and
	// CloseForeground its glyph then. Pressed fades it.
	Close, CloseForeground ui.Color
	// Names are what assistive technology calls the buttons. They default to
	// English; an application that is translated sets them.
	Minimize, Maximize, Restore, CloseName string
}

// withDefaults fills what style leaves out for a window cleared to clear.
func (style WindowControlsStyle) withDefaults(clear Color) WindowControlsStyle {
	luminance := 0.2126*clear.Red + 0.7152*clear.Green + 0.0722*clear.Blue
	ink := ui.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}
	if luminance < 0.5 {
		ink = ui.Color{Red: 0.95, Green: 0.95, Blue: 0.95, Alpha: 1}
	}
	if style.Foreground == (ui.Color{}) {
		style.Foreground = ink
	}
	if style.Hover == (ui.Color{}) {
		style.Hover = ui.Color{Red: ink.Red, Green: ink.Green, Blue: ink.Blue, Alpha: 0.10}
	}
	if style.Pressed == (ui.Color{}) {
		style.Pressed = ui.Color{Red: ink.Red, Green: ink.Green, Blue: ink.Blue, Alpha: 0.18}
	}
	if style.Close == (ui.Color{}) {
		style.Close = ui.Color{Red: 0.77, Green: 0.17, Blue: 0.11, Alpha: 1}
	}
	if style.CloseForeground == (ui.Color{}) {
		style.CloseForeground = ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	}
	if style.Minimize == "" {
		style.Minimize = "Minimize"
	}
	if style.Maximize == "" {
		style.Maximize = "Maximize"
	}
	if style.Restore == "" {
		style.Restore = "Restore"
	}
	if style.CloseName == "" {
		style.CloseName = "Close"
	}
	return style
}

// The glyphs are ten units square and stroked one unit wide, which is one
// device pixel at 1x and two at 2x.
const glyph = 10

var (
	minimizeIcon = ui.Icon{ViewBox: glyph, Stroke: 1, Cap: ui.CapButt, Join: ui.JoinMiter, Commands: []ui.PathCommand{
		ui.MoveTo(0, 5), ui.LineTo(glyph, 5),
	}}
	maximizeIcon = ui.Icon{ViewBox: glyph, Stroke: 1, Cap: ui.CapButt, Join: ui.JoinMiter, Commands: []ui.PathCommand{
		ui.MoveTo(0.5, 0.5), ui.LineTo(9.5, 0.5), ui.LineTo(9.5, 9.5), ui.LineTo(0.5, 9.5), ui.ClosePath(),
	}}
	restoreIcon = ui.Icon{ViewBox: glyph, Stroke: 1, Cap: ui.CapButt, Join: ui.JoinMiter, Commands: []ui.PathCommand{
		// The window behind: its top and right edges, clear of the one in front.
		ui.MoveTo(2.5, 2.5), ui.LineTo(2.5, 0.5), ui.LineTo(9.5, 0.5), ui.LineTo(9.5, 7.5), ui.LineTo(7.5, 7.5),
		// The window in front.
		ui.MoveTo(0.5, 2.5), ui.LineTo(7.5, 2.5), ui.LineTo(7.5, 9.5), ui.LineTo(0.5, 9.5), ui.ClosePath(),
	}}
	closeIcon = ui.Icon{ViewBox: glyph, Stroke: 1, Cap: ui.CapButt, Join: ui.JoinMiter, Commands: []ui.PathCommand{
		ui.MoveTo(0, 0), ui.LineTo(glyph, glyph), ui.MoveTo(glyph, 0), ui.LineTo(0, glyph),
	}}
)

// windowControls paints the buttons and carries out their clicks. One belongs to
// a window whose system draws no controls for it.
type windowControls struct {
	style   func() WindowControlsStyle
	clear   Color
	height  float32
	actions *platform.WindowActions
	// tree and ids are the buttons' identities in the tree they were made in.
	tree *ui.Tree
	ids  [3]ui.ID
}

func newWindowControls(config *Config) *windowControls {
	return &windowControls{style: config.WindowControls, clear: config.ClearColor, height: platform.Config{TitlebarHeight: config.TitlebarHeight}.TitlebarStrip()}
}

// maximized reports whether the window fills its screen, which has no edges to
// resize from.
func (controls *windowControls) maximized() bool {
	return controls.actions != nil && controls.actions.Maximized != nil && controls.actions.Maximized()
}

// identify names the buttons in tree. It does nothing after the first time for
// one tree.
func (controls *windowControls) identify(tree *ui.Tree) {
	if controls.tree == tree {
		return
	}
	scope := tree.Feature("viva.window-controls")
	controls.ids = [3]ui.ID{
		scope.ID(tree.Name("minimize")), scope.ID(tree.Name("maximize")), scope.ID(tree.Name("close")),
	}
	controls.tree = tree
}

// control reports which button id is, and false for any other identity.
func (controls *windowControls) control(id ui.ID) (int, bool) {
	for index, candidate := range controls.ids {
		if candidate == id && !id.IsZero() {
			return index, true
		}
	}
	return 0, false
}

// paint draws the three buttons at the trailing end of the strip and declares
// their hits. It runs after the application's own paint.
func (controls *windowControls) paint(canvas *ui.Canvas, size ui.Size) {
	style := WindowControlsStyle{}
	if controls.style != nil {
		style = controls.style()
	}
	style = style.withDefaults(controls.clear)
	maximized := controls.maximized()
	controls.identify(canvas.Tree())

	canvas.PushLayer(windowControlsLayer)
	// Under a modal dialog the buttons join its focus scope, or the dialog would
	// shut them out; they are not part of its keyboard order.
	trapped := canvas.TrappedFocusScope()
	if !trapped.IsZero() {
		canvas.PushFocusScope(trapped)
	}
	theme := detectDesktopTheme()
	hovered, pressed := canvas.Hovered(), canvas.Pressed()
	for index, id := range controls.ids {
		bounds := ui.Rect{X: size.Width - float32(3-index)*WindowControlWidth, Width: WindowControlWidth, Height: controls.height}
		icon, name, tint := minimizeIcon, style.Minimize, style.Foreground
		switch index {
		case windowControlMaximize:
			icon, name = maximizeIcon, style.Maximize
			if maximized {
				icon, name = restoreIcon, style.Restore
			}
		case windowControlClose:
			icon, name = closeIcon, style.CloseName
		}
		isHovered := (hovered == id)
		isPressed := (pressed == id)

		if theme == desktopBreeze || theme == desktopAdwaita {
			btnSize := float32(math.Min(26.0, float64(controls.height-6.0)))
			if btnSize < 16 {
				btnSize = 16
			}
			btnRect := ui.Rect{
				X:      bounds.X + (bounds.Width-btnSize)/2,
				Y:      (controls.height - btnSize) / 2,
				Width:  btnSize,
				Height: btnSize,
			}
			radius := btnSize / 2

			switch {
			case index == windowControlClose && (isPressed || isHovered):
				background := style.Close
				if isPressed {
					background.Alpha *= 0.8
				}
				canvas.Rounded(btnRect, radius, background)
				tint = style.CloseForeground
			case isPressed:
				canvas.Rounded(btnRect, radius, style.Pressed)
			case isHovered:
				canvas.Rounded(btnRect, radius, style.Hover)
			}
		} else {
			switch {
			case index == windowControlClose && (isPressed || isHovered):
				background := style.Close
				if isPressed {
					background.Alpha *= 0.8
				}
				canvas.Fill(bounds, background)
				tint = style.CloseForeground
			case isPressed:
				canvas.Fill(bounds, style.Pressed)
			case isHovered:
				canvas.Fill(bounds, style.Hover)
			}
		}
		canvas.DrawIcon(icon, ui.Rect{X: bounds.X + (WindowControlWidth-glyph)/2, Y: (controls.height - glyph) / 2, Width: glyph, Height: glyph}, tint)
		canvas.HitWithOptions(id, bounds, name, ui.HitOptions{Role: ui.RoleButton, SkipFocus: true})
	}
	if !trapped.IsZero() {
		canvas.PopFocusScope()
	}
	canvas.PopLayer()
}

// activate carries out a click on one of the buttons.
func (controls *windowControls) activate(id ui.ID) {
	actions := controls.actions
	if actions == nil {
		return
	}
	which, _ := controls.control(id)
	switch which {
	case windowControlMinimize:
		if actions.Minimize != nil {
			actions.Minimize()
		}
	case windowControlMaximize:
		if actions.ToggleMaximize != nil {
			actions.ToggleMaximize()
		}
	case windowControlClose:
		if actions.RequestClose != nil {
			actions.RequestClose()
		}
	}
}
