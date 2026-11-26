//go:build !android

package linux

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

func TestX11DecorationRefusesHiddenWindow(t *testing.T) {
	state := &linuxWindowState{win: &window{}, hidden: true}
	called := false
	state.requestDecoration(false, func(err error) {
		called = true
		if err == nil {
			t.Error("accepted a decoration change on a hidden window")
		}
	})
	if !called || state.decorationChange != nil {
		t.Fatal("hidden-window refusal left a pending request")
	}
}

func TestNativeX11DecorationPreservesClient(t *testing.T) {
	if os.Getenv("VIVA_X11_DECORATION_NATIVE") != "1" {
		t.Skip("requires a private X11 session with a window manager")
	}
	t.Setenv("VIVA_SOFTWARE_RENDERING", "1")
	t.Setenv("VIVA_SCALE", "1")
	for _, size := range [][2]int{{640, 400}, {1280, 800}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			var outcome error
			err := RunApplication(func(application platform.Application) error {
				control, err := application.OpenWindow(platform.Config{
					Title: fmt.Sprintf("Viva live window decoration %dx%d", size[0], size[1]), Width: size[0], Height: size[1],
					ClearColor: platform.Color{Alpha: 1},
				})
				if err != nil {
					return err
				}
				native := control.(*nativeWindow)
				state := native.state
				win := state.win
				extentAtom, err := win.api.atom(win.connection, "_NET_FRAME_EXTENTS")
				if err != nil {
					return err
				}
				motifAtom, err := win.api.atom(win.connection, "_MOTIF_WM_HINTS")
				if err != nil {
					return err
				}
				var guard *time.Timer
				finished := false
				finish := func(err error) {
					if !finished {
						finished, outcome = true, err
						guard.Stop()
						application.Quit()
					}
				}
				guard = time.AfterFunc(8*time.Second, func() { application.Post(func() { finish(fmt.Errorf("decoration test did not complete")) }) })
				originalID := win.id
				var start func()
				start = func() {
					if finished {
						return
					}
					extents, ok := state.decorationExtents(extentAtom)
					if !state.exposed || !ok || extents == [4]uint32{} || win.width != uint32(size[0]) || win.height != uint32(size[1]) {
						time.AfterFunc(25*time.Millisecond, func() { application.Post(start) })
						return
					}
					geometry := state.decorationGeometry()
					var initial x11DecorationHints
					if !win.readDecorationProperty(motifAtom, motifAtom, initial.words[:], &initial.present) {
						finish(fmt.Errorf("could not read original Motif hints"))
						return
					}
					if err := native.SetDecorated(false, func(err error) {
						if err != nil {
							finish(fmt.Errorf("hide decorations: %w", err))
							return
						}
						if frame, ok := state.decorationExtents(extentAtom); !ok || frame != [4]uint32{} || win.id != originalID || state.decorationGeometry() != geometry {
							finish(fmt.Errorf("borderless window did not preserve its client rectangle or id: %v", frame))
							return
						}
						if err := native.SetDecorated(true, func(err error) {
							if err != nil {
								finish(fmt.Errorf("restore decorations: %w", err))
								return
							}
							if frame, ok := state.decorationExtents(extentAtom); !ok || frame != extents || win.id != originalID || state.decorationGeometry() != geometry {
								finish(fmt.Errorf("restored window did not preserve its frame and client rectangle: %v", frame))
								return
							}
							var restored x11DecorationHints
							if !win.readDecorationProperty(motifAtom, motifAtom, restored.words[:], &restored.present) || restored != initial {
								finish(fmt.Errorf("original Motif hints were not restored: %+v != %+v", restored, initial))
								return
							}
							if size[0] == 640 && os.Getenv("VIVA_X11_DECORATION_WM_PID") != "" {
								checkIgnoredDecoration(native, motifAtom, initial, finish)
							} else {
								finish(nil)
							}
						}); err != nil {
							finish(fmt.Errorf("submit restore: %w", err))
						}
					}); err != nil {
						finish(fmt.Errorf("submit hide: %w", err))
						return
					}
					if err := native.SetDecorated(false, func(error) { finish(fmt.Errorf("duplicate request completed")) }); err == nil {
						finish(fmt.Errorf("accepted a second decoration request before the first completed"))
					}
				}
				application.Post(start)
				return nil
			})
			if err != nil || outcome != nil {
				t.Fatalf("native transition: %v; loop: %v", outcome, err)
			}
		})
	}
}

func checkIgnoredDecoration(native *nativeWindow, motif uint32, initial x11DecorationHints, finish func(error)) {
	pid, err := strconv.Atoi(os.Getenv("VIVA_X11_DECORATION_WM_PID"))
	if err != nil || pid <= 1 {
		finish(fmt.Errorf("invalid private window manager pid"))
		return
	}
	name, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil || strings.TrimSpace(string(name)) != "xfwm4" {
		finish(fmt.Errorf("private window manager is not Xfwm4"))
		return
	}
	if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
		finish(err)
		return
	}
	if err := native.SetDecorated(false, func(err error) {
		resumeErr := syscall.Kill(pid, syscall.SIGCONT)
		if err == nil || !strings.Contains(err.Error(), "previous frame restored") || resumeErr != nil {
			finish(fmt.Errorf("ignored request: %v; resume: %v", err, resumeErr))
			return
		}
		var rolledBack x11DecorationHints
		if !native.state.win.readDecorationProperty(motif, motif, rolledBack.words[:], &rolledBack.present) || rolledBack != initial {
			finish(fmt.Errorf("ignored request did not restore original hints: %+v", rolledBack))
			return
		}
		finish(nil)
	}); err != nil {
		_ = syscall.Kill(pid, syscall.SIGCONT)
		finish(err)
	}
}

func TestNativeX11DecorationCompletesWhenQueuedWindowCloses(t *testing.T) {
	if os.Getenv("VIVA_X11_DECORATION_NATIVE") != "1" {
		t.Skip("requires a private X11 session with a window manager")
	}
	t.Setenv("VIVA_SOFTWARE_RENDERING", "1")
	var outcomes []error
	if err := RunApplication(func(application platform.Application) error {
		control, err := application.OpenWindow(platform.Config{
			Title: "Viva decoration close regression", Width: 640, Height: 400, ClearColor: platform.Color{Alpha: 1},
		})
		if err != nil {
			return err
		}
		native := control.(*nativeWindow)
		if err := native.SetDecorated(false, func(err error) { outcomes = append(outcomes, err) }); err != nil {
			return err
		}
		// Close on the loop before it can drain the queued submission.
		application.(*linuxApplication).closeWindow(native.state)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0] == nil || !strings.Contains(outcomes[0].Error(), "window closed") {
		t.Fatalf("closed request outcomes = %v", outcomes)
	}
}
