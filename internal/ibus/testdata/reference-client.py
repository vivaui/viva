import json
import os
import subprocess
import sys
import time

import gi

gi.require_version("IBus", "1.0")
gi.require_version("Gdk", "3.0")
from gi.repository import Gdk, Gio, GLib, IBus

if "--activation-environment" in sys.argv:
    subprocess.run(["dconf", "write", "/viva/native-fixture/marker", "'isolated'"], check=True)
    session = Gio.bus_get_sync(Gio.BusType.SESSION, None)
    reply = session.call_sync(
        "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
        "GetConnectionUnixProcessID", GLib.Variant("(s)", ("ca.desrt.dconf",)),
        GLib.VariantType.new("(u)"), Gio.DBusCallFlags.NONE, 1000, None,
    )
    pid = reply.unpack()[0]
    with open(f"/proc/{pid}/environ", "rb") as stream:
        activated = dict(entry.split(b"=", 1) for entry in stream.read().split(b"\0") if b"=" in entry)
    names = ["XDG_RUNTIME_DIR", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME"]
    checks = {name: activated.get(name.encode()) == os.environ[name].encode() for name in names}
    print("Dconf activation environment:", json.dumps(checks), flush=True)
    if not all(checks.values()):
        raise RuntimeError("activated dconf service escaped the private profile")
    marker = subprocess.check_output(["dconf", "read", "/viva/native-fixture/marker"]).strip()
    database = os.path.join(os.environ["XDG_CONFIG_HOME"], "dconf", "user")
    if marker != b"'isolated'" or not os.path.isfile(database):
        raise RuntimeError("private dconf database was not written and read back")
    print("Dconf private database: dconf/user, marker verified", flush=True)
    sys.exit(0)

Gdk.init([])
keymap = Gdk.Keymap.get_for_display(Gdk.Display.get_default())
keycodes = {}
for symbol in [ord(key) for key in "nihaorks "] + [0xff1b, 0xff31]:
    found, entries = keymap.get_entries_for_keyval(symbol)
    keycodes[symbol] = entries[0].keycode - 8 if found else 0
if "--keycodes" in sys.argv:
    print(json.dumps(keycodes))
    sys.exit(0)

IBus.init()
print("reference GI version", gi.__version__, flush=True)
bus = IBus.Bus()
print("reference connected", bus.is_connected(), flush=True)
context = bus.create_input_context("viva-reference-probe")
context.set_capabilities(IBus.Capabilite.PREEDIT_TEXT | IBus.Capabilite.FOCUS)
context.focus_in()
requested = os.environ.get("VIVA_IBUS_ENGINE", "libpinyin")
global_engine = bus.get_use_global_engine()
print("reference UseGlobalEngine", global_engine, flush=True)
if global_engine:
    for name in ["xkb:us::eng", requested]:
        if not bus.set_global_engine(name):
            raise RuntimeError("reference global engine selection failed: " + name)
else:
    context.set_engine(requested)
deadline = time.monotonic() + 5
while True:
    engine = context.get_engine()
    if engine and engine.get_name() == requested:
        break
    if time.monotonic() >= deadline:
        raise RuntimeError("reference engine did not become ready")
    main = GLib.MainContext.default()
    while main.pending():
        main.iteration(False)
    time.sleep(0.005)
print("reference engine", engine.get_name(), flush=True)
signal_texts = []


def native_text(value):
    # libibus still checks the floating object after signal callbacks return.
    signal_texts.append(value)
    return value.get_text()


context.connect("update-preedit-text", lambda ctx, text, cursor, visible: print("reference preedit", native_text(text), cursor, visible, flush=True))
context.connect("commit-text", lambda ctx, text: print("reference commit", native_text(text), flush=True))
if requested == "hangul":
    print("reference Hangul mode", context.process_key_event(0xff31, keycodes[0xff31], 0), flush=True)
for key in ("rksk " if requested == "hangul" else "nihao "):
    print("reference key", key, context.process_key_event(ord(key), keycodes[ord(key)], 0), flush=True)
    main = GLib.MainContext.default()
    while main.pending():
        main.iteration(False)
until = time.monotonic() + 0.15
while time.monotonic() < until:
    while main.pending():
        main.iteration(False)
    time.sleep(0.001)
context.destroy()
signal_texts.clear()
