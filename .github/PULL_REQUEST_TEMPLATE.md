<!-- One kind of change per pull request, with a message that says what changed
and why. See CONTRIBUTING.md. -->

## What this changes

## Why

<!-- The problem it solves. If it fixes an issue, link it. -->

## How it was verified

<!-- Which of these ran, and what they printed. -->

- [ ] `test -z "$(gofmt -l $(git ls-files '*.go'))"`
- [ ] `go vet ./...`
- [ ] `go test ./... -count=1`
- [ ] platform vet, if the backends are touched
- [ ] benchmarks before and after, if a paint, layout, or input path changed
- [ ] stripped binary size before and after, if what links changed
- [ ] a real window on: <!-- macOS / Windows / Linux — list what you actually ran -->

## Visual evidence

<!-- Delete this section if nothing on screen changes. A picture is not the
contract; the test above is. Give a way to reach the same frame, so a reviewer
can rerun it instead of trusting an image. A screenshot from another platform,
theme or scale is not comparable; say which one yours is. -->

- Changed: <!-- the screen or page, and the state it is in -->
- Reach it by: <!-- the clicks and keys, or a -tags vivahead build with
  VIVA_HEADLESS=1, then `shot` / `layout` -->
- Evidence: <!-- a screenshot, or what the command printed -->
- On: <!-- macOS / Windows / Linux, theme, scale -->
- Protected by: <!-- the test that fails when this behaviour breaks -->

## Where the boundary sits

<!-- Does this add framework capability, or product behaviour? If it belongs to
an application, say why it is in the framework anyway. Reusable capability goes
in Viva; a protocol, a storage schema, or one application's workflow does not. -->

## Cost

<!-- What a program that does not use this feature pays. Nothing is the expected
answer. -->
