# Web Deployment

`RunCanvas` on `GOOS=js GOARCH=wasm` is a deployment target, not a visual
preview. It runs the same component tree, input router, text shaping and
accessibility model as the native applications, paints them through Canvas2D,
and keeps the browser event loop free by making every platform operation that
is asynchronous in the browser asynchronous in the API too.

The page is the window. A web build has one document, one canvas and one
application loop. Anything that needs several operating-system windows, a tray,
a native web view or a file-system path is outside that model and is reported
as unsupported rather than emulated.

`app.Run` and `app.RunCanvas` both start that page: `Run` is the ordinary
single-window entry point, and `RunCanvas` is for an application that already
owns a retained `ui.Canvas`. A nil `Paint` is accepted, leaving a canvas painted
by the caller on screen, exactly as on a native target. `app.RunApplication`
returns `ErrUnsupported` on the web because a document hosts one window; a
multi-window application needs a native backend and should branch on
`app.CurrentPlatform() == app.PlatformWeb`.

## Build and serve

```sh
viva build --target web -C path/to/app
```

The output directory contains the `.wasm` program, `wasm_exec.js` from the Go
toolchain that built it, and a generated `index.html`. The same sources build
for the native targets; a browser entry point is named by `[web] package` in
`viva.toml` or by the package argument.

Serve the files over HTTP or HTTPS, not `file://`:

- `*.wasm` should be `application/wasm` for streaming compilation. The generated
  loader falls back to `fetch` and `WebAssembly.instantiate` when a host serves
  `application/octet-stream`, so a misconfigured MIME type costs startup time
  rather than breaking the page, but fixing the header is the right deployment.
- `wasm_exec.js` must come from the same Go toolchain as the `.wasm` file.
- HTTPS (or `http://localhost`) is required for `crypto.subtle`, the Clipboard
  API, notifications and fullscreen. A page on plain HTTP still paints, but
  those integrations fail with an unsupported or permission error.
- Do not rewrite a missing `.wasm` request to `index.html`. A host that returns
  HTML for every path produces a confusing compile error instead of a 404.
- Cache `index.html` briefly and fingerprint the `.wasm` and `wasm_exec.js`
  names, or disable caching for all three while iterating.

### Headers

A restrictive starting point for a page that loads only its own assets:

```http
Content-Type: application/wasm
Content-Security-Policy: default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; object-src 'none'; base-uri 'none'
Permissions-Policy: clipboard-read=(self), clipboard-write=(self), fullscreen=(self), notifications=(self)
```

`script-src` needs `'wasm-unsafe-eval'` (or `'unsafe-eval'` on older engines)
because Go's WebAssembly runtime compiles a module. Viva also sets inline
styles on its canvas and hidden text-input element, hence `style-src
'unsafe-inline'`. `img-src blob:` is needed when an application draws a
downloaded image. Clipboard reads also require the browser's own permission;
the policy only permits the request.

Cross-Origin-Opener-Policy and Cross-Origin-Embedder-Policy are not required:
Viva does not use `SharedArrayBuffer`. Add them only if an embedding page needs
them, and verify that the application's own network integrations still work
under the resulting isolation.

## Capabilities

Capabilities are static for the build target; a runtime permission can still
refuse an individual call.

```go
if app.Supports(app.CapabilityFileDialogsAsync) {
    err := app.OpenFilesAsync(app.OpenFileOptions{AllowMultiple: true}, func(files []app.FileRef, err error) {
        // Read, save or hand the files to application work.
    })
}
```

The browser reports these integrations:

| Capability | Browser behaviour |
| --- | --- |
| `CapabilityFileDrop` | Dropped files arrive with `ui.FileDropEvent.Files`; no paths |
| `CapabilityClipboardTextAsync` | `navigator.clipboard.readText` / `writeText` |
| `CapabilityClipboardImageAsync` | `navigator.clipboard.read`; PNG only |
| `CapabilityFileDialogsAsync` | `showOpenFilePicker`, or an `<input type=file>` fallback |
| `CapabilitySecretStoreAsync` | AES-GCM through WebCrypto, ciphertext in IndexedDB |
| `CapabilityNotifications` | `Notification` after the page has permission |
| `CapabilityNotificationPermission` | `Notification.requestPermission` |
| `CapabilityFontRegistrationAsync` | `FontFace` loading a font the application carries |
| `CapabilityOpenURL` | `window.open` with a new, opener-less tab |
| `CapabilityFullscreen` | `RequestFullscreen` and `OnFullscreen` |
| `CapabilityPageVisibility` | `visibilitychange` as `OnMinimize` |
| `CapabilityReduceMotion` | `prefers-reduced-motion`, watched for changes |

