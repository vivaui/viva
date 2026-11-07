//go:build linux

package linux

import (
	"slices"
	"testing"
	"unsafe"
)

type swapchainRetireEvent struct {
	action string
	handle uintptr
}

// Use distinct handles so a retired chain cannot accidentally alias its replacement.
// Every destruction is checked while it happens, including cleanup after a failure.
type swapchainRetireHarness struct {
	renderer      *vulkanRenderer
	events        []swapchainRetireEvent
	allocated     []uintptr
	destroyed     map[uintptr]int
	acquireStatus int32
	presentStatus int32
}

func newSwapchainRetireHarness(t *testing.T) *swapchainRetireHarness {
	t.Helper()
	recreations := 0
	renderer := newSuboptimalTestRenderer(vkSuccess, vkSuccess, &recreations)
	harness := &swapchainRetireHarness{
		renderer:  renderer,
		allocated: []uintptr{renderer.swapchain},
		destroyed: make(map[uintptr]int),
	}
	last := renderer.swapchain
	renderer.api.createSwapchain = func(device uintptr, info *vkSwapchainCreateInfo, _ unsafe.Pointer, swapchain *uintptr) int32 {
		if info.OldSwapchain != last {
			t.Errorf("old swapchain = %d, want current chain %d", info.OldSwapchain, last)
		}
		if device != renderer.device || harness.destroyed[last] != 0 {
			t.Errorf("create uses device %d and destroyed old chain %d", device, last)
		}
		last++
		*swapchain = last
		harness.allocated = append(harness.allocated, last)
		harness.events = append(harness.events, swapchainRetireEvent{"create", last})
		return vkSuccess
	}
	renderer.api.destroySwapchain = func(device, swapchain uintptr, _ unsafe.Pointer) {
		if device != 10 || !slices.Contains(harness.allocated, swapchain) {
			t.Errorf("destroy uses device %d and unknown chain %d", device, swapchain)
		}
		harness.destroyed[swapchain]++
		if harness.destroyed[swapchain] != 1 {
			t.Errorf("swapchain %d destroyed %d times", swapchain, harness.destroyed[swapchain])
		}
		harness.events = append(harness.events, swapchainRetireEvent{"destroy", swapchain})
	}
	renderer.api.acquireNextImage = func(_, _ uintptr, _ uint64, _, _ uintptr, image *uint32) int32 {
		*image = 0
		return harness.acquireStatus
	}
	renderer.api.queuePresent = func(_ uintptr, info *vkPresentInfo) int32 {
		harness.events = append(harness.events, swapchainRetireEvent{"present", *info.Swapchains})
		return harness.presentStatus
	}
	t.Cleanup(renderer.destroySwapchain)
	return harness
}

func (harness *swapchainRetireHarness) assertAllDestroyed(t *testing.T) {
	t.Helper()
	for _, swapchain := range harness.allocated {
		if count := harness.destroyed[swapchain]; count != 1 {
			t.Errorf("swapchain %d destroyed %d times, want once", swapchain, count)
		}
	}
	if harness.renderer.swapchain != 0 || len(harness.renderer.retiredSwapchains) != 0 {
		t.Errorf("cleanup retained current=%d retired=%v", harness.renderer.swapchain, harness.renderer.retiredSwapchains)
	}
}

func TestSwapchainRetirementWaitsForReplacementPresent(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int32
	}{
		{name: "success", status: vkSuccess},
		{name: "suboptimal", status: vkSuboptimal},
	} {
		t.Run(test.name, func(t *testing.T) {
			harness := newSwapchainRetireHarness(t)
			renderer := harness.renderer
			if err := renderer.recreateSwapchain(); err != nil {
				t.Fatal(err)
			}
			if len(harness.destroyed) != 0 {
				t.Fatalf("destroyed old chain before replacement present: %v", harness.events)
			}
			harness.presentStatus = test.status
			if rebuild, err := renderer.presentOnce(); rebuild || err != nil {
				t.Fatalf("replacement present: rebuild=%v err=%v", rebuild, err)
			}
			want := []swapchainRetireEvent{{"create", 31}, {"present", 31}, {"destroy", 30}}
			if !slices.Equal(harness.events, want) {
				t.Fatalf("swapchain calls = %v, want %v", harness.events, want)
			}
			if len(renderer.retiredSwapchains) != 0 || renderer.swapchain != 31 {
				t.Fatalf("after present: current=%d retired=%v", renderer.swapchain, renderer.retiredSwapchains)
			}
			// A later frame must neither destroy the retired chain again nor destroy
			// the current chain while it is still used for presentation.
			if _, err := renderer.presentOnce(); err != nil {
				t.Fatal(err)
			}
			if harness.destroyed[30] != 1 || harness.destroyed[31] != 0 {
				t.Fatalf("destructions after another frame = %v", harness.destroyed)
			}
			renderer.destroySwapchain()
			harness.assertAllDestroyed(t)
		})
	}
}

