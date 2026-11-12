# rssprobe

What a Viva application keeps resident, measured from outside: the tool runs a
child binary with `GODEBUG=gctrace=1`, samples the process's resident set size
on a cadence, and reads the Go heap's live size out of the child's own GC
trace. RSS alone cannot tell retention from arena high-water, and the heap
alone cannot see textures, stacks and the runtime; the two together can.

It links nothing into the application and works on any Go binary, so the same
procedure measures a viva example and another repository's application.

## Local sanity check

The headless build has no real window, so its numbers are not the product's —
but it proves the probe works end to end:

```sh
go build -tags vivahead -o /tmp/hello-head ./examples/guides/hello
go run ./tools/rssprobe -settle 3s -duration 10s -label hello-head -- /tmp/hello-head
```

## The resident-memory scenarios

Run each on the platform's real machine and record the summary block in
`docs/benchmarks/<date>-rss-<platform>.md`, with the reference machine, the
build flags and the binary. The scenarios a resident-memory decision needs:

| Scenario | Command shape | What it answers |
| --- | --- | --- |
| one window, settled | the app with its first window, `-settle 30s -duration 60s` | the floor: runtime + one window's textures |
| more windows | open 2 and 4 windows before sampling | per-window cost (device, pipelines, label textures) |
| after a burst | drive a text-heavy page (or a terminal flood), then sample | how much of the peak stays resident |
| after closing | close the windows or sessions, then sample | what a closed window leaves behind |

On macOS run the binary from a terminal with the probe around it. On the Linux
test machine, drive the window through `tools/testenv`'s API and keep rssprobe
on the host side. On the Windows machine, run it through `tools/realwin`; the
probe reads `tasklist` there, so no extra tools are needed.

## Reading the numbers

- `heap live` is what the program still needs. It appears from the child's own
  GCs: a settled idle window may genuinely never run one — the framework's
  idle invariant is zero allocation — and then this line stays unknown, which
  is itself a result. If heap live grows across a scenario that returns to its
  starting state, retention leaked on the Go side.
- `rss final` minus `heap live` is the runtime, the binary, stacks and the
  platform's own buffers. Compare it across the window-count scenarios: the
  slope is the honest per-window cost. A large gap after a burst that never
  closes is arena high-water; decide with `GOMEMLIMIT` before calling it a
  leak.
- `rss max` is what a user watching Activity Monitor sees during the burst;
  report it beside the settled figure, never instead of it.
