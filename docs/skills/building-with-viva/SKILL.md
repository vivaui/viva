---
name: building-with-viva
description: Write, change or debug a desktop application built on the Viva framework (github.com/vivaui/viva), or check its UI without opening a window. Use when the code imports github.com/vivaui/viva/app, ui, gfx, components or headless, or when asked to add a window, control, form, list, table, dialog, layout, input handling, animation or a headless test to a Viva app.
---

# Building with Viva

Viva is a Go-first, GPU-native desktop framework. It draws its own UI: there is no WebView, HTML, CSS or JavaScript, and nothing is redrawn while the app is idle. Most mistakes come from writing it like a web or retained-mode toolkit. Read this before writing code, then read the nearest example or `go doc` before guessing an API.

## 0. The loop

1. **Find the closest existing code.** `go doc github.com/vivaui/viva/ui` lists every element; `go doc github.com/vivaui/viva/ui.Button` shows one with its comment. The example nearest the task is under `examples/guides`: `hello` (smallest window), `layout`, `textfield`, `input` (list, context menu, splitter), `overlay` (modal, popover, drawer), `theming`, `windows` (custom title bar, second window), `drawing`, `page` (filter, list, progress), `background` (effects), `webview`.
2. **Write the page** in `main.go`. Keep state in the model; handle a click where its control is declared.
3. **Fix it before running it.** `gofmt -w . && go vet ./... && viva vet`. `viva vet` reports page mistakes that compile: an unkeyed `Button` in a loop, a `ui.C`/`ui.Box` kept past its paint, a `ui.Memo` whose value misses a field its function reads.
4. **Check behaviour and the picture.** `go test ./... -count=1`, then the headless build with `layout`, `tree` and `expect` (section 5).
5. **Say what you did not verify.** A unit test does not prove IME, focus loss, a window-manager close or a real GPU.

A symbol that `go doc` does not show does not exist. Do not invent an API and wrap it in a workaround.

## 1. Decide whose code it is

Before building, sort each need into one of two kinds.

- **Application behavior**: your protocols, storage, workflows, product screens. It stays in your app.
- **A reusable desktop capability**: windows, input, text and IME, focus, layout, components, rendering, accessibility, lifecycle, headless control. If a second Viva app would copy the same helper or wiring, it belongs in Viva, not in your app. Stop and design the smallest self-contained API for it instead of working around the gap with platform calls, synthetic input or hand-written hit testing.

If you are in a checkout of Viva itself, its `AGENTS.md` is the authority and overrides this file.

## 2. Rules that review rejects

- No reflection, `map[string]any` or dynamic property bags in layout, input, scene or render paths.
- No goroutines, timers or polling loops that run while the app is idle. Animation is a scheduled, temporary reason to draw, never a heartbeat. An idle window draws nothing.
- UI state belongs to the window thread, including `ui.Field.Text()` and `text.Editor.String()`: getters may initialize state or populate caches. Read a string snapshot before starting a worker, never capture a field/editor pointer in it. Use `ui.NewEffectKeyed` with the snapshot as its key and consume `Take` in the page; independent workers publish under synchronization and deliver through `Invalidator` / `OnWake`.
- Declaring components and layout must not allocate per paint. State that is expensive to compute (shaped text, line breaks) is cached by whoever owns it and invalidated explicitly.
- Never number hit targets by hand. Identities are names in a `gfx.Tree`; a page (`ui.C`) makes them for its elements.
- Do not add a dependency to the code that ships in the app without measuring its stripped binary size, startup and idle memory.
- Do not claim "zero allocation", "fast" or "light" without a reproducible measurement.

## 3. How a screen is written

A page is `func (m *model) page(c ui.C)`, opened with `ui.Run(config, m.page)` or `ui.NewPage(ui.PageConfig{View: m.page}).Config(base)`. It runs again on every redraw, declares what is on screen with value types, and keeps no layout cache. State lives in the model; a click changes it where the control is declared.

