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

Newest first. This list was checked against [`releases/`](releases/) for every
release from v0.2.2 to the newest tag: only the two below remove or change public
members, and every other release says in its own notes that the public API only
gains members.

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

For an application that only consumes the module:

```sh
go get github.com/vivaui/viva@latest
go build ./...
go vet ./...
go test ./...
```

If a call site no longer compiles, the release that removed it is above, with
its migration. If the build is green but the screen moved, read the release
notes for the versions you skipped: they list the behaviour changes that do not
fail a compile, such as which events a hidden window still gets.

The gate a change to Viva itself has to pass is in
[CONTRIBUTING.md](../CONTRIBUTING.md).

## Where the releases are

[`releases/`](releases/) holds one file per release in the order they shipped.
Each is bilingual and is the body of the matching GitHub Release, so the two can
be compared afterwards.
