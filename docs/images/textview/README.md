# Textview frames

These are real Metal/CoreText screenshots of `ui/textview`, captured on macOS
27.3 / arm64 at 800 × 420 logical pixels, scale 2 (1600 × 840 PNGs). Both Viva
themes use the same English and Chinese content, with combining accents, emoji
sequences, a flag and an Indic conjunct in the Unicode scene.

From the repository root, capture the current implementation with:

```sh
VIVA_TEXTVIEW_SHOTS="$PWD/build/textview-review/after" \
  go test -tags vivahead ./ui/textview -run '^TestVisualCapture$' -count=1 -timeout=60s -v
```

`TestVisualCapture` opens no native window and closes every headless session. Its
Unicode scene has no selection. Its sibling scene double-clicks `Second` in the
second plain view, then right-clicks line 1 in the second scrollable view. It also
declares a button after the two views with context menus and checks strict control
identity before saving the frame.

- `unicode-before-{light,dark}.png`: production sources from
  `0e17e516264fdb0a102cca3d2fcc4d2a8807c5a7`, captured with the same Unicode scene.
  For the comparison, those five source files were copied unchanged to the ignored
  `build/textview-review/before` package and the capture test's import pointed there.
- `unicode-after-{light,dark}.png`: the implementation and capture test in this commit.
- `siblings-menu-{light,dark}.png`: the implementation and capture test in this commit.
  The old implementation panics before this scene's first frame, so it has no
  corresponding before image; `TestClosedMenusLeaveTheNextSiblingsIdentityAlone`
  reproduces the panic on the old implementation.

The comparison shows the retained accent, joined emoji and flag, and shaped Indic
conjunct. Cell advances still use the sum of Viva's `text.CellWidth` values, so an
emoji sequence can occupy more grid space than its visible glyph. This change does
not redefine Viva's fixed-pitch grid. These captures do not establish native
window, clipboard or accessibility integration on any desktop platform.