```go
type model struct {
	name   string
	notify bool
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
		ui.Text(c, "Settings").FontSize(20).Bold()
		ui.TextField(c, &m.name, "Name", "Your name")
		ui.Switch(c, &m.notify, "Notify me")
		if ui.Button(c, "Save").Clicked() {
			m.save()
		}
	})
}
```

- The core page imports `github.com/vivaui/viva/ui`: colors, geometry and the theme are re-exported there (`ui.RGB`, `ui.Rect`, `ui.Theme`, `ui.LightTheme()`). Optional page packages (`ui/calendar`, `ui/message`, `ui/chart`, …) are imported only when used. `gfx` is the layer below (canvas, paths, input events, keys, identities); reach for it only to draw or handle input yourself.
- Everything is a function of `ui` taking `c` first, and the chain is the same everywhere: `.Pad`, `.Gap`, `.Grow`, `.Width`, `.Background`, `.Children(func() { ... })`. Containers: `Row`, `Column`, `Stack`, `Grid`, `Flow`, `Scroll`. Leaves: `Text`, `Paragraph`, `Wrap`, `Spacer`, `Divider`, `Icon`, `Image`, `Button`, `Checkbox`, `Switch`, `Select`, `Choice`, `Tabs`, `Slider`, `Progress`, `Spinner`, `Splitter`, `Slot`.
- Lengths are `float32`; sizes are border boxes. A child is as big as its content unless told `Width`/`Height`/`Size`, `Fill`, or `Grow(n)`. In a `Row`, a text field needs `Grow(1)` or `Width`, or it is invisible.
- A themed control is a node, not a fixed picture: its defaults come from the recipe, and the element attributes it accepts (`.FontSize`, `.Color`, `.Radius`, `Hover`/`Pressed`/`Focus`, and `.Pad`/`.Gap`/`.Children` where the control lays content out) override them. Not every control takes every attribute; a dev build panics when one would do nothing, and `VIVA_ATTR_CHECK=warn` reports instead of stopping, for cleaning up a page.
- `Children` is the content slot of a control that has one: `ui.Button(c, "Save").Children(func() { ... })` replaces the default label with your content, laid out in the content rectangle. The `Label` argument remains the control's assistive name, and hits, focus and disabled stay on the control. Without `Children` the default label is drawn as before; the control's own parts (check mark, switch thumb, tab indicator) are not content.
- Element attributes reach the parts of composite controls: `ui.Table(...).FontSize(...).Color(...)` styles header and cells, `ui.Image(...).Radius(...).Opacity(...)` cuts and fades the picture itself, `ui.Checkbox(...).Size(...)` sizes the box and scales the mark, `ui.Switch(...).FillWidth()` / `.WidthPercent(...)` stretches the track, `ui.Tabs(...).PadSides(...)` pads all four sides, and `ui.Progress(...).Label(...)` exposes the bar as a read-only progress bar (unlabelled bars stay decorative and take no pointer).

### Lists, tables and trees

- A long list is `ui.List(c, &m.list, n, func(row ui.ListRow) { ... })`: only the rows on screen are declared. The row's container is `row.Element`, its content is declared with `row.C`. `Selected()`, `Changed()`, `Submitted()`, `ListState.Label` (typeahead), `Selection` (multi-select), `ScrollTo`/`ScrollIntoView`/`Visible`, `Header` (pinned sections), `Reorder` (drag rows), `RowWidth` (sideways scrolling), `ClickModifiers()`. Key rows that hold a `Button`: `ui.Row(c.KeyInt(item.ID))`.
- A table is `ui.Table(c, &m.table, ui.TableSpec{Headers, Sortable, Source, Columns})`; the data is a `ui.TableRows` (`[][]string`) or your own `Len() int` / `Cell(row, col int) string`. `RowClicked()`, `HeaderClicked()`, `SortBy`, `Select`.
- A tree is `ui.Tree(c, &m.tree, n, label, content)` with a `TreeState.Node` per visible row; `Toggled()` and `Chosen()` report the user's action; `CanDrop` + `Dropped()` for drag and drop. `RowID(i)` is a row's identity; a part of a row is `ui.Text(c, name).LabelFor(m.tree.RowID(i))`.
- A part that rarely changes is `ui.Memo(c, *row, fn)`: it copies the declaration while that value compares equal. Pass the row (or what the function reads), not a counter.
- Scrolling: `area := ui.Scroll(c).Grow(1)`; `area.ScrollState()` has `Offset`, `Content`, `Viewport`, `Max()`, `AtEnd()`, `Pinned()`, `ScrollTo`, `ScrollToEnd`; `.Follow(true)` keeps a log or chat at its end; `.Sideways()` and `.Scrollbars()` add horizontal movement and bars.

