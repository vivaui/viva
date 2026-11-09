//go:build darwin || windows || linux

package headless

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// testApp is a small application: a button that counts, a text field, and a
// label that shows the count. It stands in for any Viva program.
type testApp struct {
	clicks int
	// secondary counts presses of the secondary button, which is not a click.
	secondary int
	editor    text.Editor
}

func (a *testApp) config() app.Config {
	theme := gfx.DarkTheme()
	theme.Metrics = theme.Metrics.WithDensity(gfx.DensityCompact)
	return app.Config{
		Title: "test app", Width: 320, Height: 160,
		ClearColor: app.Color{Red: 0.1, Green: 0.1, Blue: 0.12, Alpha: 1},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, theme.Background)
			if components.BeginButton(canvas, theme, gfx.Item(1), gfx.Rect{X: 16, Y: 16, Width: 120, Height: 32}).Default("Add").Clicked() {
				a.clicks++
			}
			canvas.Label("Clicks: "+string(rune('0'+a.clicks)), gfx.Rect{X: 160, Y: 22, Width: 100, Height: 20},
				gfx.TextStyle{Size: 13, Color: theme.Foreground})
			focused := canvas.Focused() == gfx.Item(2)
			components.PaintInput(canvas, theme, gfx.Rect{X: 16, Y: 64, Width: 288, Height: 30}, components.InputSnapshot{
				ID: gfx.Item(2), Label: "Name", Placeholder: "Your name", Value: a.editor.String(),
				PlatformEditing: focused && a.editor.String() != "",
			})
		},
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			if event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonSecondary {
				a.secondary++
				return gfx.EventResult{Handled: true}
			}
			return gfx.EventResult{}
		},
		TextInput: func(app.TextInputContext) *app.TextInput { return &app.TextInput{Editor: &a.editor} },
	}
}

func openTestApp(t *testing.T) (*Session, *testApp) {
	t.Helper()
	target := &testApp{}
	session, err := Open(target.config(), nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session, target
}

func TestSessionOperatesTheApplicationAndAnswersQueries(t *testing.T) {
	session, target := openTestApp(t)
	if !session.HasText("Clicks: 0") {
		t.Fatalf("texts = %q", session.Texts())
	}
	if err := session.ClickLabel("Add"); err != nil {
		t.Fatal(err)
	}
	if target.clicks != 1 || !session.HasText("Clicks: 1") {
		t.Fatalf("clicks=%d texts=%q", target.clicks, session.Texts())
	}
	if err := session.ClickLabel("no such control"); err == nil {
		t.Fatal("clicking a missing control succeeded")
	}
	if err := session.ClickLabel("Name"); err != nil {
		t.Fatal(err)
	}
	if err := session.Type("Ada"); err != nil {
		t.Fatal(err)
	}
	if target.editor.String() != "Ada" {
		t.Fatalf("typed %q", target.editor.String())
	}
	if hit, ok := session.Hit("name"); !ok || hit.ID != gfx.Item(2) {
		t.Fatalf("case-insensitive lookup = %+v %v", hit, ok)
	}
}

func TestSessionHitUsesTheFrontMostReachableMatchingControl(t *testing.T) {
	var clicked []gfx.ID
	config := app.Config{Title: "layered labels", Width: 320, Height: 180}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithLabel(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 120, Height: 32}, "Add Route")
		canvas.HitWithLabel(gfx.Item(2), gfx.Rect{X: 10, Y: 55, Width: 120, Height: 32}, "Background Only")
		canvas.HitWithLabel(gfx.Item(3), gfx.Rect{X: 155, Y: 10, Width: 140, Height: 32}, "Add Route Details")
		canvas.PushFocusScope(gfx.Item(9))
		canvas.Block(gfx.Rect{X: 0, Y: 0, Width: size.Width, Height: size.Height})
		canvas.HitWithLabel(gfx.Item(4), gfx.Rect{X: 90, Y: 120, Width: 140, Height: 32}, "Add Route")
		canvas.PopFocusScope()
		canvas.TrapFocus(gfx.Item(9))
	}
	paint := config.Paint
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		paint(canvas, size)
		for _, hit := range canvas.Hits() {
			if canvas.Clicked(hit.ID) {
				clicked = append(clicked, hit.ID)
			}
		}
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	for _, test := range []struct {
		name  string
		label string
		id    uint32
		ok    bool
	}{
		{name: "duplicate label", label: "Add Route", id: 4, ok: true},
		{name: "background only", label: "Background Only"},
	} {
		t.Run(test.name, func(t *testing.T) {
			hit, ok := session.Hit(test.label)
			if hit.ID != gfx.Item(int(test.id)) || ok != test.ok {
				t.Fatalf("hit = %+v, %v; want ID %d, %v", hit, ok, test.id, test.ok)
			}
		})
	}
	if err := session.ClickLabel("Add Route"); err != nil {
		t.Fatal(err)
	}
	if len(clicked) != 1 || clicked[0] != gfx.Item(4) {
		t.Fatalf("clicks = %v; want [4]", clicked)
	}
}

