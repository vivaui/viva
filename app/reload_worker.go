//go:build vivadev && (darwin || windows || linux)

package app

import (
	"github.com/vivaui/viva/internal/reloadwire"
	"github.com/vivaui/viva/ui"
)

// runReloadWorkerIfEnabled reports whether this process is a development
// reload worker and, if so, runs it. The release build has no implementation
// that mentions the protocol package at all (see reload_disabled.go).
func runReloadWorkerIfEnabled(config Config, canvas *ui.Canvas) (bool, error) {
	if !reloadwire.Enabled() {
		return false, nil
	}
	return true, runReloadWorker(config, canvas)
}

func runReloadWorker(config Config, canvas *ui.Canvas) error {
	if canvas == nil {
		canvas = &ui.Canvas{}
	}
	return reloadwire.Serve(newReloadWorkerConfig(config, canvas))
}

func newReloadWorkerConfig(config Config, canvas *ui.Canvas) reloadwire.WorkerConfig {
	worker := reloadwire.WorkerConfig{
		Window: reloadwire.WindowConfig{
			Title: config.Title, Width: config.Width, Height: config.Height,
			MinWidth: config.MinWidth, MinHeight: config.MinHeight,
			WindowMode: uint8(config.WindowMode), TitlebarHeight: uint16(max(config.TitlebarHeight+0.5, 0)),
			ClearRed: config.ClearColor.Red, ClearGreen: config.ClearColor.Green,
			ClearBlue: config.ClearColor.Blue, ClearAlpha: config.ClearColor.Alpha,
			HasClick: config.Paint != nil, HasPointer: config.OnPointer != nil || config.Paint != nil,
			HasKey: config.OnKey != nil, HasShortcut: config.OnShortcut != nil, HasScroll: config.OnScroll != nil, HasModifierTap: config.OnModifierTap != nil,
			HasClose:    config.OnCloseRequest != nil,
			HasSnapshot: config.Reload.Snapshot != nil, HasRestore: config.Reload.Restore != nil,
		},
		Canvas: canvas, Paint: config.Paint,
		Pointer: config.OnPointer, Key: config.OnKey, Shortcut: config.OnShortcut, Scroll: config.OnScroll, ModifierTap: config.OnModifierTap,
		CloseRequest: config.OnCloseRequest,
	}
	if config.Reload.Snapshot != nil {
		worker.Snapshot = func() ([]reloadwire.StateSlot, error) {
			slots, snapshotErr := config.Reload.Snapshot()
			if snapshotErr != nil {
				return nil, snapshotErr
			}
			result := make([]reloadwire.StateSlot, len(slots))
			for index, slot := range slots {
				result[index] = reloadwire.StateSlot{Key: slot.Key, Version: slot.Version, Data: slot.Data}
			}
			return result, nil
		}
	}
	if config.Reload.Restore != nil {
		worker.Restore = func(slots []reloadwire.StateSlot) error {
			values := make([]StateSlot, len(slots))
			for index, slot := range slots {
				values[index] = StateSlot{Key: slot.Key, Version: slot.Version, Data: slot.Data}
			}
			return config.Reload.Restore(values)
		}
	}
	return worker
}