### Dialogs, menus and popups

- `ui.Modal(c, &m.open, "Title", body, footer)` or `ui.ModalWith(..., ui.ModalSpec{...}, ...)`. The title is required; Escape, the close button and a press on the scrim set `m.open` to false, so do not write an Escape handler. `ui.Dialog(c, ui.DialogSpec{...})` is the form whose identities you name: `Scope`, `DismissID`, `CloseID`, `InitialFocusID`, `.Header`, `.Body`, `.Footer`, `.Dismissed()`.
- `ui.Popover(c, anchor, &m.open, body)` / `ui.PopoverWith(c, anchor, ui.PopoverSpec{...}, &m.open, body)`; `ui.Menu(c, anchor, &m.open, func(menu *ui.MenuItems) { ... })` with `menu.Item("Rename").Clicked()`, `menu.Sub(label, body)`; `ui.ContextMenu(c, target, func(menu *ui.MenuItems) { ... })`; `ui.Drawer(c, &m.open, title, ui.DrawerSpec{...}, body, footer)`.
- An application overlay is `ui.Overlay(c, ui.OverlaySpec{Band: ui.LayerPaneChrome, Bounds: r})` (or `ui.LayerDrag`): it takes no room, paints below Viva's menus and dialogs, and a `Canvas.Block` over its empty surface keeps presses from the page behind.
- Toasts: keep a `ui.Toaster`, `m.toaster.Show(title, text, ui.ToastSuccess)`, paint `ui.Toasts(c, &m.toaster)` once; `ShowAction(..., "Undo")` + `Acted()`, `ToastLoading` + `Done(id, ...)`, `Toaster.Position`.

### Forms and fields

- `ui.TextField(c, &m.name, "Name", "placeholder")` binds a `string`; `&m.query` can bind a `ui.Field` when the caret or selection matters. `.Submitted()` is Enter, `.Secure()` a password, `.AutoFocus()`, `.Icon(icon).Action(icon, "Clear")` with `.ActionClicked()`. `ui.TextArea(c, &m.notes, "Notes", "").Height(120)` is several rows. `Config.TextInput` is not needed for typing, caret, selection, clipboard or IME.
- `ui.FormField(c, &m.proxyField, "Proxy").FillWidth().Children(func() { ... })` is a form row: one title and one control child, in the slot `components.BeginField` uses. Keep the `ui.FormFieldState` beside the value. Tagged structs get the same form from `//go:generate go tool viva form`.
- `ui.Choice(c, &m.theme, themes, nil)` is a segmented choice and `ui.Tabs(c, &m.tab, tabs, nil)` a tab strip. Both write the value as they are declared; disable them with an `if`, not a chained `.Disabled`. `Tabs`/`Select` also honor `.Disabled` for input; `Choice` does not.

### Optional page packages and custom surfaces

