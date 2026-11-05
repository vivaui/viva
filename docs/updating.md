# Updating an application

Viva ships an application as a native program with no updater inside it. The
`github.com/vivaui/viva/update` package adds one when an application imports it,
and nothing is linked into a program that does not. It performs **full updates**:
the feed names a complete archive, the client downloads it, verifies it, and the
platform installer replaces the running application. Delta updates are not
implemented, and the feed format reserves a place for them without using it yet.

The package splits the work in two:

| Step | Who does it |
| --- | --- |
| Fetch and verify the feed | `update.Client.Check` |
| Download and hash the archive | `update.Client.Download` |
| Decide whether to ask, when to install | the application |
| Replace the application and relaunch | `update.Install` |

## The feed

A feed is two files served over HTTPS:

```
https://example.com/app/stable/latest.json
https://example.com/app/stable/latest.json.sig
```

`latest.json` is a small JSON document. The signature covers its exact bytes:
the client fetches both, verifies the signature, and only then parses the
document. A feed signed by a key the application does not trust is refused with
`update.ErrSignature`, and a feed that parses but names something the client must
not act on is refused with `update.ErrManifest`.

```json
{
  "schema": 1,
  "app": "dev.example.notes",
  "channel": "stable",
  "latest": {
    "version": "1.4.2",
    "date": "2026-11-04T12:00:00Z",
    "notes_url": "https://example.com/notes/1.4.2",
    "min_os": {"darwin": "12.0", "windows": "10.0"},
    "assets": [
      {
        "os": "darwin",
        "arch": "arm64",
        "format": "zip",
        "url": "https://example.com/app/stable/Notes-1.4.2-darwin-arm64.zip",
        "size": 18432000,
        "sha256": "9f2c…"
      },
      {
        "os": "windows",
        "arch": "amd64",
        "format": "nsis",
        "url": "https://example.com/app/stable/Notes-1.4.2-windows-amd64-setup.exe",
        "size": 20111360,
        "sha256": "41ab…"
      }
    ]
  }
}
```

`app` is the application's identity (`[app] id` in `viva.toml`). `schema` is `1`;
a feed with any other schema is refused rather than guessed at. `channel` is
optional, and a client configured with a channel requires an exact match. Each
asset names an OS and architecture (`darwin`, `windows`, `linux`; `amd64`,
`arm64`), a format (`zip`, `nsis`, `tar.gz`), a size and a lowercase SHA-256.

`min_os` is carried for the application: the client does not compare it, because
only the application knows what it supports.

### Keys

A feed is signed with Ed25519. The public key is compiled into the application;
the private key stays with the release machine. `viva update-keygen` writes both:

```sh
viva update-keygen -private update.key -public update.pub
```

The private key file holds the standard-base64 form of the 64-byte key, and the
public file the base64 of its 32 bytes. The default private key is `update.key`
in the project root, which this repository's `.gitignore` excludes; never commit
a key that signs releases. `update.ParsePublicKey` reads either
base64 or hexadecimal, so the public key can live in `viva.toml`, a `go:embed`
file, or a constant. More than one public key may be configured:

```go
client, err := update.New(update.Config{
	FeedURL:    "https://example.com/app/stable/latest.json",
	PublicKeys: []ed25519.PublicKey{currentKey, nextKey},
	App:        "dev.example.notes",
	Current:    version,
})
```

Two keys let a release rotate the signing key without the previous release
refusing the next feed. `latest.json.sig` holds the signature in standard
base64; raw base64 is accepted too.

## Publishing a feed

`viva feed` reads the archives `viva package` wrote and builds the two files:

```sh
viva package --target darwin --format zip
viva package --target windows --format nsis
viva package --target linux --format tar.gz

viva feed \
  -version 1.4.2 \
  -dir build \
  -base-url https://example.com/app/stable \
  -channel stable \
  -notes https://example.com/notes/1.4.2 \
  -key update.key \
  -out build/stable/latest.json
```

It recognises the names `viva package` produces — `<program>-<version>-darwin-<arch>.zip`,
`…-windows-<arch>-setup.exe`, `…-windows-<arch>.zip`, `…-linux-<arch>.tar.gz` — and ignores
everything else. The application identifier and version come from `viva.toml`
(`[app] id`, `[app] version`) unless `-app` or `-version` overrides them. The
version in every archive's file name must equal the feed's: a feed cannot name
one release and list an archive built for another. The
private key comes from `-key` or `VIVA_UPDATE_KEY`. The command writes
`latest.json` and `latest.json.sig`; the signature is over the exact bytes of
the JSON file, so the file must be published as written.

Every archive's URL is `-base-url` joined with its file name. A GitHub Release
works as the host: upload the archives and the two feed files, and point
`-base-url` at the release's asset directory. Channels are directories:

```
app/stable/latest.json
app/beta/latest.json
```

A feed can be checked from any HTTP client; the signature is the only thing that
makes it authoritative.

## Updating in the application

Check somewhere that does not block the UI thread, and hand the result back
through the window's invalidator:

```go
func (model *model) checkForUpdate() {
	go func() {
		feed, err := model.updates.Check(context.Background())
		if err != nil || feed == nil {
			return
		}
		model.window.Invalidate() // a field now says an update is available
	}()
}
```

`Check` returns `(nil, nil)` when the running version is not older than the
feed's, so an application does not need to compare versions itself. When the
user accepts, download into a directory the application owns and then install:

