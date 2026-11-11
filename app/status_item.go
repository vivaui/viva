package app

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// ErrNoStatusArea is returned by AddStatusItem when the desktop has nowhere to
// put an icon, as a Linux session without a StatusNotifier host. The application
// is fine without one: keep its windows closable instead of hiding them.
var ErrNoStatusArea = platform.ErrStatusAreaUnavailable

// ErrNoGraphics is what opening a window returns, wrapped with what to install, on a
// machine that has no graphics driver and no CPU fallback that can draw: Windows
// without even the software Direct3D device, or Linux when the CPU rasterizer cannot
// start. Linux without a usable Vulkan driver draws the window on the CPU instead.
// A cloud server with no display needs the headless build. Test it with errors.Is.
var ErrNoGraphics = platform.ErrNoGraphics

// StatusIcon is a square image for the system's status area.
type StatusIcon struct {
	// Size is the width and height in pixels. 32 or more keeps it sharp on the
	// densest bars; the platform scales it to the size its bar draws.
	Size int
	// RGBA is Size*Size straight (not premultiplied) 8-bit pixels, top row
	// first.
	RGBA []byte
	// Template marks a shape that only has an alpha channel to say: the
	// platform colors it to suit the bar, light on dark and dark on light.
	Template bool
}

// StatusAnchor says where a status item is, so a panel can open beside it. Its
// content belongs to the platform; take it from a StatusClick and put it in
// Config.Anchor. The zero value means no anchor.
type StatusAnchor struct {
	rect platform.ScreenRect
	set  bool
}

// StatusClick is one press on a status item.
type StatusClick struct {
	Button gfx.PointerButton
	// Anchor locates the item for a panel opened in response.
	Anchor StatusAnchor
	// PanelDismissed is true when a WindowPanel closed itself, for losing
	// focus, because of this very press. A click that toggles a panel must not
	// open it again then.
	PanelDismissed bool
}

// StatusItemConfig describes a status item.
type StatusItemConfig struct {
	// Tooltip is shown while the pointer rests on the icon.
	Tooltip string
	Icon    StatusIcon
	// OnClick runs on the native event-loop thread for every press of the
	// item, whatever the button. Opening a window from it is allowed. With a
	// Menu it runs only for a press the system does not turn into the menu:
	// Windows and Linux send primary activation here and open the menu on a
	// secondary press. macOS opens the menu for both buttons and does not call
	// OnClick while Menu is set.
	OnClick func(StatusClick)
	// Menu supplies the system's own context menu. On Windows a primary press
	// also opens it when OnClick is nil. It runs on the event-loop thread each
	// time the menu is about to open, so the menu shows the application's state
	// at that moment, and nothing changes it while it is open.
	Menu func() []MenuItem
}

// MenuItem is one entry of a status item's menu.
type MenuItem struct {
	// Title is the entry's text. It is shown as written: no character in it
	// marks a shortcut.
	Title string
	// Separator makes the entry a dividing line; its other fields are ignored.
	Separator bool
	// Disabled shows the entry dimmed and not selectable, as a heading or a
	// line of status.
	Disabled bool
	// Checked puts a check mark beside the entry.
	Checked bool
	// Submenu, when not empty, makes the entry open a menu of these.
	Submenu []MenuItem
	// OnSelect runs on the event-loop thread, after the menu has closed, when
	// the user chooses the entry. An entry with a Submenu is not selectable.
	OnSelect func()
	// ID, when set, names the entry across builds, so a menu that outlives one
	// build, as the Linux tray's does, still reaches this entry after its title
	// or state changes. An ID must be unique among the entries of one menu: two
	// entries that share one are told apart only by their order, so removing one
	// of them gives a new id to the ones after it. The same ID in another submenu
	// names another entry, and an entry that moves to another submenu is another
	// entry. macOS and Windows rebuild the menu each time it opens; an entry
	// without an ID is known by its title, so a new title is a new entry.
	ID string
}

// StatusItemControl is what a Host gives back for a status item.
type StatusItemControl interface {
	SetIcon(StatusIcon) error
	SetTooltip(string)
	Remove()
}

// StatusItem is one icon the application keeps in the system's status area: the
// menu bar on macOS, the notification area on Windows, the StatusNotifier host
// (the tray of most Linux desktops) elsewhere.
type StatusItem struct {
	control StatusItemControl
	removed atomic.Bool
}

// SetIcon replaces the icon. Like SetTooltip it must be called on the event-loop
// thread: from the startup callback, a window callback or a status click.
func (item *StatusItem) SetIcon(icon StatusIcon) error {
	if item == nil || item.removed.Load() {
		return errors.New("app: status item is removed")
	}
	if err := icon.validate(); err != nil {
		return err
	}
	return item.control.SetIcon(icon)
}

// SetTooltip replaces the text shown while the pointer rests on the icon.
func (item *StatusItem) SetTooltip(tooltip string) {
	if item != nil && !item.removed.Load() {
		item.control.SetTooltip(tooltip)
	}
}

// Remove takes the icon away. It is idempotent and, unlike the setters, may be
// called from any goroutine.
func (item *StatusItem) Remove() {
	if item != nil && !item.removed.Swap(true) {
		item.control.Remove()
	}
}

func (icon StatusIcon) validate() error {
	if icon.Size < 1 || icon.Size > 1024 {
		return fmt.Errorf("app: status icon size must be between 1 and 1024 pixels, got %d", icon.Size)
	}
	if len(icon.RGBA) != icon.Size*icon.Size*4 {
		return fmt.Errorf("app: status icon of %d pixels needs %d bytes of RGBA, got %d", icon.Size, icon.Size*icon.Size*4, len(icon.RGBA))
	}
	return nil
}

func (icon StatusIcon) platform() platform.StatusIcon {
	return platform.StatusIcon{Size: icon.Size, RGBA: icon.RGBA, Template: icon.Template}
}

func platformMenu(items []MenuItem) []platform.MenuItem {
	if len(items) == 0 {
		return nil
	}
	converted := make([]platform.MenuItem, len(items))
	for index, item := range items {
		converted[index] = platform.MenuItem{
			Title: item.Title, Separator: item.Separator, Disabled: item.Disabled, Checked: item.Checked,
			Submenu: platformMenu(item.Submenu), OnSelect: item.OnSelect, ID: item.ID,
		}
	}
	return converted
}

// NewStatusClick is the press a development runner reports for a status item
// that has no real icon to click. It has no anchor.
func NewStatusClick(button gfx.PointerButton) StatusClick {
	return StatusClick{Button: button}
}
