//go:build linux && !android && vivaxembed

package linux

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

type embeddedTrayItem struct {
	loop            *embeddedTrayLoop
	config          platform.StatusItemConfig
	icon, menu      uintptr
	actions         map[uintptr]func()
	open            func(platform.ScreenRect) platform.Window
	panel           *nativeWindow
	inIcon, inPanel bool
	timer           *time.Timer
	generation      uint64
	removed         atomic.Bool
}

// Only GTK object handles cross the C callback boundary. Callbacks are shared
// by all items and menu builds; no Go pointer is retained by GTK.
var embeddedItems sync.Map

func trayItem(id uintptr) *embeddedTrayItem {
	value, _ := embeddedItems.Load(id)
	item, _ := value.(*embeddedTrayItem)
	return item
}

var trayEnter = sync.OnceValue(func() uintptr {
	return purego.NewCallback(func(_, _, id uintptr) uintptr {
		if i := trayItem(id); i != nil && !i.removed.Load() {
			i.crossIcon(true)
		}
		return 0
	})
})
var trayLeave = sync.OnceValue(func() uintptr {
	return purego.NewCallback(func(_, _, id uintptr) uintptr {
		if i := trayItem(id); i != nil && !i.removed.Load() {
			i.crossIcon(false)
		}
		return 0
	})
})
var trayActivate = sync.OnceValue(func() uintptr {
	return purego.NewCallback(func(_, id uintptr) {
		if i := trayItem(id); i != nil && !i.removed.Load() {
			if i.config.OnClick != nil {
				stamp := panelDismissedAt.Load()
				i.config.OnClick(platform.StatusClick{Button: gfx.PointerButtonPrimary, Anchor: i.anchor(), PanelDismissed: stamp != 0 && time.Duration(monotonicNow()-stamp) < panelDismissWindow})
			} else {
				i.popupMenu(0, 0)
			}
		}
	})
})
var trayContext = sync.OnceValue(func() uintptr {
	return purego.NewCallback(func(_ uintptr, button, timestamp uint32, id uintptr) {
		if i := trayItem(id); i != nil && !i.removed.Load() {
			i.popupMenu(button, timestamp)
		}
	})
})
var trayAction = sync.OnceValue(func() uintptr {
	return purego.NewCallback(func(widget, id uintptr) {
		if i := trayItem(id); i != nil && !i.removed.Load() {
			if fn := i.actions[widget]; fn != nil {
				i.loop.app.Post(func() {
					if !i.removed.Load() {
						fn()
					}
				})
			}
		}
	})
})
var trayMenuDone = sync.OnceValue(func() uintptr {
	return purego.NewCallback(func(menu, id uintptr) {
		if i := trayItem(id); i != nil {
			i.loop.app.Post(func() {
				if i.menu == menu {
					i.closeMenu()
				}
			})
		}
	})
})

func (l *embeddedTrayLoop) add(config platform.StatusItemConfig) *embeddedTrayItem {
	g := l.g
	icon := g.newIcon()
	if icon == 0 {
		return nil
	}
	i := &embeddedTrayItem{loop: l, config: config, icon: icon}
	xid := g.iconXID(icon)
	window := g.xwindow(g.display(), uintptr(xid))
	var widget uintptr
	if window != 0 {
		g.windowWidget(window, &widget)
	}
	if widget == 0 || i.SetIcon(config.Icon) != nil {
		g.iconVisible(icon, 0)
		g.unref(icon)
		return nil
	}
	embeddedItems.Store(icon, i)
	l.items[icon] = i
	g.addEvents(widget, 1<<12|1<<13) // GDK_ENTER_NOTIFY_MASK | GDK_LEAVE_NOTIFY_MASK
	i.signal(widget, "enter-notify-event", trayEnter())
	i.signal(widget, "leave-notify-event", trayLeave())
	i.signal(icon, "activate", trayActivate())
	i.signal(icon, "popup-menu", trayContext())
	g.iconTitle(icon, &cString(config.Tooltip)[0])
	i.SetTooltip(config.Tooltip)
	g.iconVisible(icon, 1)
	return i
}

func (i *embeddedTrayItem) signal(obj uintptr, name string, callback uintptr) {
	i.loop.g.connect(obj, &cString(name)[0], callback, i.icon, 0, 0)
}

func (i *embeddedTrayItem) anchor() platform.ScreenRect {
	g := i.loop.g
	var r [4]int32
	if g.iconGeometry(i.icon, 0, &r, 0) != 0 {
		scale := int32(1)
		if window := g.xwindow(g.display(), uintptr(g.iconXID(i.icon))); window != 0 {
			scale = max(1, g.windowScale(window))
		}
		return platform.ScreenRect{X: float32(r[0] * scale), Y: float32(r[1] * scale), Width: float32(r[2] * scale), Height: float32(r[3] * scale)}
	}
	if a := i.loop.app; len(a.windows) > 0 {
		x, y, _ := a.windows[0].win.pointerPosition()
		return platform.ScreenRect{X: float32(x), Y: float32(y)}
	}
	return platform.ScreenRect{}
}

func (i *embeddedTrayItem) SetIcon(icon platform.StatusIcon) error {
	if i.removed.Load() || !icon.Valid() {
		return errors.New("linux: invalid or removed status icon")
	}
	g := i.loop.g
	image := g.pixbuf(0, 1, 8, int32(icon.Size), int32(icon.Size))
	if image == 0 {
		return errors.New("linux: cannot allocate status icon")
	}
	defer g.unref(image)
	stride := int(g.stride(image))
	dst := unsafe.Slice(g.pixels(image), (icon.Size-1)*stride+icon.Size*4)
	for row := 0; row < icon.Size; row++ {
		copy(dst[row*stride:row*stride+icon.Size*4], icon.RGBA[row*icon.Size*4:(row+1)*icon.Size*4])
	}
	g.iconPixels(i.icon, image)
	return nil
}

