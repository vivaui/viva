//go:build linux

package linux

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/ui"
)

// BenchmarkX11WindowLiveResize is opt-in because it briefly opens a real window.
// It includes the WM's configure round-trip, text preparation, swapchain rebuild,
// and present; the extra metrics separate driver waits from application work.
func BenchmarkX11WindowLiveResize(b *testing.B) {
	if os.Getenv("VIVA_LINUX_RESIZE_BENCH") == "" || os.Getenv("DISPLAY") == "" {
		b.Skip("set VIVA_LINUX_RESIZE_BENCH=1 with an X11 display and Vulkan driver")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	b.Setenv("VIVA_SCALE", "1")
	for _, clipped := range []bool{false, true} {
		b.Run(fmt.Sprintf("clipped=%t", clipped), func(b *testing.B) {
			labels := make([]platform.TextLabel, 28)
			for i := range labels {
				value := fmt.Sprintf("row %02d: user@host /workspace/build ready", i)
				if clipped {
					value += " 0123456789 abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ"
				}
				labels[i] = platform.TextLabel{
					Value: value, X: 8, Y: float32(8 + i*16), Width: 624, Height: 16,
					FontSize: 12, Mono: true, CellWidth: 7,
					Color: platform.Color{Red: .9, Green: .9, Blue: .9, Alpha: 1},
					Runs:  []ui.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value)), Color: ui.RGB(230, 230, 230)}},
				}
			}
			config := platform.Config{
				Title: "Viva resize benchmark", Width: 640, Height: 480,
				WindowMode: platform.WindowTitled,
				ClearColor: platform.Color{Red: .08, Green: .08, Blue: .08, Alpha: 1},
				TextLabels: labels,
			}
			config.OnResize = func(size platform.Size) (platform.Frame, error) {
				for i := range labels {
					labels[i].Width = size.Width - 16
				}
				return platform.Frame{TextLabels: labels, Rectangles: []render.Rectangle{{Bounds: scene.Rect{Width: size.Width, Height: size.Height}, Color: scene.Color{Red: .08, Green: .08, Blue: .08, Alpha: 1}}}, Update: render.Update{Count: 1, Rebuild: true}}, nil
			}
			application := &linuxApplication{}
			control, err := application.OpenWindow(config)
			if err != nil {
				b.Fatal(err)
			}
			defer application.closeAll()
			defer application.closeDesktop()
			defer application.closeStatus()
			defer application.atspiClose()
			state := control.(*nativeWindow).state
			win, renderer := state.win, state.renderer
			var idleTime, createTime, destroyTime, submitTime, presentTime, acquireTime, fenceTime, dispatchTime time.Duration
			var creates int
			idle := renderer.api.deviceWaitIdle
			renderer.api.deviceWaitIdle = func(device uintptr) int32 {
				start := time.Now()
				result := idle(device)
				idleTime += time.Since(start)
				return result
			}
			create := renderer.api.createSwapchain
			renderer.api.createSwapchain = func(device uintptr, info *vkSwapchainCreateInfo, allocator unsafe.Pointer, swapchain *uintptr) int32 {
				start := time.Now()
				result := create(device, info, allocator, swapchain)
				createTime += time.Since(start)
				creates++
				return result
			}
			destroy := renderer.api.destroySwapchain
			renderer.api.destroySwapchain = func(device, swapchain uintptr, allocator unsafe.Pointer) {
				start := time.Now()
				destroy(device, swapchain, allocator)
				destroyTime += time.Since(start)
			}
			submit := renderer.api.queueSubmit
			renderer.api.queueSubmit = func(queue uintptr, count uint32, info *vkSubmitInfo, fence uintptr) int32 {
				start := time.Now()
				result := submit(queue, count, info, fence)
				submitTime += time.Since(start)
				return result
			}
			present := renderer.api.queuePresent
			renderer.api.queuePresent = func(queue uintptr, info *vkPresentInfo) int32 {
				start := time.Now()
				result := present(queue, info)
				presentTime += time.Since(start)
				return result
			}
			acquire := renderer.api.acquireNextImage
			renderer.api.acquireNextImage = func(device, swapchain uintptr, timeout uint64, semaphore, fence uintptr, image *uint32) int32 {
				start := time.Now()
				result := acquire(device, swapchain, timeout, semaphore, fence, image)
				acquireTime += time.Since(start)
				return result
			}
			wait := renderer.api.waitForFences
			renderer.api.waitForFences = func(device uintptr, count uint32, fences *uintptr, all uint32, timeout uint64) int32 {
				start := time.Now()
				result := wait(device, count, fences, all, timeout)
				fenceTime += time.Since(start)
				return result
			}
			resize := func(width, height uint32) {
				b.Helper()
				size := [2]uint32{width, height}
				win.api.configureWindow(win.connection, win.id, xcbConfigWindowWidth|xcbConfigWindowHeight, &size[0])
				win.api.flush(win.connection)
				deadline := time.Now().Add(3 * time.Second)
				for win.width != width || win.height != height {
					event := state.nextEvent()
					if event == nil {
						if time.Now().After(deadline) {
							b.Fatalf("window stayed %dx%d, requested %dx%d", win.width, win.height, width, height)
						}
						time.Sleep(100 * time.Microsecond)
						continue
					}
					start := time.Now()
					closeWindow, err := state.handleEvent(event)
					dispatchTime += time.Since(start)
					win.api.free(event)
					if err != nil || closeWindow {
						b.Fatalf("resize dispatch close=%t: %v", closeWindow, err)
					}
				}
			}
			resize(641, 481)
			idleTime, createTime, presentTime, acquireTime, fenceTime, dispatchTime, creates = 0, 0, 0, 0, 0, 0, 0
			destroyTime, submitTime = 0, 0
			samples := make([]time.Duration, b.N)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				resize(640+uint32(i%32), 480+uint32(i%16))
				samples[i] = time.Since(start)
			}
			b.StopTimer()
			slices.Sort(samples)
			b.ReportMetric(float64(samples[(len(samples)*95-1)/100].Nanoseconds()), "p95-ns/op")
			b.ReportMetric(float64(samples[len(samples)-1].Nanoseconds()), "max-ns/op")
			for name, duration := range map[string]time.Duration{"idle": idleTime, "create": createTime, "destroy": destroyTime, "submit": submitTime, "present": presentTime, "acquire": acquireTime, "fence": fenceTime, "dispatch": dispatchTime} {
				b.ReportMetric(float64(duration.Nanoseconds())/float64(b.N), name+"-ns/op")
			}
			b.ReportMetric(float64(creates)/float64(b.N), "swapchains/op")
		})
	}
}