Synchronous clipboard reads, synchronous file pickers, directory pickers,
clipboard-file lists, multi-window, status items, web views, custom URL
schemes, native menus, system sounds, file icons, font enumeration, and the
primary selection are not reported. The corresponding synchronous APIs return
`ErrAsyncRequired`, `ErrUnsupported` or `ErrSecretStoreUnavailable`; they never
return a fabricated path or result.

`SystemCapabilities()` describes the build, not the current permission state.
Call the operation and handle its error when a permission matters.

## Files

A browser does not grant a path. It grants the contents the user picked, for as
long as the page keeps the handle. Viva represents that with `FileRef`, which
has a name, size, MIME type and an asynchronous `Read`, and with `FileSink`,
whose `Write` appends and `Close` commits. There is deliberately no conversion
from a browser `File` to a string path.

```go
app.OpenFilesAsync(app.OpenFileOptions{Title: "Import key"}, func(files []app.FileRef, err error) {
    if err != nil {
        return // app.ErrCanceled when the picker was dismissed
    }
    files[0].ReadAll(func(data []byte, err error) {
        // data is the file the user chose.
    })
})
```

`OpenFilesAsync` and `SaveFileAsync` must be called from an input handler,
because browsers open pickers only from a user gesture. `ReadAll` allocates the
whole file; use `Read(offset, length, done)` for a file whose size is not
acceptable in memory.

The browser implementation uses the File System Access API where it exists. It
falls back to an `<input type=file>` for opening and to a Blob download for
saving. A download fallback buffers written bytes in memory until `Close`, so a
large export should use a browser with `showSaveFilePicker`. Both paths produce
the same `FileRef` / `FileSink` shape.

Native targets call `done` before `OpenFilesAsync` or `SaveFileAsync` returns,
with the same result as the synchronous picker. A browser calls it later. In
both cases `done` runs exactly once.

`OnFileDrop` carries `Files` on every target. Native targets also carry the
absolute `Paths`; browsers carry `Files` only. A directory has `IsDir` set and
cannot be read. Dropped folders are not expanded recursively.

## Secrets

`StoreSecretAsync`, `LoadSecretAsync` and `DeleteSecretAsync` are the browser
secret store. The first store generates a 256-bit AES-GCM key with
`crypto.subtle.generateKey`, keeps the key non-extractable, and stores it in
IndexedDB by structured clone. Each secret is encrypted with a fresh 12-byte IV
and the service/account pair as additional authenticated data. The ciphertext
and IV live in the same origin's IndexedDB.

This protects secrets at rest from casual inspection of site data. It does not
protect them from script running in the same origin: malicious script can ask
the browser to use the key. A deployment that loads third-party script must not
keep SSH keys or tokens in this store. HTTPS is required because
`crypto.subtle` is absent in insecure contexts. Clearing site data removes the
key and therefore makes the stored ciphertext unrecoverable; that is the
intended failure mode.

The synchronous `StoreSecret`, `LoadSecret` and `DeleteSecret` return
`ErrSecretStoreUnavailable` on the Web target. Native builds keep their
Keychain, Credential Manager or Secret Service implementation.

## Fonts and text

`RegisterFontAsync` loads embedded TrueType/OpenType data with `FontFace`. The
family, OS/2 weight and italic flag are read from the font itself, so the
regular, italic and bold files of one family combine as they do natively.
Registering the same bytes again shares the registration; the last unregister
removes the face. Use it before the first paint, or repaint after it completes.

`SetInterfaceFontFamily` works on the Web target through the same shaped-line
engine. `FontFamilies` and `MonospaceFamilies` are empty because browsers do not
expose the machine's installed fonts. Use `ui.MonoSystem`, `ui.MonoClassic`, or
a family registered by the application.

Text is shaped and measured in CSS pixels through the document, and painted by
Canvas2D at `devicePixelRatio`. Widths, caret positions, selections and hits
therefore agree with the glyphs the browser draws. They are not promised to be
pixel-identical to macOS CoreText, Windows DirectWrite or Linux Pango: font
fallback, hinting and DPI rounding differ. A browser zoom change invalidates
measurements, as does a font that finishes loading.

## Lifecycle

The page is the window, so the lifecycle callbacks map onto document events:

