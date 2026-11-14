// Package update checks for and downloads updates to an application built
// with Viva.
//
// It is an opt-in package: an application that never imports it links none of
// it, and nothing here starts a goroutine, a timer or a connection on import.
//
// # The feed
//
// An update feed is a small JSON document and a detached Ed25519 signature
// beside it, both served over HTTPS:
//
//	https://example.com/app/latest.json
//	https://example.com/app/latest.json.sig
//
// The signature covers the exact bytes of latest.json, so the document cannot
// be read unless a key the application trusts signed it. latest.json.sig is
// the signature in standard base64. A feed names one application, one release
// channel and one release, and lists the archives that release ships, each
// with its size and SHA-256.
//
// # Checking and downloading
//
// Check fetches the feed, verifies the signature and returns the release when
// it is newer than the running version: a check that finds nothing newer
// returns (nil, nil). Download streams the archive for the running platform,
// hashing as it writes, and refuses a file whose size or digest is not the one
// the feed declared. Neither function sends an identifier, telemetry or
// anything else the caller did not configure.
//
// # What this package does not do
//
// It does not show a dialog, decide whether updating is allowed, check on a
// timer, or stage a release. The application decides when to call Check, what
// to ask, and when to call Download and Install.
//
// # Delta updates
//
// A feed may list patches for an asset, each between an earlier version's
// installed payload and this one. PatchFrom finds one for the running version;
// DownloadPatch fetches and verifies it; ApplyPatch applies it to the running
// application and returns a staging directory holding the new payload. Install
// accepts that payload in place of an archive. Windows downloads the full
// installer: the NSIS installer owns the installation, so it has no payload to
// patch.
//
// # Installing
//
// Install hands the verified archive to the running platform: on macOS a
// detached helper waits for the process to exit, swaps the .app bundle with two
// renames while keeping the old one until the new version starts, and reopens
// it; on Windows the signed NSIS installer runs silently
// and starts the new program; on Linux the executable is renamed into place and
// started. An application a package manager owns reports ErrManagedExternally
// instead. The application supplies the quit function, so it keeps control of
// its own event loop and of when the process ends.
package update
