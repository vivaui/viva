# Test environment

`tools/testenv` wraps a shared Linux machine used for real-window and platform
tests. It runs what cross-compiling and `go vet` cannot: a real window, a real
event loop, and a real desktop.

It lives on a local network and the control API has no authentication, so
anything that can reach it can run commands as its user. Use it for throwaway
work only, and never put credentials, keys, or personal files on it.

Its address, port and password are deliberately not in this repository. Put
them in `tools/testenv/local.sh`, which git ignores; `tools/testenv/env.sh`
sources it.

Windows is not part of this harness. Run the Windows build on a real Windows
machine and record what was not automated.

## Machine

| | Linux |
| --- | --- |
| System | Debian 12 x86_64, KDE Plasma on X11 (a real StatusNotifier tray), Xvnc 1920x1080 |
| Graphics | software Vulkan only (lavapipe), no GPU |
| API | `${VIVA_LINUX_ENV}` |
| Watch the screen | a VNC view served beside the API, self-signed |
| User | an ordinary desktop user with sudo |

## HTTP API

The machine serves:

- `POST /execute` with JSON `{"command": [...], "shell": false}`
- `POST /setup/upload` with form fields `file_path` and `file_data`
- `GET /screenshot`, returning a 1920x1080 PNG

Calls are synchronous and cut off after about a minute. Start long work in the
background and read its log. Disk changes persist until the environment is
recreated.

The machine has no Go. Cross-compile with `GOOS=linux` and `CGO_ENABLED=0`,
upload, then run.

## Commands

```sh
tools/testenv/exec.sh linux 'command'                  # run a command
tools/testenv/upload.sh linux LOCAL REMOTE
tools/testenv/screenshot.sh linux out.png
tools/testenv/go-test.sh linux ./internal/pkg [flags]  # cross-compile tests, run them there
```

Set `VIVA_LINUX_ENV` when calling `tools/conformance/run.sh` or
`tools/visualparity/run_all.sh`, or put the export in `tools/testenv/local.sh`
(ignored by git); see `tools/testenv/env.sh`.

## Linux notes

- GUI programs need `DISPLAY=:1` and `XAUTHORITY=/home/user/.Xauthority`. A
  changed `HOME` loses the latter.
- The session bus is the one in `/proc/<plasmashell pid>/environ`.
- Software Vulkan is either a private copy under a private `/tmp` directory (the variables
  are in `tools/conformance/run.sh`) or, on a freshly rebuilt machine, the
  system's. Install it with `sudo apt-get update` followed by
  `apt-get install mesa-vulkan-drivers`, as a `setsid nohup` script; the package
  index is stale until updated.
- `xdotool` is installed.
- Many threads in a Viva process there are lavapipe's pool, not Viva's.

## Platform conformance

`tools/conformance/run.sh all` builds `cmd/viva-conformance` and runs it on this
Mac and the Linux container. The scenarios and the checks that still need eyes
are documented in [`docs/conformance.md`](../../docs/conformance.md). Windows
conformance is run by hand on a real Windows machine.
