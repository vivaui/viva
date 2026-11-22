# Maintaining controls

How a control is built, what keeps its two forms the same, and what adding one touches.
Written from the code and from the history of earlier additions; where a step is a
convention rather than something a test checks, it says so.

## Layers

```
gfx -> style -> base -> ui
          \-> components   (style recipes; may read base)
app is the platform loop below ui; it knows nothing of pages.
```

- `style` carries look values only: variants, sizes, resolved looks and the
  values painters read. No state machines, no canvas measurement.
- `base` is the engine: element tree, layout, input/focus routing, control state
  and behaviour. It never imports `components`, `ui` or `app`. It is published
  because the styled layer and the recipes copied by `viva add` import it.
- `components` is the style library: recipes and canvas painters. It may import
  `base` for the state a copied recipe reads, but not `ui` or `app`.
- `ui` composes `base` and `components`, installs the painting hooks, and is the
  application-facing facade. Applications declare pages with `ui.C`.

`internal/layering` checks both forbidden and allowed edges; a new dependency is
an explicit edit there.

## Page controls, recipes and Begin

There are two authoring paths, and they are not duplicates:

- **Page controls** are `ui.*` or `ui.*Base` + Draw. They own page identity,
  focus, input and state; the recipe gives their node its default look and
  measurement, and the styled painter draws only the parts the control owns.
  `ui` exports no `Begin*`.
- **Canvas recipes** are the `Begin*` constructors in package `components` and their `Paint`/`Layout` methods.
  They construct a value, paint through `gfx.Canvas` and declare hits; they are
  used inside a `ui.Control`/`Draw`, for pure measurement, or in source copied by
  `viva add`. They are not a second page API.

“Zero Begin” is not a goal. A new control that is declarable on a page belongs in
`ui`; a reusable but heavy composite belongs in an opt-in package that uses
`ui.Control`; a recipe that only paints stays in `components`. Exceptions that
may keep Begin are pure measurement, whole-canvas drawing, pixel-fixed slots and
copied source. See `private/docs/decisions/0085-begin-stays-in-components.md`.

Opt-in page packages live under `ui/<name>` (`ui/codeeditor`, `ui/chart`,
`ui/calendar`, `ui/carousel`, `ui/markdown`, `ui/columnresize`,
`ui/avatareditor`, `ui/reordergrid`, `ui/avatar`, `ui/attachment`,
`ui/message`, `ui/messagescroller`, `ui/command`, `ui/menubar`,
`ui/navigationmenu`, `ui/hovercard`, `ui/image`, `ui/aspectratio`,
`ui/skeleton`, `ui/inputgroup`, `ui/inputotp`, `ui/questionnaire`,
`ui/metriccard`, `ui/marker`, `ui/sidebar`, `ui/buttongroup`,
`ui/togglegroup`, `ui/multiselect`,
`ui/hierarchypicker`).
They import only the public `base`, `components`, `gfx`, `ui` and `window`
packages, plus the focused `text` editor for editor-like controls and the root
`markdown` parser for the Markdown view. They own stable handlers, and are never
imported by package `ui`; importing one is the explicit opt-in. Do not add a new
directory at the repository root for a page widget.
See `private/docs/decisions/0086-opt-in-page-views.md`.

### Naming and types

An opt-in package is named for the capability, never with a `view` suffix:
`ui/calendar`, `ui/message`, `ui/chart`. Its exported state types are named by
role, not by repeating the package: `calendar.State`, `message.Row`,
`attachment.Card`, `hovercard.Card`. Keep local variables out of the package's
name (`cal`, `msg`, `pic`); a model field may keep the domain noun
(`m.calendar`) because a selector never shadows a package. A package exposes
either `pkg.View(c, label, &state)` or a `state.View(c, label)` method; keep new
packages to one of the two and document which one.

### Page controls inside a custom slot

A page control declared inside a `ui.Slot` at the rectangle a component painted
paints the same picture and declares the same control hit. This is the bridge
for an application surface that keeps its own layout but wants a page control:

```go
slot := ui.Slot(c).Size(22, 24).Shrink(0)
slot.Children(func() {
    ui.Checkbox(c, &copy, "").
        Absolute().Left(0).Top(0).Size(16, 16).
        Hit(id, label)
})
```

Bind a copy when the application's dispatcher owns the real value, and give the
control the same identity with `Hit`. `ui/slot_parity_test.go` compares the
Checkbox, Switch and FormField paths with the `Begin*` constructors in package
`components` and requires equal rectangles, text and control hits. Do not
rewrite a whole custom canvas surface into page declarations merely to use a
page control; keep the surface and move the leaves that fit this pattern.

## Two forms of one control

A control that ships with a look has a **styled form** and a **Base**:

- `ui.Button`, `ui.Checkbox`, `ui.Select`, … take the theme's look.
- `ui.ButtonBase`, `ui.CheckboxBase`, `ui.SelectBase`, … do everything the control does
  (pointer, keyboard, focus, value binding, what assistive technology is told) and paint
  nothing, so an application with a look of its own builds on them and does not copy a
  behavior. They are in `base/page_bases.go` (page level) and `base/bases.go`,
  `base/select.go`, `base/slider.go`, `base/frame.go`, `base/field.go` (frame level).
  Package `ui` forwards the application-facing names.

