---
title: Build with a coding agent
summary: Install the Viva skill so an agent follows the framework's rules and checks the interface without opening a window.
order: 16
group: start
skill: building-with-viva
---

Viva draws its own interface, repaints only on request and has no layout cache, so an agent that writes it like a web or retained-mode toolkit produces code that review rejects. The `building-with-viva` skill is a short briefing that an agent loads before it touches a Viva application. It is one Markdown file with no code to run.

## What the skill covers

- How to decide whether a need is application behavior or a reusable desktop capability that belongs in Viva rather than in the application.
- The rules that review rejects: no reflection or property bags in the paint path, no work while the application is idle, no per-paint allocation, no hand-numbered hit targets and no unmeasured claims about speed.
- How a screen is written: identities in a `gfx.Tree`, clicks handled where the control is declared, layout with `ui`.
- How to verify a change without opening a window, in the order described in [Test without a window](/en/docs/testing/), and what to say about the parts that could not be checked.
- Where the guides, examples and measurements are.

It points to the guides instead of repeating them, so it stays short enough to read in full and does not go stale when a guide changes. Its full text is at the bottom of this page.

## Install

Download the file into your project's skills directory. Claude Code reads `.claude/skills`; other agents that follow the same convention read `.agents/skills`.

```sh
mkdir -p .claude/skills/building-with-viva
curl -fsSL https://vivaui.dev/skills/building-with-viva/SKILL.md \
  -o .claude/skills/building-with-viva/SKILL.md
```

In a checkout of Viva itself, copy or symlink `docs/skills/building-with-viva` instead. The repository's `AGENTS.md` takes precedence over the skill there.

If your agent does not load skills, copy the full text at the bottom of this page into the instructions file it does read, such as `AGENTS.md` or `CLAUDE.md`, or point it at this page.

## Keep it current

The website serves the file that is in `docs/skills` in the repository, rebuilt with every site build, so the download and the text shown on this page always match the guides next to it. Run the same command again after you upgrade Viva, and commit the file with your project so everyone who works on it, human or agent, reads the same briefing. The [programming model](/en/docs/programming-model/) guide is the place to send a person who wants the reasoning behind the rules.