| Callback | Browser event | Meaning |
| --- | --- | --- |
| `OnResize` | `resize` | Viewport size changed; not called for the initial size |
| `OnMinimize` | `visibilitychange` | The tab became hidden or visible; a hidden tab is not necessarily minimized |
| `OnFullscreen` | `fullscreenchange` | Entered or left the browser's fullscreen mode |
| `OnCloseRequest` | `beforeunload` | Ask before the page unloads; `allow` must be called synchronously |
| `OnClosed` | `pagehide` (not persisted) and `unload` | The page is being torn down |
| `OnFileDrop` | `dragenter`, `dragover`, `dragleave`, `drop` | Files dragged from outside the page |

`OnCloseRequest` cannot guarantee that the page stays open. Browsers require a
prior user interaction before showing the confirmation, may ignore a refused
unload, and never wait for asynchronous work. Save critical state before the
user closes the tab; use the callback only for a final synchronous check.

`OnClosed` is not called when the page enters the back/forward cache
(`pagehide` with `persisted` true). The runtime is frozen there and resumes on
`pageshow`; it has not closed.

`OnFrameChange`, `OnMaximize`, `OnReopen`, `TitlebarHeight`, `WindowMode`,
window controls, tray items, native menus and native web views have no browser
counterpart. They are ignored by `RunCanvas` rather than simulated.

Frames are event-driven. `Invalidator` and `Waker` schedule work through
`requestAnimationFrame`; a hidden page does not animate. The browser may
throttle timers in a background tab, so a background connection should be
prepared to pause.

## Input and accessibility

The canvas owns pointer, wheel, keyboard and IME input. It removes the browser
context menu over the canvas, sets `touch-action: none` and
`overscroll-behavior: none`, and uses pointer capture for drags. Browser-reserved
chords (for example Command/Control+W, Command/Control+T, F5 and the browser's
own fullscreen key) may never reach the page; an application cannot override
them.

Text fields and terminals use the page's IME through a hidden editable element.
Focus, selection and caret positions remain the application's. A terminal is a
preedit surface: its committed text arrives through `TextInput.OnEvent`, and
the browser editor itself is transparent over the cells the canvas paints.

`TextInput.KeyReleases` works in the browser as it does natively. Modifier,
named and printable keys arrive as matching press/release pairs with stable
physical-key identity; releases are never sent for presses the application did
not see, and every held key is released when the page loses the keyboard or
becomes hidden. `AltSendsEscape` is honoured for printable Alt/Option chords.
Plain text, including a bare space, still goes through the browser text service
unless the terminal asked for raw Alt handling, so input-method composition
keeps working.

The web painter uses the same input router and the same hit-region contract as
the native painter: regions are ordered by compositing layer, semantic
containers are not hit targets, `HitOptions.Autofocus` and
`Canvas.RequestFocus` move focus once, and `HitOptions.Cursor` /
`CursorImage` update the CSS pointer. Autofocus and focus requests are resolved
against the hit list of the repaint that reveals them, so an overlay may move
focus in the same frame that paints it.

The accessibility mirror publishes the same semantic tree as the native builds
as ARIA; it is not a second DOM widget tree. A screen reader's activation and
focus moves are routed through the same focus and click pass as a pointer, then
publish the resulting frame and semantic tree. A hidden page performs no
painting: a wake still runs `OnWake` so application state can advance, but the
frame is deferred until the page becomes visible, mirroring a covered native
window.

The browser mirror publishes activation, focus and range changes. A slider with
a `Range` is exposed as a visually hidden `input[type=range]` with the same
bounds and step; when a screen reader changes it, the runtime presses the same
arrow, Home and End keys the application already handles, so the canvas value
stays the application's state rather than a second copy. Text fields,
terminals and comboboxes that request platform editing share the hidden
editable element, including committed text, selection and input-method
composition.

The browser does not expose the native adapters' direct text-replacement and
selection actions as separate DOM actions. Those edits go through the focused
textbox, which is the browser's equivalent path.

### Parity boundary

The application-facing API, component tree, canvas contract, input router and
semantic model are shared. The interaction core — hit regions and layer order,
focus, autofocus, pending focus, Escape targets, held-key pairing, press
capture, click completion, cursor selection and text-editor ownership — lives in
`internal/interaction` and is used by both painters. The painters themselves
remain platform implementations: the native one publishes to the retained
scene, renderer and native views, while the browser one paints Canvas2D and
drives the DOM accessibility mirror. The browser backend is still an
intentional subset where the document cannot supply a platform service. A
behavior added to one painter must still be checked against the other and
covered where it is observable without a real window.

## Design references

Viva's web backend follows patterns that established cross-platform frameworks
use rather than inventing a private browser runtime:

- Flutter's platform dispatcher and asynchronous platform channels for
  capabilities that only the host can answer;
- Compose Multiplatform's `expect`/`actual` split for shared UI state with
  platform-specific services;
