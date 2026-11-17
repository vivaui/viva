# Maintaining controls

How a control is built, what keeps its two forms the same, and what adding one touches.
Written from the code and from the history of earlier additions; where a step is a
convention rather than something a test checks, it says so.

## Two forms of one control

A control that ships with a look has a **styled form** and a **Base**:

- `ui.Button`, `ui.Checkbox`, `ui.Select`, … take the theme's look.
- `ui.ButtonBase`, `ui.CheckboxBase`, `ui.SelectBase`, … do everything the control does
  (pointer, keyboard, focus, value binding, what assistive technology is told) and paint
  nothing, so an application with a look of its own builds on them and does not copy a
  behavior. They are in `ui/page_bases.go` (page level) and `ui/bases.go`, `ui/select.go`,
  `ui/slider.go`, `ui/frame.go`, `ui/field.go` (frame level).

The two forms are the same control. What differs is the look and nothing else.

### How that is kept true

- `TestStyledControlsDeclareWhatTheirBasesDeclare` (`ui/base_parity_test.go`) declares each
  styled control and its Base in every state a caller can put it in (on, off, disabled,
  chosen) and compares what each tells the pointer, the keyboard and assistive technology:
  role, name, value, checked, mixed, selected, expanded, focusable, disabled, range.
- `TestEveryPageBaseIsComparedOrExplained` (`ui/base_scan_test.go`) fails for a page Base
  that is in neither that comparison nor the list of Bases without a styled form, so a new
  Base cannot be added without deciding.
- `TestEveryRemainingBaseIsItsControlWithoutALook` (`ui/base_contract_test.go`) checks that
  a Base paints nothing and declares one focusable hit with its role and state.

### Where the behavior lives

Select, Slider, Input, Textarea and Field are built as `X = XBase` plus the theme's look
(`flagThemed`), so their behavior is written once. Button, Checkbox, Switch and Tabs are
separate code paths from their Bases: the styled form is painted by `components.BeginX`,
which declares the hit itself, and the Base declares it again in `ui/bases.go`. The parity
test is what keeps those two declarations equal until they share one.

## Where a control's look is decided

The look of a control is a recipe in `components` (`components/button.go`, …). Variants and
sizes of a styled control are chosen in `ui` (for a button, `Box.Outline`, `Box.Small`, and
the switch in `frame.paintButton`) and drawn by the recipe. A change to how a control looks
belongs in the recipe; a change to what it does belongs in `ui`.

## Adding a control

Each step names the file to look at; the tests listed are the ones that fail when it is
missed.

1. **Recipe** — `components/<name>.go`, one component per file, with its test and its look
   for light and dark themes. Run `go generate ./cmd/viva` (the installable copies and
   `cmd/viva/components_gen.go` are generated; `go run ./tools/componentgen -check` fails
   when they are stale).
2. **Page control** — in `ui`: a node kind and constructor on the frame (`ui/frame.go`), its
   layout (`ui/layout.go`) and paint (`ui/paint.go`), the page function and its control kind
   (`ui/elements.go`, `ui/page.go`). Look at how `Switch` is wired and search for `kindSwitch`
   and `pkSwitch` to see every place a kind appears.
3. **Base** — add the Base in `ui/bases.go` and `ui/page_bases.go`, and a row for it in
   `TestStyledControlsDeclareWhatTheirBasesDeclare` and `comparedWithTheirStyledForm`.
   `TestEveryPageBaseIsComparedOrExplained` fails until you do.
4. **Guide and docs** — every exported `Box` method must appear in
   `cmd/viva/initfiles/AGENTS.md.txt` (`ui/guide_test.go`), and a program shown in a guide is
   the one in `examples/guides` (`website`: `npm run sync:examples`).
5. **Catalog** — a demo in `internal/componentdemo`, registered in `state.go` and the
   catalog, so the component browser shows it.
6. **Cost** — a benchmark for the declaration, and a zero-allocation check for a warm
   repaint (see `TestSelectDisabledOptionsDeclareWithoutAllocating` for the form).

## Where the packages stop

`TestImportDirections` (`internal/layering`) fails when `gfx`, `text`, `components`, `ui` or
`app` imports a package above it. `components` paints into a `gfx.Canvas` and does not import
`ui`; `ui` paints its controls with `components`.
