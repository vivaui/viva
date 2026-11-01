//go:build darwin || windows || linux

package app

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestDriverMenuRevealsPendingKeyboardFocus(t *testing.T) {
	scroll := components.MenuScroll{Offset: 340}
	config := Config{Title: "scrolling select", Width: 300, Height: 200}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.SetInitialFocus(ui.Item(14))
		selectControl := components.BeginSelect(canvas, ui.LightTheme(), ui.Rect{Width: 220, Height: 32}, components.SelectOptions{
			TriggerID: ui.Item(90), Label: "Language", Value: "Last", Open: true,
			Content: ui.Rect{Y: 40, Width: 220, Height: 110}, Layer: 1,
			Scope: ui.Item(100), DismissID: ui.Item(101), Scroll: &scroll,
		})
		for index := 1; index <= 14; index++ {
			selectControl.AddItem(components.SelectItem{ID: ui.Item(index), Label: "Language", Selected: index == 14})
		}
		selectControl.End()
	}
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		if target, handled := scroll.Navigate(event.Key, event.Target, true); handled {
			return ui.EventResult{Focus: target, FocusVisible: true, Handled: true, Repaint: true}
		}
		return ui.EventResult{}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if driver.Snapshot().Focused != ui.Item(14) {
		t.Fatal("did not open at the selected row")
	}
	for _, step := range []struct {
		key    ui.Key
		target ui.ID
	}{{ui.KeyHome, ui.Item(1)}, {ui.KeyEnd, ui.Item(14)}, {ui.KeyHome, ui.Item(1)}} {
		if err := driver.Key(step.key, 0); err != nil {
			t.Fatal(err)
		}
		if got := driver.Snapshot().Focused; got != step.target {
			t.Fatalf("key %v focused %v, want newly revealed %v", step.key, got, step.target)
		}
	}
}

func TestDriverClicksTabsAndReportsFocusWithoutAWindow(t *testing.T) {
	var clicked []ui.ID
	config := Config{Title: "driver", Width: 300, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		theme := ui.LightTheme()
		components.BeginButton(canvas, theme, ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 90, Height: 32}).Default("Save")
		components.BeginButton(canvas, theme, ui.Item(2), ui.Rect{X: 120, Y: 10, Width: 90, Height: 32}).Secondary("Cancel")
	}
	onClick(&config, func(id ui.ID) { clicked = append(clicked, id) })
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if before := driver.Snapshot(); before.Width != 300 || before.Height != 100 || len(before.Primitives) == 0 || len(before.Hits) != 2 {
		t.Fatalf("first frame = %+v", before)
	}
	if err := driver.Move(20, 20); err != nil {
		t.Fatal(err)
	}
	if hovered := driver.Snapshot().Hovered; hovered != ui.Item(1) {
		t.Fatalf("hovered = %d, want the Save button", hovered)
	}
	if err := driver.Click(130, 20); err != nil {
		t.Fatal(err)
	}
	if len(clicked) != 1 || clicked[0] != ui.Item(2) {
		t.Fatalf("clicks = %v, want [2]", clicked)
	}
	if err := driver.Key(ui.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	if focused := driver.Snapshot().Focused; focused.IsZero() {
		t.Fatal("Tab focused nothing")
	}
	if err := driver.Key(ui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if len(clicked) != 2 {
		t.Fatalf("Enter on the focused button did not activate it: %v", clicked)
	}
}

func TestDriverHitTestUsesPointerScopeAndOcclusionRules(t *testing.T) {
	open := false
	config := Config{Title: "hit test", Width: 240, Height: 140}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 90, Height: 30}, "Background")
		canvas.HitWithLabel(ui.Item(2), ui.Rect{X: 120, Y: 10, Width: 90, Height: 30}, "Covered")
		if !open {
			return
		}
		canvas.PushFocusScope(ui.Item(9))
		canvas.Block(ui.Rect{X: 110, Y: 0, Width: 120, Height: 100})
		canvas.HitWithLabel(ui.Item(3), ui.Rect{X: 120, Y: 50, Width: 90, Height: 30}, "Dialog")
		canvas.PopFocusScope()
		canvas.TrapFocus(ui.Item(9))
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if target, ok := driver.HitTest(20, 20); !ok || target != ui.Item(1) {
		t.Fatalf("background hit = %d, %v; want 1", target, ok)
	}
	open = true
	if err := driver.Resize(240, 140); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		x, y   float32
		target uint32
		ok     bool
	}{
		{name: "outside active scope", x: 20, y: 20},
		{name: "behind block", x: 130, y: 20},
		{name: "dialog control", x: 130, y: 60, target: 3, ok: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			target, ok := driver.HitTest(test.x, test.y)
			if target != ui.Item(int(test.target)) || ok != test.ok {
				t.Fatalf("hit = %d, %v; want %d, %v", target, ok, test.target, test.ok)
			}
		})
	}
}

