---
title: Build and package
summary: viva.toml, reproducible release builds, hot reload during development, and installers for each desktop.
order: 12
group: distribute
---

The `viva` command builds, packages and develops applications. It is development tooling: nothing in it is linked into your program.

Install it once, then run it from your project. Pick whichever fits your machine:

| Where | Command |
| --- | --- |
| macOS, Linux | `brew install vivaui/tap/viva` |
| Windows | `scoop bucket add vivaui https://github.com/vivaui/homebrew-tap`, then `scoop install viva` |
| Any system | Download the archive for your platform from the [releases page](https://github.com/vivaui/viva/releases), check it against `SHA256SUMS`, and put `viva` on your `PATH`. |
| You already use Go | `go install github.com/vivaui/viva/cmd/viva@latest` |

Prebuilt archives and the Homebrew and Scoop packages start with `v0.1.3`; for `v0.1.2` and earlier use `go install`.

```sh
viva version           # what you installed
viva doctor            # is this machine ready to build?
viva help              # every command
viva help package      # one command's flags
```

### You do not need Go installed first

`viva build`, `viva dev` and `viva init` run the `go` tool. When there is none, or it is older than Go 1.26, viva downloads the official toolchain once into your user cache (`~/Library/Caches/viva/go` on macOS, `~/.cache/viva/go` on Linux, `%LocalAppData%\viva\go` on Windows), checks it against the SHA-256 that go.dev publishes, and uses it for its own commands only. It says so before it downloads. Your `PATH` and any Go you installed are left alone. Three environment variables control it:

- `VIVA_GO` names a `go` binary, or a Go installation directory, to use instead.
- `VIVA_GO_DOWNLOAD=0` forbids downloading; viva then stops with instructions instead.
- `VIVA_GO_MIRROR` is the download index to use instead of `https://go.dev/dl/`, for example `https://golang.google.cn/dl/`.

The download goes through `HTTPS_PROXY` like any Go program. On macOS you still need the Xcode command-line tools, and on Linux the XCB and Vulkan libraries: those are system software viva cannot install for you, but `viva doctor` names what is missing and the command that fixes it.

### Pin viva to the project

The recipes `viva add` writes come with the command, so the command and the Viva your project requires should be the same version. `viva init` therefore adds a `tool` line to `go.mod`, and `go tool viva` runs exactly the version the project pins; upgrading Viva upgrades the command with it, and CI needs no separate install:

```sh
go tool viva add --out ./components/ui button
go get -tool github.com/vivaui/viva/cmd/viva@v0.1.3   # pin an existing project
```

`viva doctor` and `viva add` warn when the command in your `PATH` and the project disagree.

## viva.toml

`viva build` and `viva package` read an optional `viva.toml` next to your `go.mod`:

```toml
[app]
name = "Notes"                 # shown in the menu bar; names the .app
id = "dev.example.notes"       # bundle identifier
version = "0.1.0"
executable = "notes"           # default: the name, lowercased
package = "./cmd/notes"        # the main package to build
icon = "assets/icon.png"       # PNG, square, 128 px or more (1024 recommended), or .icns

[web]
package = "."                      # the main package that calls app.RunCanvas builds for the browser too; name another when the app uses native-only code

[build]
tags = []                      # build tags for every target
out = "build"

[mac]
min_system = "12.0"            # the macOS the program is compiled for; default and floor 12.0
category = "public.app-category.productivity"
entitlements = "app.entitlements"   # optional: signed into the bundle with the hardened runtime
```

The file is decoded strictly. An unknown table or key, a duplicate or a value of the wrong type is an error that names the line, so a typo never changes what gets built. `[mac.dmg]` shapes the window the `.dmg` opens in: `background` (a PNG, tagged 144 dpi for a 2x picture; its size in points is the window's size), `icon_size`, and `app_position` / `applications_position` as `[x, y]` points from the window's top left to an icon's middle. Without it the image opens in a 540×380 window with a plain light ground and an arrow from the application to the Applications shortcut. There are also `[mac.info]` for extra `Info.plist` strings, `[windows]` (publisher, install scope, URL) and `[linux]` (menu category, comment, maintainer, dependencies, keywords) for the installers.

## Release builds

```sh
viva build                                  # a macOS program in build/ (target darwin, arm64, by default)
viva build --target linux --arch amd64
viva build --target windows --arch amd64
viva build --target web                     # build/web/: program, loader, index.html
```

A release build uses `-trimpath`, strips symbols and fixes the build ID, so the same sources give the same bytes. It reports the size of the program on its own, apart from anything around it. macOS programs are compiled for `min_system`, not for the macOS of the machine building them. Windows release builds use the GUI subsystem; `--debug` keeps symbols and the console.

## Packaging

```sh
viva package                                    # build/Notes.app
viva package --format app,zip,dmg
viva package --target windows --format nsis,zip
viva package --target linux --format tar.gz,deb
viva package --program ./notes-signed           # package an executable you built or signed yourself
```

| Target | Formats |
| --- | --- |
| macOS | `.app`, reproducible `.zip`, `.dmg` |
| Windows | NSIS installer (needs `makensis`), or `.zip` |
| Linux | `tar.gz` with an install script, or `.deb` |

**Signing is optional.** With nothing configured nothing is signed: macOS blocks a downloaded bundle and Windows SmartScreen warns about the installer. `--program` exists so you can build, sign, and then package that exact executable yourself.

On macOS `viva package` can sign and notarize for you. It signs only when an identity is given, and everything secret comes from the environment, never from `viva.toml` or the command line:

| Variable | Meaning |
| --- | --- |
| `APPLE_SIGNING_IDENTITY` | the identity to sign with (`Developer ID Application: …` or its hash); `-` signs ad hoc, which only the Mac that made it accepts. `--sign-identity` overrides it |
| `APPLE_CERTIFICATE`, `APPLE_CERTIFICATE_PASSWORD` | a base64 `.p12` imported into a temporary keychain of its own, for a CI machine; its identity is used if none is named |
| `APPLE_NOTARY_PROFILE` | a profile made by `xcrun notarytool store-credentials` |
| `APPLE_API_KEY`, `APPLE_API_KEY_ID`, `APPLE_API_ISSUER` | an App Store Connect API key (path to the `.p8`) |
| `APPLE_ID`, `APPLE_PASSWORD`, `APPLE_TEAM_ID` | an Apple ID with an app-specific password |

The bundle is signed with the hardened runtime and a secure timestamp (and `[mac] entitlements`, or `--entitlements`), then verified. With notarization credentials as well, a `.dmg` is signed, submitted, waited for and stapled; a bundle without a `.dmg` is submitted as a zip and the ticket stapled to the bundle before it is zipped. A rejection fails the run with the reasons Apple gives. `--notarize` makes missing credentials an error instead of a skipped step, and `--no-sign` skips everything on a machine that is set up for it. 

On Windows the program and the installer are signed with Authenticode when a certificate is configured, again only from the environment: `WINDOWS_CERTIFICATE_FILE` (a `.pfx`) or `WINDOWS_CERTIFICATE` (the same, base64) with `WINDOWS_CERTIFICATE_PASSWORD`, and `WINDOWS_TIMESTAMP_URL` (an RFC 3161 server, DigiCert's by default). The signing is done by `osslsigncode`, which runs on any host (`brew install osslsigncode`, `apt install osslsigncode`), or by `signtool` on Windows. A certificate whose key stays in a hardware token or a cloud service, such as Azure Trusted Signing, cannot be a file: give `--sign-command` (or `WINDOWS_SIGN_COMMAND`) a command that signs `{file}` in place, for example `--sign-command 'mysigner --file "{file}"'`. It is split into words at blanks (double quotes group) and run without a shell. A program you supply with `--program` is used as it is, signed or not; the installer built around it is still signed. `--no-sign` skips all of it. Linux packages are not signed. The Homebrew build of `makensis` on Apple silicon crashes, so build Windows installers on Linux or Windows.

## Hot reload

```sh
viva dev ./cmd/notes
```

`viva dev` rebuilds on change and swaps the worker process while the window and its state stay. The reload machinery is development-only: release builds do not link it.
