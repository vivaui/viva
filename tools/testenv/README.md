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
| System | Ubuntu 22.04.3 x86_64 desktop VM, GNOME on X, 1920x1080, `DISPLAY=:0` (set by `VIVA_LINUX_DISPLAY`) |
| Graphics | no discrete GPU; software Vulkan (lavapipe) is installed |
| Audio | PulseAudio with an `auto_null` sink; ALSA's `default` PCM plays there |
| API | `${VIVA_LINUX_ENV}` |
| Watch the screen | a web desktop at the port beside the API (no password), and a native VNC port on the other side; `local.sh` records both |
| User | an ordinary desktop user with sudo (`user`, password in `local.sh`) |

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

- GUI programs need `DISPLAY=$VIVA_LINUX_DISPLAY` (currently `:0`) and
  `XDG_RUNTIME_DIR=$VIVA_LINUX_RUNTIME_DIR`. The current machine needs no X
  authority file; `VIVA_LINUX_XAUTHORITY` is empty. An older Xvnc machine used
  `:1` and `/home/user/.Xauthority`; both scripts read the variables so neither
  is hard-coded.
- The session bus is `/run/user/1000/bus`. The conformance script falls back to
  the desktop shell's environment (`gnome-shell` or `plasmashell`) if the socket
  is absent.
- Software Vulkan is the system's lavapipe (the ICD is in
  `/usr/share/vulkan/icd.d`). The private `/tmp/viva-vulkan` copy remains a
  fallback for machines without it.
- `xdotool` and `vulkaninfo` are not installed on the current machine.
- Many threads in a Viva process there are lavapipe's pool, not Viva's.

## Platform conformance

`tools/conformance/run.sh all` builds `cmd/viva-conformance` and runs it on this
Mac and the Linux container. The scenarios and the checks that still need eyes
are documented in [`docs/conformance.md`](../../docs/conformance.md). Windows
conformance is run by hand on a real Windows machine.
