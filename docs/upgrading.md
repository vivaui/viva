# Upgrading an application

How to move an application to a newer Viva, and what each release changed in a
way that needs a call-site edit.

## Why this page exists

A Go module picks a version, not a range. An application that has not been
updated for a while jumps straight to the newest tag, so the incompatible change
it meets may be several releases back. Reading only the newest release notes is
not enough: the `apidiff` sentence in one release's notes compares that release
with its own predecessor, not with whatever the application is on.

## Incompatible changes

Newest first. This guide covers the incompatible changes documented here from
v0.2.2 through v0.6.0, including package moves as well as removed members and
changed signatures. Read every entry newer than the application's current
version, together with the corresponding [release notes](releases/). For a
target newer than v0.6.0, also check its intervening release notes; this page is
not a promise that later releases only add API. Releases before v0.2.2 are not
covered: the release-note archive begins there.

### v0.6.0 - package moves and Page-only authoring

There are two package moves **and** an authoring-model change. Updating imports
alone does not migrate an application that uses `flex.Frame` or `flex.View`.

#### `flex` becomes `ui`

The module no longer contains `github.com/vivaui/viva/flex`. Import
`github.com/vivaui/viva/ui` for page declarations, layout, fields, controls and
their state. Names such as `flex.Box`, `flex.Field`, `flex.Look`,
`flex.ScrollState`, `flex.TableState`, `flex.DialogSpec` and the alignment
constants `Start`, `Center`, `End` and `Between` now use the `ui` qualifier.

```go
import "github.com/vivaui/viva/ui"

var scroll ui.ScrollState
var table ui.TableState
```

Do not globally replace every old `ui.` reference with `gfx.` after replacing
`flex.` with `ui.`: the old packages now have different destinations. Classify
the original references first, and then edit imports and call sites together.

#### Low-level `ui` becomes `gfx`

Import `github.com/vivaui/viva/gfx` for the former low-level `ui` APIs. These
moved names keep their spelling; representative mappings are:

| In v0.5.1 and earlier | In v0.6.0 | Purpose |
| --- | --- | --- |
| `ui.Canvas`, `ui.Bitmap`, `ui.NativeView` | `gfx.Canvas`, `gfx.Bitmap`, `gfx.NativeView` | Drawing and embedded surfaces |
| `ui.Icon`, `ui.IconLayer` | `gfx.Icon`, `gfx.IconLayer` | Icons |
| `ui.Tree`, `ui.ID`, `ui.Scope` | `gfx.Tree`, `gfx.ID`, `gfx.Scope` | Interaction identities |
| `ui.Hit`, `ui.HitOptions`, `ui.EventResult` | `gfx.Hit`, `gfx.HitOptions`, `gfx.EventResult` | Hit regions and event results |
| `ui.KeyEvent`, `ui.PointerEvent`, `ui.ScrollEvent`, `ui.FileDropEvent` | `gfx.KeyEvent`, `gfx.PointerEvent`, `gfx.ScrollEvent`, `gfx.FileDropEvent` | Input events |
| `ui.KeyEnter`, `ui.ModifierShift`, `ui.Role` | `gfx.KeyEnter`, `gfx.ModifierShift`, `gfx.Role` | Input and accessibility vocabulary |
| `ui.PathCommand`, `ui.CapRound` | `gfx.PathCommand`, `gfx.CapRound` | Paths and strokes |
| `ui.TextStyle`, `ui.MonoFace`, `ui.WeightMedium` | `gfx.TextStyle`, `gfx.MonoFace`, `gfx.WeightMedium` | Text rendering |
| `ui.Metrics`, `ui.RGB24` | `gfx.Metrics`, `gfx.RGB24` | Theme metrics and colors |

