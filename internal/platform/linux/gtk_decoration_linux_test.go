//go:build linux && vivagtk && !android

package linux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

func TestNativeGTKDecorationRoundTrip(t *testing.T) {
	if os.Getenv("VIVA_GTK_DECORATION_NATIVE") != "1" {
		t.Skip("requires a private X11 session with a window manager")
	}
	for _, size := range [][2]int{{640, 400}, {1280, 800}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			var outcome error
			completed := false
			err := RunGTKApplication(func(application platform.Application) error {
				a := application.(*gtkApplication)
				above := size[0] == 1280
				control, err := a.OpenWindow(platform.Config{
					Title: "Viva GTK decoration probe", Width: size[0], Height: size[1], ClearColor: platform.Color{Alpha: 1},
				})
				if err != nil {
					return err
				}
				w := control.(*gtkWindow)
				aboveRequested := false
				var xid func(uintptr) uintptr
				if a.wayland || !a.g.lookup(&xid, "gdk_x11_window_get_xid") {
					return fmt.Errorf("this test requires GTK on X11")
				}
				api, err := loadXCB()
				if err != nil {
					return err
				}
				connection := api.connect(nil, nil)
				if connection == 0 || api.connectionHasError(connection) != 0 {
					if connection != 0 {
						api.disconnect(connection)
					}
					return fmt.Errorf("could not open the X11 test connection")
				}
				atom, err := api.atom(connection, "_NET_FRAME_EXTENTS")
				if err != nil {
					api.disconnect(connection)
					return err
				}
				finish := func(err error) {
					if !completed {
						completed, outcome = true, err
						api.disconnect(connection)
						w.destroy()
					}
				}
				deadline := time.Now().Add(5 * time.Second)
				var observed string
				var start func()
				start = func() {
					if time.Now().After(deadline) {
						finish(fmt.Errorf("window manager did not settle: %s", observed))
						return
					}
					native := w.g.widgetGetWindow(w.toplevel)
					if native == 0 {
						a.g.timeoutAdd(20, callbacks().task, a.tasks.add(start))
						return
					}
					initialID := uint32(xid(native))
					var initial [4]uint32
					var present bool
					frame, placed := w.Frame()
					hasFrame := api.readDecorationProperty(connection, initialID, atom, xcbAtomCardinal, initial[:], &present) && present && initial != [4]uint32{}
					observed = fmt.Sprintf("placed=%t focused=%t extents=%v wantAbove=%t", placed, w.focused, initial, above)
					if !placed || !w.focused || !hasFrame {
						a.g.timeoutAdd(20, callbacks().task, a.tasks.add(start))
						return
					}
					if above && !aboveRequested {
						aboveRequested = true
						if err := w.SetAlwaysOnTop(true); err != nil {
							finish(err)
							return
						}
					}
					readAbove := func() (bool, error) {
						output, err := exec.Command("xprop", "-id", fmt.Sprintf("0x%x", initialID), "_NET_WM_STATE").Output()
						return strings.Contains(string(output), "_NET_WM_STATE_ABOVE"), err
					}
					initialAbove, err := readAbove()
					if err != nil {
						finish(err)
						return
					}
					observed += fmt.Sprintf(" gotAbove=%t", initialAbove)
					if initialAbove != above {
						a.g.timeoutAdd(20, callbacks().task, a.tasks.add(start))
						return
					}
					geometry := api.decorationGeometry(connection, initialID)
					verify := func(extents [4]uint32) error {
						var got [4]uint32
						current, placed := w.Frame()
						currentAbove, err := readAbove()
						if err != nil {
							return err
						}
						if !api.readDecorationProperty(connection, initialID, atom, xcbAtomCardinal, got[:], &present) || !present || got != extents ||
							api.decorationGeometry(connection, initialID) != geometry ||
							!placed || current != frame || w.g.widgetGetWindow(w.toplevel) != native || !w.focused || currentAbove != above {
							return fmt.Errorf("expected extents %v and unchanged focused window %v, got extents %v, client %v, focused=%t", extents, frame, got, current, w.focused)
						}
						return nil
					}
					if err := w.SetDecorated(false, func(err error) {
						if err == nil {
							err = verify([4]uint32{})
						}
						if err != nil {
							finish(fmt.Errorf("remove decorations: %w", err))
							return
						}
						if err := w.SetDecorated(true, func(err error) {
							if err == nil {
								err = verify(initial)
							}
							if err != nil {
								finish(err)
								return
							}
							checkHidden := func(err error) {
								if err != nil {
									finish(err)
									return
								}
								w.g.widgetHide(w.toplevel)
								if err := w.SetDecorated(false, func(err error) {
									if err == nil {
										finish(fmt.Errorf("accepted a decoration change on a hidden window"))
									} else {
										finish(nil)
									}
								}); err != nil {
									finish(err)
								}
							}
							if above {
								checkHidden(nil)
							} else {
								checkGTKDecorationRollback(w, func(err error) {
									if err == nil {
										err = verify(initial)
									}
									checkHidden(err)
								})
							}
						}); err != nil {
							finish(err)
						}
					}); err != nil {
						finish(err)
						return
					}
					if err := w.SetDecorated(false, func(error) {
						finish(fmt.Errorf("rejected duplicate called its completion"))
					}); err == nil {
						finish(fmt.Errorf("accepted two concurrent decoration changes"))
					}
				}
				a.enqueue(start)
				return nil
			})
			if err != nil || outcome != nil || !completed {
				t.Fatalf("GTK transition: completed=%t, loop=%v, request=%v", completed, err, outcome)
			}
		})
	}
}

