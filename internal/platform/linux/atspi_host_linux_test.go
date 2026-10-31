package linux

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// With accessibility off the host holds nothing, and a frame, a focus change
// or a closed window costs it no allocation and no bookkeeping.
func TestAtspiOffCostsNothing(t *testing.T) {
	nodes := make([]platform.AccessibilityNode, 200)
	state := &linuxWindowState{}
	for name, application := range map[string]*linuxApplication{
		"never asked":        {},
		"no service present": {atspi: &atspiHost{windows: map[any]*atspiWindow{}, latest: map[any][]platform.AccessibilityNode{}}},
	} {
		if fd := application.atspi.fd(); fd != -1 {
			t.Errorf("%s: polls descriptor %d", name, fd)
		}
		allocations := testing.AllocsPerRun(100, func() {
			application.atspiFrame(state, nodes)
			application.atspiActive(state, true)
			application.atspiClosed(state)
		})
		if allocations != 0 {
			t.Errorf("%s: %v allocations per frame", name, allocations)
		}
		if application.atspi != nil && len(application.atspi.latest) != 0 {
			t.Errorf("%s: the host keeps nodes nobody asked for", name)
		}
	}
}
