---
title: Components
summary: Use the catalog directly, or copy a component's Go source into your project and own it.
order: 10
group: guides
---

## What is in the catalog

The catalog is a statically typed Go translation of the pinned shadcn/ui component set: buttons and forms, overlays and menus, calendars, tables, charts and feedback surfaces. They are plain Go functions that paint through `gfx.Canvas`. There is no reflection, no runtime style parser and no browser underneath. The [component catalog](/en/ui/button/) shows each one running, with its examples, source and keyboard behavior.

## Using a component

A page is written with package `ui`, whose controls (`ui.Button`, `ui.Checkbox`, `ui.Select` and the rest) paint with these components, as the [first window](/en/docs/getting-started/) does. Import `github.com/vivaui/viva/components` yourself when you paint into a `gfx.Canvas` by hand, or install a component's source to change it.

```go
components.BeginButton(canvas, theme, hitSave, bounds).Outline("Save")
```

A theme is a value, `gfx.LightTheme()` or `gfx.DarkTheme()`. Switching appearance means painting with the other one.

`Begin*` is the style layer's constructor, not a second page API. Declare page controls with `ui.*`, `ui.*Base` and `ui.Control`; use the `Begin*` recipes in package `components` inside a `Draw`, for pure measurement, or in source copied by `viva add`. “Zero Begin” is not a goal: the boundary is that page behaviour is not hand-built, not that the painter constructor disappears. Reusable heavy composites such as `ui/multiselect` and `ui/hierarchypicker` are opt-in packages.

## Optional page packages

A page control for a heavy or stateful component lives in its own package under `ui/<capability>`. Import it only when the page uses it; the package is the pay-for-use boundary.

- Editors and visuals: `ui/codeeditor`, `ui/textview`, `ui/chart`, `ui/markdown`, `ui/image`, `ui/calendar`, `ui/carousel`, `ui/columnresize`, `ui/reordergrid`
- Navigation and overlays: `ui/command`, `ui/menubar`, `ui/navigationmenu`, `ui/sidebar`, `ui/hovercard`
- Data and inputs: `ui/avatar`, `ui/avatareditor`, `ui/attachment`, `ui/metriccard`, `ui/marker`, `ui/skeleton`, `ui/aspectratio`, `ui/questionnaire`, `ui/messagescroller`, `ui/message`, `ui/inputgroup`, `ui/inputotp`, `ui/multiselect`, `ui/hierarchypicker`, `ui/buttongroup`, `ui/togglegroup`

`ui/textview` displays read-only text that the user can select and copy, such as logs and process output. Keep a `textview.View` in the model and call `m.output.Declare(c, textview.Spec{Source: lines, Scroll: true})`; `textview.Strings` adapts a string slice. The caller owns the lines, styles and actions. The view owns selection and drawing without an editing or undo model.

The package name is the capability, never a `view` suffix, and its state type is named by role: `calendar.State`, `message.Row`, `attachment.Card`. Keep locals out of the package name (`cal`, `msg`, `pic`); a model field may keep the noun (`m.calendar`) because a selector never shadows a package.

Some packages expose `pkg.View(c, label, &state)` (`calendar.View`, `markdown.View`); others expose a method on the state (`avatar.New().View(c, label)`, `questionnaire.New().View(c, label)`). `go doc github.com/vivaui/viva/ui/<package>` shows which.

```go
cal := &m.calendar
calendar.View(c, "Calendar", cal).FillWidth()
msg := message.New().Name("Ari").Body("Ready")
message.View(c, "Ari", msg)
```

A custom surface that keeps its own layout can host a page control in a `ui.Slot` at the rectangle a component painted:

```go
slot := ui.Slot(c).Size(22, 24).Shrink(0)
slot.Children(func() {
    ui.Checkbox(c, &copy, "").Absolute().Left(0).Top(0).Size(16, 16).Hit(id, "Select")
})
```

Bind a copy when the application's dispatcher owns the value. Keep the rest of the surface as `Draw`/canvas; do not rewrite it into page declarations only for uniformity. `ui/slot_parity_test.go` keeps the Checkbox, Switch and FormField paths pixel- and hit-identical to the `Begin*` recipes in package `components`.

## Tree navigation

`BeginTree` paints rows from an application-owned flattened hierarchy. For keyboard navigation, pass the visible rows' actual depths, expansion state and disabled state to `ui.NavigateTree`; do not pass display-clamped indentation as the depth. It returns the next visible index or an expand/collapse action. The application still owns loading children, expansion, selection and focus, so no hidden model or background work is created. Keep the navigation slice as reusable caller-owned scratch when handling keys.

## Charts

`PaintAreaChart` and `PaintBarChart` draw a complete chart (legend, axes, grid, series, hover mark and tooltip) from typed slices. There is no map of fields and no reflection: `Values[k]` holds the Y values of `Series[k]`, one per point, and `NaN` marks a gap.

```go
chart := components.AreaChart{
    Series: series,                       // label and color, shared with the legend and tooltip
    Values: [][]float32{desktop, mobile}, // one slice per series
    Curve:  components.ChartCurveMonotone,
    XAxis:  components.ChartAxis{Format: formatMonth},
    Grid:   components.ChartGridHorizontal,
}
plot := components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)
```

The chart owns no input. Keep a `components.ChartState` next to your data, and feed it from your own events: `state.Hover(plot.IndexAt(event.X))` on a pointer move, `state.Clear()` on exit, `components.ChartStepForKey` for the arrow keys. A steady chart allocates nothing per paint once its state has warmed up, and a chart that nobody touches schedules no frame.

Two limits are deliberate. A chart draws every point it is given, so a very long series should go through `components.ChartDownsample` once when the data changes, not on every paint. And stacked series must not be negative: a negative value stacks as zero.

## Owning a copy

If you want to change a component, copy it. `viva list` names the recipes the installer can copy, and `viva add` writes them into your project as ordinary Go files. A copy is a painting recipe: it draws into a `gfx.Canvas` in the rectangle you give it (`BeginButton(canvas, theme, id, bounds)`), so you use it from a `Draw` callback of a page's element or from a whole-window `Paint`, and its look is the code you edit. To change how every `ui.Button` looks without copying anything, give the page `PageConfig.Styles`; to build a control of your own look on the behavior `ui` ships, use `ui.ButtonBase` and the other Bases.

```sh
# install viva once (see "Build and package"); in a project made by viva init, use go tool viva
viva list
viva add --out ./components button card badge
```

| Flag | Meaning |
| --- | --- |
| `--out` | Directory to write into. Default `components`. |
| `--package` | Package name of the generated files. Default `components`. |
| `--force` | Replace files that already exist. |

The copied files are yours to edit. They still import Viva's `gfx` package for the canvas and theme types, so they depend on the Viva module, but nothing registers or updates them centrally. Every component is a recipe the installer can copy, and a recipe that calls into another one, such as `badge` into `button`, is copied together with it so the set compiles on its own. The copies are generated from the same source files the `github.com/vivaui/viva/components` package builds, so they match it. Importing `github.com/vivaui/viva/components` without calling a recipe links nothing from it, so copy a component only when you need to change more than its options allow. Every component page in the catalog has a **Download** link for its source file and its install command.
