# Windows text field editing

The Windows native text overlay owns editing for an ordinary text field: one that
is not a terminal and does not set `TextInput.Placed`. It takes the standard
clipboard chords, select all, Home/End, word and document navigation, and word
delete before the application's `OnShortcut` sees them
(`internal/platform/windows/shortcut_windows.go`, `keys_windows.go`). The mapping
is shared with the headless `app.Driver` through
`internal/platform/text_editing.go`, so driver tests exercise the same chords.

The native field is one line: `painter.applyTextEvent` folds every insert through
`text.SingleLine`. That path is shared, so a single-line field strips control
characters on every platform, and the clean case allocates nothing.

## Overlay texture

The overlay's label is clipped to the field. A scrolled line rasterizes only the
device columns its clip shows: `platform.LabelWindow` computes the skip from the
label origin and clip, and the Direct2D rasterizer draws the shaped line that many
whole device columns to the left. A long value therefore costs its visible width
in texture memory, not its whole width, and the whole-column skip leaves the
ClearType phase unchanged. This matters because the label is rebuilt on every
keystroke while the field has focus.

## Verification

The Win32 package tests pass under the race detector, but they do not open a
window, own the system clipboard, or show an input-method candidate list. Real
window and physical-keyboard behaviour still needs the manual checks in
[conformance.md](conformance.md) and `tools/conformance/run.sh all`. On a host
that is not Windows only the geometry (`LabelWindow`) is tested; the Direct2D
drawing itself is not.
## Disabling the input method

`TextInput.DisableIME` is for a surface that shows another machine's desktop,
a remote-desktop or VNC view. While it is focused the window's IMM32 context is
disassociated (`ImmAssociateContext(hwnd, NULL)`): the host's input method
neither composes nor answers its own switch hotkeys there, so those keys travel
to the machine on the other side, which runs its own input method — the way
Microsoft's own client behaves. A window whose input stops asking, or loses
focus to none, gets its default context back.

## Performance baseline

Windows amd64, Intel Core Ultra 5 125H, Go 1.26.8, 2026-10-26, measured on this
branch before the texture-window change:

```sh
go test ./internal/platform/windows -run '^$' -bench '^BenchmarkNative(TextVisuals|EditingKeyMap)$' -benchmem -count=1
```

These are warmed native text-overlay redraws with the real text engine and an
unchanged value. They measure the text engine's work for one frame, not the
texture upload, GPU presentation, or OS clipboard latency; one sample is a local
baseline, not a performance guarantee.

| Field / runes | Before ns/op | After ns/op | Before B/op / allocs | After B/op / allocs |
|---|--:|--:|--:|--:|
| Ordinary / 32 | 276.6 | 235.8 | 0 / 0 | 0 / 0 |
| Ordinary / 256 | 1150 | 801.1 | 0 / 0 | 0 / 0 |
| Ordinary / 4096 | 15989 | 10410 | 0 / 0 | 0 / 0 |
| Secure / 32 | 1032 | 529.3 | 768 / 8 | 0 / 0 |
| Secure / 256 | 5270 | 3124 | 6144 / 8 | 0 / 0 |
| Secure / 4096 | 77972 | 46839 | 98304 / 8 | 0 / 0 |

The initial PR is the "Before" column. The Ctrl+Shift+Left command map measured
2.955 ns/op, 0 B/op, 0 allocs/op after review (2.944 ns/op before). A password
length change still allocates a new mask; redraws of the same length reuse it.

## Persistent memory and linked size

`platform.TextState` is 64 B, 8 B more than before the PR for the selection
direction. `windowState` grows 56 B per window (1,608 to 1,664 B), including the
two embedded text-state layouts. A secure field keeps a bullet mask of 3 UTF-8
bytes per rune plus allocator overhead, replaced when the length changes and
never holding the password: a 32-rune password holds 96 B of mask.

Stripped framework examples, windows/amd64, `-trimpath -ldflags="-s -w"`, against
main before the change:

| Executable | Before | After | Delta |
|---|--:|--:|--:|
| hello (no text field) | 3,700,224 | 3,731,968 | +31,744 B (+0.86%) |
| textfield | 3,708,928 | 3,740,160 | +31,232 B (+0.84%) |

Both examples grow by about the same 31 KiB: the shared `text` and `platform`
packages the runtime links are what grew, not the example. The paired Motty
build, a 23.7 MB binary, measured +14,848 B (0.063%) over the same change with
`-H=windowsgui`; the two artifacts differ, so compare each with its own base.

