---
title: Components
summary: Use the catalog directly, or copy a component's Go source into your project and own it.
order: 10
group: guides
---

## What is in the catalog

The catalog is a statically typed Go translation of the pinned shadcn/ui component set: buttons and forms, overlays and menus, calendars, tables, charts and feedback surfaces. They are plain Go functions that paint through `ui.Canvas`. There is no reflection, no runtime style parser and no browser underneath. The [component catalog](/en/ui/button/) shows each one running, with its examples, source and keyboard behavior.

## Using a component

Import `github.com/vivaui/viva/components` and declare the component while painting. This is the normal path, and it is what the [first window](/en/docs/getting-started/) does.

```go
components.BeginButton(canvas, theme, hitSave, bounds).Outline("Save")
```

A theme is a value, `ui.LightTheme()` or `ui.DarkTheme()`. Switching appearance means painting with the other one.

## Tree navigation

`BeginTree` paints rows from an application-owned flattened hierarchy. For keyboard navigation, pass the visible rows' actual depths, expansion state and disabled state to `components.NavigateTree`; do not pass display-clamped indentation as the depth. It returns the next visible index or an expand/collapse action. The application still owns loading children, expansion, selection and focus, so no hidden model or background work is created. Keep the navigation slice as reusable caller-owned scratch when handling keys.

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

If you want to change a component, copy it. `viva list` names the recipes the installer can copy, and `viva add` writes them into your project as ordinary Go files:

```sh
# install viva once (see "Build and package"); in a project made by viva init, use go tool viva
viva list
viva add --out ./components/ui button card badge
```

| Flag | Meaning |
| --- | --- |
| `--out` | Directory to write into. Default `components/ui`. |
| `--package` | Package name of the generated files. Default `ui`. |
| `--force` | Replace files that already exist. |

The copied files are yours to edit. They still import Viva's `ui` package for the canvas and theme types, so they depend on the Viva module, but nothing registers or updates them centrally. Every component is a recipe the installer can copy, and a recipe that calls into another one, such as `badge` into `button`, is copied together with it so the set compiles on its own. The copies are generated from the same source files the `github.com/vivaui/viva/components` package builds, so they match it. Importing `github.com/vivaui/viva/components` without calling a recipe links nothing from it, so copy a component only when you need to change more than its options allow. Every component page in the catalog has a **Download** link for its source file and its install command.
