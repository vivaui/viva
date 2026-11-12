package platform

import (
	"strings"
	"testing"
)

func TestMainFrameOnlyBridgeGuardsAndLeavesTheBridgeAlone(t *testing.T) {
	if !strings.Contains(WebBridgeMainFrameOnlyScript, "window.top !== window") {
		t.Fatal("the main-frame-only bridge does not stop in a subframe")
	}
	if !strings.Contains(WebBridgeMainFrameOnlyScript, "Object.defineProperty(window, 'viva'") {
		t.Fatal("the main-frame-only bridge does not define window.viva")
	}
	if strings.Contains(WebBridgeScript, "window.top") {
		t.Fatal("WebBridgeScript was changed: an application that wants every frame still gets it")
	}
}
