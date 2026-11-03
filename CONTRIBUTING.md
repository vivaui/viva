# Contributing to Viva

Thanks for looking. Viva is a Go-first, GPU-native desktop framework. It ships
source-owned components and links only what an application uses, so a change that
makes the framework bigger or busier for people who do not use it is a
regression even when the feature works.

## Before you start

- **Any day of the week:** bug reports, platform findings, documentation fixes,
  and failing tests are always welcome. Open an issue.
- **Before a large change:** open an issue describing what you want and why. API
  shape, package boundaries, and anything on the rendering or input hot path are
  worth agreeing on before the code exists.
- **Small fixes:** send the pull request directly.

## Claiming an issue

Comment `/claim` on an issue to take it: the bot assigns you and posts a
confirmation. Comment `/unclaim` (or `/unclaimed`) to release it. Once the pull
request that fixes it has merged, the assignee comments `/close` if GitHub did
not close the issue itself. What each command refuses, and how to run the bot as
a GitHub App, is in [docs/issue-commands.md](docs/issue-commands.md).

## Reporting a bug

Include what you ran, what you expected, and what happened. For anything visual
or platform-specific, the operating system, the GPU or the software renderer, and
the window system (Cocoa, Win32, X11 with which compositor) carry most of the
diagnostic value. If a program misbehaves, the smallest program that reproduces
it is better than a description of your application.

## Where a change belongs

Viva is developed against a real application, so the first question is always
whether something belongs in the framework or in the app.

- Reusable capability — windows, input, text, layout, components, rendering,
  accessibility, lifecycle, headless control — belongs in Viva.
- Product behaviour — a protocol, a storage schema, a workflow, a specific
  feature of one application — stays in the application.

A missing framework capability must not be hidden behind application-local
wiring: fix the gap, or say in the pull request why the behaviour is genuinely
product-specific.

## What a pull request needs

Before opening one, run the gate:

```sh
test -z "$(gofmt -l $(git ls-files '*.go'))"
go vet ./...
go test ./... -count=1
```

Also run these when your change touches the platform backends or the headless
surface:

```sh
go vet -tags vivahead ./internal/platform/darwin ./headless/...
GOOS=windows GOARCH=amd64 go vet ./internal/... ./app/
GOOS=linux   GOARCH=amd64 go vet ./internal/... ./app/
GOOS=js      GOARCH=wasm  go vet ./internal/platform/web/ ./app/
```

For a change on a drawing, input, or layout path, include the before and after
numbers from the relevant benchmark. For a change that alters what gets linked,
include the stripped binary size before and after. A change to a persistent
structure needs its memory numbers. `go test -bench` and `benchstat` are the
usual tools; the budgets and the reasons behind them are in
[README.md](README.md).

## Continuous integration

`.github/workflows/ci.yml` runs that gate in parallel pieces, and
`.github/scripts/test-packages.sh` decides which package goes where:

- **Linux** — formatting, `go vet ./...` for the Linux build, a type-check of
  every other target Viva ships for, and the tests, split over three runners. A
  package with no darwin-only file compiles to the same source on every
  operating system, so it is tested here once instead of on macOS as well.
- **macOS** — the packages that carry a darwin-only file, which only a macOS
  runner can build. It is the only job that spends macOS minutes, and those cost
  ten times what Linux minutes do.
- **Windows** — the window-material contracts, which need WARP.

The check to require in branch protection is **ci passed**. It is the one job
that always runs: the rest are skipped when a change only touches documentation,
and a required check that a path filter skipped would stay pending and hold the
pull request shut. It never skips a release tag.

A pull request keeps only its newest run. A commit on `main` is never cancelled,
so every commit a release is cut from has run the whole gate to the end.

Write the test that fails when the behaviour breaks, not the test that fails when
someone next tidies the file. A single test that protects a contract is worth
more than five that pin down incidental geometry or exact draw order.

Real windows matter. A change to window, focus, tray, close, or frame scheduling
behaviour should be checked on an actual desktop, not only in a unit test, and
the pull request should say which platforms you saw it on. Do not describe
something as working on a platform you did not run it on.

## Style

- Match the file you are editing. Comments explain why, not what.
- Public API is defined at the point of use. Implementation stays `internal`
  until a real external case needs it.
- Avoid `common`, `shared`, `helpers`, `utils`, and `base` packages. Resolve an
  import cycle through ownership rather than by moving code into a dumping
  ground.
- Convenience must not cost an allocation on a paint or input path, and must not
  use reflection or `map[string]any` to hide what it does.

## Code of conduct

Issues, pull requests, and discussions are covered by
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Discuss the code and the measurement,
not the person.

## Commits

One kind of change per commit, with a message that says what changed and why.
Prefixes in use: `ui:`, `app:`, `flex:`, `components:`, `text:`, `scene:`,
`platform:`, `darwin:`, `windows:`, `linux:`, `docs:`, `website:`, `tools:`,
`build:`, `test:`.

## License

Viva is licensed under the Apache License 2.0. By contributing you agree your
contribution is licensed under the same terms. There is no copyright assignment
and no contributor license agreement to sign.