- A heavy or stateful page control lives in an opt-in package under `ui/<capability>`; import it only when the page uses it.
- Editors and visuals: `ui/codeeditor`, `ui/chart`, `ui/markdown`, `ui/image`, `ui/calendar`, `ui/carousel`, `ui/columnresize`, `ui/reordergrid`. Navigation and overlays: `ui/command`, `ui/menubar`, `ui/navigationmenu`, `ui/sidebar`, `ui/hovercard`. Data and inputs: `ui/avatar`, `ui/avatareditor`, `ui/attachment`, `ui/metriccard`, `ui/marker`, `ui/skeleton`, `ui/aspectratio`, `ui/questionnaire`, `ui/messagescroller`, `ui/message`, `ui/inputgroup`, `ui/inputotp`, `ui/multiselect`, `ui/hierarchypicker`, `ui/buttongroup`, `ui/togglegroup`.
- The package name is the capability, never a `view` suffix. Name its state type by role, not by repeating the package: `calendar.State`, `message.Row`, `attachment.Card`, `hovercard.Card`. Keep locals out of the package name (`cal`, `msg`, `pic`); a model field may keep the domain noun (`m.calendar`).
- Some packages expose `pkg.View(c, label, &state)` (`calendar.View`, `markdown.View`); others expose a method on the state (`avatar.New().View(c, label)`, `questionnaire.New().View(c, label)`). `go doc github.com/vivaui/viva/ui/<package>` shows which.
- A page control can sit inside a `ui.Slot` at the rectangle a component painted: keep the slot's size, declare the control with `.Absolute().Left(0).Top(0).Size(...).Hit(id, label)`, and bind a copy when the application's dispatcher owns the value. `ui/slot_parity_test.go` keeps this exact for Checkbox, Switch and FormField.
- Keep a wholly custom surface as `ui.Slot(...).Draw(...)` with `gfx.Canvas` and `Begin*` recipes in package `components`. Do not rewrite the whole surface as page declarations only for uniformity: its layout and hit model belong to the application.

### Background work and animation

- Page work is a `ui.Effect`, never a goroutine started in the page: `var load = ui.NewEffect[Result, Key](run)`, declared with `ui.Effect(c, m.load, key)` and read with `m.load.Take()`. A new key cancels and restarts; `.Sequential()` waits for the cancelled run; `NewEffectKeyed` passes the key to the run. The run must not touch a `C`, `Box` or `Page`.
- A long-lived worker of your own stores its result under a lock, calls `Invalidator.Invalidate()`, and `Config.OnWake` copies it into the model on the window thread.
- Animation is a scheduled reason to draw, not a loop: read `c.Canvas().Now()`, call `c.Canvas().RequestFrame()` while it is still moving, and stop asking when it stops. `Progress` and `Spinner` schedule their own frames.

### Custom controls, drawing and drag

- A control you paint is `ui.Control(c, label, &m.handlers).Grow(1).Draw(m.draw)`. `ui.Handlers` receives keys, pointer, wheel and, with `gfx.RoleTextField` or `gfx.RoleTerminal`, typing and IME through `Text`. Its parts are declared in `Draw` with `d.Canvas.Hit(part, r)`; an event on a part reaches the same handlers with the part as its target.
- Custom layout: `ui.CustomLayout(c, &m.layout).Children(...)` with `ui.Layout`'s `MeasureWidth`, `PlaceWidth`, `MeasureHeight`, `PlaceHeight`. The page keeps identity, focus and hits; the layout owns the children's geometry.
- Drag and drop: a `ui.DragState` in the model, `.DragSource(&m.drag)` and `.DropTarget(&m.drag, accept, drop)` on `Hit` boxes, or `d.DragSource`/`d.DropTarget` inside a `Draw`. The page routes the pointer; `Dragging()`, `Source()`, `Target()`, `At()` paint the gesture. A press that does not move is still a click. A split pane is `ui.Splitter(c, &m.split)` with a `ui.SplitterState`, not a drag.
- Painting your own thing: `ui.Slot(c).Draw(func(d *ui.DrawContext) { ... })`, with `d.Canvas.Fill`, `Rounded`, `Border`, `RoundedBorder`, `Elevate`, `DrawPath`, `Label`, `LabelCenter`, `d.Fade`, `d.DashedBorder`. A circle is a `Rounded` square. `Box.Rect()` is only valid in the paint that declared it.
- A shortcut is declared where it acts: `if ui.Shortcut(c, ui.Primary, "s") { m.save() }`; `box.Shortcut(0, "delete")` acts while the focus is inside that element. Plain keys stay with a field being typed in.

### Look finished

