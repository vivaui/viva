package app

import (
	"errors"
	"fmt"
	"slices"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// TextMenuLabels are the words of the menu a text field opens on a secondary
// press. A label left empty is English. The menu is the system's own where the
// platform has one to show (macOS and Windows) and the page's where it has none.
type TextMenuLabels = gfx.TextMenuLabels

// TextMenuCommand is a command of a text field's menu.
type TextMenuCommand = platform.TextMenuCommand

// The commands of a text field's menu.
const (
	TextMenuNone      = platform.TextMenuNone
	TextMenuCut       = platform.TextMenuCut
	TextMenuCopy      = platform.TextMenuCopy
	TextMenuPaste     = platform.TextMenuPaste
	TextMenuSelectAll = platform.TextMenuSelectAll
)

// TextMenuItem is a row of a text field's menu, as a Driver shows it: the
// command, its words, and whether it is off. A zero Command is a separator.
type TextMenuItem = platform.TextMenuItem

// pointToMenu is where a text menu opens: the press that asked for it.
type pointToMenu struct {
	x, y float32
	at   gfx.ID
}

// requestTextMenu notes a secondary press that the application did not take, over
// a text field the window has an editor for, as the one the system's text menu
// opens for. The press focused the field already; the repaint it needs is done
// here and not left to the frame clock, so the editor the menu acts on is the
// one installed when the menu is taken.
func (painter *painter) requestTextMenu(event platform.PointerEvent) (platform.Frame, error, bool) {
	if painter.textMenu == nil || !painter.textMenuNative || event.Kind != platform.PointerDown ||
		event.Button != gfx.PointerButtonSecondary || event.Device == gfx.PointerTouch {
		return platform.Frame{}, nil, false
	}
	if painter.pointer.HoveredRole() != gfx.RoleTextField || painter.pointer.Hovered().IsZero() ||
		painter.pointer.Hovered() != painter.focus.Focused() {
		return platform.Frame{}, nil, false
	}
	painter.textMenuAt = &pointToMenu{x: event.X, y: event.Y, at: painter.pointer.Hovered()}
	frame, err := painter.repaint()
	return frame, err, true
}

// takeTextMenu is the menu the last press asked for, now that the frame it caused
// is painted: nil unless the field it was asked for still has the keyboard and the
// platform gave it an editor, which a read-only field has none of.
func (painter *painter) takeTextMenu() (platform.TextMenuRequest, bool) {
	at := painter.textMenuAt
	painter.textMenuAt = nil
	if at == nil || !painter.textEnabled || painter.nativeText.Terminal ||
		painter.nativeText.Target != at.at || painter.focus.Focused() != at.at {
		return platform.TextMenuRequest{}, false
	}
	return platform.TextMenuRequest{
		Items:   painter.textMenuItems(),
		X:       at.x,
		Y:       at.y,
		Perform: painter.performTextMenu,
	}, true
}

// textMenuItems are the rows of the menu, with the same rules the keyboard has: a
// secret never leaves through Cut or Copy, and nothing is cut or copied from an
// empty selection.
func (painter *painter) textMenuItems() []platform.TextMenuItem {
	labels := painter.textMenu.WithDefaults()
	input := painter.nativeText
	selection := input.ClipboardState('c').Selection
	selected := selection.Start != selection.End
	composing := input.State.Composing
	canCopy := selected && !input.Secure && !composing
	return []platform.TextMenuItem{
		{Command: platform.TextMenuCut, Title: labels.Cut, Disabled: !canCopy},
		{Command: platform.TextMenuCopy, Title: labels.Copy, Disabled: !canCopy},
		{Command: platform.TextMenuPaste, Title: labels.Paste, Disabled: composing},
		{},
		{Command: platform.TextMenuSelectAll, Title: labels.SelectAll, Disabled: input.State.Value == "" || composing},
	}
}

// performTextMenu runs a command chosen from the menu as the keyboard's chord of
// the same name would, and returns the frame it caused.
func (painter *painter) performTextMenu(command platform.TextMenuCommand) (platform.Frame, error) {
	var err error
	switch command {
	case platform.TextMenuCut:
		err = painter.clipboardEdit('x', painter.readClipboardText, painter.writeClipboardText)
	case platform.TextMenuCopy:
		err = painter.clipboardEdit('c', painter.readClipboardText, painter.writeClipboardText)
	case platform.TextMenuPaste:
		err = painter.clipboardEdit('v', painter.readClipboardText, painter.writeClipboardText)
	case platform.TextMenuSelectAll:
		_, err = painter.applyTextEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: platform.TextSelectAll})
	default:
		return platform.Frame{}, nil
	}
	if err != nil {
		return platform.Frame{}, err
	}
	return painter.repaint()
}

func (painter *painter) readClipboardText() (string, error) {
	if painter.clipboardReader == nil {
		return "", ErrUnsupported
	}
	return painter.clipboardReader()
}

func (painter *painter) writeClipboardText(value string) error {
	if painter.setClipboardText == nil {
		return ErrUnsupported
	}
	painter.setClipboardText(value)
	return nil
}

// clipboardEdit is a clipboard chord of the focused text field ('c', 'x' or 'v')
// against the clipboard read and write give.
func (painter *painter) clipboardEdit(action byte, read func() (string, error), write func(string) error) error {
	input := painter.nativeText
	event, changed, err := platform.TextClipboardEvent(input.ClipboardState(action), input.Secure, action, read, write)
	if err != nil || !changed {
		return err
	}
	_, err = painter.applyTextEvent(event)
	return err
}

// enableTextMenu says whether the window can show the system's text menu. The
// canvas hears it too, so the page leaves its own menu undrawn when the system's
// is shown.
func (painter *painter) enableTextMenu(native bool) {
	painter.textMenuNative = native && painter.textMenu != nil
	painter.canvas.SetNativeTextMenu(painter.textMenuNative)
}

// textMenuTaker is what the adapter asks for the menu after a press: nil when the
// window shows none, so an adapter without one is never asked.
func (painter *painter) textMenuTaker() func() (platform.TextMenuRequest, bool) {
	if !painter.textMenuNative {
		return nil
	}
	return painter.takeTextMenu
}

// TextMenu is the rows of the system's text menu a secondary press on a text field
// opened (see Config.TextMenu), nil when none is open. The driver shows no window:
// a test reads the rows here and chooses one with ChooseTextMenu.
func (driver *Driver) TextMenu() []TextMenuItem {
	if driver.textMenu == nil {
		return nil
	}
	return slices.Clone(driver.textMenu.Items)
}

// ChooseTextMenu chooses a command of the open text menu, as the user would, and
// closes it. It is an error when no menu is open or the command is not a row of
// it or is off.
func (driver *Driver) ChooseTextMenu(command TextMenuCommand) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	request := driver.textMenu
	if request == nil {
		return errors.New("app: no text menu is open")
	}
	for _, item := range request.Items {
		if item.Command != command || command == TextMenuNone {
			continue
		}
		if item.Disabled {
			return fmt.Errorf("app: the text menu's %q is off", item.Title)
		}
		driver.textMenu = nil
		_, err := request.Perform(command)
		return err
	}
	return fmt.Errorf("app: the text menu has no command %d", command)
}

// DismissTextMenu closes the open text menu without choosing from it.
func (driver *Driver) DismissTextMenu() { driver.textMenu = nil }