func TestSessionHitPrefersAnExactReachableLabelOverSubstringMatches(t *testing.T) {
	config := app.Config{Title: "exact label", Width: 320, Height: 120}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithLabel(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 120, Height: 32}, "Save")
		canvas.HitWithLabel(gfx.Item(2), gfx.Rect{X: 160, Y: 10, Width: 140, Height: 32}, "Save As")
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if hit, ok := session.Hit("Save"); !ok || hit.ID != gfx.Item(1) {
		t.Fatalf("hit = %+v, %v; want the exact label 1", hit, ok)
	}
}

func BenchmarkSessionHitLabel(b *testing.B) {
	config := app.Config{Title: "hit label bench", Width: 640, Height: 480}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		for index := uint32(0); index < 128; index++ {
			label := "Cell"
			if index == 127 {
				label = "Selected Cell"
			}
			canvas.HitWithLabel(gfx.Item(int(index+1)), gfx.Rect{X: float32(index%16) * 40, Y: float32(index/16) * 40, Width: 38, Height: 38}, label)
		}
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer session.Close()
	b.ReportAllocs()
	for b.Loop() {
		hit, ok := session.Hit("Selected Cell")
		if !ok || hit.ID != gfx.Item(128) {
			b.Fatalf("hit = %+v, %v; want 128", hit, ok)
		}
	}
}

