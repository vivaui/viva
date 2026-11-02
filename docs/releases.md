# Releases

Viva ships as a Go module tag. A release is an annotated tag on `main` plus the
GitHub Release that carries the notes, and `.github/workflows/release.yml`
publishes that Release once the tag is pushed. The only binaries are the `viva`
command's archives, which the workflow builds and attaches itself: no agent or
person uploads anything. They are what Homebrew, Scoop and a plain download use.
Applications built with Viva are never part of a release.

## Who owns what

The releasing agent owns the tag and the notes. The workflow owns going public:
it runs the full gate from `ci.yml`, checks the draft the agent left behind, and
flips it from draft to published. Nothing is published by a local command.

1. The agent writes the bilingual notes for `v<version>` to `docs/releases/<version>.md`
   (for example `docs/releases/0.2.2.md`; the format is in the releasing skill) and shows
   them, the exact `main` HEAD, the gate results and the notes digest at a confirmation
   gate. The file stays untracked until the gate is confirmed.
2. After confirmation the agent commits the notes (`docs(release): v<version> notes`),
   pushes `main`, creates an annotated tag at that commit whose body is a short English
   summary, and pushes the tag. The notes are in the repository at the tag, so the
   Release body can be compared with them afterwards.
3. The agent creates the draft release immediately after the push, so GitHub does
   not create a lightweight tag of its own:

   ```sh
   gh release create "v<version>" --repo vivaui/viva --verify-tag --draft \
     --title "Viva v<version>" --notes-file docs/releases/<version>.md
   ```

4. The tag push starts `release.yml`. Its `gate` job calls `ci.yml`; only if that
   passes do `build-darwin` (macOS runner: the program reaches Cocoa through cgo)
   and `build-other` (Linux and Windows, no cgo) build the `viva` archives with
   `tools/release/build.sh`. Then `publish` waits up to ten minutes for the draft,
   requires the release name `Viva v<version>` and a non-empty body, writes
   `SHA256SUMS` (`tools/release/sums.sh`) and, for a final release, the Homebrew
   formula and Scoop manifest (`tools/releasepkg`), checks that the linux program
   reports the tag in `viva version`, uploads everything to the draft, requires the
   draft's asset list to be exactly what was built, and sets `draft=false` by
   release id (and `prerelease=true` for a tag that contains `-`). The `tap` job
   then copies the manifests into `vivaui/homebrew-tap` (below).
5. The agent verifies the live release with the command below, which only reads:

   ```sh
   git fetch --tags origin
   go run ./tools/releaseverify -tag v<version>
   ```

   It checks the title, that the release is published and is a prerelease only for a tag
   with a `-`, the exact asset list (six archives, `SHA256SUMS`, and `viva.rb` and
   `viva.json` unless it is a prerelease), that each archive's GitHub digest equals its
   line in `SHA256SUMS`, that the tag is annotated and on the release commit, and that the
   Release body is `docs/releases/<version>.md` at that commit. It also lists the "current
   release" references that do not name the version yet. For a final release the agent also
   checks `brew install vivaui/tap/viva` or the tap's commit, and that `viva version`
   prints the tag. Then it commits the "current release" references (`README.md`,
   `README.zh-CN.md` and the two `getting-started` guides), runs the command again so
   nothing is left to do, and records the release in its ledger.

A release is a straight sequence: the notes are written before the tag, and the
tag never waits on a background notes task.

## The viva archives

Six archives, `viva_<tag>_<os>_<arch>.tar.gz` (`.zip` on Windows) for darwin,
linux and windows on amd64 and arm64, each holding `viva` (or `viva.exe`) and
`LICENSE`. They are stripped, `-trimpath` builds with the tag linked in
(`-X main.version`), and byte-for-byte reproducible from the same source and Go
toolchain. They are not code-signed or notarized: a browser download on macOS
carries a quarantine flag that Gatekeeper checks, while `brew`, `curl` and
`go install` do not set it. Notarizing them is a separate, optional step that
needs the Apple credentials `viva package` already documents; `SHA256SUMS` and
the digests inside the formula and manifest are the integrity check today.
`build.sh` also runs locally, for any target the host can build:
`tools/release/build.sh v0.1.3 /tmp/dist linux/amd64`.

## The tap

`vivaui/homebrew-tap` is one repository serving both package managers:
`Formula/viva.rb` for `brew install vivaui/tap/viva` and `bucket/viva.json` for
`scoop bucket add vivaui https://github.com/vivaui/homebrew-tap`. It must exist
before the first release that carries archives (create it empty, public), and the
Viva repository needs an Actions secret `TAP_TOKEN` holding a token that can push
to it. Without the secret the `tap` job only says so and the release is still
complete: commit the output of a local `go run ./tools/releasepkg -version
v<version> -sums <SHA256SUMS> -out <dir>` into the tap by hand. Prereleases are
never sent to the tap. winget is not automated; submit a manifest to
`microsoft/winget-pkgs` (for example with `komac`) from the published archives
when someone asks for it.

## What the workflow cannot check

- Platform conformance. `cmd/viva-conformance` needs a real desktop on macOS, a
  Windows VM and a Linux X session; the release runner has none of them. Run
  `tools/conformance/run.sh` from [conformance.md](conformance.md) before
  tagging, and record what was not run in the confirmation gate.
- The release notes' wording and their consistency with the committed tree. The
  workflow can only require a non-empty body; `tools/releaseverify` compares the
  published body with `docs/releases/<version>.md` at the tag afterwards, and a person
  still has to read the notes against the commits.
- The "current release" references. They are a separate commit after the
  release exists, so the published notes can be the only place that names a
  version for a few minutes.

## Rerunning

`workflow_dispatch` takes a tag, so a tag whose workflow failed can be retried
without moving it:

```sh
gh workflow run release.yml -f tag=v<version>
```

A release that is already published is left alone (the job records "already
published" and skips, and does not touch its assets). If the draft was never
created the publish job fails after ten minutes with the `gh release create
--draft` command to run. A draft that already carries archives from an earlier
failed run has them replaced; any other asset on it stops the job.

## Rolling back

Before anything consumes the tag, delete the release and then the tag:

```sh
gh release delete "v<version>" --yes
git push origin ":refs/tags/v<version>"
git tag -d "v<version>"
```

Once a downstream `go.mod` has pinned the version, do not delete the tag. Ship a
patch release, explain the problem in its notes, and only mark the old release
as a prerelease if it must stop resolving as latest. After either path, check
that `releases/latest` and the version references point at the release you meant
to keep.
