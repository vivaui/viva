//go:build linux && vivagtk

package linux

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func mockGTKIMEClientWindow(library *gtkLibrary) {
	library.gdkWindowNew = func(parent uintptr, attrs *gdkWindowAttr, mask int32) uintptr { return 29 }
	library.widgetRegisterWindow = func(widget, client uintptr) {}
	library.cairoRegionRect = func(rectangle *[4]int32) uintptr { return 31 }
	library.gdkWindowInputShape = func(client, region uintptr, x, y int32) {}
	library.cairoRegionDestroy = func(region uintptr) {}
	library.gdkWindowShow = func(client uintptr) {}
}

func TestGTKWaylandIMECursorTracksContentOrigin(t *testing.T) {
	library := &gtkLibrary{}
	mockGTKIMEClientWindow(library)
	window := &gtkWindow{g: library, toplevel: 17, area: 18, ratio: 1}
	window.wl = &waylandSurface{w: window}
	window.text = &linuxTextInput{input: platform.TextInput{
		Placed: true, CaretX: 44, CaretY: 59, CaretWidth: 2, CaretHeight: 21,
	}}
	ime := &gtkIME{w: window, g: library, ctx: 23}
	library.objectRef = func(context uintptr) uintptr { return context }
	library.objectUnref = func(context uintptr) {}
	library.widgetGetWindow = func(widget uintptr) uintptr { return widget }
	var clientWindow uintptr
	library.imSetClientWindow = func(context, client uintptr) { clientWindow = client }
	offsetX, offsetY := int32(26), int32(60)
	library.widgetTranslate = func(source, dest uintptr, x, y int32, destX, destY *int32) int32 {
		*destX, *destY = x+offsetX, y+offsetY
		return 1
	}
	var observed [][4]int32
	library.imSetCursorRect = func(context uintptr, rectangle *[4]int32) {
		position := *rectangle
		if clientWindow == window.area {
			position[0] += offsetX
			position[1] += offsetY
		}
		observed = append(observed, position)
	}

	ime.realized()
	ime.updateCursor()
	offsetX, offsetY = 0, 0
	ime.updateCursor()
	ime.updateCursor()
	want := [][4]int32{{70, 119, 2, 21}, {44, 59, 2, 21}}
	if !slices.Equal(observed, want) {
		t.Fatalf("cursor updates after content origin change: got %v, want %v", observed, want)
	}
}

func TestGTKWaylandUnmapEndsPointerActivatedIME(t *testing.T) {
	g := &gtkLibrary{}
	mockGTKIMEClientWindow(g)
	app := &gtkApplication{g: g}
	window := &gtkWindow{app: app, g: g, toplevel: 17, area: 18, ratio: 1}
	window.pointer.clicks = &input.ClickCounter{}
	window.text = &linuxTextInput{enabled: true, input: platform.TextInput{Placed: true}}
	window.wl = &waylandSurface{w: window, gdkWindow: 19}
	ime := &gtkIME{w: window, g: g, ctx: 23}
	window.ime = ime
	g.objectRef = func(context uintptr) uintptr { return context }
	g.objectUnref = func(context uintptr) {}
	g.widgetGetWindow = func(widget uintptr) uintptr { return widget }
	g.widgetTranslate = func(source, dest uintptr, x, y int32, destX, destY *int32) int32 {
		*destX, *destY = x, y
		return 1
	}
	g.imSetClientWindow = func(context, client uintptr) {}
	g.imSetCursorRect = func(context uintptr, rectangle *[4]int32) {}
	g.imFocusIn = func(context uintptr) {}
	focusOuts := 0
	g.imFocusOut = func(context uintptr) { focusOuts++ }

	window.deliverPointer(platform.PointerEvent{Kind: platform.PointerDown})
	if window.focused || !ime.filtering() {
		t.Fatalf("pointer did not activate IME before native focus: window=%t ime=%t", window.focused, ime.active)
	}
	window.mapChanged(false)
	if ime.focused || ime.filtering() || focusOuts != 1 {
		t.Fatalf("unmap left pointer-activated IME active: focused=%t active=%t focus-outs=%d", ime.focused, ime.active, focusOuts)
	}
}

