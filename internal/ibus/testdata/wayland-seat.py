#!/usr/bin/python3
"""Drive a native GTK test through XTest and its real X11/Wayland seat."""

import ctypes
import ctypes.util
import json
import os
import sys
import time
import traceback

import gi

gi.require_version("Gio", "2.0")
gi.require_version("GLib", "2.0")
from gi.repository import Gio, GLib


ACCESSIBLE = "org.a11y.atspi.Accessible"
COMPONENT = "org.a11y.atspi.Component"
PROPERTIES = "org.freedesktop.DBus.Properties"
ROOT = "/org/a11y/atspi/accessible/root"


def call(bus, destination, path, interface, member, arguments=None):
    return bus.call_sync(destination, path, interface, member, arguments, None,
                         Gio.DBusCallFlags.NONE, 1500, None).unpack()


class Seat:
    def __init__(self):
        self.xlib = ctypes.CDLL(ctypes.util.find_library("X11"))
        self.xtst = ctypes.CDLL(ctypes.util.find_library("Xtst"))
        x = self.xlib
        x.XOpenDisplay.argtypes = [ctypes.c_char_p]
        x.XOpenDisplay.restype = ctypes.c_void_p
        x.XDefaultRootWindow.argtypes = [ctypes.c_void_p]
        x.XDefaultRootWindow.restype = ctypes.c_ulong
        x.XQueryTree.argtypes = [ctypes.c_void_p, ctypes.c_ulong,
                                 ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.c_ulong),
                                 ctypes.POINTER(ctypes.POINTER(ctypes.c_ulong)), ctypes.POINTER(ctypes.c_uint)]
        x.XFetchName.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.POINTER(ctypes.c_void_p)]
        x.XFree.argtypes = [ctypes.c_void_p]
        x.XSetInputFocus.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int, ctypes.c_ulong]
        x.XGetInputFocus.argtypes = [ctypes.c_void_p, ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.c_int)]
        x.XStringToKeysym.argtypes = [ctypes.c_char_p]
        x.XStringToKeysym.restype = ctypes.c_ulong
        x.XKeysymToKeycode.argtypes = [ctypes.c_void_p, ctypes.c_ulong]
        x.XKeysymToKeycode.restype = ctypes.c_ubyte
        x.XSync.argtypes = [ctypes.c_void_p, ctypes.c_int]
        x.XCloseDisplay.argtypes = [ctypes.c_void_p]
        self.xtst.XTestQueryExtension.argtypes = [ctypes.c_void_p, *([ctypes.POINTER(ctypes.c_int)] * 4)]
        self.xtst.XTestFakeMotionEvent.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_ulong]
        self.xtst.XTestFakeButtonEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
        self.xtst.XTestFakeKeyEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
        self.display = x.XOpenDisplay(None)
        if not self.display:
            raise RuntimeError("XTest cannot open the private Xvfb display")
        extension = [ctypes.c_int() for _ in range(4)]
        if not self.xtst.XTestQueryExtension(self.display, *(ctypes.byref(item) for item in extension)):
            raise RuntimeError("XTest extension is missing")
        self.session = Gio.bus_get_sync(Gio.BusType.SESSION, None)
        address = call(self.session, "org.a11y.Bus", "/org/a11y/bus", "org.a11y.Bus", "GetAddress")[0]
        flags = Gio.DBusConnectionFlags.AUTHENTICATION_CLIENT | Gio.DBusConnectionFlags.MESSAGE_BUS_CONNECTION
        self.accessible = Gio.DBusConnection.new_for_address_sync(address, flags, None, None)
        self.editor = None
        self.window = None

    def children(self, reference):
        return call(self.accessible, reference[0], reference[1], ACCESSIBLE, "GetChildren")[0]

    def name(self, reference):
        value = call(self.accessible, reference[0], reference[1], PROPERTIES, "Get",
                     GLib.Variant("(ss)", (ACCESSIBLE, "Name")))[0]
        return value.unpack() if isinstance(value, GLib.Variant) else value

    def find_node(self, pid, label):
        refs = self.children(("org.a11y.atspi.Registry", ROOT))
        applications = []
        for reference in refs:
            try:
                owner = call(self.accessible, "org.freedesktop.DBus", "/org/freedesktop/DBus",
                             "org.freedesktop.DBus", "GetConnectionUnixProcessID",
                             GLib.Variant("(s)", (reference[0],)))[0]
                if owner == pid:
                    applications.append(tuple(reference))
            except GLib.Error:
                continue
        if len(applications) != 1:
            return None
        queue, seen, nodes = applications, set(), []
        while queue:
            reference = tuple(queue.pop(0))
            if reference in seen or reference[1].endswith("/null"):
                continue
            seen.add(reference)
            if len(seen) > 100:
                raise RuntimeError("AT-SPI tree exceeds the native probe bound")
            if self.name(reference) == label:
                nodes.append(reference)
            queue.extend(self.children(reference))
        if len(nodes) > 1:
            raise RuntimeError("multiple %s nodes for the test PID" % label)
        return nodes[0] if nodes else None

    def x11_windows(self):
        x = self.xlib
        root, parent = ctypes.c_ulong(), ctypes.c_ulong()
        children, count = ctypes.POINTER(ctypes.c_ulong)(), ctypes.c_uint()
        if not x.XQueryTree(self.display, x.XDefaultRootWindow(self.display),
                            ctypes.byref(root), ctypes.byref(parent),
                            ctypes.byref(children), ctypes.byref(count)):
            raise RuntimeError("XQueryTree failed")
        names = []
        try:
            for index in range(count.value):
                title = ctypes.c_void_p()
                value = ""
                if x.XFetchName(self.display, children[index], ctypes.byref(title)) and title.value:
                    value = ctypes.string_at(title.value).decode("utf-8", "replace")
                    x.XFree(title)
                names.append((children[index], value))
        finally:
            if children:
                x.XFree(children)
        return names

    def bind(self, pid, backend):
        deadline = time.monotonic() + 8
        while time.monotonic() < deadline:
            try:
                editor = self.find_node(pid, "Seat editor")
                windows = self.x11_windows()
                if backend == "wayland":
                    target = int(os.environ["WESTON_X11_WINDOW_ID"])
                    matches = [wid for wid, _ in windows if wid == target]
                else:
                    matches = [wid for wid, name in windows if name == "Viva seat keyboard probe"]
                if editor is not None and len(matches) == 1:
                    rect = call(self.accessible, editor[0], editor[1], COMPONENT, "GetExtents",
                                GLib.Variant("(u)", (0,)))[0]
                    x, y, width, height = rect
                    if width <= 0 or height <= 0 or x < 0 or y < 0 or x + width > 800 or y + height > 600:
                        raise RuntimeError("editor is outside the private 800x600 Xvfb viewport: %r" % (rect,))
                    self.editor, self.window = editor, matches[0]
                    return {"window": self.window, "editor": list(rect)}
                if len(matches) > 1:
                    raise RuntimeError("multiple X11 windows match the native probe")
            except GLib.Error:
                pass
            time.sleep(0.025)
        raise RuntimeError("native editor or matching X11/Weston window did not appear")

    def focus(self):
        x = self.xlib
        x.XSetInputFocus(self.display, self.window, 1, 0)
        x.XSync(self.display, False)
        focused, revert = ctypes.c_ulong(), ctypes.c_int()
        if not x.XGetInputFocus(self.display, ctypes.byref(focused), ctypes.byref(revert)) or focused.value != self.window:
            raise RuntimeError("outer X11 window did not acquire input focus")
        return {"window": focused.value}

    def grab(self):
        accepted = call(self.accessible, self.editor[0], self.editor[1], COMPONENT, "GrabFocus")[0]
        if not accepted:
            raise RuntimeError("AT-SPI editor rejected GrabFocus")
        return {"accepted": True}

    def editor_point(self, reference=None):
        reference = reference or self.editor
        x, y, width, height = call(self.accessible, reference[0], reference[1], COMPONENT,
                                   "GetExtents", GLib.Variant("(u)", (0,)))[0]
        if width <= 0 or height <= 0 or x < 0 or y < 0 or x + width > 800 or y + height > 600:
            raise RuntimeError("native click target is outside the private viewport")
        return x + width // 2, y + height // 2

    def click(self, reference=None):
        return self.click_at(*self.editor_point(reference))

    def move(self, x, y):
        if not 0 <= x < 800 or not 0 <= y < 600:
            raise RuntimeError("pointer position is outside the private viewport")
        if not self.xtst.XTestFakeMotionEvent(self.display, -1, x, y, 0):
            raise RuntimeError("XTest pointer motion failed")
        self.xlib.XSync(self.display, False)
        return {"point": [x, y]}

    def click_at(self, x, y):
        self.move(x, y)
        for pressed in (True, False):
            if not self.xtst.XTestFakeButtonEvent(self.display, 1, pressed, 0):
                raise RuntimeError("XTest pointer button failed")
        self.xlib.XSync(self.display, False)
        return {"point": [x, y]}

    def probe_switch(self, pid):
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            try:
                target = self.find_node(pid, "Switch window")
                if target is not None:
                    rect = call(self.accessible, target[0], target[1], COMPONENT, "GetExtents",
                                GLib.Variant("(u)", (0,)))[0]
                    if rect[2] > 0 and rect[3] > 0:
                        return {"node": "Switch window", "extent": list(rect)}
            except GLib.Error:
                pass
            time.sleep(0.025)
        raise RuntimeError("Switch window AT-SPI node did not become visible")

    def key(self, symbol, down):
        keysym = self.xlib.XStringToKeysym(symbol.encode("ascii"))
        code = self.xlib.XKeysymToKeycode(self.display, keysym)
        if not keysym or not code or not self.xtst.XTestFakeKeyEvent(self.display, code, down, 0):
            raise RuntimeError("XTest key failed: %r" % symbol)
        self.xlib.XSync(self.display, False)
        return {"code": code}

    def close(self):
        self.accessible.close_sync(None)
        self.xlib.XCloseDisplay(self.display)


def main():
    seat = Seat()
    try:
        for line in sys.stdin:
            command = json.loads(line)
            operation = command["op"]
            if operation == "bind":
                result = seat.bind(int(command["pid"]), command["backend"])
            elif operation == "focus":
                result = seat.focus()
            elif operation == "grab":
                result = seat.grab()
            elif operation == "click":
                result = seat.click()
            elif operation == "move-editor":
                result = seat.move(*seat.editor_point())
            elif operation == "probe-switch":
                result = seat.probe_switch(int(command["pid"]))
            elif operation == "move":
                result = seat.move(int(command["x"]), int(command["y"]))
            elif operation == "click-at":
                result = seat.click_at(int(command["x"]), int(command["y"]))
            elif operation in ("down", "up"):
                result = seat.key(command["key"], operation == "down")
            elif operation == "quit":
                print(json.dumps({"ok": True}), flush=True)
                return
            else:
                raise RuntimeError("unknown seat command: %r" % operation)
            print(json.dumps({"ok": True, **result}), flush=True)
    finally:
        seat.close()


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        traceback.print_exc(file=sys.stderr)
        print(json.dumps({"ok": False, "error": str(error)}), flush=True)
        sys.exit(1)
