package input

import (
	"github.com/vivaui/viva/gfx"
	"testing"
)

// fuzzID maps a byte to an identity, with zero meaning none.
func fuzzID(n byte) gfx.ID {
	if n == 0 {
		return gfx.ID{}
	}
	return gfx.Item(int(n))
}

// routerOperation drives the router from fuzz bytes and checks that focus never
// rests on something the router would refuse to focus.
func runRouterOperations(t testing.TB, data []byte) {
	var router Router
	next := func() byte {
		if len(data) == 0 {
			return 0
		}
		value := data[0]
		data = data[1:]
		return value
	}
	for len(data) > 0 {
		switch next() % 6 {
		case 0:
			count := int(next() % 9)
			regions := make([]Region, count)
			for index := range regions {
				flags := next()
				regions[index] = Region{
					Target:    gfx.Item(int(next() % 6)),
					Bounds:    Rect{X: float32(next() % 40), Y: float32(next() % 40), Width: float32(next() % 30), Height: float32(next() % 30)},
					Scope:     gfx.Item(int(uint32(next() % 3))),
					Focusable: flags&1 != 0,
					Disabled:  flags&2 != 0,
					Blocks:    flags&4 != 0,
				}
			}
			router.SetRegions(regions)
		case 1:
			router.PointerDown(float32(next()%60), float32(next()%60))
		case 2:
			router.KeyDown(KeyEvent{Key: KeyTab, Modifiers: Modifiers(next() % 2)})
		case 3:
			router.Focus(gfx.Item(int(next() % 6)))
		case 4:
			router.SetActiveScope(fuzzID(next()%3), fuzzID(next()%6))
		case 5:
			router.Blur()
		}
		if target, focused := router.Focused(); focused {
			ok := false
			for _, region := range router.Regions() {
				if region.Target == target && router.focusAvailableRegion(region) {
					ok = true
				}
			}
			if !ok {
				t.Fatalf("focus rests on target %d, which no region lets take focus (scope %d, regions %+v)", target, router.ActiveScope(), router.Regions())
			}
		}
		if hit, ok := router.HitTest(float32(len(data)%60), 7); ok && hit.IsZero() {
			t.Fatalf("a hit reported the zero target")
		}
	}
}

func FuzzRouterKeepsFocusValid(f *testing.F) {
	f.Add([]byte{0, 3, 1, 1, 0, 0, 20, 20, 1, 3, 1, 4, 4, 0, 2, 0, 3, 2})
	f.Add([]byte{0, 2, 5, 2, 1, 1, 0, 30, 30, 0, 0, 3, 5, 2, 4, 1, 2, 2, 5})
	f.Fuzz(func(t *testing.T, data []byte) {
		runRouterOperations(t, data)
	})
}
