//go:build linux

package linux

import (
	"strings"
	"testing"
	"unsafe"
)

// TestWaylandFallbackNewIDTypes guards the types arrays of the hand-written
// fallback interfaces. libwayland dereferences wl_message.types without a NULL
// check when it prints a new-id request under WAYLAND_DEBUG, so a request that
// creates an object must carry a non-nil array or the client segfaults.
func TestWaylandFallbackNewIDTypes(t *testing.T) {
	interfaces := []struct {
		name  string
		iface *wlInterface
	}{
		{"xdg_wm_base", &xdgWmBaseInterface},
		{"xdg_surface", &xdgSurfaceInterface},
		{"xdg_toplevel", &xdgToplevelInterface},
		{"zxdg_decoration_manager_v1", &zxdgDecorationManagerV1Interface},
		{"zxdg_toplevel_decoration_v1", &zxdgToplevelDecorationV1Interface},
		{"wp_fractional_scale_manager_v1", &wpFractionalScaleManagerV1Interface},
		{"wp_fractional_scale_v1", &wpFractionalScaleV1Interface},
		{"wp_viewporter", &wpViewporterInterface},
		{"wp_viewport", &wpViewportInterface},
	}
	for _, tc := range interfaces {
		checkWaylandNewIDTypes(t, tc.name, tc.iface.Methods, tc.iface.MethodCount)
		checkWaylandNewIDTypes(t, tc.name, tc.iface.Events, tc.iface.EventCount)
	}

	modeled := []struct {
		name    string
		message wlMessage
		want    *wlInterface
	}{
		{"xdg_wm_base.create_positioner", xdgWmBaseRequests[1], nil},
		{"xdg_wm_base.get_xdg_surface", xdgWmBaseRequests[2], &xdgSurfaceInterface},
		{"xdg_surface.get_toplevel", xdgSurfaceRequests[1], &xdgToplevelInterface},
		{"xdg_surface.get_popup", xdgSurfaceRequests[2], nil},
		{"zxdg_decoration_manager_v1.get_toplevel_decoration", zxdgDecorationManagerV1Requests[1], &zxdgToplevelDecorationV1Interface},
		{"wp_fractional_scale_manager_v1.get_fractional_scale", wpFractionalScaleManagerV1Requests[1], &wpFractionalScaleV1Interface},
		{"wp_viewporter.get_viewport", wpViewporterRequests[1], &wpViewportInterface},
	}
	for _, request := range modeled {
		if request.message.Types == nil {
			t.Errorf("%s has nil types", request.name)
			continue
		}
		if got := *request.message.Types; got != request.want {
			t.Errorf("%s types[0] = %p, want %p", request.name, got, request.want)
		}
	}
}

func checkWaylandNewIDTypes(t *testing.T, ifaceName string, messages *wlMessage, count int32) {
	t.Helper()
	if messages == nil {
		return
	}
	for _, message := range unsafe.Slice(messages, int(count)) {
		if !strings.Contains(waylandGoString(message.Signature), "n") {
			continue
		}
		if message.Types == nil {
			t.Errorf("%s.%s has a new-id argument but nil types", ifaceName, waylandGoString(message.Name))
		}
	}
}