// A field backed by a bare TextInput is edited exactly as the native editor
// would: typing, editing keys, and selection all go through the text system.
func TestDriverEditsAFocusedTextFieldThroughTheTextSystem(t *testing.T) {
	theme := ui.DarkTheme()
	editor := text.NewEditor("")
	secret := text.NewEditor("")
	config := Config{Title: "text", Width: 300, Height: 140}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.PaintInput(canvas, theme, ui.Rect{X: 10, Y: 10, Width: 200, Height: 30}, components.InputSnapshot{
			ID: ui.Item(5), Label: "Name", Value: editor.String(), PlatformEditing: canvas.Focused() == ui.Item(5) && editor.String() != "",
		})
		components.PaintInput(canvas, theme, ui.Rect{X: 10, Y: 60, Width: 200, Height: 30}, components.InputSnapshot{
			ID: ui.Item(6), Label: "Password", Secure: true, Value: secret.String(), PlatformEditing: canvas.Focused() == ui.Item(6) && secret.String() != "",
		})
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		if context.ID == ui.Item(6) {
			return &TextInput{Editor: secret}
		}
		return &TextInput{Editor: editor}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Type("x"); !errors.Is(err, ErrNoTextInput) {
		t.Fatalf("typing with nothing focused: %v", err)
	}
	if err := driver.Click(50, 25); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("hello world"); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "hello world" {
		t.Fatalf("editor = %q", editor.String())
	}
	snapshot := driver.Snapshot()
	if snapshot.Editor == nil || snapshot.Editor.Value != "hello world" || snapshot.Editor.FontSize != theme.Metrics.FontSizeM {
		t.Fatalf("editor overlay = %+v", snapshot.Editor)
	}
	// The overlay sits where the field painted its own text.
	if want := (ui.Rect{X: 21, Y: 10, Width: 178, Height: 30}); snapshot.Editor.Bounds != want {
		t.Fatalf("editor bounds = %+v, want %+v", snapshot.Editor.Bounds, want)
	}

	// Editing keys reach the text system, not the application.
	if err := driver.Key(ui.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyLeft, ui.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "hello worl" || editor.Selection() != (text.Range{Start: 9, End: 10}) {
		t.Fatalf("after Backspace and Shift+Left: %q %+v", editor.String(), editor.Selection())
	}

	// Select all, copy, cut, paste through the driver's private clipboard.
	for _, step := range []struct {
		character rune
		text      string
		clipboard string
	}{
		{'a', "hello worl", ""},
		{'c', "hello worl", "hello worl"},
		{'x', "", "hello worl"},
		{'v', "hello worl", "hello worl"},
	} {
		handled, err := driver.Shortcut(step.character)
		if err != nil || !handled {
			t.Fatalf("Command+%c: handled=%v err=%v", step.character, handled, err)
		}
		if editor.String() != step.text || driver.Clipboard() != step.clipboard {
			t.Fatalf("after Command+%c: text %q clipboard %q, want %q / %q", step.character, editor.String(), driver.Clipboard(), step.text, step.clipboard)
		}
	}
	if handled, err := driver.Shortcut('z'); err != nil || !handled || editor.String() != "" {
		t.Fatalf("Command+Z: handled=%v err=%v text=%q", handled, err, editor.String())
	}
	if handled, err := driver.ShortcutChord(ui.ModifierMeta|ui.ModifierShift, 'z'); err != nil || !handled || editor.String() != "hello worl" {
		t.Fatalf("Command+Shift+Z: handled=%v err=%v text=%q", handled, err, editor.String())
	}
	editor.ClearHistory()
	for _, pasted := range []string{" one", " two", " three"} {
		driver.SetClipboard(pasted)
		if handled, err := driver.Shortcut('v'); err != nil || !handled {
			t.Fatalf("paste %q: handled=%v err=%v", pasted, handled, err)
		}
	}
	if handled, err := driver.Shortcut('z'); err != nil || !handled || editor.String() != "hello worl one two" {
		t.Fatalf("one undo after three pastes: handled=%v err=%v text=%q", handled, err, editor.String())
	}

	// A password is never exported, but a paste is accepted. Line breaks are
	// flattened in a single-line field.
	if err := driver.Key(ui.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	if driver.Snapshot().Focused != ui.Item(6) {
		t.Fatalf("Tab moved focus to %d, want the password field", driver.Snapshot().Focused)
	}
	driver.SetClipboard("pass\r\nword\n")
	if handled, err := driver.Shortcut('v'); err != nil || !handled {
		t.Fatalf("paste into the password field: %v %v", handled, err)
	}
	if secret.String() != "pass word" {
		t.Fatalf("password = %q, want the flattened paste", secret.String())
	}
	if handled, err := driver.Shortcut('z'); err != nil || !handled || secret.String() != "pass word" {
		t.Fatalf("password undo: handled=%v err=%v text=%q", handled, err, secret.String())
	}
	driver.SetClipboard("keep")
	for _, character := range []rune{'a', 'c', 'x'} {
		if _, err := driver.Shortcut(character); err != nil {
			t.Fatal(err)
		}
	}
	if driver.Clipboard() != "keep" {
		t.Fatalf("a password reached the clipboard: %q", driver.Clipboard())
	}
	if secret.String() != "pass word" {
		t.Fatalf("cut removed password text it must not export: %q", secret.String())
	}
	if snapshot := driver.Snapshot(); snapshot.Editor == nil || !snapshot.Editor.Secure {
		t.Fatalf("the password overlay is not marked secure: %+v", snapshot.Editor)
	}
}

// NavigationKeys delivers Up, Down, PageUp and PageDown to the application's key
// handler while typing, Home/End, selection and pointer placement remain native.
func TestDriverNavigationKeysPassToApplicationWhileTextEditingRemainsNative(t *testing.T) {
	theme := ui.DarkTheme()
	editor := text.NewEditor("")
	var appKeys []ui.Key
	config := Config{Title: "nav-keys", Width: 300, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.PaintInput(canvas, theme, ui.Rect{X: 10, Y: 10, Width: 200, Height: 30}, components.InputSnapshot{
			ID: ui.Item(1), Label: "Search", Value: editor.String(),
			PlatformEditing: canvas.Focused() == ui.Item(1) && editor.String() != "",
			NavigationKeys:  true,
		})
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		if context.ID != ui.Item(1) {
			return nil
		}
		if !context.NavigationKeys {
			t.Fatal("TextInputContext.NavigationKeys was not set from the component snapshot")
		}
		return &TextInput{Editor: editor}
	}
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		appKeys = append(appKeys, event.Key)
		return ui.EventResult{Handled: true}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Click(50, 25); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("abcdef"); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "abcdef" {
		t.Fatalf("editor = %q, want abcdef", editor.String())
	}

	// Home moves the caret to the beginning of the field via the native text system.
	if err := driver.Key(ui.KeyHome, 0); err != nil {
		t.Fatal(err)
	}
	if editor.Selection() != (text.Range{Start: 0, End: 0}) {
		t.Fatalf("after Home: caret = %+v, want 0", editor.Selection())
	}
	// Typing at Home inserts at index 0.
	if err := driver.Type("z"); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "zabcdef" {
		t.Fatalf("editor after Home+insert = %q, want zabcdef", editor.String())
	}

	// End moves the caret to the end of the field natively.
	if err := driver.Key(ui.KeyEnd, 0); err != nil {
		t.Fatal(err)
	}
	if editor.Selection() != (text.Range{Start: 7, End: 7}) {
		t.Fatalf("after End: caret = %+v, want 7", editor.Selection())
	}

	// Navigation keys (Down, Up, PageDown, PageUp) reach the application.
	appKeys = nil
	for _, key := range []ui.Key{ui.KeyDown, ui.KeyUp, ui.KeyPageDown, ui.KeyPageUp} {
		if err := driver.Key(key, 0); err != nil {
			t.Fatalf("delivering %v: %v", key, err)
		}
	}
	wantKeys := []ui.Key{ui.KeyDown, ui.KeyUp, ui.KeyPageDown, ui.KeyPageUp}
	if !slices.Equal(appKeys, wantKeys) {
		t.Fatalf("app keys = %v, want %v", appKeys, wantKeys)
	}
}

func TestChangingFocusedEditorsBreaksTypingUndoGroups(t *testing.T) {
	first, second := text.NewEditor(""), text.NewEditor("")
	var input TextInput
	driver, err := NewDriver(Config{
		Title: "focus history", Width: 220, Height: 80,
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 100, Height: 30}, "First", ui.HitOptions{Role: ui.RoleTextField})
			canvas.HitWithOptions(ui.Item(2), ui.Rect{X: 110, Width: 100, Height: 30}, "Second", ui.HitOptions{Role: ui.RoleTextField})
		},
		TextInput: func(context TextInputContext) *TextInput {
			input.Editor = first
			if context.ID == ui.Item(2) {
				input.Editor = second
			}
			return &input
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	for _, step := range []struct {
		x    float32
		text string
	}{{10, "first"}, {120, "other"}, {10, " second"}} {
		if err := driver.Click(step.x, 10); err != nil {
			t.Fatal(err)
		}
		if err := driver.Type(step.text); err != nil {
			t.Fatal(err)
		}
	}
	if handled, err := driver.Shortcut('z'); err != nil || !handled || first.String() != "first" {
		t.Fatalf("undo handled=%v err=%v first=%q", handled, err, first.String())
	}
}

// Animations only move when virtual time does, and an idle application
// repaints nothing.
func TestDriverAdvancesAnimationsOnVirtualTimeOnly(t *testing.T) {
	paints := 0
	config := Config{Title: "motion", Width: 100, Height: 60}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		paints++
		if canvas.Now() < 100*time.Millisecond {
			canvas.RequestFrame()
		}
		canvas.Fill(ui.Rect{Width: 10, Height: 10}, ui.Color{Alpha: 1})
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Advance(40 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	midway := paints
	if midway < 3 {
		t.Fatalf("paints after 40ms of an animation = %d, want several frames", midway)
	}
	if err := driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	settled := paints
	if driver.Now() != 1040*time.Millisecond {
		t.Fatalf("virtual time = %v", driver.Now())
	}
	if err := driver.Advance(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	if paints != settled {
		t.Fatalf("an idle application repainted %d times", paints-settled)
	}
}

func TestDriverAppliesBackgroundWakesOnTheCallingGoroutine(t *testing.T) {
	var invalidator Invalidator
	published, observed := 0, 0
	config := Config{Title: "wake", Width: 100, Height: 60, Invalidator: &invalidator}
	config.OnWake = func() { observed = published }
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	published = 7
	go invalidator.Invalidate()
	select {
	case <-driver.Wakes():
	case <-time.After(2 * time.Second):
		t.Fatal("the invalidation never reached the driver")
	}
	if observed != 0 {
		t.Fatal("OnWake ran on the background goroutine")
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if observed != 7 {
		t.Fatalf("OnWake observed %d after Pump, want 7", observed)
	}
	driver.Close()
	invalidator.Invalidate() // after Close this must be harmless
}

func TestDriverResizeRepaintsAtTheNewSize(t *testing.T) {
	var last ui.Size
	config := Config{Title: "resize", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) { last = size }
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 480); err != nil {
		t.Fatal(err)
	}
	if last != (ui.Size{Width: 640, Height: 480}) || driver.Snapshot().Width != 640 {
		t.Fatalf("paint size = %+v, snapshot = %dx%d", last, driver.Snapshot().Width, driver.Snapshot().Height)
	}
	if err := driver.Resize(0, 10); err == nil {
		t.Fatal("a zero-width resize was accepted")
	}
}

func TestDriverUsesHoveredAndCapturedHitCursor(t *testing.T) {
	config := Config{Title: "cursor", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, _ ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 80, Height: 100}, "columns", ui.HitOptions{Cursor: ui.PointerCursorResizeHorizontal})
		canvas.HitWithOptions(ui.Item(2), ui.Rect{X: 120, Width: 80, Height: 100}, "rows", ui.HitOptions{Cursor: ui.PointerCursorResizeVertical})
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Move(20, 20); err != nil || driver.Snapshot().Cursor != CursorResizeHorizontal {
		t.Fatalf("horizontal cursor = %v, err = %v", driver.Snapshot().Cursor, err)
	}
	if err := driver.Press(20, 20, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(150, 20); err != nil || driver.Snapshot().Cursor != CursorResizeHorizontal {
		t.Fatalf("captured cursor = %v, err = %v", driver.Snapshot().Cursor, err)
	}
	if err := driver.Release(150, 20, ui.PointerButtonPrimary); err != nil || driver.Snapshot().Cursor != CursorResizeVertical {
		t.Fatalf("released cursor = %v, err = %v", driver.Snapshot().Cursor, err)
	}
	if err := driver.Move(100, 20); err != nil || driver.Snapshot().Cursor != CursorArrow {
		t.Fatalf("default cursor = %v, err = %v", driver.Snapshot().Cursor, err)
	}
}

func BenchmarkDriverClickAndSnapshot(b *testing.B) {
	theme := ui.LightTheme()
	config := Config{Title: "bench", Width: 400, Height: 200}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		for index := uint32(0); index < 12; index++ {
			components.BeginButton(canvas, theme, ui.Item(int(index+1)), ui.Rect{X: float32(10 + 30*index), Y: 10, Width: 26, Height: 32}).Default("x")
		}
	}
	onClick(&config, func(ui.ID) {})
	driver, err := NewDriver(config, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	b.ReportAllocs()
	for b.Loop() {
		if err := driver.Click(20, 20); err != nil {
			b.Fatal(err)
		}
		_ = driver.Snapshot()
	}
}

func BenchmarkDriverHitTest(b *testing.B) {
	config := Config{Title: "hit test bench", Width: 640, Height: 480}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		for index := uint32(0); index < 128; index++ {
			canvas.HitWithLabel(ui.Item(int(index+1)), ui.Rect{X: float32(index%16) * 40, Y: float32(index/16) * 40, Width: 38, Height: 38}, "Cell")
		}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	b.ReportAllocs()
	for b.Loop() {
		target, ok := driver.HitTest(619, 299)
		if !ok || target != ui.Item(128) {
			b.Fatalf("hit = %d, %v; want 128", target, ok)
		}
	}
}

func TestDriverDoubleClickDeliversTheSequenceTheSystemWould(t *testing.T) {
	var seen []ui.PointerEvent
	var clicks []ui.ID
	config := Config{Title: "double", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 80, Height: 30}, "Row")
	}
	config.OnPointer = func(event ui.PointerEvent) ui.EventResult {
		if event.Kind == ui.PointerDown || event.Kind == ui.PointerUp {
			seen = append(seen, event)
		}
		return ui.EventResult{}
	}
	onClick(&config, func(id ui.ID) { clicks = append(clicks, id) })
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.DoubleClick(20, 20); err != nil {
		t.Fatal(err)
	}
	type step struct {
		kind  ui.PointerEventKind
		count int
	}
	want := []step{{ui.PointerDown, 1}, {ui.PointerUp, 1}, {ui.PointerDown, 2}, {ui.PointerUp, 2}}
	if len(seen) != len(want) {
		t.Fatalf("events = %+v", seen)
	}
	for index, event := range seen {
		if event.Kind != want[index].kind || event.ClickCount != want[index].count {
			t.Fatalf("event %d = kind %d count %d, want %+v", index, event.Kind, event.ClickCount, want[index])
		}
	}
	if len(clicks) != 2 {
		t.Fatalf("a double click is two clicks to OnClick, got %v", clicks)
	}

	seen = seen[:0]
	if err := driver.Click(20, 20); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0].ClickCount != 1 || seen[1].ClickCount != 1 {
		t.Fatalf("a plain click = %+v", seen)
	}
	seen = seen[:0]
	if err := driver.ClickCount(20, 20, 3); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 6 || seen[4].ClickCount != 3 {
		t.Fatalf("a triple click = %+v", seen)
	}
}

// A pulse that asks for a frame every 125 ms paints eight times in a second of
// virtual time, not a hundred and twenty, and stops when it stops asking.
func TestDriverPaintsADelayedFrameRequestAtItsOwnCadence(t *testing.T) {
	paints := 0
	pulsing := true
	driver, err := NewDriver(Config{Title: "pulse", Width: 100, Height: 50, Paint: func(canvas *ui.Canvas, size ui.Size) {
		paints++
		canvas.Fill(ui.Rect{Width: 10, Height: 10}, ui.Color{Alpha: 1})
		if pulsing {
			canvas.RequestFrameIn(125 * time.Millisecond)
		}
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	start := paints
	if err := driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	if got := paints - start; got < 7 || got > 8 {
		t.Fatalf("a second of 125ms pulses painted %d frames, want about 8", got)
	}
	pulsing = false
	if err := driver.Advance(125 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	settled := paints
	if err := driver.Advance(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	if paints != settled {
		t.Fatalf("%d paints after the pulse stopped asking", paints-settled)
	}
}

// A frame the application asked for later reaches OnWake before it paints, as
// the window's timer does, so state that waits on the clock can advance.
func TestDriverRunsOnWakeBeforeADelayedFrame(t *testing.T) {
	wakes, asked := 0, false
	driver, err := NewDriver(Config{Title: "wake", Width: 100, Height: 50,
		OnWake: func() { wakes++ },
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			if !asked {
				asked = true
				canvas.RequestFrameIn(150 * time.Millisecond)
			}
		}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	before := wakes
	if err := driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	if wakes-before != 1 {
		t.Fatalf("OnWake ran %d times for one delayed frame", wakes-before)
	}
}

// Enter and Shift+Enter in a text field are different commands, as on the
// platforms: a chat box submits on one and adds a line on the other.
func TestDriverEnterAndShiftEnterAreDifferentTextCommands(t *testing.T) {
	theme := ui.DarkTheme()
	var commands []TextCommand
	config := Config{Title: "text", Width: 300, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.PaintTextarea(canvas, theme, ui.Rect{X: 10, Y: 10, Width: 200, Height: 60}, components.TextareaSnapshot{ID: ui.Item(5), Label: "Message"})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{OnEvent: func(event TextEvent) (TextState, error) {
			if event.Kind == TextCommandEvent {
				commands = append(commands, event.Command)
			}
			return TextState{}, nil
		}}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(50, 25); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyEnter, ui.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || commands[0] != TextInsertNewline || commands[1] != TextInsertLineBreak {
		t.Fatalf("commands = %v, want Newline then LineBreak", commands)
	}
}

func TestDriverMenuBackdropCatchesPressesOutsideTheSurface(t *testing.T) {
	for _, name := range []string{"dropdown", "context", "select"} {
		t.Run(name, func(t *testing.T) {
			viewport := ui.Rect{Width: 240, Height: 140}
			surface := ui.Rect{X: 10, Y: 10, Width: 160, Height: 80}
			config := Config{Title: "menu backdrop", Width: 240, Height: 140}
			config.Paint = func(canvas *ui.Canvas, size ui.Size) {
				canvas.HitWithLabel(ui.Item(1), ui.Rect{X: 180, Y: 100, Width: 50, Height: 30}, "Page")
				if name == "select" {
					control := components.BeginSelect(canvas, ui.DarkTheme(), ui.Rect{X: 10, Y: 100, Width: 100, Height: 30}, components.SelectOptions{
						TriggerID: ui.Item(2), Label: "Density", Open: true, Content: surface, Layer: 7, Scope: ui.Item(9), DismissID: ui.Item(91), Viewport: viewport,
					})
					control.AddItem(components.SelectItem{ID: ui.Item(11), Label: "One"})
				} else if name == "dropdown" {
					menu := components.BeginDropdownMenu(canvas, ui.DarkTheme(), surface, components.DropdownMenuOptions{Layer: 7, Scope: ui.Item(9), DismissID: ui.Item(91), Viewport: viewport})
					menu.AddItem(components.DropdownMenuItem{ID: ui.Item(11), Label: "One"})
				} else {
					menu := components.BeginContextMenu(canvas, ui.DarkTheme(), surface, components.ContextMenuOptions{Layer: 7, Scope: ui.Item(9), DismissID: ui.Item(91), Viewport: viewport})
					menu.AddItem(components.ContextMenuItem{ID: ui.Item(11), Label: "One"})
				}
			}
			driver, err := NewDriver(config, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			for _, test := range []struct {
				name   string
				x, y   float32
				target uint32
			}{
				{"empty space", 200, 30, 91},
				{"a page control beneath", 200, 110, 91},
				{"the menu item", 40, 25, 11},
			} {
				if target, ok := driver.HitTest(test.x, test.y); !ok || target != ui.Item(int(test.target)) {
					t.Errorf("%s: hit = %d, %v; want %d", test.name, target, ok, test.target)
				}
			}
		})
	}
}

func TestDriverSecondaryPressOnAMenuBackdropClosesItAndReachesTheControlBeneath(t *testing.T) {
	for _, test := range []struct {
		name    string
		button  ui.PointerButton
		passing bool
		reaches bool
	}{
		{"secondary", ui.PointerButtonSecondary, true, true},
		{"primary", ui.PointerButtonPrimary, true, false},
		{"secondary on a backdrop that keeps the page out of reach", ui.PointerButtonSecondary, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			open := true
			var reached []ui.PointerEvent
			config := Config{Title: "pass secondary", Width: 240, Height: 140}
			config.Paint = func(canvas *ui.Canvas, size ui.Size) {
				canvas.HitWithLabel(ui.Item(1), ui.Rect{X: 180, Y: 100, Width: 50, Height: 30}, "Page")
				if event, ok := canvas.PointerOn(ui.Item(1)); ok && event.Kind == ui.PointerDown {
					reached = append(reached, event)
					canvas.Consume()
				}
				if canvas.Clicked(ui.Item(91)) {
					open = false
				}
				if open {
					canvas.PushLayer(7)
					canvas.HitWithOptions(ui.Item(91), ui.Rect{Width: 240, Height: 140}, "Dismiss", ui.HitOptions{
						Role: ui.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: test.passing,
					})
					canvas.PopLayer()
				}
			}
			driver, err := NewDriver(config, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			if err := driver.ClickButton(200, 110, test.button); err != nil {
				t.Fatal(err)
			}
			if test.reaches != (len(reached) == 1) || len(reached) > 1 {
				t.Fatalf("the control beneath saw %d presses, want reached=%v", len(reached), test.reaches)
			}
			if test.reaches && (reached[0].Button != ui.PointerButtonSecondary || reached[0].Target != ui.Item(1)) {
				t.Fatalf("the control saw %+v", reached[0])
			}
			if test.passing && open {
				t.Fatal("the press left the overlay open")
			}
		})
	}
}

func TestDriverTheHigherLayerWinsWhateverIsDeclaredLast(t *testing.T) {
	var clicked []ui.ID
	config := Config{Title: "layers", Width: 300, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		theme := ui.LightTheme()
		// A popup on layer 2 is painted first and covers part of a button on
		// layer 1 that the code paints after it.
		canvas.PushLayer(2)
		components.BeginButton(canvas, theme, ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 100, Height: 60}).Default("Popup")
		canvas.PopLayer()
		canvas.PushLayer(1)
		components.BeginButton(canvas, theme, ui.Item(2), ui.Rect{X: 60, Y: 10, Width: 100, Height: 60}).Secondary("Behind")
		canvas.PopLayer()
	}
	onClick(&config, func(id ui.ID) { clicked = append(clicked, id) })
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Move(80, 30); err != nil {
		t.Fatal(err)
	}
	if hovered := driver.Snapshot().Hovered; hovered != ui.Item(1) {
		t.Fatalf("hovered = %d where a layer-2 popup covers a layer-1 button, want the popup", hovered)
	}
	if err := driver.Click(80, 30); err != nil {
		t.Fatal(err)
	}
	if len(clicked) != 1 || clicked[0] != ui.Item(1) {
		t.Fatalf("clicks = %v, want the popup (1) only", clicked)
	}
	if err := driver.Click(140, 30); err != nil {
		t.Fatal(err)
	}
	if len(clicked) != 2 || clicked[1] != ui.Item(2) {
		t.Fatalf("clicks = %v, want the uncovered part of the button behind (2)", clicked)
	}
}

// A press on a menu's own padding, a group label, a separator or a disabled row
// is inside the menu, not on the dismiss backdrop behind it: the menu stays
// open, as it does in Base UI.
func TestDriverAMenuSurfaceKeepsPressesThatAreNotOnAnEnabledRow(t *testing.T) {
	var dismissed, chose bool
	config := Config{Title: "menu inside", Width: 240, Height: 200}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		if canvas.Clicked(ui.Item(91)) {
			dismissed = true
		}
		if canvas.Clicked(ui.Item(11)) {
			chose = true
		}
		menu := components.BeginDropdownMenu(canvas, ui.DarkTheme(), ui.Rect{X: 10, Y: 10, Width: 180, Height: 170}, components.DropdownMenuOptions{
			Layer: 7, Scope: ui.Item(9), DismissID: ui.Item(91), Viewport: ui.Rect{Width: 240, Height: 200},
		})
		menu.AddLabel("View", false)
		menu.AddSeparator()
		menu.AddItem(components.DropdownMenuItem{ID: ui.Item(10), Label: "Disabled", Disabled: true})
		menu.AddItem(components.DropdownMenuItem{ID: ui.Item(11), Label: "One"})
		menu.End()
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	for _, point := range [...]struct {
		name string
		x, y float32
	}{
		{"padding", 12, 12},
		{"group label", 40, 28},
		{"separator", 40, 46},
		{"disabled row", 40, 66},
		{"empty part of the surface", 40, 150},
	} {
		if target, ok := driver.HitTest(point.x, point.y); ok && target == ui.Item(91) {
			t.Errorf("%s: the backdrop is the hit", point.name)
		}
		if err := driver.ClickButton(point.x, point.y, ui.PointerButtonPrimary); err != nil {
			t.Fatal(err)
		}
		if dismissed {
			t.Fatalf("%s: the menu was dismissed", point.name)
		}
	}
	if err := driver.ClickButton(40, 100, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if dismissed || !chose {
		t.Fatalf("the enabled row: dismissed=%v chose=%v", dismissed, chose)
	}
}

// A non-modal popover dismisses on an outside press and lets that same press do
// what the user aimed at, while a press on its own surface goes nowhere else.
func TestDriverAPopoverPassesAnOutsidePressThroughAndKeepsInsidePresses(t *testing.T) {
	var dismissed, pageClicks int
	open := true
	config := Config{Title: "popover", Width: 300, Height: 200}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.BeginButton(canvas, ui.LightTheme(), ui.Item(1), ui.Rect{X: 200, Y: 100, Width: 70, Height: 30}).Default("Page")
		if canvas.Clicked(ui.Item(1)) {
			pageClicks++
		}
		if canvas.Clicked(ui.Item(91)) {
			dismissed++
			open = false
		}
		if open {
			components.BeginPopover(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 200}, ui.Rect{X: 40, Y: 40, Width: 80, Height: 30}, components.PopoverOptions{
				Layer: 5, Scope: ui.Item(9), DismissID: ui.Item(91), ContentSize: ui.Size{Width: 140, Height: 100},
			}).Paint("Filters", "Narrow the results.")
		}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.ClickButton(230, 110, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if dismissed != 1 || pageClicks != 1 {
		t.Fatalf("outside press: dismissed=%d page=%d, want both once", dismissed, pageClicks)
	}
	if err := driver.ClickButton(55, 55, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if dismissed != 1 || pageClicks != 1 {
		t.Fatalf("press on the popover surface: dismissed=%d page=%d, want no change", dismissed, pageClicks)
	}
}

func TestDriverPopoverAnchorDismissDoesNotReopenTheTrigger(t *testing.T) {
	var tree ui.Tree
	feature := tree.Feature("popover-anchor")
	trigger, pageID := tree.ID(feature, "trigger"), tree.ID(feature, "page")
	dismiss, scope := tree.ID(feature, "dismiss"), tree.ID(feature, "scope")
	open, toggles, pageClicks := false, 0, 0
	anchor := ui.Rect{X: 20, Y: 10, Width: 80, Height: 24}
	page := ui.Rect{X: 250, Y: 120, Width: 40, Height: 30}
	config := Config{Title: "popover anchor", Width: 300, Height: 200, Tree: &tree}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.BeginButton(canvas, ui.LightTheme(), trigger, anchor).Default("Toggle")
		components.BeginButton(canvas, ui.LightTheme(), pageID, page).Default("Page")
		if canvas.Clicked(trigger) {
			open = !open
			toggles++
		}
		if canvas.Clicked(pageID) {
			pageClicks++
		}
		if canvas.Clicked(dismiss) {
			open = false
		}
		if open {
			components.BeginPopover(canvas, ui.LightTheme(), ui.Rect{Width: size.Width, Height: size.Height}, anchor, components.PopoverOptions{
				Layer: 3, Scope: scope, DismissID: dismiss, AnchorDismiss: true, ContentSize: ui.Size{Width: 160, Height: 100},
			}).Paint("Details", "Content")
		}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	for _, button := range []ui.PointerButton{ui.PointerButtonPrimary, ui.PointerButtonSecondary} {
		if err := driver.Click(40, 20); err != nil {
			t.Fatal(err)
		}
		if !open {
			t.Fatal("trigger did not open")
		}
		before := toggles
		if err := driver.ClickButton(40, 20, button); err != nil {
			t.Fatal(err)
		}
		if open || toggles != before {
			t.Fatalf("anchor press: button=%v open=%v toggles=%d before=%d", button, open, toggles, before)
		}
	}
	driver.Click(40, 20)
	if err := driver.Click(270, 135); err != nil {
		t.Fatal(err)
	}
	if open || pageClicks != 1 {
		t.Fatal("outside page press did not dismiss and pass through")
	}
}
