// Package flex lays out a page the way CSS flexbox does, from a declaration
// instead of coordinates:
//
//	var view flex.View // in the application's state; the zero value is ready
//
//	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
//		view.Draw(canvas, theme, ui.Rect{Width: size.Width, Height: size.Height}, func(f *flex.Frame) {
//			f.Column().Pad(24).Gap(12).Children(func() {
//				f.Text("Hello").FontSize(20).Bold()
//				f.Row().Gap(8).Children(func() {
//					if f.Button(save, "Save").Clicked() {
//						model.save()
//					}
//					f.Spacer()
//					f.Button(cancel, "Cancel").Outline()
//				})
//			})
//		})
//	}
//
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
// Nothing here allocates once a View has seen its largest page: nodes live in
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
// Progress and Spinner are leaves that animate only while they are on screen, and a
// Splitter between two panes is dragged (pointer captured, resize cursor, minimum sizes) with
// its position in a SplitterState; a container given Hit that has no Children and no size
// fails at once, because Hit paints nothing.
//
// A ScrollArea keeps its offset in the View under the identity it is given, so the
// application holds no scroll state: f.ScrollArea(listID), View.Wheel for the wheel,
// Follow for a log that stays at its end, and View.ScrollState(listID) to move it by
// program.
//
// Tables, tab strips, selects and dialogs are declared in the same way (Table,
// Tabs, Select, Dialog): each is painted by the component of that name in package
// components, so it looks exactly like the one an application would paint by hand,
// and keeps what it must (the selected row, the sort, whether the list is open) in a
// small state value the application owns. The wheel scrolls an open Select's list (SelectState.Wheel) and SelectState.Key does type-ahead.
// A table's keys (TableState.Key) move its
// selection and scroll it into view, and its header stays put in a ScrollArea.
package flex
