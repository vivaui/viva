---
name: building-with-viva
description: Write, change or debug a desktop application built on the Viva framework (github.com/vivaui/viva), or check its UI without opening a window. Use when the code imports github.com/vivaui/viva/app, ui, flex or components, or when asked to add a window, control, layout, input handling or a headless test to a Viva app.
---

# Building with Viva

Viva is a Go-first, GPU-native desktop framework. It draws its own UI: there is no WebView, HTML, CSS or JavaScript, and nothing is redrawn while the app is idle. Most mistakes come from writing it like a web or retained-mode toolkit. Read this before writing code, and read the guides it points to before guessing an API.

## 1. Decide whose code it is

Before building, sort each need into one of two kinds.

- **Application behavior**: your protocols, storage, workflows, product screens. It stays in your app.
- **A reusable desktop capability**: windows, input, text and IME, focus, layout, components, rendering, accessibility, lifecycle, headless control. If a second Viva app would copy the same helper or wiring, it belongs in Viva, not in your app. Stop and design the smallest self-contained API for it instead of working around the gap with platform calls, synthetic input or hand-written hit testing.

If you are in a checkout of Viva itself, its `AGENTS.md` is the authority and overrides this file.

## 2. Rules that review rejects

- No reflection, `map[string]any` or dynamic property bags in layout, input, scene or render paths.
- No goroutines, timers or polling loops that run while the app is idle. Animation is a scheduled, temporary reason to draw, never a heartbeat. An idle window draws nothing.
- Declaring components and layout must not allocate per paint. State that is expensive to compute (shaped text, line breaks) is cached by whoever owns it and invalidated explicitly.
- Never number hit targets by hand. Identities are names in a `ui.Tree`.
- Do not add a dependency to the code that ships in the app without measuring its stripped binary size, startup and idle memory.
- Do not claim "zero allocation", "fast" or "light" without a reproducible measurement.

## 3. How a screen is written

The painting function is the UI. It runs again on every redraw, declares what is on screen with value types, and keeps no layout cache. The smallest complete program is [`hello/main.go`](https://github.com/vivaui/viva/blob/main/examples/guides/hello/main.go); the others in [`examples/guides`](https://github.com/vivaui/viva/tree/main/examples/guides) show one concept each (layout, input, text field, overlays, windows, theming, drawing, background).

- Declare identities once: `tree = ui.Tree{}`, a `tree.Feature("name")` per area, `tree.ID(feature, "control")` per control. Keep them stable between frames; focus, hover and press are tracked by ID, and `Canvas.Focused`, `Canvas.FocusVisible` and `Canvas.Hovered` report them back.
- Handle a click where the control is declared: `f.Button(id, label).Clicked()` is true for the one paint that delivers the click, and only then may state change.
- Lay out with `flex` inside `flex.View.Draw`. Components are source-distributed, typed Go, one file per component; read the file rather than looking for a hidden style engine.
- Make it look finished by taking, not inventing: colors from the theme (`theme.Card`, `theme.MutedForeground`, `theme.Border`), radii from `theme.Metrics`, lengths in multiples of 4, and state from looks (`Hover`, `Pressed`, `Focus`, `Transition`) rather than hand-read canvas state. [`examples/recipes`](https://github.com/vivaui/viva/tree/main/examples/recipes) holds nine finished components written that way (settings row, list row with a hover pencil, avatar with presence dot, badge, stat card, gradient hero, toolbar icon button, empty state, dimmable section), each tested in both themes: copy the nearest one and change it before composing from scratch. [`recipes.md`](recipes.md) says which to start from and what to avoid.
- Prefer a few intention-revealing parameters. When same-typed positional arguments pile up, use a small struct or chainable options.

## 4. Verify without opening a window

Do not open a real window unless the user agrees; it interrupts them and depends on which display it lands on. Use these in order of cost.

1. **`app.Driver`** in a Go test: runs your real painter, router and focus rules from explicit calls (`Click`, `Key`, `Type`, `Scroll`, `Advance`) and returns a `Snapshot` of labels, hit regions and focus. See [`hello/main_test.go`](https://github.com/vivaui/viva/blob/main/examples/guides/hello/main_test.go). Test behavior a user depends on, not pixel offsets or exact primitive counts.
2. **Headless build and line protocol**: import `github.com/vivaui/viva/headless/auto` from a file guarded by `//go:build vivahead`, so release builds never contain it. Build with `-tags vivahead` and start the program with `VIVA_HEADLESS=1` and a throwaway `HOME`: `RunCanvas` then reads one command per line from stdin instead of opening a window (`click "Label"`, `type`, `key`, `waitfor`, `shot`, `texts`, `layout` ...). Each reply ends in one `ok` or `error` line. The complete command list is the comment above `Serve` in [`headless/control.go`](https://github.com/vivaui/viva/blob/main/headless/control.go); do not copy it elsewhere, it would go stale.
3. **`layout`** lists text that is clipped, spills out of its control or lies over other text. It is faster to read than a screenshot; use `shot` when the look itself matters.
   **`tree`** lists every control with its role, identity, accessible name, value and state (focused, disabled, checked, covered ...); read it first to learn what the frame can do. A control with no name is reached by identity or role: `click id:home/auto`, `click checkbox:`, `click "button:Save"`. **`expect`** turns a look into a check that fails with what the frame has instead: `expect text "Connected" within 5000`, `expect state id:home/auto checked`, `expect layout clean`, `expect named`. Prefer a script that ends in `expect` lines to one you read by eye, and put `expect` lines in the Go test for anything a user depends on (`Session.Expect` takes the same words). When a step fails: not found, run `tree`; text cut off, `layout`; focus wrong, `focus`; a press does nothing, look for `covered` in `tree`.
4. **Real windows** on each target platform for anything the above cannot see: IME, focus loss, window-manager close, physical keyboard, high-DPI. Say plainly what you did not test.

Before finishing, run `gofmt`, `go vet ./...` and `go test ./... -count=1`. For painting or input paths, report allocations per paint and steady-state time with `go test -bench`; for a change that affects linking, report the stripped binary size before and after.

## 5. Where to look

- Guides: <https://vivaui.dev/en/docs/> (programming model, declarative layout, handling input, text input, overlays, theming, performance, testing, platforms).
- Platform behavior and what still needs a real window: [`docs/conformance.md`](https://github.com/vivaui/viva/blob/main/docs/conformance.md).
- Measurements and the machines they were taken on: [`docs/benchmarks/`](https://github.com/vivaui/viva/tree/main/docs/benchmarks).
