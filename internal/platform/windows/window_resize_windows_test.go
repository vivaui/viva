package windows

import (
	"fmt"
	"runtime"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// A programmatic resize reaches the window as WM_SIZE outside a drag loop. The
// frame for the new size must reach the swap chain before WM_SIZE returns, or
// the desktop has only the previous one to show.
func TestResizePresentsTheNewSizeBeforeReturning(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva resize present test", Width: 640, Height: 400,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
			ClearColor: platform.Color{Alpha: 1},
			OnResize: func(platform.Size) (platform.Frame, error) {
				return platform.Frame{Update: render.Update{Rebuild: true}}, nil
			},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		if err := state.renderer.present(640, 400, 1); err != nil {
			return err
		}
		// What the window manager sends while the user drags the edge.
		windowProc(state.window, wmSize, 0, uintptr(uint32(880)|uint32(520)<<16))
		if state.renderer.width != 880 || state.renderer.height != 520 {
			t.Fatalf("the swap chain is %dx%d after WM_SIZE, want 880x520: the previous frame would be shown at the old size over the new client area", state.renderer.width, state.renderer.height)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Old pixels keep their native size; newly exposed pixels use the clear color.
func TestWindowSwapChainDoesNotStretchStaleFrames(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva swap chain scaling test", Width: 320, Height: 200,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
			ClearColor: platform.Color{Red: .2, Green: .4, Blue: .6, Alpha: 1},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		var description swapChainDesc1
		// IDXGISwapChain1::GetDesc1.
		if err := hresult(comCall(state.renderer.swap, 18, uintptr(unsafe.Pointer(&description)))); err != nil {
			return err
		}
		if description.Scaling != 1 { // DXGI_SCALING_NONE
			t.Fatalf("swap chain scaling = %d, want DXGI_SCALING_NONE (1)", description.Scaling)
		}
		var background [4]float32
		// IDXGISwapChain1::GetBackgroundColor follows SetBackgroundColor.
		if err := hresult(comCall(state.renderer.swap, 26, uintptr(unsafe.Pointer(&background)))); err != nil {
			return err
		}
		color := state.renderer.color
		if background != [4]float32{float32(color.Red), float32(color.Green), float32(color.Blue), float32(color.Alpha)} {
			t.Fatalf("swap chain background = %v, want renderer clear color", background)
		}
		if description.SwapEffect != 3 { // DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL
			t.Fatalf("swap chain effect = %d, want DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL (3)", description.SwapEffect)
		}
		if description.Buffers != 2 {
			t.Fatalf("swap chain buffers = %d, want 2", description.Buffers)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A burst of native sizes is laid out once at the next modal timer tick.
func TestLiveResizeCoalescesLayoutBeforeExit(t *testing.T) {
	for _, software := range []string{"0", "1"} {
		t.Run("software="+software, func(t *testing.T) {
			t.Setenv("VIVA_SOFTWARE_RENDERING", software)
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			err := RunApplication(func(application platform.Application) error {
				resizes := 0
				var laidOut platform.Size
				opened, err := application.OpenWindow(platform.Config{
					Title: "Viva live resize test", Width: 640, Height: 400,
					WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
					ClearColor: platform.Color{Alpha: 1},
					OnResize: func(size platform.Size) (platform.Frame, error) {
						laidOut = size
						resizes++
						return platform.Frame{Update: render.Update{Rebuild: true}}, nil
					},
				})
				if err != nil {
					return err
				}
				defer opened.Close()
				state := opened.(*nativeWindow).state
				if err := state.renderer.present(640, 400, 1); err != nil {
					return err
				}
				windowProc(state.window, 0x0231, 0, 0) // WM_ENTERSIZEMOVE
				if !state.sizingMove || state.resizeTimerInterval < 10 {
					return fmt.Errorf("modal entry did not arm the resize timer")
				}
				before := resizes
				for step := range 12 {
					width, height := uintptr(700+step*11), uintptr(450+step*7)
					if ok, _, err := setWindowPosition.Call(state.window, 0, 0, 0, width, height, 0x16); ok == 0 {
						return err
					}
					if resizes != before {
						return fmt.Errorf("drag step %d rendered: OnResize ran %d times during the drag", step, resizes-before)
					}
					if state.renderer.width != 640 || state.renderer.height != 400 {
						return fmt.Errorf("drag step %d resized the swap chain to %dx%d", step, state.renderer.width, state.renderer.height)
					}
				}
				presents := resizeTestPresentCount(t, state)
				windowProc(state.window, wmTimer, resizeTimerID, 0)
				if resizes != before+1 || resizeTestPresentCount(t, state) != presents+1 {
					return fmt.Errorf("timer must lay out and present once before exit")
				}
				windowProc(state.window, wmTimer, resizeTimerID, 0)
				windowProc(state.window, wmExitSizeMove, 0, 0)
				if state.sizingMove || state.resizeTimerInterval != 0 {
					return fmt.Errorf("modal exit retained resize timer state")
				}
				if resizeTestPresentCount(t, state) != presents+1 {
					return fmt.Errorf("clean tick or exit presented again")
				}
				var client rect
				getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
				if state.renderer.width != int(client.Right) || state.renderer.height != int(client.Bottom) {
					return fmt.Errorf("after the drag the swap chain is %dx%d, client is %dx%d", state.renderer.width, state.renderer.height, client.Right, client.Bottom)
				}
				if resizes != before+1 || laidOut != state.logicalSize(int(client.Right), int(client.Bottom)) {
					return fmt.Errorf("timer did not lay out the latest client size exactly once")
				}
				windowProc(state.window, wmEnterSizeMove, 0, 0)
				if ok, _, err := setWindowPosition.Call(state.window, 0, 0, 0, 900, 600, 0x16); ok == 0 {
					return err
				}
				windowProc(state.window, wmExitSizeMove, 0, 0)
				getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
				if resizes != before+2 || resizeTestPresentCount(t, state) != presents+2 || laidOut != state.logicalSize(int(client.Right), int(client.Bottom)) {
					return fmt.Errorf("exit failed to flush geometry pending before the next tick")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFlipWindowResizeAndRecoveryWithoutBackdrop(t *testing.T) {
	for _, software := range []string{"0", "1"} {
		t.Run("software="+software, func(t *testing.T) {
			t.Setenv("VIVA_SOFTWARE_RENDERING", software)
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			err := RunApplication(func(application platform.Application) error {
				var resized platform.Size
				opened, err := application.OpenWindow(platform.Config{
					Title: "Viva flip resize test", Width: 640, Height: 400,
					WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
					ClearColor: platform.Color{Red: .2, Green: .4, Blue: .6, Alpha: 1},
					OnResize: func(size platform.Size) (platform.Frame, error) {
						resized = size
						return platform.Frame{Update: render.Update{Rebuild: true}}, nil
					},
				})
				if err != nil {
					return err
				}
				defer opened.Close()
				state := opened.(*nativeWindow).state
				check := func() error {
					style, _, _ := readCompositionWindowStyle.Call(state.window, ^uintptr(19))
					if style&wsExNoRedirectionBitmap != 0 {
						return fmt.Errorf("window without a backdrop must keep the baseline redirection style")
					}
					var client rect
					getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
					if state.renderer.width != int(client.Right) || state.renderer.height != int(client.Bottom) {
						return fmt.Errorf("presented %dx%d, client is %dx%d", state.renderer.width, state.renderer.height, client.Right, client.Bottom)
					}
					if resized != state.logicalSize(int(client.Right), int(client.Bottom)) {
						return fmt.Errorf("layout size %+v does not match client", resized)
					}
					return nil
				}
				for _, scale := range []float32{1, 1.25, 1.5, 2} {
					state.scale = scale
					for step := range 12 {
						width, height := uintptr(640+step*13), uintptr(400+step*7)
						if step%2 == 1 {
							width, height = 800-width/4, 600-height/4
						}
						ok, _, err := setWindowPosition.Call(state.window, 0, 0, 0, width, height, 0x16)
						if ok == 0 {
							return err
						}
						if err := check(); err != nil {
							return err
						}
					}
				}
				if err := state.renderer.recover(software == "1"); err != nil {
					return err
				}
				setWindowPosition.Call(state.window, 0, 0, 0, 720, 480, 0x16)
				if err := check(); err != nil {
					return err
				}
				showWindow.Call(state.window, swMinimize)
				showWindow.Call(state.window, swRestore)
				return check()
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Redraw notifications must yield to input until WM_PAINT. Use a counted post
// callback so delivery is deterministic without pumping unrelated native messages.
func TestRedrawWaitsForPaintAndRetainsInvalidations(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva deferred redraw test", Width: 320, Height: 200,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
			ClearColor: platform.Color{Alpha: 1},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		windowProc(state.window, wmPaint, 0, 0)
		posts, redraws, moves := 0, 0, 0
		post := func() bool { posts++; return true }
		invalidate := func() {
			for range 20 {
				state.redraws.post(post)
			}
		}
		state.config.OnRedraw = func() (platform.Frame, error) {
			redraws++
			if redraws == 1 {
				invalidate() // Changes arriving while this frame is being built.
			}
			return platform.Frame{Update: render.Update{Rebuild: true}}, nil
		}
		state.config.OnPointer = func(event platform.PointerEvent) (platform.Frame, error) {
			if event.Kind == platform.PointerMove {
				moves++
			}
			return platform.Frame{}, nil
		}
		invalidate()
		windowProc(state.window, wmRedraw, 0, 0)
		invalidate() // Still coalesced after delivery, until painting begins.
		windowProc(state.window, wmMouseMove, 0, uintptr(40|60<<16))
		if posts != 1 || redraws != 0 || moves != 1 {
			return fmt.Errorf("before paint: posts=%d redraws=%d mouse moves=%d, want 1, 0, 1", posts, redraws, moves)
		}
		windowProc(state.window, wmPaint, 0, 0)
		if redraws != 1 || posts != 2 {
			return fmt.Errorf("first paint: redraws=%d posts=%d, want 1, 2 (one retained redraw)", redraws, posts)
		}
		windowProc(state.window, wmRedraw, 0, 0)
		invalidate()
		windowProc(state.window, wmMouseMove, 0, uintptr(41|61<<16))
		if redraws != 1 || posts != 2 || moves != 2 {
			return fmt.Errorf("between paints: redraws=%d posts=%d mouse moves=%d, want 1, 2, 2", redraws, posts, moves)
		}
		windowProc(state.window, wmPaint, 0, 0)
		presented := resizeTestPresentCount(t, state)
		windowProc(state.window, wmPaint, 0, 0)
		if resizeTestPresentCount(t, state) != presented {
			return fmt.Errorf("redraw left a dirty region after paint: an empty paint presented again")
		}
		if redraws != 2 || posts != 2 {
			return fmt.Errorf("after paints: redraws=%d posts=%d, want 2, 2", redraws, posts)
		}
		return state.err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestContinuousRedrawRequestsYieldToNativePaint(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	redraws, presents := 0, 0
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva redraw fairness test", Width: 320, Height: 200,
			ClearColor: platform.Color{Alpha: 1},
		})
		if err != nil {
			return err
		}
		state := opened.(*nativeWindow).state
		request := func() {
			state.redraws.post(func() bool {
				posted, _, _ := postMessage.Call(state.window, wmRedraw, 0, 0)
				return posted != 0
			})
		}
		state.config.OnRedraw = func() (platform.Frame, error) {
			redraws++
			if redraws >= 64 {
				opened.Close()
			} else {
				request()
			}
			return platform.Frame{Update: render.Update{Rebuild: true}}, nil
		}
		state.config.FramePresented = func() {
			if redraws > 0 {
				presents++
				opened.Close()
			}
		}
		request()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if presents == 0 || redraws >= 64 {
		t.Fatalf("continuous redraw requests starved native paint: redraws=%d presents=%d", redraws, presents)
	}
}

func TestNoOpFrameDoesNotAcknowledgePendingPaint(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva pending paint test", Width: 320, Height: 200,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
			ClearColor: platform.Color{Alpha: 1},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		windowProc(state.window, wmPaint, 0, 0)
		acknowledged := 0
		state.config.FramePresented = func() { acknowledged++ }
		state.frame(platform.Frame{}, nil)
		if acknowledged != 1 {
			return fmt.Errorf("idle no-op acknowledged %d frames, want 1", acknowledged)
		}
		before := resizeTestPresentCount(t, state)
		state.frame(platform.Frame{Update: render.Update{Rebuild: true}}, nil)
		for range 3 {
			state.frame(platform.Frame{}, nil)
		}
		if acknowledged != 1 || resizeTestPresentCount(t, state) != before {
			return fmt.Errorf("dirty frame was acknowledged or presented before WM_PAINT: acknowledgements=%d, want 1", acknowledged)
		}
		windowProc(state.window, wmPaint, 0, 0)
		if acknowledged != 2 || resizeTestPresentCount(t, state) != before+1 {
			return fmt.Errorf("paint must present and acknowledge the pending frame exactly once: acknowledgements=%d, want 2", acknowledged)
		}
		state.frame(platform.Frame{}, nil)
		if acknowledged != 3 {
			return fmt.Errorf("no-op after paint: acknowledgements=%d, want 3", acknowledged)
		}
		return state.err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A move-only drag can consume dirty paints without changing the client size.
// The exit must still present them, then forget the deferred work.
func TestSameSizeDragExitPresentsPendingUpdates(t *testing.T) {
	for _, software := range []string{"0", "1"} {
		t.Run("software="+software, func(t *testing.T) {
			t.Setenv("VIVA_SOFTWARE_RENDERING", software)
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			err := RunApplication(func(application platform.Application) error {
				opened, err := application.OpenWindow(platform.Config{
					Title: "Viva same-size drag test", Width: 320, Height: 200,
					WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
					ClearColor: platform.Color{Alpha: 1},
				})
				if err != nil {
					return err
				}
				defer opened.Close()
				state := opened.(*nativeWindow).state
				var client rect
				getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
				state.presentClientSize(int(client.Right), int(client.Bottom))
				if state.err != nil {
					return state.err
				}
				before := resizeTestPresentCount(t, state)
				acknowledged, resizes := 0, 0
				state.config.FramePresented = func() { acknowledged++ }
				state.config.OnResize = func(platform.Size) (platform.Frame, error) {
					resizes++
					return platform.Frame{Update: render.Update{Rebuild: true}}, nil
				}
				windowProc(state.window, wmEnterSizeMove, 0, 0)
				for range 3 {
					state.frame(platform.Frame{Update: render.Update{Rebuild: true}}, nil)
					windowProc(state.window, wmPaint, 0, 0)
				}
				if acknowledged != 0 || resizes != 0 || resizeTestPresentCount(t, state) != before {
					return fmt.Errorf("during drag: acknowledgements=%d resizes=%d; want 0, 0 and no native presents", acknowledged, resizes)
				}
				windowProc(state.window, wmExitSizeMove, 0, 0)
				if resizes != 0 || acknowledged != 1 || resizeTestPresentCount(t, state) != before+1 {
					return fmt.Errorf("same-size exit: resizes=%d acknowledgements=%d; want 0, 1 and one native present", resizes, acknowledged)
				}
				// No new updates: another move-only drag should do no rendering.
				windowProc(state.window, wmEnterSizeMove, 0, 0)
				windowProc(state.window, wmExitSizeMove, 0, 0)
				if resizes != 0 || acknowledged != 1 || resizeTestPresentCount(t, state) != before+1 {
					return fmt.Errorf("clean move-only exit rendered or acknowledged an extra frame")
				}
				return state.err
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestResizeTimerRetainsCallbackUpdates(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva resize callback test", Width: 320, Height: 200,
			ClearColor: platform.Color{Alpha: 1},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		windowProc(state.window, wmPaint, 0, 0)
		before := resizeTestPresentCount(t, state)
		redraws, acknowledgements := 0, 0
		state.config.OnRedraw = func() (platform.Frame, error) {
			redraws++
			// A nested timer must not enter layout/render again. A new redraw
			// delivered inside this callback belongs to the following tick.
			windowProc(state.window, wmTimer, resizeTimerID, 0)
			if redraws == 1 {
				windowProc(state.window, wmRedraw, 0, 0)
			}
			return platform.Frame{Update: render.Update{Rebuild: true}}, nil
		}
		state.config.FramePresented = func() {
			acknowledgements++
			if acknowledgements == 1 {
				state.frame(platform.Frame{Update: render.Update{Rebuild: true}}, nil)
			}
		}
		windowProc(state.window, wmEnterSizeMove, 0, 0)
		for range 10 {
			windowProc(state.window, wmRedraw, 0, 0)
			windowProc(state.window, wmPaint, 0, 0)
		}
		if redraws != 0 || acknowledgements != 0 || !state.redrawRequested {
			return fmt.Errorf("paint bypassed modal cadence or consumed pending redraw")
		}
		windowProc(state.window, wmTimer, resizeTimerID, 0)
		if redraws != 1 || acknowledgements != 1 || !state.redrawRequested || !state.paintPending {
			return fmt.Errorf("first tick lost callback updates: redraws=%d acknowledgements=%d", redraws, acknowledgements)
		}
		windowProc(state.window, wmExitSizeMove, 0, 0)
		if redraws != 2 || acknowledgements != 2 || resizeTestPresentCount(t, state) != before+2 {
			return fmt.Errorf("exit did not flush callback updates once")
		}
		windowProc(state.window, wmTimer, resizeTimerID, 0)
		if resizeTestPresentCount(t, state) != before+2 {
			return fmt.Errorf("stale modal timer presented after exit")
		}
		return state.err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestZeroSizeFrameAcknowledgesAndRestoresPendingPaint(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva zero-size acknowledgement test", Width: 320, Height: 200,
			ClearColor: platform.Color{Alpha: 1},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		windowProc(state.window, wmPaint, 0, 0)
		var client rect
		getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
		acknowledged := 0
		callbackSawPending := false
		queueNext := false
		state.config.FramePresented = func() {
			acknowledged++
			callbackSawPending = callbackSawPending || state.paintPending
			if queueNext {
				state.frame(platform.Frame{Update: render.Update{Rebuild: true}}, nil)
			}
		}
		for _, size := range [][2]int{{0, 200}, {320, 0}, {0, 0}} {
			before := resizeTestPresentCount(t, state)
			acks := acknowledged
			state.presentClientSize(size[0], size[1])
			if state.err != nil {
				return state.err
			}
			if acknowledged != acks+1 || state.paintPending || callbackSawPending || resizeTestPresentCount(t, state) != before {
				return fmt.Errorf("empty size %v did not acknowledge and clear the pending frame without presenting", size)
			}
			queueNext = true
			state.presentClientSize(size[0], size[1])
			queueNext = false
			if acknowledged != acks+2 || !state.paintPending || callbackSawPending || resizeTestPresentCount(t, state) != before {
				return fmt.Errorf("empty size %v lost the acknowledgement callback's pending frame", size)
			}
			state.presentClientSize(int(client.Right), int(client.Bottom))
			if state.err != nil {
				return state.err
			}
			if acknowledged != acks+3 || state.paintPending || resizeTestPresentCount(t, state) != before+1 {
				return fmt.Errorf("positive-size restoration did not present and acknowledge the pending frame once")
			}
		}
		return state.err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestResizeTimerRearmsOnlyWhenIntervalChanges(t *testing.T) {
	state := windowState{}
	var intervals []uintptr
	armErr := fmt.Errorf("SetTimer failed")
	fail := false
	arm := func(interval uintptr) error {
		intervals = append(intervals, interval)
		if fail {
			return armErr
		}
		return nil
	}
	check := func(interval, cached uintptr, calls int, wantErr error) {
		t.Helper()
		if err := state.rearmResizeTimer(interval, arm); err != wantErr {
			t.Fatalf("rearm(%d) error = %v, want %v", interval, err, wantErr)
		}
		if state.resizeTimerInterval != cached || len(intervals) != calls {
			t.Fatalf("rearm(%d): cached=%d calls=%v, want cache %d and %d calls", interval, state.resizeTimerInterval, intervals, cached, calls)
		}
		if calls > 0 && intervals[calls-1] < 10 {
			t.Fatalf("timer interval below Win32 minimum: %v", intervals)
		}
	}
	check(17, 0, 0, nil) // No modal timer outside a drag.
	state.sizingMove = true
	check(17, 17, 1, nil)
	for range 10 {
		check(17, 17, 1, nil) // Moves on the same refresh cadence must not defer the tick.
	}
	check(7, 10, 2, nil) // Move to a faster monitor, clamped to 10 ms.
	check(9, 10, 2, nil)
	check(34, 34, 3, nil) // Display mode changed while still dragging.
	fail = true
	check(17, 34, 4, armErr) // A failed rearm must not cache the requested interval.
	fail = false
	check(17, 17, 5, nil)
	state.sizingMove = false
	state.resizeTimerInterval = 0
	check(34, 0, 5, nil)
	state.sizingMove = true
	check(17, 17, 6, nil) // A new drag must arm even at the previous cadence.
}

// A drag whose timer cannot start keeps the window open and presents each size
// change synchronously; the next drag arms the timer again.
func TestResizeTimerFailureDegradesToSynchronousResize(t *testing.T) {
	state := windowState{sizingMove: true}
	state.paceResize(17, func(uintptr) error { return fmt.Errorf("SetTimer failed") })
	if state.err != nil || state.sizingMove || state.resizeTimerInterval != 0 {
		t.Fatalf("failed timer: err=%v sizingMove=%v cached=%d, want an open window and synchronous resizing", state.err, state.sizingMove, state.resizeTimerInterval)
	}
	var armed uintptr
	state.sizingMove = true // WM_ENTERSIZEMOVE of the next drag
	state.paceResize(17, func(interval uintptr) error { armed = interval; return nil })
	if armed != 17 || state.resizeTimerInterval != 17 || state.err != nil {
		t.Fatalf("next drag armed %d, cached %d, err=%v", armed, state.resizeTimerInterval, state.err)
	}
}

// Query actual swap-chain presents: dimensions and frame acknowledgements alone
// cannot distinguish a suppressed paint from a successful same-size present.
func resizeTestPresentCount(t *testing.T, state *windowState) uint32 {
	t.Helper()
	var count uint32
	// IDXGISwapChain::GetLastPresentCount.
	if err := hresult(comCall(state.renderer.swap, 17, uintptr(unsafe.Pointer(&count)))); err != nil {
		t.Fatal(err)
	}
	return count
}

// BenchmarkResizePresent measures what one live-resize step costs on the window
// thread: apply a frame and present it at a new client size, including
// ResizeBuffers. A drag is one such step per pointer move.
func BenchmarkResizePresent(b *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva resize benchmark", Width: 640, Height: 400,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
			ClearColor: platform.Color{Alpha: 1},
			OnResize: func(platform.Size) (platform.Frame, error) {
				return platform.Frame{Update: render.Update{Rebuild: true}}, nil
			},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			width := 640 + i%320
			height := 400 + i%240
			if ok, _, err := setWindowPosition.Call(state.window, 0, 0, 0, uintptr(width), uintptr(height), 0x16); ok == 0 {
				return err
			}
		}
		b.StopTimer()
		return nil
	})
	if err != nil {
		b.Fatal(err)
	}
}
