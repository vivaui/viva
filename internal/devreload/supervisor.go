package devreload

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/reloadwire"
)

type supervisor struct {
	mu           sync.Mutex
	active       *workerProcess
	window       reloadwire.WindowConfig
	lastRequest  reloadwire.PaintRequest
	lastSnapshot reloadwire.CanvasSnapshot
	invalidator  app.Invalidator
	pendingClose func()
	errorOutput  io.Writer
	lastError    string
}

func newSupervisor(worker *workerProcess, request reloadwire.PaintRequest, snapshot reloadwire.CanvasSnapshot, errorOutput io.Writer) *supervisor {
	value := &supervisor{
		active: worker, window: worker.config, lastRequest: request, lastSnapshot: snapshot,
		errorOutput: errorOutput,
	}
	worker.client.SetCloseAllowed(func() { value.closeAllowed(worker) })
	return value
}

func (value *supervisor) hostConfig() app.Config {
	config := app.Config{
		Title: value.window.Title, Width: value.window.Width, Height: value.window.Height,
		MinWidth: value.window.MinWidth, MinHeight: value.window.MinHeight,
		WindowMode: app.WindowMode(value.window.WindowMode), TitlebarHeight: float32(value.window.TitlebarHeight),
		ClearColor: app.Color{Red: value.window.ClearRed, Green: value.window.ClearGreen, Blue: value.window.ClearBlue, Alpha: value.window.ClearAlpha},
		Paint:      value.paint, OnPointer: value.pointer,
		OnKey: value.key, OnShortcut: value.shortcut, OnScroll: value.scroll,
		OnCloseRequest: value.requestClose, Invalidator: &value.invalidator,
	}
	if value.window.HasModifierTap {
		config.OnModifierTap = value.modifierTap
	}
	return config
}

func (value *supervisor) paint(canvas *gfx.Canvas, size gfx.Size) {
	if id, ok := canvas.ClickTarget(); ok {
		// The event pass of a click: the application's state lives in the worker,
		// which runs the pass there. Nothing is drawn; the host repaints next.
		info, _ := canvas.ClickInfo()
		value.click(id, info)
		return
	}
	request := reloadwire.PaintRequest{
		Width: size.Width, Height: size.Height, Hovered: canvas.Hovered(), Pressed: canvas.Pressed(),
		Focused: canvas.Focused(), FocusVisible: !canvas.FocusVisible().IsZero(),
		ScrollX: canvas.ScrollX(), ScrollY: canvas.ScrollY(),
		Now: uint64(canvas.Now()), ReduceMotion: canvas.ReduceMotion(),
	}
	value.mu.Lock()
	value.lastRequest = request
	worker := value.active
	listens := worker != nil && worker.config.HasClick
	snapshot := value.lastSnapshot
	fresh := false
	if worker != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		painted, err := worker.client.Paint(ctx, request)
		cancel()
		if err == nil {
			snapshot = painted
			value.lastSnapshot = painted
			value.clearErrorLocked()
			fresh = true
		} else {
			value.reportErrorLocked(fmt.Errorf("paint worker: %w", err))
		}
	}
	value.mu.Unlock()
	if !fresh {
		// A stale snapshot must not keep the host's frame loop alive.
		snapshot.Animating = false
	}
	reloadwire.ApplyCanvas(canvas, snapshot)
	if listens {
		// The worker's paint reads clicks, so every click goes to it. The host
		// cannot see which controls ask, so it declares that it listens.
		canvas.Clicked(gfx.ID{})
	}
}