func TestSwapchainRetirementSurvivesOutOfDate(t *testing.T) {
	for _, operation := range []string{"acquire", "present"} {
		t.Run(operation, func(t *testing.T) {
			harness := newSwapchainRetireHarness(t)
			renderer := harness.renderer
			if err := renderer.recreateSwapchain(); err != nil {
				t.Fatal(err)
			}
			if operation == "acquire" {
				harness.acquireStatus = vkErrorOutOfDate
			} else {
				harness.presentStatus = vkErrorOutOfDate
			}
			if rebuild, err := renderer.presentOnce(); !rebuild || err != nil {
				t.Fatalf("out-of-date present: rebuild=%v err=%v", rebuild, err)
			}
			if len(harness.destroyed) != 0 {
				t.Fatalf("out-of-date frame destroyed the last presented chain: %v", harness.events)
			}
			if err := renderer.recreateSwapchain(); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(renderer.retiredSwapchains, []uintptr{30, 31}) {
				t.Fatalf("retained chains = %v, want [30 31]", renderer.retiredSwapchains)
			}
			harness.acquireStatus, harness.presentStatus = vkSuccess, vkSuccess
			if rebuild, err := renderer.presentOnce(); rebuild || err != nil {
				t.Fatalf("recovered present: rebuild=%v err=%v", rebuild, err)
			}
			want := []swapchainRetireEvent{{"create", 31}}
			if operation == "present" {
				want = append(want, swapchainRetireEvent{"present", 31})
			}
			want = append(want, swapchainRetireEvent{"create", 32}, swapchainRetireEvent{"present", 32}, swapchainRetireEvent{"destroy", 30}, swapchainRetireEvent{"destroy", 31})
			if !slices.Equal(harness.events, want) {
				t.Fatalf("swapchain calls = %v, want %v", harness.events, want)
			}
			renderer.destroySwapchain()
			renderer.destroySwapchain()
			harness.assertAllDestroyed(t)
		})
	}
}

func TestSwapchainRetirementCleansFailedRebuild(t *testing.T) {
	for _, operation := range []string{"capabilities", "create", "images", "framebuffer", "commands"} {
		t.Run(operation, func(t *testing.T) {
			harness := newSwapchainRetireHarness(t)
			renderer := harness.renderer
			if err := renderer.recreateSwapchain(); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "capabilities":
				renderer.api.getSurfaceCapabilities = func(_, _ uintptr, _ *vkSurfaceCapabilities) int32 { return vkErrorInitializationFailed }
			case "create":
				renderer.api.createSwapchain = func(_ uintptr, _ *vkSwapchainCreateInfo, _ unsafe.Pointer, _ *uintptr) int32 {
					return vkErrorInitializationFailed
				}
			case "images":
				renderer.api.getSwapchainImages = func(_, _ uintptr, _ *uint32, _ *uintptr) int32 { return vkErrorInitializationFailed }
			case "framebuffer":
				renderer.api.createFramebuffer = func(_ uintptr, _ *vkFramebufferCreateInfo, _ unsafe.Pointer, _ *uintptr) int32 {
					return vkErrorInitializationFailed
				}
			case "commands":
				renderer.api.endCommandBuffer = func(uintptr) int32 { return vkErrorInitializationFailed }
			}
			if err := renderer.recreateSwapchain(); err == nil {
				t.Fatal("recreateSwapchain accepted an injected failure")
			}
			harness.assertAllDestroyed(t)
			renderer.destroySwapchain()
			harness.assertAllDestroyed(t)
		})
	}
}

func TestSwapchainRetirementIsBoundedWithoutPresent(t *testing.T) {
	harness := newSwapchainRetireHarness(t)
	renderer := harness.renderer
	for step := 1; step <= 3*maxRetiredSwapchains; step++ {
		if err := renderer.recreateSwapchain(); err != nil {
			t.Fatal(err)
		}
		if len(renderer.retiredSwapchains) > maxRetiredSwapchains {
			t.Fatalf("step %d retained %d chains, limit %d", step, len(renderer.retiredSwapchains), maxRetiredSwapchains)
		}
		live := 0
		for _, swapchain := range harness.allocated {
			if harness.destroyed[swapchain] == 0 {
				live++
			}
		}
		if want := 1 + min(step, maxRetiredSwapchains); live != want {
			t.Fatalf("step %d: %d live chains, want %d including current", step, live, want)
		}
		if step <= maxRetiredSwapchains && len(harness.destroyed) != 0 {
			t.Fatalf("destroyed a chain before reaching the retention limit: %v", harness.events)
		}
	}
	renderer.destroySwapchain()
	renderer.destroySwapchain()
	harness.assertAllDestroyed(t)
}

func TestSwapchainRetirementReleasesBeforeDevice(t *testing.T) {
	for _, operation := range []string{"none", "acquire", "submit", "present"} {
		t.Run(operation, func(t *testing.T) {
			harness := newSwapchainRetireHarness(t)
			renderer := harness.renderer
			for range 2 {
				if err := renderer.recreateSwapchain(); err != nil {
					t.Fatal(err)
				}
			}
			switch operation {
			case "acquire":
				harness.acquireStatus = vkErrorSurfaceLost
			case "submit":
				renderer.api.queueSubmit = func(uintptr, uint32, *vkSubmitInfo, uintptr) int32 { return vkErrorDeviceLost }
			case "present":
				harness.presentStatus = vkErrorDeviceLost
			}
			if operation != "none" {
				if _, err := renderer.presentOnce(); !isGraphicsLoss(err) {
					t.Fatalf("present failure = %v, want graphics loss", err)
				}
				if len(harness.destroyed) != 0 {
					t.Fatalf("failed present destroyed retained chains: %v", harness.events)
				}
			}
			renderer.api.destroyFence = func(uintptr, uintptr, unsafe.Pointer) {}
			destroyedDevice := 0
			renderer.api.destroyDevice = func(device uintptr, _ unsafe.Pointer) {
				destroyedDevice++
				if device != 10 {
					t.Errorf("destroyed device %d, want 10", device)
				}
				harness.assertAllDestroyed(t)
			}
			// Recovery and final teardown must drain retired chains before their
			// owning device disappears, even if no replacement frame was shown.
			renderer.releaseDevice()
			renderer.releaseDevice()
			if destroyedDevice != 1 || renderer.device != 0 {
				t.Fatalf("released device %d times; device still %d", destroyedDevice, renderer.device)
			}
			harness.assertAllDestroyed(t)
		})
	}
}
