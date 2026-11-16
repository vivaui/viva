import json
import os
import subprocess
import time

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("Gdk", "3.0")
from gi.repository import Gdk, GLib, Gtk

Gtk.init([])
display = Gdk.Display.get_default()
backend = display.__gtype__.name
expected = "GdkWaylandDisplay" if os.environ["VIVA_FCITX_BACKEND"] == "wayland" else "GdkX11Display"
if backend != expected:
    raise RuntimeError(f"GTK reference backend {backend}, expected {expected}")
print("GTK reference backend", backend, flush=True)
codes = {int(key): value for key, value in json.loads(os.environ["VIVA_IBUS_KEYCODES"]).items()}
engine = os.environ.get("VIVA_FCITX_ENGINE", "pinyin")
main = GLib.MainContext.default()


def drain(duration=0.1):
    until = time.monotonic() + duration
    while time.monotonic() < until:
        while main.pending():
            main.iteration(False)
        time.sleep(0.001)


window = Gtk.Window()
window.set_default_size(360, 160)
window.show_all()
drain()
for ending in ["commit", "escape", "reset", "focus-out", "reset-then-focus-out"]:
    context = Gtk.IMMulticontext()
    context.set_context_id("fcitx")
    context.set_client_window(window.get_window())
    events = []
    context.connect("preedit-start", lambda _: events.append(["start"]))
    context.connect("preedit-end", lambda _: events.append(["end"]))
    context.connect("preedit-changed", lambda ctx: events.append(["update", ctx.get_preedit_string()[0], ctx.get_preedit_string()[2]]))
    context.connect("commit", lambda _, value: events.append(["commit", value]))
    context.focus_in()
    deadline = time.monotonic() + 2
    while True:
        drain(0.01)
        focused = subprocess.check_output(["fcitx5-remote", "--check", "-n"], timeout=2).decode().strip()
        if focused:
            break
        if time.monotonic() > deadline:
            raise RuntimeError("GTK reference did not establish a focused Fcitx context")
    for args in [["--check", "-s", engine], ["--check", "-o"]]:
        subprocess.run(["fcitx5-remote"] + args, check=True, timeout=2)
    actual = subprocess.check_output(["fcitx5-remote", "--check", "-n"], timeout=2).decode().strip()
    if actual != engine or context.get_context_id() != "fcitx":
        raise RuntimeError(f"GTK reference engine={actual}, module={context.get_context_id()}")
    drain()

    def press(symbol):
        event = Gdk.Event.new(Gdk.EventType.KEY_PRESS)
        event.window = window.get_window()
        event.send_event = True
        event.keyval = symbol
        event.hardware_keycode = codes[symbol] + 8
        event.group = 0
        event.state = Gdk.ModifierType(0)
        event.set_device(display.get_default_seat().get_keyboard())
        handled = context.filter_keypress(event.key)
        drain(0.05)
        events.append(["answer", symbol, handled])

    for symbol in "rksk" if engine == "hangul" else "nihao":
        press(ord(symbol))
    print("GTK reference live", engine, ending, json.dumps(events, ensure_ascii=False), flush=True)
    if ending in ["commit", "escape"]:
        press(ord(" ") if ending == "commit" else 0xff1b)
    else:
        if ending in ["reset", "reset-then-focus-out"]:
            context.reset()
        if ending in ["focus-out", "reset-then-focus-out"]:
            context.focus_out()
    drain(0.15)
    print("GTK reference settled", engine, ending, json.dumps(events, ensure_ascii=False), flush=True)
    context.set_client_window(None)
    context.focus_out()
    del context
    drain()
window.destroy()