func TestGTKDestroyDetachesInputMethodBeforeNativeWidget(t *testing.T) {
	g := &gtkLibrary{}
	a := &gtkApplication{g: g}
	w := &gtkWindow{app: a, g: g, toplevel: 17}
	w.ime = &gtkIME{w: w, g: g, ctx: 23, active: true}
	detached, destroyed := false, 0
	references, retired := 1, false
	g.objectRef = func(context uintptr) uintptr {
		if context != 23 || references == 0 {
			t.Fatal("native call referenced a released input context")
		}
		references++
		return context
	}
	g.objectUnref = func(context uintptr) {
		if context != 23 || references == 0 {
			t.Fatal("input context released more than once")
		}
		references--
	}
	g.idleAddFull = func(priority int32, function, data, destroy uintptr) uint32 {
		if priority != 200 || function != callbacks().retireIME || data != 23 || destroy != callbacks().releaseIME || !detached || destroyed != 0 || references != 1 || retired {
			t.Error("input context retirement did not retain its final native reference")
		}
		retired = true
		return 1
	}
	g.sourceFind = func(context uintptr, id uint32) uintptr {
		if context != 0 || id != 1 {
			t.Error("unexpected retired input context source")
		}
		if retired {
			return 23
		}
		return 0
	}
	g.sourceRemove = func(id uint32) int32 {
		if id != 1 || !retired {
			t.Fatal("retired input context source removed more than once")
		}
		retired = false
		g.objectUnref(23)
		return 1
	}
	g.imFocusOut = func(context uintptr) {
		if context != 23 || !w.destroying || w.ime != nil {
			t.Error("native focus-out did not keep the context and destruction guard")
		}
		w.destroy()
	}
	g.imSetClientWindow = func(context, window uintptr) {
		if context != 23 || window != 0 || destroyed != 0 {
			t.Errorf("detach context=%d window=%d after %d native destructions", context, window, destroyed)
		}
		detached = true
	}
	g.widgetDestroy = func(window uintptr) {
		destroyed++
		if window != 17 || !detached || w.ime != nil || references != 1 || !retired {
			t.Error("native widget destruction preceded input-method detachment")
		}
	}
	w.destroy()
	w.destroy()
	if destroyed != 1 || !detached {
		t.Fatalf("native destruction count=%d detached=%t", destroyed, detached)
	}
	a.releaseRetiredIME()
	a.releaseRetiredIME()
	if references != 0 || retired || len(a.retiredIMESources) != 0 {
		t.Fatalf("shutdown retained context: references=%d retired=%t sources=%v", references, retired, a.retiredIMESources)
	}
}

func TestGTKNativeInputMethodWindowTeardown(t *testing.T) {
	if os.Getenv("VIVA_GTK_CLOSE_NATIVE") != "1" {
		t.Skip("native close probe requires a private display and the Fcitx GTK module")
	}
	modes := []string{"programmatic", "default", "deferred"}
	if os.Getenv("GDK_BACKEND") == "wayland" {
		modes = append(modes, "external")
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			closed, requests, events := 0, 0, 0
			allowed := false
			var failures []string
			err := RunGTKApplication(func(application platform.Application) error {
				a := application.(*gtkApplication)
				window, err := application.OpenWindow(platform.Config{
					Title: "Viva native input method close probe", Width: 360, Height: 160,
					DynamicTextInput: true, OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil },
				})
				if err != nil {
					return err
				}
				w := window.(*gtkWindow)
				a.enqueue(func() {
					editor := text.NewEditor("abc")
					field := platform.TextInput{
						Target: gfx.Item(1), X: 20, Y: 20, Width: 300, Height: 100, FontSize: 16,
						State: platform.EditorState(editor),
						OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
							events++
							event.Apply(editor)
							return platform.EditorState(editor), nil
						},
					}
					w.present(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}, nil)
					w.ime.setFocused(true)
					ime, before := w.ime, events
					var typeFromName func(*byte) uintptr
					var closeWindow func(uintptr)
					if !a.g.lookup(&typeFromName, "g_type_from_name") || !a.g.lookup(&closeWindow, "gtk_window_close") {
						a.fail(fmt.Errorf("native close probe cannot bind GTK/GObject functions"))
						return
					}
					name := cString("FcitxIMContext")
					if typeFromName(&name[0]) == 0 {
						a.fail(fmt.Errorf("native close probe did not load FcitxIMContext"))
						return
					}
					t.Logf("native backend=%s input module=FcitxIMContext mode=%s", typeNameOf(a.g, a.g.gdkDisplayDefault()), mode)
					w.config.OnClosed = func() {
						closed++
						if !w.closed || w.ime != nil || ime.ctx != 0 || events != before || editor.String() != "abc" || closed != 1 {
							failures = append(failures, "native close retained input ownership, ran twice or delivered a closing edit")
						}
						if mode == "deferred" && (!allowed || requests != 1) {
							failures = append(failures, "native close bypassed application approval")
						}
					}
					if mode == "deferred" {
						w.config.OnCloseRequest = func(allow func()) {
							requests++
							a.g.timeoutAdd(20, callbacks().task, a.tasks.add(func() {
								if w.closed || w.destroying || w.ime != ime || !ime.active {
									failures = append(failures, "native close tore down input before application approval")
								}
								allowed = true
								allow()
							}))
						}
					}
					switch mode {
					case "programmatic":
						w.Close()
					case "external":
						a.g.widgetDestroy(w.toplevel)
					default:
						closeWindow(w.toplevel)
					}
				})
				return nil
			})
			if err != nil || closed != 1 {
				t.Fatalf("native application error=%v close count=%d", err, closed)
			}
			for _, failure := range failures {
				t.Error(failure)
			}
		})
	}
}
