# Test environments

`tools/testenv` wraps two shared machines used for real-window and platform
tests. They run what cross-compiling and `go vet` cannot: a real window, a real
event loop, and a real desktop.

They live on a local network and the control APIs have no authentication, so
anything that can reach them can run commands as their user. Use them for
throwaway work only, and never put credentials, keys, or personal files on them.

Their addresses, ports and passwords are deliberately not in this repository.
Put them in `tools/testenv/local.sh`, which git ignores; `tools/testenv/env.sh`
sources it.

## Machines

| | Linux | Windows |
| --- | --- | --- |
| System | Debian 12 x86_64, KDE Plasma on X11 (a real StatusNotifier tray), Xvnc 1920x1080 | Windows 11 Pro (Chinese), x86_64, 4 cores, 8 GB, KVM |
| Graphics | software Vulkan only (lavapipe), no GPU | display-only GPU, so Direct3D 11 runs on WARP |
| API | `${VIVA_LINUX_ENV}` | `${VIVA_WINDOWS_ENV}` |
| Watch the screen | a VNC view served beside the API, self-signed | a noVNC view served beside the API |
| Native VNC | — | a second port beside the API |
| RDP | — | a port beside the API |
| Local tunnel | — | proxied to a local port |
| User | an ordinary desktop user with sudo | a local administrator |

## HTTP API

Both machines serve:

- `POST /execute` with JSON `{"command": [...], "shell": false}`
- `POST /setup/upload` with form fields `file_path` and `file_data`
- `GET /screenshot`, returning a 1920x1080 PNG

Calls are synchronous and cut off after about a minute. Start long work in the
background and read its log. Disk changes persist until the environment is
recreated.

Neither machine has Go. Cross-compile with `GOOS=linux` or `windows` and
`CGO_ENABLED=0`, upload, then run.

## Commands

```sh
tools/testenv/exec.sh linux|windows|cmd 'command'      # run a command (cmd: cmd.exe)
tools/testenv/upload.sh linux|windows LOCAL REMOTE
tools/testenv/screenshot.sh linux|windows out.png
tools/testenv/go-test.sh linux|windows ./internal/pkg [flags]   # cross-compile tests, run them there
tools/testenv/windows-click.sh X Y [left|right]        # the VM has no xdotool
```

Set `VIVA_LINUX_ENV` and `VIVA_WINDOWS_ENV` when calling
`tools/conformance/run.sh`, or put the exports in `tools/testenv/local.sh`
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

## Windows notes

- PowerShell 5.1 splits `-test.run=x` at the dot, so pass flags through
  `cmd.exe`.
- Starting a GUI program from a call that keeps its output handles makes the call
  hang. Start it with `Start-Process` and redirect both streams.
- Output with control characters breaks the JSON reply; filter logs to printable
  text.
- Windows 11 puts a new tray icon in the overflow flyout: the arrow at the right of the taskbar,
  then the icon in the flyout.
- Pass `APPDATA` to give a program its own settings directory.
- Scoop is installed (user-level, `C:\Users\Docker\scoop`) with the `vivaui`
  bucket added, so `scoop install <tool>` works for dependencies and for
  reproducing the documented `scoop install viva` path. The service process
  holds the PATH it started with, so a shim added inside the VM is not on
  `PATH` for later calls; prefix it yourself:
  `$env:PATH = "$env:USERPROFILE\scoop\shims;" + $env:PATH`.
  Scoop refuses to install as administrator unless passed `-RunAsAdmin`, which
  is what the elevated API needs; it still writes a per-user tree.
- Keep tools installed between tasks. `git`, `python`, PowerShell 7 and archive
  tools come up in later work, and reinstalling costs more than the disk saved.
  Only remove something genuinely one-off and large, and say what was removed.

## Platform conformance

`tools/conformance/run.sh all` builds `cmd/viva-conformance` and runs it on this
Mac, the Linux container, and the Windows VM. The scenarios and the checks that
still need eyes are documented in [`docs/conformance.md`](../../docs/conformance.md).