The two forms are the same control. What differs is the look and nothing else.

### How that is kept true

- `TestStyledControlsDeclareWhatTheirBasesDeclare` (`base/base_parity_test.go`) declares each
  styled control and its Base in every state a caller can put it in (on, off, disabled,
  chosen) and compares what each tells the pointer, the keyboard and assistive technology:
  role, name, value, checked, mixed, selected, expanded, focusable, disabled, range.
- `TestEveryPageBaseIsComparedOrExplained` (`base/base_scan_test.go`) fails for a page Base
  that is in neither that comparison nor the list of Bases without a styled form, so a new
  Base cannot be added without deciding.
- `TestEveryRemainingBaseIsItsControlWithoutALook` (`base/base_contract_test.go`) checks that
  a Base paints nothing and declares one focusable hit with its role and state.

### Where the behavior lives

A themed control is a node. Its box, padding, content, state look and children are the
engine's node attributes; the recipe in `components` supplies only the defaults that an
explicit `Box` call overrides. Behaviour — pointer, keyboard, focus, disabled, role, value —
is declared once on the control's root node in `base`, not by its painter. The painter in
`ui` (`ui/controls_paint.go`, `ui/tabs_paint.go`, `ui/select_paint.go`, …) draws only the
parts the control owns — check mark, switch thumb, track, chevron, indicator — and declares
no hit.

Select, Slider, Input, Textarea and Field are built as `X = XBase` plus the theme's look
(`flagThemed`), so their behavior is written once. Button, Checkbox, Switch, Tabs and the
other migrated controls have their own declarations, but share the same node contract; the
parity tests keep both forms reporting the same role, state and actions, and
`ui/slot_parity_test.go` keeps a control in a slot painting and hitting like its `Begin*`
recipe.

Every universal `Box` attribute a control accepts must either do its job or be rejected in
a dev build; neither outcome may be silent. The rejection rules live in
`base/attrcheck_dev.go` (empty in release builds) and their messages name the element, the
attribute and the reason; `VIVA_ATTR_CHECK=warn` prints them instead of panicking.
`base/attrs.go` only records the few defaults a node's value cannot express, and
`base/attrcheck_dev_test.go` / `attrcheck_release_test.go` cover both builds.

The behaviour lives in package `base`, the appearance in package `components`: `base` never
imports `components`, and the painters (in `ui`) read the engine's state through its
exported accessors and call the recipes. `internal/layering` enforces the direction.

## Where a control's look is decided

The look of a control is a recipe in `components` (`components/button.go`, …). Variants and
sizes of a styled control are declared through `base.Box` (`Box.Outline`, `Box.Small`, …)
and resolved onto the node's `Look`; an explicit element attribute (`.Pad`, `.FontSize`,
`.Color`, `.Radius`, …) overrides the recipe's default. A change to how a control looks
belongs in the recipe; a change to what it does belongs in `base`.

## Adding a control

Each step names the file to look at; the tests listed are the ones that fail when it is
missed.

1. **Recipe** — `components/<name>.go`, one component per file, with its test and its look
   for light and dark themes. Run `go generate ./cmd/viva` (the installable copies and
   `cmd/viva/components_gen.go` are generated; `go run ./tools/componentgen -check` fails
   when they are stale).
2. **Page control** — in `base`: a node kind and constructor on the frame (`base/frame.go`),
   its layout (`base/layout.go`) and paint dispatch (`base/paint.go`), the page function and
   its control kind (`base/elements.go`, `base/page.go`). The look's painter goes in `ui`
   (`ui/controls_paint.go`) and is installed through `ui/hooks_install.go`. Look at how
   `Switch` is wired and search for `kindSwitch` and `pkSwitch` to see every place a kind
   appears.
3. **Base** — add the Base in `base/bases.go` and `base/page_bases.go`, and a row for it in
   `TestStyledControlsDeclareWhatTheirBasesDeclare` and `comparedWithTheirStyledForm`.
   `TestEveryPageBaseIsComparedOrExplained` fails until you do.
4. **Attribute rule** — decide which universal attributes the new element takes. Each
   attribute either works on it or is rejected by `base/attrcheck_dev.go` with a message
   saying why; add the positive and negative cases to `base/attrcheck_dev_test.go`. A
   control with a content slot takes `Children`; its own parts stay its own.
5. **Guide and docs** — every exported `Box` method must appear in
   `cmd/viva/initfiles/AGENTS.md.txt` (`base/guide_test.go`), and a program shown in a guide is
   the one in `examples/guides` (`website`: `npm run sync:examples`).
6. **Catalog** — a demo in `internal/componentdemo`, registered in `state.go` and the
   catalog, so the component browser shows it.
7. **Cost** — a benchmark for the declaration, and a zero-allocation check for a warm
   repaint (see `TestSelectDisabledOptionsDeclareWithoutAllocating` for the form).

## Where the packages stop

`TestImportDirections` (`internal/layering`) fails when `gfx`, `text`, `components`, `ui` or
`app` imports a package above it. `components` paints into a `gfx.Canvas` and does not import
`ui`; `ui` paints its controls with `components`.
