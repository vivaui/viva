//go:build linux && vivagtk

package linux

import (
	"fmt"
	"os"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestGTKDestroyDetachesInputMethodBeforeNativeWidget(t *testing.T) {
	g := &gtkLibrary{}
	a := &gtkApplication{g: g}
	w := &gtkWindow{app: a, g: g, toplevel: 17}
	w.ime = &gtkIME{w: w, g: g, ctx: 23, active: true}
	detached, destroyed := false, 0
	g.objectUnref = func(uintptr) {}
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
		if window != 17 || !detached || w.ime != nil {
			t.Error("native widget destruction preceded input-method detachment")
		}
	}
	w.destroy()
	w.destroy()
	if destroyed != 1 || !detached {
		t.Fatalf("native destruction count=%d detached=%t", destroyed, detached)
	}
}

func TestGTKNativeInputMethodWindowTeardown(t *testing.T) {
	if os.Getenv("VIVA_GTK_CLOSE_NATIVE") != "1" {
		t.Skip("native close probe requires a private display and the Fcitx GTK module")
	}
	for _, mode := range []string{"programmatic", "default", "deferred"} {
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
					if mode == "programmatic" {
						w.Close()
					} else {
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
