// Package ui is how a Viva page is written. A page is a function of the model that
// declares, every paint, what is on screen; each element is a function of this
// package taking the page's C first, and nothing is named by hand:
//
//	func (m *model) page(c ui.C) {
//		ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
//			ui.Text(c, "Hello").FontSize(20).Bold()
//			ui.Switch(c, &m.sync, "Sync over SSH")
//			if ui.Button(c, "Save").Clicked() {
//				m.save()
//			}
//		})
//	}
//
//	ui.Run(app.Config{Title: "Settings", Width: 480, Height: 320}, m.page)
//
// Controls write into the values they are given, a list's rows are keyed with
// c.Key, a long list is a List, a part that rarely changes a Memo, and Popover and
// Modal open while a *bool is true: see page.go, elements.go, list.go and memo.go.
// Geometry, colors, the theme and identities come from package gfx, re-exported here
// (Rect, Color, RGB, Theme, ID and the rest of vocab.go) so a page imports ui alone.
//
// The page and its model, including Field and text.Editor getters, belong to the
// window thread. Capture immutable values there before background work starts.
// Use NewEffectKeyed to give a worker a value snapshot and EffectState.Take to
// consume its result in the page; other workers publish under synchronization
// and use app.Config.Invalidator with app.Config.OnWake to deliver results.
//
// The engine underneath is not part of the public model. A component is a
// function of the page (func(c ui.C) ui.Box); a custom control uses Control
// with Handlers, a custom layout uses CustomLayout, and a preview draws a Page
// into a canvas region.
//
// Layout is CSS flexbox's, from a declaration instead of coordinates.
// A Row lays its children out left to right, a Column top to bottom, a Stack on
// top of each other. A child is as large as its content unless it is given a
// size (Size, Width, Height), is told to fill (FillWidth, Fill) or to grow
// (Grow); free space is shared between the children that grow, and Justify and
// Align place what is left. Sizes are border boxes: padding and border are
// inside them, margins are not.
//
// A Grid child may span columns and rows (ColSpan, RowSpan); any child may be taken out
// of the flow and placed by its edges inside its container (Absolute, Top, Right,
// Bottom, Left, Inset), painted above the in-flow children and winning pointer hits
// over them; and a Popup is placed on a side of an anchor, flipped to the side with
// more room when the requested one is too small, kept inside the frame (or Within a
// rectangle) on both axes, and limited to the frame's size: see popup.go, grid.go and
// absolute.go for the exact rules.
//
// The declaration runs once a frame, then the tree is laid out in one pass and
// painted in order; pointer and keyboard state are read where a control is
// declared (Clicked), exactly as with the components in package components.
//
// Nothing here allocates once a page has been laid out at its largest size: nodes live in
// a slice that is reused, a Box is a two-word value, and the closure given to
// Children is only called, never stored. A Text is one line: a line that does
// not fit is clipped, and a Text shrinks before a fixed-size sibling does.
// Wrap, Paragraph and MaxLines make one that breaks into lines instead, at
// spaces, between East Asian characters (never leaving closing punctuation or
// a small kana at the start of a line, nor an opening bracket at the end), after
// the separators of a long path or URL, and as a last resort between whole
// characters (an emoji sequence or a letter with its marks is never split); its
// height follows its width, and the last line of one that is cut ends with an
// ellipsis.
//
// How an element looks under the pointer, held down and focused is declared on it (Hover,
// Pressed, Focus, each a Look, with StateFrom to follow a container and Transition to fade
// between them), and Ring, Opacity, Gradient and GradientStops are effects drawn without
// a class string or any lookup at run time: see style.go.
//
// A control with the theme's look is a base element dressed by a recipe, and the base is
// there to be used: InputBase, TextareaBase (and FieldBase), SliderBase, ButtonBase, CheckboxBase
// and SwitchBase are the controls without a look, and ToggleBase, TabBase, RadioBase, OptionBase
// and ComboboxBase the items of the composite ones (tabs, a segmented control, a select), so what they look like is the element's own
// Background, Border, Radius, Shadow, Pad and looks, with Draw and DrawOver for what those
// cannot say. Draw runs after the element's background and before its children, DrawOver
// after them, each with the laid-out rectangle, the element's gfx.State and its opacity, so a
// custom drawing sits where it belongs in the page instead of over all of it. Input and
// Textarea take the same calls: what the element sets replaces the recipe's value
// (components.TextEntryRecipe), the rest stays, and no variant is needed to make a field
// square or thick-framed. See recipe.go, bases.go and draw.go.
//
// A control of the application's own is Control(c, label, handlers): a focusable
// empty element painted with Draw, whose Handlers take its keys, pointer and
// wheel, and whose regions declared with Canvas.Hit inside Draw reach the same
// Handlers with the region as the event target (Handlers.Parts). A region can also
// be a drag source or target of a DragState, declared by the same Draw with
// DrawContext.DragSource and DrawContext.DropTarget: a custom grid keeps its cards
// as parts of one Control with stable identities and one group gesture, instead of
// a page element per card. See control.go, drag.go and draw.go.
//
// A form row is FormField(c, state, label): the title, and the one control its
// Children declare, in the control slot components.BeginField uses. The state is
// kept in the model next to the value and reused every paint. An application
// overlay is Overlay(c, OverlaySpec{Band, Bounds}): a floating page region
// painted after the page on a band below Viva's menus and dialogs, with no
// dismissal or focus trap. LayerPaneChrome is for chrome over the workspace and
// LayerDrag for a drag ghost or drop indicator.
//
// Progress and Spinner are leaves that animate only while they are on screen, and a
// Splitter between two panes is dragged (pointer captured, resize cursor, minimum sizes) with
// its position in a SplitterState; a container given Hit that has no Children and no size
// fails at once, because Hit paints nothing.
//
// A ScrollArea keeps its offset in the page under the identity it is given, so
// the application holds no scroll state: ui.Scroll(c), ScrollAt for an offset,
// Page.Wheel for the wheel, and Box.ScrollState() to move it by program.
//
// Tables, tab strips, selects and dialogs are declared in the same way (Table,
// Tabs, Select, Dialog): each is painted by the component of that name in package
// components, so it looks exactly like the one an application would paint by hand,
// and keeps what it must (the selected row, the sort, whether the list is open) in a
// small state value the application owns. The wheel scrolls an open Select's list (SelectState.Wheel) and SelectState.Key does type-ahead.
// A table's keys (TableState.Key) move its
// selection and scroll it into view, and its header stays put in a ScrollArea.
package ui