func checkGTKDecorationRollback(w *gtkWindow, done func(error)) {
	pid, err := strconv.Atoi(os.Getenv("VIVA_X11_DECORATION_WM_PID"))
	if err != nil || pid <= 1 {
		done(fmt.Errorf("invalid private window manager pid"))
		return
	}
	name, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil || strings.TrimSpace(string(name)) != "xfwm4" {
		done(fmt.Errorf("private window manager is not Xfwm4"))
		return
	}
	if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
		done(err)
		return
	}
	if err := w.SetDecorated(false, func(err error) {
		resumed := syscall.Kill(pid, syscall.SIGCONT)
		if err == nil || !strings.Contains(err.Error(), "previous frame restored") || resumed != nil || w.g.windowGetDecorated(w.toplevel) == 0 {
			done(fmt.Errorf("ignored decoration request = %v; resume = %v", err, resumed))
			return
		}
		done(nil)
	}); err != nil {
		_ = syscall.Kill(pid, syscall.SIGCONT)
		done(err)
	}
}

func TestGTKDecorationRefusesWayland(t *testing.T) {
	w := &gtkWindow{app: &gtkApplication{wayland: true}}
	if err := w.SetDecorated(false, func(error) { t.Error("rejected request completed") }); !errors.Is(err, platform.ErrWindowDecorationUnsupported) {
		t.Fatalf("Wayland request = %v, want unsupported", err)
	}
}

func TestNativeGTKDecorationCompletesWhenQueuedWindowCloses(t *testing.T) {
	if os.Getenv("VIVA_GTK_DECORATION_NATIVE") != "1" {
		t.Skip("requires a private X11 session")
	}
	var outcomes []error
	if err := RunGTKApplication(func(application platform.Application) error {
		control, err := application.OpenWindow(platform.Config{Title: "Viva GTK decoration close probe", Width: 640, Height: 400})
		if err != nil {
			return err
		}
		w := control.(*gtkWindow)
		if err := w.SetDecorated(false, func(err error) { outcomes = append(outcomes, err) }); err != nil {
			return err
		}
		// Synchronous destruction overtakes the accepted GLib idle submission.
		w.destroy()
		if err := w.SetDecorated(true, func(error) { outcomes = append(outcomes, nil) }); err == nil {
			return fmt.Errorf("accepted a request on a closed window")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0] == nil {
		t.Fatalf("completions = %v, want one error", outcomes)
	}
}
