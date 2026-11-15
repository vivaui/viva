import json
import os
import subprocess
import time

import gi

gi.require_version("Gdk", "3.0")
gi.require_version("IBus", "1.0")
from gi.repository import Gdk, Gio, GLib, IBus

Gdk.init([])
backend = Gdk.Display.get_default().__gtype__.name
expected = "GdkWaylandDisplay" if os.environ["VIVA_FCITX_BACKEND"] == "wayland" else "GdkX11Display"
if backend != expected:
    raise RuntimeError(f"native backend {backend}, expected {expected}")
print("reference backend", backend, flush=True)
codes = {int(key): value for key, value in json.loads(os.environ["VIVA_IBUS_KEYCODES"]).items()}
IBus.init()
main = GLib.MainContext.default()
requested = os.environ.get("VIVA_FCITX_ENGINE", "pinyin")


def drain(duration=0.05):
    until = time.monotonic() + duration
    while time.monotonic() < until:
        while main.pending():
            main.iteration(False)
        time.sleep(0.001)


for client_mode in [False, True]:
    for ending in ["commit", "escape", "reset", "focus-out"]:
        bus = IBus.Bus()
        if not bus.is_connected():
            raise RuntimeError("reference did not connect to Fcitx IBus frontend")
        context = bus.create_input_context("viva-fcitx-reference")
        events = []
        context.set_capabilities(IBus.Capabilite.PREEDIT_TEXT | IBus.Capabilite.FOCUS)
        if client_mode:
            context.set_client_commit_preedit(True)
        context.connect("update-preedit-text", lambda _, text, cursor, visible: events.append(["update", text.get_text(), cursor, visible]))
        context.connect("update-preedit-text-with-mode", lambda _, text, cursor, visible, mode: events.append(["mode", text.get_text(), cursor, visible, mode]))
        context.connect("commit-text", lambda _, text: events.append(["commit", text.get_text()]))
        connection = context.get_connection()
        connection.call_sync("org.freedesktop.IBus", context.get_object_path(), "org.freedesktop.IBus.InputContext", "FocusIn", GLib.Variant("()", ()), None, Gio.DBusCallFlags.NONE, 1000, None)
        subprocess.run(["fcitx5-remote", "--check", "-s", requested], check=True, timeout=2)
        subprocess.run(["fcitx5-remote", "--check", "-o"], check=True, timeout=2)
        actual = subprocess.check_output(["fcitx5-remote", "--check", "-n"], timeout=2).decode().strip()
        if actual != requested:
            raise RuntimeError(f"reference engine {actual}")
        print("reference engine", actual, flush=True)
        for key in ("rksk" if requested == "hangul" else "nihao"):
            handled = context.process_key_event(ord(key), codes[ord(key)], 0)
            drain(0.01)
            events.append(["answer", key, handled])
        print("reference live", client_mode, ending, json.dumps(events, ensure_ascii=False), flush=True)
        if ending in ["commit", "escape"]:
            symbol = ord(" ") if ending == "commit" else 0xff1b
            events.append(["answer", ending, context.process_key_event(symbol, codes[symbol], 0)])
        elif ending == "reset":
            context.reset()
        else:
            context.focus_out()
        drain(0.15)
        print("reference settled", client_mode, ending, json.dumps(events, ensure_ascii=False), flush=True)
        context.destroy()
        drain(0.01)
