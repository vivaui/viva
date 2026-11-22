package vulkan

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestGraphicsLossIsRecognizedThroughWrapping(t *testing.T) {
	lost := &vulkanResultError{action: "present swapchain image", status: vkErrorDeviceLost}
	if !isGraphicsLoss(lost) || !isGraphicsLoss(fmt.Errorf("linux: upload label 3: %w", lost)) {
		t.Fatal("a lost device is a loss, wrapped or not")
	}
	if !isGraphicsLoss(&vulkanResultError{status: vkErrorSurfaceLost}) {
		t.Fatal("a lost surface is a loss")
	}
	for _, err := range []error{nil, errors.New("other"), &vulkanResultError{status: -3}, &vulkanResultError{status: -1000001004}} {
		if isGraphicsLoss(err) {
			t.Fatalf("%v is not a loss", err)
		}
	}
}

func TestVulkanResultErrorKeepsItsMessage(t *testing.T) {
	message := (&vulkanResultError{action: "create swapchain", status: -4}).Error()
	if !strings.Contains(message, "create swapchain failed with Vulkan result -4") {
		t.Fatalf("message = %q", message)
	}
}