Take, do not invent:

- Colors and space come from the theme: `theme.Card`, `theme.MutedForeground`, `theme.Border`, radii from `theme.Metrics`, lengths in multiples of 4.
- Hover, pressed and focus are looks (`Hover`, `Pressed`, `Focus`, `Transition`), not hand-read canvas state.
- [`examples/recipes`](https://github.com/vivaui/viva/tree/main/examples/recipes) holds nine finished components (settings row, list row, avatar, badge, stat card, hero, icon button, empty state, dimmable section), each tested in both themes. Copy the nearest one and change it.
- [`recipes.md`](https://vivaui.dev/skills/building-with-viva/recipes.md) says which one to start from and what to avoid.
- Components are source-distributed, typed Go, one file per component: read the file rather than looking for a hidden style engine.

A component is a function of the page: `func Name(c ui.C) ui.Box`, declared where it is used. `ui.ID(c)` names a custom control by its place (or the Key or `c.Part("word")` before it). A component does not reach a lower layout engine and does not declare a `gfx.Tree`.

## 4. Where to look

- The project's own `AGENTS.md`, written by `viva init`, is the compact API reference. Read it before inventing a name.
- Guides: <https://vivaui.dev/en/docs/> — programming model, declarative layout, handling input, text input, overlays, theming, performance, testing, platforms.
- Platform behaviour and what still needs a real window: [`docs/conformance.md`](https://github.com/vivaui/viva/blob/main/docs/conformance.md).
- Measurements and the machines they were taken on: [`docs/benchmarks/`](https://github.com/vivaui/viva/tree/main/docs/benchmarks).

## 5. Verify without opening a window

Do not open a real window unless the user agrees; it interrupts them and depends on which display it lands on. Use these in order of cost.

1. **A Go test with `app.Driver`.** `driver, err := app.NewDriver(ui.NewPage(ui.PageConfig{View: m.page}).Config(base), nil)` runs the real painter, router and focus rules. Drive it with `Click`, `Key`, `Type`, `Scroll`, `Resize`, `Advance`, and read `driver.Snapshot()` for the labels, hit regions and focus. Find controls by their accessible name (`Hit.Label`), not by pixel. See `examples/guides/hello/main_test.go`.
2. **The headless line protocol.** Import `github.com/vivaui/viva/headless/auto` from a file guarded by `//go:build vivahead`, build with `-tags vivahead`, and start the program with `VIVA_HEADLESS=1` and a throwaway `HOME`. It reads one command per line: `click "Label"`, `type`, `key`, `waitfor`, `shot`, `texts`, `layout`, `tree`, `focus`, `expect`. Each reply ends in `ok` or `error`. The full command list is the comment above `Serve` in [`headless/control.go`](https://github.com/vivaui/viva/blob/main/headless/control.go); do not copy it elsewhere.
3. **Read the frame.**
   - `layout` lists text that is clipped, spills or overlaps.
   - `tree` lists every control with its role, identity, name, value and state (`focused`, `disabled`, `covered` ...).
   - `expect` turns a look into a check that fails with what the frame has instead: `expect text "Connected" within 5000`, `expect state id:home/auto checked`, `expect layout clean`, `expect named`. `Session.Expect` takes the same words in a Go test.
   - When a step fails: not found → `tree`; text cut off → `layout`; focus wrong → `focus`; a press does nothing → look for `covered`.
4. **A real window** on each target platform for IME, focus loss, window-manager close, physical keyboard and high-DPI. Say plainly what you did not test.

## 6. Before you finish

- `gofmt`, `go vet ./...`, `viva vet` (or `go tool viva vet`), `go test ./... -count=1`.
- If the page uses `Memo`, run the test with `ui.MemoCheck(true)` (or `PageConfig.MemoCheck`): it panics where the value misses something the part reads.
- For a painting or input path, report allocations per paint and steady-state time from `go test -bench`; for a linking change, report the stripped binary size before and after.
- State what you verified and what you did not: unit tests, headless `layout`/`expect`, and real-window behaviour are different claims.
