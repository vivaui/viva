# Address of your own Linux test environment. Source this file.
# Export VIVA_LINUX_ENV, or put that export in tools/testenv/local.sh, which is
# ignored by git.
here=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
if [ -f "$here/local.sh" ]; then . "$here/local.sh"; fi
VIVA_LINUX_ENV=${VIVA_LINUX_ENV:-}
# The desktop's display and X authority file. The current machine runs GNOME on
# X at :0 and needs no authority file; an older Xvnc machine used :1 and
# /home/user/.Xauthority.
VIVA_LINUX_DISPLAY=${VIVA_LINUX_DISPLAY:-:0}
VIVA_LINUX_XAUTHORITY=${VIVA_LINUX_XAUTHORITY:-}
# The session's runtime directory (PulseAudio and the session bus live in it).
VIVA_LINUX_RUNTIME_DIR=${VIVA_LINUX_RUNTIME_DIR:-/run/user/1000}
export VIVA_LINUX_ENV VIVA_LINUX_DISPLAY VIVA_LINUX_XAUTHORITY VIVA_LINUX_RUNTIME_DIR
# Keep a developer proxy out of the way when the environment is on the LAN.
export no_proxy="${no_proxy:-},localhost,127.0.0.1"