This table is not a complete identifier count: the number used by a particular
application is not the size of the package API. The complete destination API is
in the version-pinned [v0.6.0 `gfx` package documentation](https://pkg.go.dev/github.com/vivaui/viva@v0.6.0/gfx).
Compare it with the [v0.5.1 `ui` package](https://pkg.go.dev/github.com/vivaui/viva@v0.5.1/ui)
when resolving an old name, rather than guessing from whether it sounds graphical.

For page-level use, `ui` still exposes `Color`, `Rect`, `Size`, `Point` and
`Theme` as aliases of the corresponding `gfx` types, plus `LightTheme`,
`DarkTheme`, `RGB` and `RGBA`. This is the complete shared vocabulary in
[v0.6.0 `ui`](https://pkg.go.dev/github.com/vivaui/viva@v0.6.0/ui).
**The old `ui.ID` type is not retained**; type annotations and composite
literals use `gfx.ID`. The new `ui.ID(c)` is a page helper function returning
`gfx.ID`, not a type alias. A page can use `ui.Rect`, but an application doing
custom drawing or input handling also needs `gfx`.

#### `Frame` / `View` become `Page` / `C`

The old `flex.Frame` and `flex.View` authoring API is no longer public. The
package reorganization briefly named these `ui.Frame` and `ui.View`; neither
is exported in v0.6.0. Declare an application or component with `func(c ui.C)`
and use functions such as `ui.Row(c)` and `ui.Text(c, value)` instead of frame
methods.

For an embedded region, the change looks like this:

```go
// v0.5.1 and earlier
var view flex.View
view.Draw(canvas, theme, bounds, func(frame *flex.Frame) {
    frame.Column().Fill().Children(func() {
        frame.Text("Hello")
    })
})
```

```go
// v0.6.0: canvas is *gfx.Canvas; bounds is ui.Rect or gfx.Rect.
page := ui.NewPage(ui.PageConfig{Theme: &theme})
page.Draw(canvas, bounds, func(c ui.C) {
    ui.Column(c).Fill().Children(func() {
        ui.Text(c, "Hello")
    })
})
```

Keep the `Page` in application state, not in a per-paint local: it owns the
page's identities, focus and retained state. For a whole window, use
`ui.Run(config, declaration)` or `page.Config(config)` to wire the window.
`PageConfig.View` is a declaration function, not the removed `View` type.

An embedded host forwards input through `Page.Key`, `Page.Pointer`,
`Page.Wheel` and `Page.TextInput`. Page drag and drop uses
`Box.DragSource` / `Box.DropTarget`; custom-painted parts use the corresponding
`DrawContext` methods. Do not replace old View-only drag wiring with `ui.Drag`,
`DragState.Pointer` or `DragState.Key`: these are not public in v0.6.0. The
`Frame` / `View` aliases in Viva's `_test.go` files are test-only, not a
compatibility API available to applications.

See the [v0.6.0 migration notes](releases/0.6.0.md#迁移--migration) for custom
controls, layout and host routing. When jumping from an older version, also
apply the v0.4.0 and v0.3.0 changes below.

#### Custom renderers: block elements become cell glyphs

The former `ui.ReplaceBlockElements` function and `ui.Canvas.BlockRectangles`
method are removed, not simply moved under the same names. Custom renderers
use `gfx.ReplaceCellGlyphs` and `gfx.Canvas.CellGlyphGeometry` instead. The
latter accepts and returns `gfx.CellGlyphShapes`, containing both `Rectangles`
and `Triangles`; render both, not just rectangles. The geometry now covers box
drawing, braille and geometric shapes as well as block elements.

Preserve the original text for copying and accessibility. Only the font-rendering
copy uses `ReplaceCellGlyphs`, after emitting the cell geometry, so a cell glyph
is not drawn twice. Applications using Viva's normal painter already get this
handling; they do not need to add a separate rendering pass.

### v0.4.0 - `flex.Box.HoverBackground` is removed

`Box.HoverBackground(ui.Color)` became `Box.Hover(Look)`: a hover state is a
whole look, not only a background.

```go
// before
row.HoverBackground(theme.Hover)

// after
row.Hover(flex.Look{}.Background(theme.Hover))
```

`Look` also carries `Border`, `Color`, `Ring`, `Shadow` and `Opacity`, so a hover
state can change more than the fill. `Box.Pressed` and `Box.Focus` take the same
type, which is how a control now says what it looks like while the pointer, or
the keyboard, is on it.

These are the names at v0.4.0. When targeting v0.6.0, the final spelling is
`row.Hover(ui.Look{}.Background(theme.Hover))` after the package migration above.

### v0.3.0 - `components` removes public members

`CodeEditor` is rebuilt on a character grid instead of the `Textarea` base. The
`CodeEditor.Layout` field and the `TextareaSnapshot` embedded in
`CodeEditorSnapshot` are gone, and the promotion it provided for
`DisplayValue`, `HideEditorText`, `InsetTop`, `InsetBottom`, `InsetRight`,
`Invalid`, `Layout`, `NativeEditing` and `NoWrap` no longer reaches
`CodeEditorSnapshot`.

To migrate:

- movement, pointer placement and scrolling are methods on `CodeEditor`:
  `Move`, `OffsetAt`, `Reveal`, `ScrollBy`, `Scroll`, `ScrollY` and
  `SetScrollY`;
- the display and geometry options that lived on the embedded field are written
  directly on `CodeEditorSnapshot`;
- `NoWrap` becomes `Wrap`, with the opposite sense.

## Checking an upgrade

For an application that only consumes the module, pin the intended target
instead of silently changing it while following this guide:

```sh
go get github.com/vivaui/viva@v0.6.0
go build ./...
go vet ./...
go test ./...
```

Use your chosen version in place of v0.6.0. For this target, remove all `flex`
imports, resolve low-level names against `gfx`, and migrate Frame/View call
sites to Page before treating a missing member as an unrelated error. If the
application uses a local `replace` directive, check that checkout too: the
selected dependency may not be the tag named by `go get`.

A successful build is not proof that the UI behaves correctly. Read the release
notes for the versions you skipped and exercise drawing, focus, text input,
scrolling, overlays and drag and drop on the paths the application uses. Notes
also list behaviour changes that do not fail a compile, such as which events a
hidden window still gets.

The gate a change to Viva itself has to pass is in
[CONTRIBUTING.md](../CONTRIBUTING.md).

## Where the releases are

[`releases/`](releases/) holds one file per release in the order they shipped,
from v0.2.2 on. Each is bilingual and supplies the matching GitHub Release body
at publication. Documentation corrections on `main` do not rewrite an existing
tag or automatically update its published release body; use this guide on
`main` for subsequent corrections to an older release's migration instructions.
