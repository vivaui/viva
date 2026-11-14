#!/bin/sh
set -eu
[ "$#" -gt 0 ]
: "${VIVA_IBUS_TEST_BINARY:?set the native test binary path}"
command -v timeout >/dev/null
fixture_dir=$(dirname "$0")
umask 077
task_dir=$(mktemp -d /tmp/viva-ibus-native.XXXXXX)
export XDG_RUNTIME_DIR=$task_dir/runtime
export XDG_CONFIG_HOME=$task_dir/config
export XDG_CACHE_HOME=$task_dir/cache
export XDG_DATA_HOME=$task_dir/data
export GDK_BACKEND=x11
export NO_AT_BRIDGE=1
mkdir -p "$XDG_RUNTIME_DIR" "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME" "$XDG_DATA_HOME"
export IBUS_ADDRESS=unix:path=$task_dir/ibus-bus
session_pid=
cleanup() {
    if [ -n "$session_pid" ]; then
        kill "$session_pid" 2>/dev/null || true
        wait "$session_pid" 2>/dev/null || true
    fi
    rm -rf "$task_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' HUP TERM
# Activated services must inherit the profile when the session bus starts.
timeout --kill-after=5s 90s dbus-run-session -- sh "$fixture_dir/native-daemon.sh" "$@" &
session_pid=$!
status=0
wait "$session_pid" || status=$?
session_pid=
exit "$status"