func TestKeysParsesChordsAndRoutesThem(t *testing.T) {
	session, target := openTestApp(t)
	if err := session.Keys("tab"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("enter"); err != nil { // activates the focused button
		t.Fatal(err)
	}
	if target.clicks != 1 {
		t.Fatalf("Tab then Enter: clicks = %d", target.clicks)
	}
	if err := session.Keys("tab"); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []string{"h", "i", "cmd+a", "cmd+c", "backspace", "cmd+v"} {
		if err := session.Keys(spec); err != nil {
			t.Fatalf("%s: %v", spec, err)
		}
	}
	if target.editor.String() != "hi" {
		t.Fatalf("typed, selected, copied, deleted and pasted: %q, want hi", target.editor.String())
	}
	if err := session.Keys("cmd+z"); err != nil || target.editor.String() != "" {
		t.Fatalf("undo paste: err=%v value=%q", err, target.editor.String())
	}
	if err := session.Keys("cmd+shift+z"); err != nil || target.editor.String() != "hi" {
		t.Fatalf("redo paste: err=%v value=%q", err, target.editor.String())
	}
	for _, bad := range []string{"cmd+nope", "hyper+a", "ctrl+x"} {
		if err := session.Keys(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if err := session.Keys("shift+tab"); err != nil {
		t.Fatal(err)
	}
}

func TestKeysPreservesCompleteCommandShortcutModifiers(t *testing.T) {
	want := gfx.ModifierMeta | gfx.ModifierAlt | gfx.ModifierShift
	var got gfx.KeyEvent
	session, err := Open(app.Config{
		Title: "shortcut", Width: 320, Height: 180,
		Paint: func(*gfx.Canvas, gfx.Size) {},
		OnShortcut: func(event gfx.KeyEvent) gfx.EventResult {
			got = event
			return gfx.EventResult{Handled: true}
		},
	}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if err := session.Keys("cmd+alt+shift+x"); err != nil {
		t.Fatal(err)
	}
	if got.Modifiers != want || got.Text != "x" {
		t.Fatalf("shortcut = %+v, want modifiers=%d text=x", got, want)
	}
}

func TestScreenshotsAreRealPixelsAndDeterministic(t *testing.T) {
	if !rendererAvailable {
		session, _ := openTestApp(t)
		if _, err := session.Image(); !errors.Is(err, ErrNoRenderer) {
			t.Fatalf("Image without a renderer: %v", err)
		}
		return
	}
	session, _ := openTestApp(t)
	first, err := session.Image()
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	if first.Bounds().Dx() != 320*defaultScale || first.Bounds().Dy() != 160*defaultScale {
		t.Fatalf("size = %v, want 320x160 at the default scale of %d", first.Bounds(), defaultScale)
	}
	// The background is the dark theme's, and the primary button is its
	// blue: 25,60,184 comes straight out of the Metal pipeline.
	corner, button := first.Bounds().Max.Sub(image.Pt(40, 20)), image.Pt(30*defaultScale, 30*defaultScale)
	if r, g, b, _ := first.At(corner.X, corner.Y).RGBA(); r>>8 > 40 || g>>8 > 40 || b>>8 > 40 {
		t.Errorf("background is not dark: %v", first.At(corner.X, corner.Y))
	}
	if r, g, b, _ := first.At(button.X, button.Y).RGBA(); r>>8 != 25 || g>>8 != 60 || b>>8 != 184 {
		t.Errorf("the primary button fill = %v, want the theme's blue 25,60,184", first.At(button.X, button.Y))
	}
	again, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Pix, again.Pix) {
		t.Error("two renders of the same frame differ")
	}

	// Typing changes the picture: the native editor overlay draws the text. Only the
	// macOS renderer draws that overlay so far.
	if rendersTextEditor {
		if err := session.ClickLabel("Name"); err != nil {
			t.Fatal(err)
		}
		if err := session.Type("Ada Lovelace"); err != nil {
			t.Fatal(err)
		}
		typed, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(first.Pix, typed.Pix) {
			t.Error("typing did not change the screenshot")
		}
		if lit := brightPixels(typed, image.Rect(40, 130, 400, 190)); lit < 40 {
			t.Errorf("the typed text drew only %d bright pixels in the field", lit)
		}
	}

	// Scale 1 halves the pixel dimensions.
	small, err := Open((&testApp{}).config(), nil, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer small.Close()
	picture, err := small.Image()
	if err != nil || picture.Bounds().Dx() != 320 || picture.Bounds().Dy() != 160 {
		t.Fatalf("scale 1: %v %v", picture.Bounds(), err)
	}

	path := filepath.Join(t.TempDir(), "shot.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() < 500 {
		t.Fatalf("png = %v %v", info, err)
	}
}

func brightPixels(picture *image.RGBA, area image.Rectangle) int {
	count := 0
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if r, g, b, _ := picture.At(x, y).RGBA(); r>>8+g>>8+b>>8 > 420 {
				count++
			}
		}
	}
	return count
}

func TestSplitArgumentsHonorsQuotes(t *testing.T) {
	for input, want := range map[string][]string{
		``:                     nil,
		`a b  c`:               {"a", "b", "c"},
		`"New Host"`:           {"New Host"},
		`click "say \"hi\"" 3`: {"click", `say "hi"`, "3"},
		`"" x`:                 {"", "x"},
		`  "a b"   "c"  `:      {"a b", "c"},
	} {
		got, err := splitArguments(input)
		if err != nil || len(got) != len(want) || strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Errorf("splitArguments(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := splitArguments(`"open`); err == nil {
		t.Error("an unterminated quote was accepted")
	}
}

func TestServeSpeaksTheLineProtocol(t *testing.T) {
	session, target := openTestApp(t)
	script := strings.Join([]string{
		`size`,
		`find "Add"`,
		`click "Add"`,
		`texts Clicks`,
		`dblclick "Add"`,
		`click 150 80`,
		`type Hello World`,
		`editor`,
		`key cmd+a cmd+c`,
		`clip`,
		`focus`,
		`hits Name`,
		`waitfor "Clicks: 3" 200`,
		`waitfor "never appears" 60`,
		`bogus`,
		`move x y`,
		`quit`,
		`size`, // after quit: never read
	}, "\n")
	var out bytes.Buffer
	if err := Serve(session, strings.NewReader(script), &out); err != nil {
		t.Fatal(err)
	}
	replies := strings.Split(strings.TrimSpace(out.String()), "\n")
	want := []string{
		fmt.Sprintf("ok 320x160 scale=%d", defaultScale),
		`ok id=#1 x=16 y=16 w=120 h=26 label="Add"`,
		`ok`,
		`x=160 y=22 w=100 h=20 size=13 "Clicks: 1"`,
		`ok`,
		`ok`,
		`ok`,
		`ok`,
		`ok value="Hello World" selection=11,11 secure=false`,
		`ok`,
		`ok "Hello World"`,
		`ok id=#2 label="Name"`,
		`id=#2 x=16 y=64 w=288 h=30 label="Name"`,
		`ok`,
		`ok`,
	}
	for index, line := range want {
		if index >= len(replies) || replies[index] != line {
			t.Fatalf("reply %d = %q, want %q\nall replies:\n%s", index, safeIndex(replies, index), line, out.String())
		}
	}
	tail := replies[len(want):]
	if len(tail) != 4 || !strings.HasPrefix(tail[0], `error "never appears" did not appear within 60ms; screen has`) ||
		!strings.HasPrefix(tail[1], `error unknown command "bogus"`) || !strings.HasPrefix(tail[2], `error move: "x" is not a number`) || tail[3] != "ok bye" {
		t.Fatalf("tail = %q", tail)
	}
	if target.clicks != 3 || target.editor.String() != "Hello World" {
		t.Fatalf("the script did not operate the app: clicks=%d text=%q", target.clicks, target.editor.String())
	}
}

func safeIndex(values []string, index int) string {
	if index < len(values) {
		return values[index]
	}
	return "<missing>"
}

func TestWaitForAppliesBackgroundWork(t *testing.T) {
	var invalidator app.Invalidator
	var mutex = make(chan string, 1)
	value := ""
	config := app.Config{Title: "bg", Width: 200, Height: 80, Invalidator: &invalidator}
	config.OnWake = func() {
		select {
		case value = <-mutex:
		default:
		}
	}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.Label(value, gfx.Rect{Width: 100, Height: 20}, gfx.TextStyle{Size: 13, Color: gfx.Color{Alpha: 1}})
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	go func() {
		time.Sleep(30 * time.Millisecond)
		mutex <- "arrived"
		invalidator.Invalidate()
	}()
	err = session.WaitFor(func(s app.Snapshot) bool { return len(s.Texts) > 0 && s.Texts[0].Value == "arrived" }, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkScreenshot(b *testing.B) {
	target := &testApp{}
	session, err := Open(target.config(), nil, Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer session.Close()
	if _, err := session.Image(); err != nil {
		b.Skipf("no renderer: %v", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := session.Image(); err != nil {
			b.Fatal(err)
		}
	}
}

// An application that only paints is clicked through the headless
// driver exactly as through a window: the click reaches the control that declared it.
func TestHeadlessClickReachesTheControlThatDeclaredIt(t *testing.T) {
	theme := gfx.LightTheme()
	count := 0
	var tree gfx.Tree
	save := tree.ID(tree.Feature("page"), "save")
	config := app.Config{
		Title: "decl", Width: 320, Height: 160, Tree: &tree,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			if components.BeginButton(canvas, theme, save, gfx.Rect{X: 16, Y: 16, Width: 120, Height: 32}).Default("Save").Clicked() {
				count++
			}
		},
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.ClickLabel("Save"); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("Save was handled %d times, want 1", count)
	}
	hit, ok := session.Hit("Save")
	if !ok || tree.Text(hit.ID) != "page/save" {
		t.Fatalf("the control prints as %q", tree.Text(hit.ID))
	}
}

// A shape a clip cuts reaches the GPU whole and is cut by its scissor: the part
// inside the clip is painted, the part outside is not, and a stroke is cut the
// same way. This is the path from the application's canvas to real pixels.
func TestClippedShapesAreCutByTheScissorInRealPixels(t *testing.T) {
	if !rendererAvailable {
		t.Skip("no renderer on this build")
	}
	red := gfx.Color{Red: 1, Alpha: 1}
	session, err := Open(app.Config{
		Title: "clip", Width: 100, Height: 100,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.Clip(gfx.Rect{X: 20, Y: 20, Width: 40, Height: 40})
			canvas.Fill(gfx.Rect{X: 0, Y: 0, Width: 100, Height: 100}, red)
			canvas.Unclip()
			canvas.Clip(gfx.Rect{X: 20, Y: 70, Width: 40, Height: 20})
			canvas.DrawPath(gfx.NewPath(100, 10, gfx.MoveTo(0, 5), gfx.LineTo(100, 5)), gfx.Rect{X: 0, Y: 75, Width: 100, Height: 10},
				gfx.PathStyle{Stroke: red, StrokeWidth: 6, Cap: gfx.CapButt})
			canvas.Unclip()
		},
	}, nil, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	img, err := session.Image()
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	isRed := func(x, y int) bool { r, g, b, _ := img.At(x, y).RGBA(); return r>>8 > 200 && g>>8 < 60 && b>>8 < 60 }
	for _, c := range []struct {
		x, y int
		want bool
		name string
	}{
		{40, 40, true, "inside the cut rectangle"},
		{10, 40, false, "left of the clip"},
		{80, 40, false, "right of the clip"},
		{40, 10, false, "above the clip"},
		{40, 80, true, "inside the cut stroke"},
		{10, 80, false, "stroke left of the clip"},
		{90, 80, false, "stroke right of the clip"},
	} {
		if got := isRed(c.x, c.y); got != c.want {
			t.Errorf("%s: pixel (%d,%d) red=%v, want %v (%v)", c.name, c.x, c.y, got, c.want, img.At(c.x, c.y))
		}
	}
}

func TestRightClickDeliversTheSecondaryButtonAndNoClick(t *testing.T) {
	session, target := openTestApp(t)
	if err := session.RightClickLabel("Add"); err != nil {
		t.Fatal(err)
	}
	if err := session.RightClick(20, 20); err != nil {
		t.Fatal(err)
	}
	if err := session.RightClickLabel("no such control"); err == nil {
		t.Fatal("right-clicking a missing control succeeded")
	}
	if target.secondary != 2 || target.clicks != 0 {
		t.Fatalf("secondary presses = %d, clicks = %d; want 2 and 0", target.secondary, target.clicks)
	}

	var out bytes.Buffer
	if err := Serve(session, strings.NewReader("rightclick \"Add\"\nrightclick 20 20\nrightclick x y\nquit\n"), &out); err != nil {
		t.Fatal(err)
	}
	replies := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(replies) != 4 || replies[0] != "ok" || replies[1] != "ok" || !strings.HasPrefix(replies[2], "error") || target.secondary != 4 {
		t.Fatalf("replies = %q, secondary presses = %d", replies, target.secondary)
	}
}

// A screenshot must show the layout a window would have, so a session measures with
// the platform's engine; a tool that compares layouts across machines asks for the
// reference metrics.
func TestSessionMeasuresWithThePlatformEngineUnlessAskedForTheReference(t *testing.T) {
	engine := app.PlatformTextEngine()
	if engine == nil {
		t.Skip("this platform has no text engine yet")
	}
	const sample = "192.168.1.1 主机 Hello"
	width := func(options Options) float32 {
		var canvas gfx.Canvas
		session, err := Open((&testApp{}).config(), &canvas, options)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		return canvas.TextWidth(sample, 14)
	}
	if got, want := width(Options{}), engine.Width(sample, 14, gfx.WeightRegular); got != want {
		t.Errorf("a default session measures %v, the platform engine %v", got, want)
	}
	if got, want := width(Options{ReferenceText: true}), gfx.ReferenceTextEngine().Width(sample, 14, gfx.WeightRegular); got != want {
		t.Errorf("a reference session measures %v, the reference metrics %v", got, want)
	}
	if gfx.ReferenceTextEngine().Width(sample, 14, gfx.WeightRegular) == engine.Width(sample, 14, gfx.WeightRegular) {
		t.Fatal("the two engines agree on the sample; the test cannot tell them apart")
	}
}

func TestKeysTypeIntoAHitThatDeclaresTypeAhead(t *testing.T) {
	var query string
	var keys []gfx.Key
	session, err := Open(app.Config{
		Title: "type-ahead", Width: 320, Height: 180,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 320, Height: 180}, "Files", gfx.HitOptions{Role: gfx.RoleGroup, TypeAhead: true})
		},
		OnKey: func(event gfx.KeyEvent) gfx.EventResult {
			keys = append(keys, event.Key)
			return gfx.EventResult{Handled: true}
		},
		TextInput: func(context app.TextInputContext) *app.TextInput {
			if !context.TypeAhead {
				return nil
			}
			return &app.TextInput{OnEvent: func(event app.TextEvent) (app.TextState, error) {
				if event.Kind == app.TextInsert {
					query += event.Text
				}
				return app.TextState{}, nil
			}}
		},
	}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Click(100, 50); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []string{"f", "o", "o", "down"} {
		if err := session.Keys(spec); err != nil {
			t.Fatal(err)
		}
	}
	if query != "foo" || len(keys) != 1 || keys[0] != gfx.KeyDown {
		t.Fatalf("query = %q, keys = %v", query, keys)
	}
}
