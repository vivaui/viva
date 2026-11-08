# Documentation in this directory

These are **repository documents**, not user documentation. They record how the
framework is built, measured, and verified — they assume you have a checkout and
can run its programs.

| If you want to | Read |
| --- | --- |
| Use Viva in an application | the guides in [`website/content/guides`](../website/content/guides), or run the site: `cd website && npm run dev` |
| Look up an API | the API reference on that site, generated from the Go source and its comments |
| Build the framework itself | [../CONTRIBUTING.md](../CONTRIBUTING.md) and [../README.md](../README.md) |
| Check platform behaviour | [conformance.md](conformance.md) |
| Have a coding agent build on Viva | [skills/building-with-viva](skills/building-with-viva/SKILL.md), published on the website as the [coding agents guide](../website/content/guides/en/coding-agents.md) and at `/skills/building-with-viva/SKILL.md`: copy or symlink the directory into your project's `.claude/skills` or `.agents/skills` |
| See what was measured | [benchmarks/](benchmarks/) |

## What is here

- [conformance.md](conformance.md) — the scenarios that need a real window on a
  real platform, and the ones that still need eyes.
- [accessibility.md](accessibility.md) — the semantic tree and native
  accessibility.
- [system-integrations.md](system-integrations.md) — tray, menus, notifications,
  file drops, and the rest of the native surface.
- [web-deployment.md](web-deployment.md) — the browser build, its capability
  matrix, lifecycle mapping, static-hosting headers and security limits.
- [diagnostics.md](diagnostics.md) — the structured event stream, its slog
  adapters, the recent-event ring and fault context.
- [updating.md](updating.md) — the signed feed, the client API, and what each
  platform's installer replaces.
- [initial-frame.md](initial-frame.md) — what the first frame does and does not
  do.
- [issue-commands.md](issue-commands.md) — the `/claim`, `/unclaim` and `/close`
  commands on issues, and the optional bot identity.
- [releases.md](releases.md) — who writes the notes, who publishes the Release,
  and what the tag-triggered workflow does and cannot check.
- [windows-text-editing.md](windows-text-editing.md) — what the Windows text
  field overlay owns, how its texture is bounded, and what still needs a real
  window.
- [benchmarks/](benchmarks/) — every measurement, with the machine, the build
  mode, and the method recorded alongside the numbers.
- [benchmarks/2026-10-25-performance-audit-darwin-arm64.md](benchmarks/2026-10-25-performance-audit-darwin-arm64.md)
  — where the next performance gains are: hot spots, phase attribution, and a
  prioritized roadmap, each backed by a measured number. §10 records what was
  implemented afterwards and re-measures it against the same commit.

## Writing a document here

A document that makes a claim about speed, size, or memory must also record the
reference machine, the operating system, the build mode, and how the number was
taken. Numbers without that context do not belong in this directory.