func (value *supervisor) click(id gfx.ID, info gfx.ClickInfo) {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.active == nil || !value.active.config.HasClick {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	err := value.active.client.Click(ctx, id, info)
	cancel()
	if err != nil {
		value.reportErrorLocked(fmt.Errorf("click worker: %w", err))
	}
}

func (value *supervisor) pointer(event gfx.PointerEvent) gfx.EventResult {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.active == nil || !value.active.config.HasPointer {
		return gfx.EventResult{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	result, err := value.active.client.Pointer(ctx, event)
	cancel()
	if err != nil {
		value.reportErrorLocked(fmt.Errorf("pointer worker: %w", err))
		return gfx.EventResult{}
	}
	return result
}

func (value *supervisor) key(event gfx.KeyEvent) gfx.EventResult {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.active == nil || !value.active.config.HasKey {
		return gfx.EventResult{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	result, err := value.active.client.Key(ctx, event)
	cancel()
	if err != nil {
		value.reportErrorLocked(fmt.Errorf("key worker: %w", err))
		return gfx.EventResult{}
	}
	return result
}

func (value *supervisor) shortcut(event gfx.KeyEvent) gfx.EventResult {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.active == nil || !value.active.config.HasShortcut {
		return gfx.EventResult{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	result, err := value.active.client.Shortcut(ctx, event)
	cancel()
	if err != nil {
		value.reportErrorLocked(fmt.Errorf("shortcut worker: %w", err))
		return gfx.EventResult{}
	}
	return result
}

func (value *supervisor) scroll(event gfx.ScrollEvent) gfx.EventResult {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.active == nil || !value.active.config.HasScroll {
		return gfx.EventResult{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	result, err := value.active.client.Scroll(ctx, event)
	cancel()
	if err != nil {
		value.reportErrorLocked(fmt.Errorf("scroll worker: %w", err))
		return gfx.EventResult{}
	}
	return result
}

func (value *supervisor) modifierTap(event gfx.ModifierTapEvent) gfx.EventResult {
	value.mu.Lock()
	defer value.mu.Unlock()
	if value.active == nil || !value.active.config.HasModifierTap {
		return gfx.EventResult{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	result, err := value.active.client.ModifierTap(ctx, event)
	cancel()
	if err != nil {
		value.reportErrorLocked(fmt.Errorf("modifier tap worker: %w", err))
		return gfx.EventResult{}
	}
	return result
}

func (value *supervisor) requestClose(allow func()) {
	value.mu.Lock()
	if value.active == nil || !value.active.config.HasClose {
		value.mu.Unlock()
		allow()
		return
	}
	worker := value.active
	value.pendingClose = allow
	value.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	err := worker.client.RequestClose(ctx)
	cancel()
	if err != nil {
		value.mu.Lock()
		if value.active == worker && value.pendingClose != nil {
			value.pendingClose = nil
			value.reportErrorLocked(fmt.Errorf("close worker: %w", err))
			value.mu.Unlock()
			allow()
			return
		}
		value.mu.Unlock()
	}
}

func (value *supervisor) closeAllowed(worker *workerProcess) {
	value.mu.Lock()
	if value.active != worker || value.pendingClose == nil {
		value.mu.Unlock()
		return
	}
	allow := value.pendingClose
	value.pendingClose = nil
	value.mu.Unlock()
	allow()
}

func (value *supervisor) swap(candidate *workerProcess) error {
	value.mu.Lock()
	if err := compatibleWindow(value.window, candidate.config); err != nil {
		value.mu.Unlock()
		return err
	}
	old := value.active
	request := value.lastRequest
	if old != nil && old.config.HasSnapshot && candidate.config.HasRestore {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		slots, err := old.client.SnapshotState(ctx)
		cancel()
		if err != nil {
			value.mu.Unlock()
			return fmt.Errorf("snapshot old worker: %w", err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
		err = candidate.client.RestoreState(ctx, slots)
		cancel()
		if err != nil {
			value.mu.Unlock()
			return fmt.Errorf("restore candidate worker: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	snapshot, err := candidate.client.Paint(ctx, request)
	cancel()
	if err != nil {
		value.mu.Unlock()
		return fmt.Errorf("validate candidate worker: %w", err)
	}
	candidate.client.SetCloseAllowed(func() { value.closeAllowed(candidate) })
	value.active = candidate
	value.lastSnapshot = snapshot
	pendingClose := value.pendingClose != nil
	value.clearErrorLocked()
	value.mu.Unlock()

	value.invalidator.Invalidate()
	if pendingClose && candidate.config.HasClose {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = candidate.client.RequestClose(ctx)
			cancel()
		}()
	}
	if old != nil {
		old.stop()
	}
	return nil
}

func (value *supervisor) stop() {
	value.mu.Lock()
	worker := value.active
	value.active = nil
	value.pendingClose = nil
	value.mu.Unlock()
	if worker != nil {
		worker.stop()
	}
}

func (value *supervisor) reportErrorLocked(err error) {
	message := err.Error()
	if message == value.lastError {
		return
	}
	value.lastError = message
	fmt.Fprintf(value.errorOutput, "viva dev: %s\n", message)
}

func (value *supervisor) clearErrorLocked() { value.lastError = "" }

func compatibleWindow(current, next reloadwire.WindowConfig) error {
	if current.Title != next.Title || current.Width != next.Width || current.Height != next.Height ||
		current.WindowMode != next.WindowMode || current.TitlebarHeight != next.TitlebarHeight || current.ClearRed != next.ClearRed ||
		current.ClearGreen != next.ClearGreen || current.ClearBlue != next.ClearBlue || current.ClearAlpha != next.ClearAlpha {
		return fmt.Errorf("candidate changed immutable window configuration; restart viva dev")
	}
	return nil
}
