---
title: Performance and size
summary: The budgets and long-term goals Viva is built to, what has actually been measured, the gap between them, and what to avoid in your own code.
order: 15
group: more
---

Viva is built for small binaries and low idle cost. Those are **design targets**, not claims. Every result below comes from a report in the repository, on the machine and build named in it. Treat them as baselines for that machine rather than promises about yours.

## Budgets

For a minimal native application:

| Budget | Target |
| --- | --- |
| Stripped executable | at most 4 MiB (review above 8 MiB) |
| Idle CPU, GPU submissions and presents | effectively zero |
| Framework goroutines while idle | at most 2 |
| An unchanged frame | no layout, no scene rebuild, no GPU submit |

Startup and memory are tracked on two lines. The **budget** is the acceptance line: going over it is a regression. The **goal** is the long-term target: it may be missed for a while, but every release reports the gap.

| Metric | Budget (acceptance) | Goal (long term) |
| --- | --- | --- |
| Warm start to first interactive frame | at most 400 ms | at most 50 ms |
| Idle resident memory | at most 32 MiB | at most 15 MiB |

When goals conflict, the order is low memory, then runtime performance, then small artifacts, then correctness, then ergonomics and cross-platform consistency, and last the optional browser target.

## What has been measured

M0 is the minimal macOS window: a Metal clear and present with no components. Measured on Apple silicon, macOS 15.5, Go 1.26.5, built with `-trimpath -ldflags='-s -w -buildid='`.

| Metric | Result | Budget | Verdict |
| --- | --- | --- | --- |
| Stripped executable | 1,697,506 bytes | at most 4 MiB | pass |
| Settled CPU | 0.0% | effectively zero | pass |
| Idle GPU submissions | 0 after initial present | zero | pass |
| Physical footprint after a real Metal present | 26.3–26.8 MiB | at most 32 MiB | pass (goal 15 MiB missed) |
| First scheduled frame | 123.075–373.643 ms | at most 400 ms warm | pass (goal 50 ms missed) |

An earlier 15 MiB figure was taken before any Metal command was submitted, and was rejected as a baseline. The fixed cost of AppKit and Metal has not yet been separated from Viva's own, so the startup and memory budgets sit at the measured envelope: going over them is a regression. The 15 MiB and 50 ms goals stand, and the report records the gap to each. The full reports, with machine details and reproduction commands, are in the repository's `docs/benchmarks/`.

The retained scene (M1) drains an unchanged 10,000-node scene in 2.012 ns with no allocations, and updates one node of 10,000 in 12.39 ns with no allocations.

## What to avoid in your own code

Viva can only keep the idle cost at zero if the application does too.

- **Do not repaint on a timer.** Call `Invalidator.Invalidate` when state changes, and let the window sleep otherwise.
- **Do not allocate in `Paint`.** It runs on every repaint, and the framework's own steady-state paths are written to allocate as little as possible, close to nothing. Reuse buffers and keep per-frame work proportional to what changed.
- **Keep callbacks short.** They run on the window thread. Do slow work on another goroutine and publish the result through `OnWake`.
- **Link only what you use.** Optional subsystems live in their own packages and build tags, so an unused one adds no code to your binary.

## Text working sets

The platform text engine shares shaped lines between measurement, caret queries,
hit-testing, and drawing. Its cache is bounded: currently 1,024 lines per style on
Windows, Linux, and the browser, and 2,048 on macOS. Different text, font sizes, or
weights use different entries; regular and italic text have separate caches.
Font changes invalidate the affected lines. Native memory per entry also depends
on the length and complexity of the text.

Once a working set fits and is cached, repeated queries reuse those lines without
shaping or allocating again. An oversized working set cannot all stay resident.
A recency window and frequency-based admission protect reused lines from
one-pass scans, rather than clearing a whole generation at once; misses still
require platform shaping. Keep large lists virtualized and avoid measuring an
entire history on every repaint. Cache retention is not a substitute for that.

## Reproducing

The `benchmarks/` directory holds the scripts behind the macOS and Linux baselines, and every report names the command that produced it. `viva build` reports a program's size on its own, separate from anything around it.
