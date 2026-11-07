# Recipes: making a Viva screen look finished

Read after `SKILL.md`. The code is in [`examples/recipes`](../../../examples/recipes), compiled and tested in light and dark, so it cannot drift from the API. This page says when to use which, and what makes the result look designed instead of assembled.

## Start from the nearest recipe

| You are building | Start from |
| --- | --- |
| A page of preferences | `SettingsRow` inside `Section` (a card with a divider between rows: `f.Divider()`) |
| A list of hosts, files, mailboxes | `ListRow`: hover and keyboard focus reveal the trailing action |
| A person or a host | `Avatar`, with the presence dot when it has one |
| A tag, a count, a status | `Badge` with `ToneNeutral`, `ToneAccent` or `ToneDanger` |
| A dashboard figure | `StatCard`; put several in a `Row` with `Grow(1)` each |
| A landing banner or onboarding step | `Hero` (the one place literal colors belong: the gradient stops) |
| A toolbar | `IconButton`: muted glyph that brightens with the button |
| A list with nothing in it | `EmptyState` |
| Anything that reacts but is none of these | `Interactive(box, theme, id, label)` around your own `Row` |

## What makes it look right

- **Take colors from the theme.** `theme.Card` for a surface, `theme.Border` for its line, `theme.MutedForeground` for secondary text, `theme.Accent` for a selected row. A literal color is right in one theme and wrong in the other. Translucent tokens that sit under a border are flattened with `Color.Over(background)` first.
- **One scale for space.** Lengths are 4, 8, 12, 16, 24. A row of controls uses `Gap(8)`, a card `Pad(16)`, a page `Pad(24)`. Equal gaps between siblings read as order; mixed ones read as accident.
- **Two radii.** Controls and rows take `theme.Metrics.RadiusControl`, cards and panels `RadiusSurface`, a circle is half its size or `ui.RadiusFull`. Do not invent a third.
- **Hierarchy by weight and color, not size.** A title is `Weight(ui.WeightMedium)`, its explanation is `.Muted()` at `FontSizeS`. Reserve a large `FontSize` for one figure or heading per screen.
- **Every reactive thing has all three looks.** Hover to show it is live, Pressed to show it took the press, Focus with `Ring(3, ...)` so the keyboard user can see where they are. `Interactive` gives the standard set; `Transition()` makes them fade instead of snap.
- **Reveal with `StateFrom`.** Secondary actions that appear on hover (`Opacity(0)` at rest, `Hover(Look{}.Opacity(1))`, and `Focus` too so a keyboard user can find them) keep a list calm. Do not read `canvas.Hovered()` by hand.
- **Shadow sparingly.** `ShadowXS` for a card on the page, `ShadowMD` for something lifted (a hero, a popover). Never on every row.
- **Gradients are an accent.** A thin bar (`StatCard`), a hero, an avatar background. Two or three stops, hues close together; a large gradient behind body text needs white text and a contrast check.
- **Fade with `Opacity`**, not by hand-mixing colors, to show something is off: `Section(..., enabled=false)` dims the group including its controls.

## What to avoid

- Hand-numbered hit identities, hand-measured offsets, or a `Canvas.Fill` where a `Row().Background(...)` would do.
- Per-frame allocation in a recipe: building a string, a slice or a closure with captured state each paint. Cache it in your state.
- Changing a recipe's structure to add one option. Add a parameter, or copy the recipe into your app and edit it there: a recipe is a starting point, not a library surface.

## Checking your own

Draw it in both themes and run `layout` (see `SKILL.md`, section 4). Add a test like `recipes_test.go`'s: every state you declared appears with the right trigger, nothing overflows its column, and a warm page allocates nothing.
