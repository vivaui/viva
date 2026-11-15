#!/bin/sh
set -eu
backend=${1:?set x11 or wayland}
shift
case "$backend" in x11|wayland) ;; *) exit 2 ;; esac
fixture_dir=$(dirname "$0")
umask 077
task_dir=$(mktemp -d /tmp/viva-fcitx-native.XXXXXX)
export XDG_RUNTIME_DIR=$task_dir/runtime
export XDG_CONFIG_HOME=$task_dir/config
export XDG_CACHE_HOME=$task_dir/cache
export XDG_DATA_HOME=$task_dir/data
export NO_AT_BRIDGE=1
export GTK_IM_MODULE=fcitx
export QT_IM_MODULE=fcitx
export XMODIFIERS=@im=fcitx
export VIVA_FCITX_BACKEND=$backend
unset IBUS_ADDRESS
mkdir -p "$XDG_RUNTIME_DIR" "$XDG_CONFIG_HOME/fcitx5" "$XDG_CACHE_HOME" "$XDG_DATA_HOME"
cp "$fixture_dir/fcitx-profile" "$XDG_CONFIG_HOME/fcitx5/profile"
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
timeout --kill-after=5s 90s dbus-run-session -- sh "$fixture_dir/native-fcitx-daemon.sh" "$@" &
session_pid=$!
status=0
wait "$session_pid" || status=$?
session_pid=
exit "$status"