```go
feed, err := client.Check(ctx)
if err != nil || feed == nil {
	return
}
asset, err := feed.Asset(runtime.GOOS, runtime.GOARCH)
if err != nil {
	return
}
archive, err := client.Download(ctx, asset, downloadDir, progress)
if err != nil {
	return
}
err = update.Install(update.InstallOptions{
	Artifact: archive,
	Asset:    asset,
	Version:  feed.Latest.Version,
	App:      "dev.example.notes",
	Args:     os.Args[1:],
}, application.Quit)
```

`Install` returns only after it has started the helper that replaces the
application; the helper waits for this process to exit. If `Install` returns an
error nothing has been changed and `quit` was not called.

## What each platform does

| | macOS | Windows | Linux |
| --- | --- | --- | --- |
| Archive | the signed `.app` in a zip | the signed NSIS installer | the executable in a tar.gz |
| Checks | the bundle id and version in `Info.plist`, then `codesign --verify --deep --strict` | the installer's Authenticode signature when it has one; the feed's Ed25519 signature always | the feed's Ed25519 signature |
| Replaces | the `.app` bundle, by swapping it with two renames while the previous one is kept until the new version starts | runs the installer silently (`/S`), user scope | renames the new executable over the old one; the old inode keeps running until exit |
| Relaunches | `open -n` through a detached helper after the process exits | a detached PowerShell helper waits, installs, and starts the program | starts the new executable, then the application quits |
| Refuses | a translocated bundle, a read-only parent, a bundle without a valid signature | an installer whose signature fails to verify, a read-only install directory | an AppImage, a `.deb` installed by dpkg, a read-only directory |

A program that a package manager installed is not updated by itself. Linux
detects the `.deb` case with `dpkg-query` and reports
`update.ErrManagedExternally`; a macOS Homebrew Cask and a Windows package-managed
install cannot be told apart from a drag-installed copy, so the application has
to avoid offering self-update there. Windows installations made by the Viva NSIS
installer are per-user by default and need no elevation.

## Security properties

- **HTTPS only.** The feed, its signature and every archive must be `https`.
  A redirect that leaves HTTPS is refused (`ErrInsecureURL`).
- **Signature before parse.** The detached signature covers the exact feed
  bytes and is checked before the JSON is decoded.
- **One identity.** A client configured with an app or channel refuses a signed
  feed for another one, and `Install` refuses an archive the feed listed for
  another operating system or architecture.
- **Exact bytes.** A download is written while it is hashed and is kept only
  when both its length and SHA-256 match the feed (`ErrSize`, `ErrChecksum`).
  A refused download leaves no file behind. `Install` hashes the file again
  immediately before handing it to the platform, because the file sits on disk
  between the two calls.
- **Bounded.** `latest.json` is limited to 1 MiB, its signature to 4 KiB, an
  archive to 512 MiB by default (`Config.MaxAssetBytes`), and an unpacked
  archive to 2 GiB.
- **Signed before replacing.** The macOS installer verifies the new bundle with
  `codesign --verify --deep --strict`, and the Windows installer verifies an
  Authenticode signature when the installer carries one. A signature that
  exists but fails is `ErrUntrusted`; an unsigned Windows installer is accepted
  because the feed is the trust anchor.
- **No downgrade.** A feed older than the running version is treated as
  "nothing newer".
- **No telemetry.** The package sends no identifier and nothing but the
  configured requests. It does not check at import or on a timer.

The feed's signature says the release came from whoever holds the signing key;
that is the trust anchor. The macOS installer additionally verifies that the new
bundle's code signature is intact (`codesign --verify --deep --strict`), and the
Windows installer verifies an Authenticode signature when one is present.
Neither pins a publisher identity: a valid signature from any developer passes.
An unsigned Windows installer is accepted because the feed authenticated its
bytes; SmartScreen still warns about it, and signing it is what removes that
warning.

## Limits

- **No delta updates.** A download is the whole archive. The feed schema leaves
  room for patches; no client reads them yet.
- **No update service.** The package checks a URL the application names. It does
  not host, stage, roll out or force anything.
- **No automatic install.** The application decides when to check, whether to
  ask, and when to call `Install`.
- **No Linux AppImage support** and no self-update of a `.deb`. `viva package`
  makes `tar.gz` and `.deb`; the `.deb` is the package manager's.
- **No elevation.** The Windows helper runs the installer without asking for
  administrator rights, so a `[windows] scope = "machine"` installer cannot
  replace a program installed under `Program Files`; the default per-user scope
  needs none.
- **macOS needs a code signature.** The new bundle is checked with `codesign`;
  an ad-hoc signature is enough for local testing, an unsigned build is refused.
- **No Windows arm64 installer changes.** The NSIS installer is per-architecture,
  as `viva package` makes it.

## Testing an update path

`go test ./update` covers the feed parser, signatures, version ordering,
download checks and archive unpacking on every host. The parts that replace a
real application need a machine and a person:

1. Build version 1, package it, install it, run it.
2. Build version 2, package it, run `viva feed`, host the files over HTTPS.
3. Start version 1, check, download, install, and confirm version 2 starts.
4. Repeat with the network dropped mid-download, with a tampered archive, and
   with a feed signed by another key.
5. On macOS, confirm `codesign --verify --deep --strict` passes on the new
   bundle and that the app opens with the network off (the ticket is stapled).
6. On Windows, confirm the installer is silent, per-user, and that the new
   program starts after the old one exits.
