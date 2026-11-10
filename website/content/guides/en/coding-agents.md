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

## Move an existing application to the page model

An application written with `ui.Frame` can be moved to pages one window at a time. A window is one page; a page cannot be embedded in a Frame window with its input forwarded by the host any more. Give the agent a prompt like this, after installing the skill:

```text
Use the building-with-viva skill. Move the window in <file or package> from ui.Frame to the
page model: one page function `func (m *model) page(c ui.C)` opened with ui.Run or
ui.NewPage(...).Config(base). Replace hand-made gfx.Tree identities and ID comparisons in
OnPointer with .Clicked() where each control is declared, hand-painted controls with
ui.Control and ui.Handlers, dialogs with ui.Dialog or ui.Modal, and drags with ui.DragState.
Do not keep a Frame, Frame.Page, C.Frame() or Page.Key/Pointer/Owns/Overlay bridge, and do not
add application-local hit testing for what Viva provides. Keep behavior and layout the same.
Do one window at a time; after each, run gofmt, go vet, viva vet and the tests, and check the
interface with the headless driver. Report what you could not check.
```

Keep a `ui.Frame` only where the code is a component that paints through `ui.View.Draw`.

## Keep it current

The website serves the file that is in `docs/skills` in the repository, rebuilt with every site build, so the download and the text shown on this page always match the guides next to it. Run the same command again after you upgrade Viva, and commit the file with your project so everyone who works on it, human or agent, reads the same briefing. The [programming model](/en/docs/programming-model/) guide is the place to send a person who wants the reasoning behind the rules.
