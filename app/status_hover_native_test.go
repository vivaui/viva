//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

type hoverTestHost struct {
	platform.Application
	config platform.StatusItemConfig
	item   hoverTestItem
}

func (h *hoverTestHost) AddStatusItem(config platform.StatusItemConfig) (platform.StatusItem, error) {
	h.config = config
	return &h.item, nil
}

type hoverTestItem struct {
	platform.StatusItem
	open      func(platform.ScreenRect) platform.Window
	refreshes int
}

func (i *hoverTestItem) SetHoverPanel(open func(platform.ScreenRect) platform.Window) bool {
	i.open = open
	return true
}
func (*hoverTestItem) SetMenuOnHover(bool) bool { return false }
func (i *hoverTestItem) RefreshMenu()           { i.refreshes++ }
func (*hoverTestItem) Remove()                  {}

func TestStatusHoverNativeContract(t *testing.T) {
	h := &hoverTestHost{}
	a := &Application{host: nativeHost{h}}
	item, err := a.AddStatusItem(StatusItemConfig{Icon: StatusIcon{Size: 1, RGBA: make([]byte, 4)}, LinuxTrayBackend: LinuxTrayXEmbed})
	if err != nil {
		t.Fatal(err)
	}
	if h.config.LinuxTrayBackend != platform.LinuxTrayXEmbed || h.config.OnClick != nil {
		t.Fatal("backend request or menu-only activation lost")
	}
	bounds := platform.ScreenRect{X: 12, Y: 34, Width: 16, Height: 16}
	panel := &fakeWindow{}
	if !item.SetHoverPanel(func(anchor StatusAnchor) *Window {
		if !anchor.set || anchor.rect != bounds {
			t.Fatalf("invalid anchor %+v", anchor)
		}
		return &Window{control: resultWindow{WindowControl: panel}}
	}) || h.item.open(bounds) != panel {
		t.Fatal("native hover window was not returned to its owner")
	}
	if item.SetMenuOnHover(true) {
		t.Fatal("unsupported menu reported success")
	}
	item.RefreshMenu()
	if !item.SetHoverPanel(nil) || h.item.open != nil {
		t.Fatal("callback retained after disabling hover")
	}
	item.Remove()
	item.RefreshMenu()
	if h.item.refreshes != 1 || item.SetHoverPanel(nil) {
		t.Fatal("removed item still forwards operations")
	}
}
