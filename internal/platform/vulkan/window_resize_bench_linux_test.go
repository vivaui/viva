//go:build linux

package vulkan

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// BenchmarkLiveResize is a window growing and shrinking a pixel at a time: text
// preparation, swapchain rebuild and present, on a surface that needs no window
// system. The extra metrics separate the driver's waits from the renderer's work.
func BenchmarkLiveResize(b *testing.B) {
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
					Runs:  []gfx.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value)), Color: gfx.RGB(230, 230, 230)}},
				}
			}
			renderer, err := New(Config{
				Surface: headlessSurfaceSource(b), Width: 640, Height: 480, Scale: 1,
				Clear: platform.Color{Red: .08, Green: .08, Blue: .08, Alpha: 1}, Labels: labels, Rasterizer: newSolidRasterizer,
			})
			if errors.Is(err, platform.ErrNoGraphics) {
				b.Skipf("no Vulkan device: %v", err)
			}
			if err != nil {
				b.Skipf("no headless Vulkan surface: %v", err)
			}
			defer renderer.Close()
			var idleTime, createTime, destroyTime, submitTime, presentTime, acquireTime, fenceTime time.Duration
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
				for i := range labels {
					labels[i].Width = float32(width) - 16
				}
				frame := platform.Frame{
					TextLabels: labels,
					Rectangles: []render.Rectangle{{Bounds: scene.Rect{Width: float32(width), Height: float32(height)}, Color: scene.Color{Red: .08, Green: .08, Blue: .08, Alpha: 1}}},
					Update:     render.Update{Count: 1, Rebuild: true},
				}
				if _, err := renderer.Stage(frame); err != nil {
					b.Fatal(err)
				}
				if err := renderer.Resize(width, height); err != nil {
					b.Fatal(err)
				}
				if err := renderer.Present(); err != nil {
					b.Fatal(err)
				}
			}
			resize(641, 481)
			idleTime, createTime, destroyTime, submitTime, presentTime, acquireTime, fenceTime, creates = 0, 0, 0, 0, 0, 0, 0, 0
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
			for name, duration := range map[string]time.Duration{"idle": idleTime, "create": createTime, "destroy": destroyTime, "submit": submitTime, "present": presentTime, "acquire": acquireTime, "fence": fenceTime} {
				b.ReportMetric(float64(duration.Nanoseconds())/float64(b.N), name+"-ns/op")
			}
			b.ReportMetric(float64(creates)/float64(b.N), "swapchains/op")
		})
	}
}