// The XEmbed menu is rebuilt on the opening press.
func (*embeddedTrayItem) RefreshMenu()             {}
func (*embeddedTrayItem) SetMenuOnHover(bool) bool { return false }

func (i *embeddedTrayItem) SetTooltip(value string) {
	if i.removed.Load() {
		return
	}
	i.config.Tooltip = value
	if i.open != nil || value == "" {
		i.loop.g.iconTooltip(i.icon, nil)
	} else {
		i.loop.g.iconTooltip(i.icon, &cString(value)[0])
	}
}

func (i *embeddedTrayItem) SetHoverPanel(open func(platform.ScreenRect) platform.Window) bool {
	if i.removed.Load() {
		return false
	}
	i.open = open
	i.SetTooltip(i.config.Tooltip)
	i.cancelTimer()
	if open == nil {
		i.closePanel()
	} else if i.inIcon {
		i.scheduleOpen()
	}
	return true
}

func (i *embeddedTrayItem) cancelTimer() {
	i.generation++
	if i.timer != nil {
		i.timer.Stop()
		i.timer = nil
	}
}

func (i *embeddedTrayItem) after(delay time.Duration, fn func()) {
	i.cancelTimer()
	gen := i.generation
	i.timer = time.AfterFunc(delay, func() {
		i.loop.app.Post(func() {
			if !i.removed.Load() && i.generation == gen {
				i.timer = nil
				fn()
			}
		})
	})
}

func (i *embeddedTrayItem) scheduleOpen() {
	if i.open != nil && i.panel == nil && i.menu == 0 {
		i.after(350*time.Millisecond, func() {
			if w, ok := i.open(i.anchor()).(*nativeWindow); ok && w != nil && !w.closed.Load() {
				if i.removed.Load() || i.open == nil {
					w.Close()
					return
				}
				i.panel = w
				w.state.hoverItem = i
			}
		})
	}
}

func (i *embeddedTrayItem) crossIcon(inside bool) {
	if inside == i.inIcon {
		return
	}
	i.inIcon = inside
	i.cancelTimer()
	if inside {
		i.scheduleOpen()
	} else {
		i.scheduleClose()
	}
}

func (i *embeddedTrayItem) panelCrossing(inside bool) {
	if i.inPanel == inside {
		return
	}
	i.inPanel = inside
	i.cancelTimer()
	i.scheduleClose()
}

func (i *embeddedTrayItem) scheduleClose() {
	if i.panel != nil && !i.inPanel && !i.inIcon {
		i.after(180*time.Millisecond, i.closePanel)
	}
}

func (i *embeddedTrayItem) detachPanel() {
	if i.panel != nil {
		i.panel.state.hoverItem = nil
		i.panel = nil
		i.inPanel = false
	}
}

func (i *embeddedTrayItem) closePanel() {
	if p := i.panel; p != nil {
		i.detachPanel()
		p.Close()
	}
}

func (i *embeddedTrayItem) popupMenu(button, timestamp uint32) {
	i.cancelTimer()
	i.closePanel()
	if i.config.Menu == nil {
		if i.config.OnClick != nil {
			i.config.OnClick(platform.StatusClick{Button: gfx.PointerButtonSecondary, Anchor: i.anchor()})
		}
		return
	}
	entries := i.config.Menu()
	if len(entries) == 0 {
		return
	}
	i.closeMenu()
	i.actions = make(map[uintptr]func())
	i.menu = i.buildMenu(entries)
	g := i.loop.g
	g.refSink(i.menu)
	i.signal(i.menu, "selection-done", trayMenuDone())
	g.showAll(i.menu)
	g.popup(i.menu, 0, 0, g.positionMenu, i.icon, button, timestamp)
}

func (i *embeddedTrayItem) buildMenu(entries []platform.MenuItem) uintptr {
	g := i.loop.g
	menu := g.newMenu()
	for _, entry := range entries {
		var w uintptr
		switch {
		case entry.Separator:
			w = g.separator()
		case entry.Checked:
			w = g.checkItem(&cString(entry.Title)[0])
			g.setChecked(w, 1)
		default:
			w = g.menuItem(&cString(entry.Title)[0])
		}
		g.menuAppend(menu, w)
		if entry.Disabled {
			g.sensitive(w, 0)
		}
		if len(entry.Submenu) > 0 {
			g.submenu(w, i.buildMenu(entry.Submenu))
		} else if entry.OnSelect != nil && !entry.Disabled {
			i.actions[w] = entry.OnSelect
			i.signal(w, "activate", trayAction())
		}
	}
	return menu
}

func (i *embeddedTrayItem) closeMenu() {
	if m := i.menu; m != 0 {
		i.menu = 0
		i.actions = nil
		i.loop.g.destroy(m)
		i.loop.g.unref(m)
	}
}

func (i *embeddedTrayItem) Remove() {
	if !i.removed.Swap(true) {
		i.loop.app.Post(func() {
			i.dispose()
			if i.loop.app.trayGTK == i.loop && len(i.loop.items) == 0 {
				i.loop.app.closeEmbeddedTray()
			}
		})
	}
}

func (i *embeddedTrayItem) dispose() {
	if i.icon == 0 {
		return
	}
	i.removed.Store(true)
	i.cancelTimer()
	i.closePanel()
	i.closeMenu()
	delete(i.loop.items, i.icon)
	embeddedItems.Delete(i.icon)
	i.loop.g.iconVisible(i.icon, 0)
	i.loop.g.unref(i.icon)
	i.icon = 0
}