- Tauri's explicit capability/permission model, which keeps browser-visible
  APIs an intentional subset rather than an accidental one;
- Fyne, Gio and Ebiten for Go event-loop and `js/wasm` input/rendering
  integration;
- xterm.js, Monaco and CodeMirror for hidden-input IME bridges, keyboard
  press/release handling and accessible text-editor mirrors;
- MDN and web.dev guidance for Page Visibility, `pagehide`/bfcache, File System
  Access, Clipboard, WebCrypto, IndexedDB and Fullscreen;
- W3C ARIA Authoring Practices for the ARIA mirror and keyboard semantics.

These are design references, not dependencies. Viva keeps Canvas2D drawing,
the Go input router and the application-facing API under its own control; it
does not embed a browser engine in a native build.

## Verification

The WebAssembly backend is built in CI with `GOOS=js GOARCH=wasm go vet ./...`,
the wasm tests run under `go_js_wasm_exec`, and the component preview, gallery,
hello guide and webview guide are built for the browser.

The browser-only integrations were exercised in Chrome 157 on localhost:

- `RegisterFontAsync` loaded an embedded TrueType font and returned its family;
- `StoreSecretAsync` / `LoadSecretAsync` / `DeleteSecretAsync` completed an
  AES-GCM round trip through IndexedDB;
- a DOM `File` was read through `FileRef.Read` with the expected bytes;
- the `<input type=file>` fallback opened through CDP, accepted a chosen file,
  and returned its name and contents;
- the download fallback of `SaveFileAsync` wrote the expected bytes to the
  browser's download directory.

The shared interaction core is covered by platform-independent contract tests
in `internal/interaction`: layered hit ordering, semantic-container exclusion,
focus surviving a repaint, pending focus visible to the paint that reveals it,
one-shot autofocus, key repeat and orphan-release pairing, press capture and
click completion, cursor precedence, text-input ownership and slider steps.
The web painter adds wasm tests for pointer event passes, pass-through
backdrops, pointer cancel, a press that leaves the page, terminal key
press/release pairing, focus-loss release, accessibility activation/focus
routing and a static caller-painted canvas with a nil `Paint`.

`tools/webcheck` is the browser-level gate. Its fixture paints a counter, a
slider, a pass-through backdrop and a real single-line text field backed by a
`text.Editor`. The Playwright driver asserts that a real pointer click, Enter
on the focused button, Tab traversal, a captured slider drag delivered through
`Canvas.PointerOn`, a screen-reader range change and a backdrop that dismisses
and then reaches the control beneath all work in Chromium, Firefox and WebKit.

The text field covers plain keyboard entry through the browser's editing
element, a synthetic composition (`compositionstart`, `compositionupdate`,
`compositionend`) whose marked text is published while composing and committed
exactly once to the Go editor, a canceled composition that removes its marked
text, and typing that resumes after composition ends.
The composition is generated at the DOM level: it verifies the event contract
between the browser editor and the Go text service, not an operating system
input method or candidate window.

CI builds the fixture with `viva build --target web`, installs the pinned
Playwright package and its three engines, and runs the driver once per engine
in a dedicated `web-browser` job that runs in parallel with the wasm tests.
The same gate runs locally with:

```sh
npm ci --prefix tools/webcheck
tools/webcheck/node_modules/.bin/playwright install chromium firefox webkit
go run ./cmd/viva build --target web ./tools/webcheck/fixture -o build/webcheck
node tools/webcheck/check.mjs --dir build/webcheck --browser all
```

A headless Chrome run served the generated `index.html`, `.wasm` and
`wasm_exec.js` over HTTP, rendered the gallery, and then invoked the DOM
`click` action on an ARIA component node. The application opened the component
page and the ARIA mirror published the new tree, which exercises the complete
browser callback -> input router -> click pass -> frame/accessibility publish
path. That run caught and now guards a missing frame request in the ARIA
bridge; the unit tests alone would not have observed it because they end at the
painter.

The same browser run opened the Slider page, found the published
`input[type=range]` with its bounds and step, set it to a valid value and
dispatched an input event. The application moved from 40 to 60 and the ARIA
`aria-valuenow` followed, exercising the screen-reader range path end to end.

Drag-and-drop delivery is covered by the shared drop router test with browser
file handles; a physical drag from Finder/Explorer still needs a real-window
check on each target. The engine gate runs Playwright's Chromium, Firefox and
WebKit builds; system Safari through `safaridriver`, a physical IME candidate
window, system screen readers and the browser's own reserved shortcuts still
need real-hardware checks.
