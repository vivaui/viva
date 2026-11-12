# Address of your own Linux test environment. Source this file.
# Export VIVA_LINUX_ENV, or put that export in tools/testenv/local.sh, which is
# ignored by git.
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ -f "$here/local.sh" ]; then . "$here/local.sh"; fi
VIVA_LINUX_ENV=${VIVA_LINUX_ENV:-}
export VIVA_LINUX_ENV
# Keep a developer proxy out of the way when the environment is on the LAN.
export no_proxy="${no_proxy:-},localhost,127.0.0.1"
