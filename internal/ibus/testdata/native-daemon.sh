#!/bin/sh
set -eu
command -v gdbus >/dev/null
reference_client=$(dirname "$0")/reference-client.py
task_dir=$(dirname "$XDG_RUNTIME_DIR")
daemon_pid=
cleanup() {
    if [ -n "$daemon_pid" ]; then
        kill "$daemon_pid" 2>/dev/null || true
        wait "$daemon_pid" 2>/dev/null || true
    fi
    if [ -f "$task_dir/daemon.log" ]; then cat "$task_dir/daemon.log"; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' HUP TERM
ibus-daemon --panel=disable --config=/usr/libexec/ibus-dconf --emoji-extension=disable --cache=refresh --address "$IBUS_ADDRESS" > "$task_dir/daemon.log" 2>&1 &
daemon_pid=$!
attempt=0
while [ ! -S "$task_dir/ibus-bus" ]; do
    kill -0 "$daemon_pid"
    attempt=$((attempt + 1))
    [ "$attempt" -lt 100 ]
    sleep 0.02
done
attempt=0
while ! gdbus call --timeout 1 --address "$IBUS_ADDRESS" --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus --method org.freedesktop.DBus.NameHasOwner org.freedesktop.IBus.Config | grep -q true; do
    kill -0 "$daemon_pid"
    attempt=$((attempt + 1))
    [ "$attempt" -lt 100 ]
    sleep 0.02
done
ibus version
VIVA_IBUS_KEYCODES=$(/usr/bin/python3 "$reference_client" --keycodes)
export VIVA_IBUS_KEYCODES
printf 'Native keycodes: %s\n' "$VIVA_IBUS_KEYCODES"
for engine in "$@"; do
    if [ "${VIVA_IBUS_REFERENCE:-0}" = 1 ]; then
        VIVA_IBUS_ENGINE=$engine /usr/bin/python3 "$reference_client"
    fi
    VIVA_IBUS_NATIVE=1 VIVA_IBUS_ENGINE=$engine "$VIVA_IBUS_TEST_BINARY" -test.run "${VIVA_IBUS_TEST_RUN:-^TestNativeEngineLifecycle$}" -test.count=1 -test.v -test.timeout=30s
done
/usr/bin/python3 "$reference_client" --activation-environment
