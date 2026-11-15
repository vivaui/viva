#!/bin/sh
set -eu
fixture_dir=$(dirname "$0")
task_dir=$(dirname "$XDG_RUNTIME_DIR")
daemon_pid=
compositor_pid=
cleanup() {
    for pid in "$daemon_pid" "$compositor_pid"; do
        if [ -n "$pid" ]; then
            kill "$pid" 2>/dev/null || true
            wait "$pid" 2>/dev/null || true
        fi
    done
    for log in "$task_dir/fcitx.log" "$task_dir/weston.log"; do
        if [ -f "$log" ]; then cat "$log"; fi
    done
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' HUP TERM
if [ "$VIVA_FCITX_BACKEND" = wayland ]; then
    export WAYLAND_DISPLAY=wayland-vivapad
    weston --backend=x11 --renderer=pixman --socket="$WAYLAND_DISPLAY" --idle-time=0 --no-config --width=800 --height=600 --log="$task_dir/weston.log" > "$task_dir/weston.stdout" 2>&1 &
    compositor_pid=$!
    attempt=0
    while [ ! -S "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY" ]; do
        kill -0 "$compositor_pid"
        attempt=$((attempt + 1))
        [ "$attempt" -lt 100 ]
        sleep 0.02
    done
    export GDK_BACKEND=wayland
else
    export GDK_BACKEND=x11
fi
export IBUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS
export IBUS_ADDRESS_FILE=$task_dir/ibus-address
fcitx5 -D --disable=notificationitem,notifications,classicui,xim > "$task_dir/fcitx.log" 2>&1 &
daemon_pid=$!
attempt=0
while ! gdbus call --timeout 1 --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus --method org.freedesktop.DBus.NameHasOwner org.freedesktop.IBus | grep -q true; do
    kill -0 "$daemon_pid"
    attempt=$((attempt + 1))
    [ "$attempt" -lt 100 ]
    sleep 0.02
done
fcitx5 --version
VIVA_IBUS_KEYCODES=$(/usr/bin/python3 "$fixture_dir/reference-client.py" --keycodes)
export VIVA_IBUS_KEYCODES
printf 'Native keycodes: %s\n' "$VIVA_IBUS_KEYCODES"
if [ "${VIVA_FCITX_REFERENCE:-0}" = 1 ]; then
    /usr/bin/python3 "$fixture_dir/fcitx-reference.py"
fi
if [ "${VIVA_FCITX_GTK_REFERENCE:-0}" = 1 ]; then
    /usr/bin/python3 "$fixture_dir/fcitx-gtk-reference.py"
fi
if [ -n "${VIVA_FCITX_TEST_BINARY:-}" ]; then
    VIVA_IBUS_NATIVE=1 VIVA_IBUS_FRONTEND=fcitx5 VIVA_IBUS_ENGINE="${VIVA_FCITX_ENGINE:-pinyin}" "$VIVA_FCITX_TEST_BINARY" -test.run "${VIVA_IBUS_TEST_RUN:-^TestNativeContextRetirement$}" -test.count=1 -test.v -test.timeout=45s
fi
dconf write /viva/native-fixture/marker "'isolated'"
/usr/bin/python3 "$fixture_dir/reference-client.py" --activation-environment
