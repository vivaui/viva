# Accessibility

Viva keeps a semantic tree apart from the scene (`internal/semantic`) and hands
each platform a flat list of `platform.AccessibilityNode`s in paint order, in
`Frame.Accessibility`, whenever it changes. The adapters turn it into what the
system's assistive technology reads, and turn what the technology asks for back
into `platform.AccessibilityRequest`s, which `app` carries out through the same
paths a key or a click takes (`app/accessibility.go`).

## What a node carries

Role, bounds, name (`Label`), `Value`, `Hint`, `Description`, the states
(`Focused`, `Disabled`, `Checked`, `Mixed`, `Selected`, `Expanded`,
`ReadOnly`, `Secure`), `Focusable`, `Expandable`, a heading `Level`, `PosInSet`
and `SetSize`, a `Live` setting, a numeric `Range` for sliders, and, for the
focused text field, its `Selection` in code points. Components set the extras
through `gfx.HitOptions` (and `Canvas.ContainerWithOptions` for regions).

What a node can do follows from its role and state (`AccessibilityNode.Patterns`),
so every platform offers the same set. A node with no `ID` is a region
(heading, status, list); it names nothing to act on. Containment is geometric
(`platform.AccessibilityContainer`): the list, grid, menu or menu bar whose
bounds hold the node.

## Requests

| Request | Press | Focus | Increment / Decrement | SetNumber | SetText | SetSelection |
|---|---|---|---|---|---|---|
| Carried out as | focus, then a completed click (`Clicked`) | focus | Up / Down on the focused slider | Home / End / Up / Down | a native text event over the whole text | a native selection event |

A field with no native editor, a read-only or secure field, and a disabled
control refuse. Each repaint a request causes is presented in order.

## Platforms

| | macOS | Windows | Linux | Web |
|---|---|---|---|---|
| API | NSAccessibilityElement | UI Automation providers | AT-SPI on the accessibility bus | ARIA on a hidden DOM mirror |
| Actions | press, increment, decrement, focus, set value, set selected text range | Invoke, Toggle, Value, RangeValue, SelectionItem, Selection, ExpandCollapse, Text | Action, Value, Text, EditableText, Component.GrabFocus | the browser's own |
| Events | focus change, live-region announcement | focus, property changes, text changed / selection changed, live region changed, structure | state-changed, property-change, children-changed, text-changed, caret-moved, announcement | the browser's own |

Cost when nobody is listening:

- Windows builds no element until a client first asks (`WM_GETOBJECT`), keeping
  only the newest slice header; once a client exists it raises events only while
  `UiaClientsAreListening`.
- Linux asks the session bus once, at the first window, whether accessibility is
  on. If the desktop has the service but it is off, one idle D-Bus connection
  waits for it to be switched on (no thread, no timer); if the service is absent
  nothing is kept. `VIVA_ACCESSIBILITY=0` skips the question.
- macOS elements are rebuilt when the tree changes, as before.

## Limits

- The tree is flat in paint order. A client sees containers (lists, grids, menus)
  as parents on Linux; Windows and macOS list every node under the window.
- Text ranges are in code points. Windows `GetBoundingRectangles` and AT-SPI
  character extents answer with the field's rectangle, and `RangeFromPoint` with
  the caret: the adapters cannot ask the text layout for a glyph's place. A line
  is a hard line (up to `\n`); soft-wrapped rows are not known to them.
- AT-SPI sentences are lines. `Selection` pattern support is for lists only.
- No `Table`/`Grid` patterns, no `Drag`, no `Transform`; no Windows UIA on
  arm64 (the callbacks need a floating-point thunk that exists for amd64 only).
- The `viva dev` reload worker does not carry requests from assistive technology.

## Verification

- Pure logic (roles, states, patterns, text units, the AT-SPI object model and
  signals, the host's off cost) runs under `go test` on every host.
- Windows 11 (WARP VM): an external `System.Windows.Automation` client read the
  tree, called Invoke, Toggle, RangeValue.SetValue, Value.SetValue, ExpandCollapse
  and the Text pattern (document range, selection, `Move` by word, `Select`), and
  received focus, property, text-changed and text-selection-changed events.
  Live-region delivery was not observed by a client (the managed client has no
  such event).
- Linux KDE/X11 VM: `libatspi` (PyGObject `Atspi`) listed the application, walked
  its tree with roles, interfaces, states and screen extents, called
  `DoAction`, `Value.set_current_value`, `EditableText.set_text_contents`,
  `Text.set_selection`, `Component.grab_focus`, and received state, property,
  text, caret and announcement events. Switching accessibility on after the
  application started was picked up. Orca itself was not run.
- macOS: the Objective-C compiles and the staging of UTF-16 selections is tested;
  VoiceOver was not run, and no window was opened.
