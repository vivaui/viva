package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// EditMenuLabels are the words of the menu a TextField or TextArea opens on a
// secondary press. A label left empty is English.
type EditMenuLabels = gfx.TextMenuLabels

// The system text clipboard, which the styled layer installs for the page's edit
// menus. A page drawn by a host that installs neither has no Cut, Copy or Paste.
var (
	ClipboardText    func() (string, error)
	SetClipboardText func(string) error
)

// editMenuEntry is a TextField or TextArea declared in the paint that has the
// standard edit menu: the menu is declared once the whole page is, when a
// ReadOnly or Disabled chained on the box is known.
type editMenuEntry struct {
	box   Box
	field *Field
	area  bool
	layer uint16 // the widget layer the field was declared on, so a dialog's field has its menu over the dialog
}

// WithoutEditMenu keeps a TextField or TextArea from the standard edit menu, for
// a field whose application declares a ContextMenu of its own on it.
func WithoutEditMenu() Option { return Option{plain: true} }

func hasEditMenu(opts []Option) bool {
	for _, option := range opts {
		if option.plain {
			return false
		}
	}
	return true
}

// addEditMenu records a field declared in this paint for its edit menu. A field
// in a Memo is left out: the menu keeps its state by being declared, as any
// ContextMenu does, and a copied part would lose it.
func (p *Page) addEditMenu(c C, b Box, field *Field, area bool) {
	if len(p.recording) > 0 || ClipboardText == nil || PaintMenu == nil {
		return
	}
	p.editMenus = append(p.editMenus, editMenuEntry{box: b, field: field, area: area, layer: WidgetsOf(c.f).layer})
}

// declareEditMenus declares the edit menus of the fields of the paint.
func (p *Page) declareEditMenus(c C) {
	if !p.editFocus.IsZero() {
		if canvas := c.f.Surface; canvas != nil {
			canvas.RequestFocusOnce(p.editFocus)
		}
		p.editFocus = gfx.ID{}
	}
	var labels EditMenuLabels
	if p.config.EditMenu != nil {
		labels = *p.config.EditMenu
	}
	labels = labels.WithDefaults()
	for _, entry := range p.editMenus {
		p.declareEditMenu(c, entry, labels)
	}
}

func (p *Page) declareEditMenu(c C, entry editMenuEntry, labels EditMenuLabels) {
	if !entry.box.Valid() {
		return
	}
	n := entry.box.N()
	var readOnly, disabled, secure bool
	switch n.Kind {
	case kindInput:
		snapshot := &WidgetsOf(entry.box.f).inputs[n.Aux]
		readOnly, disabled, secure = snapshot.ReadOnly, snapshot.Disabled, snapshot.Secure
	case kindTextarea:
		snapshot := &WidgetsOf(entry.box.f).textareas[n.Aux]
		readOnly, disabled, secure = snapshot.ReadOnly, snapshot.Disabled, snapshot.Secure
	default:
		return
	}
	if disabled || n.Flags&flagDisabled != 0 {
		return
	}
	if canvas := c.f.Surface; canvas != nil && canvas.NativeTextMenu() && !readOnly {
		// The window shows the system's menu for a secondary press on a text field
		// it has an editor for; a read-only field has none, and keeps this one.
		return
	}
	widgets := WidgetsOf(c.f)
	layer := widgets.layer
	widgets.layer = entry.layer
	defer func() { widgets.layer = layer }()

	field, id := entry.field, n.ID
	contextMenu(c, entry.box, 0, nil, func(menu *MenuItems) {
		editor := field.ensure()
		// The item chosen acts on this field and leaves the keyboard on it, in the
		// paint after the menu has closed: a menu in a Dialog gives the keyboard back
		// to where it was only while the menu's own scope still traps it.
		chosen := func() bool {
			p.editFocus = id
			return true
		}
		selection := editor.Selection()
		_, composing := editor.Composition()
		selected := selection.Start != selection.End && editor.SelectionCount() == 1 && !composing
		editable := !readOnly && !composing
		if !secure {
			if menu.Item(labels.Cut).Keys(Primary, "x").Disabled(!selected || !editable).Clicked() && chosen() {
				if SetClipboardText != nil && SetClipboardText(selectedText(editor, selection)) == nil {
					editor.Paste("")
				}
			}
			if menu.Item(labels.Copy).Keys(Primary, "c").Disabled(!selected).Clicked() && chosen() && SetClipboardText != nil {
				_ = SetClipboardText(selectedText(editor, selection))
			}
		}
		if menu.Item(labels.Paste).Keys(Primary, "v").Disabled(!editable).Clicked() && chosen() {
			if value, err := ClipboardText(); err == nil {
				if !entry.area {
					value = text.SingleLine(value)
				}
				if value != "" {
					editor.Paste(value)
				}
			}
		}
		menu.Separator()
		if menu.Item(labels.SelectAll).Keys(Primary, "a").Disabled(editor.RuneCount() == 0 || composing).Clicked() && chosen() {
			editor.SetSelection(text.Range{Start: 0, End: editor.RuneCount()})
		}
	})
}

// selectedText is the text of selection, a range of runes of editor.
func selectedText(editor *text.Editor, selection text.Range) string {
	return editor.String()[editor.ByteOffset(selection.Start):editor.ByteOffset(selection.End)]
}
