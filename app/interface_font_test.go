package app

import (
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

const interfaceFontSample = "The quick brown fox 0123"

func TestFontFamiliesAreSortedUniqueAndIncludeTheSystemsOwn(t *testing.T) {
	families := FontFamilies()
	if runtime.GOOS == "js" {
		if len(families) != 0 {
			t.Fatalf("a page listed fonts: %v", families)
		}
		return
	}
	if len(families) == 0 {
		t.Skipf("%s reports no families here", runtime.GOOS)
	}
	if !slices.IsSortedFunc(families, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }) {
		t.Fatalf("not sorted: %v", families)
	}
	seen := map[string]bool{}
	for _, name := range families {
		if name == "" || seen[name] || strings.ContainsAny(name, "\n\x00") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") {
			t.Fatalf("bad entry %q in %v", name, families)
		}
		seen[name] = true
	}
	// A family the system draws its own interface in, and no terminal of the
	// system ships without: the list holds every family, not the fixed-pitch
	// ones alone.
	want := map[string]string{"darwin": "Helvetica", "windows": "Segoe UI"}[runtime.GOOS]
	if want != "" && !seen[want] {
		t.Fatalf("%s is missing from %v", want, families)
	}
	for _, name := range MonospaceFamilies() {
		if !seen[name] {
			t.Errorf("the fixed-pitch family %q is not among all the families", name)
		}
	}
}

// aFamilyThatShapesDifferently is an installed family whose text measures unlike
// the system interface font's: a fixed-pitch one, whose glyphs share one width.
func aFamilyThatShapesDifferently(t *testing.T) string {
	t.Helper()
	families := MonospaceFamilies()
	if len(families) == 0 {
		t.Skipf("%s has no fixed-pitch family here", runtime.GOOS)
	}
	return families[0]
}

// interfaceEngine is the platform's engine with the interface family restored to
// the system's when the test ends: the engine is the process's.
func interfaceEngine(t *testing.T) ui.TextEngine {
	t.Helper()
	engine := PlatformTextEngine()
	if engine == nil {
		t.Skipf("%s has no native text engine here", runtime.GOOS)
	}
	if !SetInterfaceFontFamily("") {
		t.Fatal("the platform's engine cannot follow a family")
	}
	t.Cleanup(func() { SetInterfaceFontFamily("") })
	return engine
}

func TestAPageCannotFollowAnInterfaceFamily(t *testing.T) {
	if runtime.GOOS != "js" {
		t.Skip("only a browser has no engine to follow a family")
	}
	if SetInterfaceFontFamily("Anything") {
		t.Fatal("a page accepted an interface family")
	}
}

func TestTheInterfaceFamilyIsWhatTextIsShapedIn(t *testing.T) {
	engine := interfaceEngine(t)
	family := aFamilyThatShapesDifferently(t)
	system := engine.Width(interfaceFontSample, 14, 0.15)
	if system <= 0 {
		t.Fatalf("the engine measures %v", system)
	}
	if !SetInterfaceFontFamily(family) {
		t.Fatal("the engine refused a family it listed")
	}
	if named := engine.Width(interfaceFontSample, 14, 0.15); named <= 0 || named == system {
		t.Fatalf("%q measures %v, as the system font does (%v): the family was not used", family, named, system)
	}
	SetInterfaceFontFamily("")
	if back := engine.Width(interfaceFontSample, 14, 0.15); back != system {
		t.Fatalf("the system font came back as %v, want %v", back, system)
	}
}

func TestAFamilyNothingHasShapesAsTheSystemFont(t *testing.T) {
	engine := interfaceEngine(t)
	system := engine.Width(interfaceFontSample, 14, 0)
	SetInterfaceFontFamily("No such family, certainly not installed")
	if width := engine.Width(interfaceFontSample, 14, 0); width != system {
		t.Fatalf("a missing family measures %v, want the system font's %v", width, system)
	}
}

func TestANameThatCannotBeAFamilyIsTheSystemFont(t *testing.T) {
	engine := interfaceEngine(t)
	family := aFamilyThatShapesDifferently(t)
	system := engine.Width(interfaceFontSample, 14, 0)
	SetInterfaceFontFamily(family)
	if engine.Width(interfaceFontSample, 14, 0) == system {
		t.Fatalf("%q does not shape unlike the system font here", family)
	}
	for _, name := range []string{strings.Repeat("a", maxInterfaceFamilyName+1), "Bad\x00Name", "Bad\nName", "  ", ""} {
		SetInterfaceFontFamily(family)
		SetInterfaceFontFamily(name)
		if got := engine.Width(interfaceFontSample, 14, 0); got != system {
			t.Errorf("%q measures %v, want the system font's %v", name, got, system)
		}
	}
}

// A window shows what its last paint sent, and a label whose text, size and box did
// not change is not sent again: the painter must send it when the family did, or the
// renderer keeps drawing the picture it made in the old font.
func TestAFamilyChangeSendsTheSameTextToTheRendererAgain(t *testing.T) {
	interfaceEngine(t)
	family := aFamilyThatShapesDifferently(t)
	canvas := &ui.Canvas{}
	canvas.SetTextEngine(PlatformTextEngine())
	config := Config{Width: 240, Height: 80, Paint: func(canvas *ui.Canvas, _ ui.Size) {
		canvas.Label("Settings", ui.Rect{X: 8, Y: 8, Width: 200, Height: 20}, ui.TextStyle{Size: 14})
		canvas.Label("ls -la", ui.Rect{X: 8, Y: 40, Width: 200, Height: 20}, ui.TextStyle{Size: 14, Mono: true})
	}}
	painter, err := newPainter(canvas, &config)
	if err != nil {
		t.Fatal(err)
	}
	generation := func() (interfaceText, fixedPitch uint32) {
		if len(painter.labels) != 2 {
			t.Fatalf("labels = %+v", painter.labels)
		}
		return painter.labels[0].FontGeneration, painter.labels[1].FontGeneration
	}
	before, _ := generation()
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if frame.TextLabels != nil {
		t.Fatal("an unchanged repaint sent its text again")
	}
	SetInterfaceFontFamily(family)
	frame, err = painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if frame.TextLabels == nil {
		t.Fatal("the family changed and the text was not sent to the renderer again")
	}
	after, fixedPitch := generation()
	if after == before {
		t.Fatalf("the interface text carries generation %d before and after the change", after)
	}
	if fixedPitch != 0 {
		t.Fatalf("fixed-pitch text follows the interface font: generation %d", fixedPitch)
	}
	if frame, err = painter.repaint(); err != nil || frame.TextLabels != nil {
		t.Fatalf("a repaint after the change sent its text again (%v)", err)
	}
}

// BenchmarkInterfaceFamilyWidth measures a line the engine has shaped before: the
// system font's and a named family's, each remembered once the family is in force.
func BenchmarkInterfaceFamilyWidth(b *testing.B) {
	engine := PlatformTextEngine()
	if engine == nil || !SetInterfaceFontFamily("") {
		b.Skip("no engine that follows a family")
	}
	b.Cleanup(func() { SetInterfaceFontFamily("") })
	families := MonospaceFamilies()
	if len(families) == 0 {
		b.Skip("no installed fixed-pitch family")
	}
	for _, name := range []struct{ label, family string }{{"system", ""}, {"named", families[0]}} {
		b.Run(name.label, func(b *testing.B) {
			SetInterfaceFontFamily(name.family)
			engine.Width(interfaceFontSample, 14, 0.15)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				engine.Width(interfaceFontSample, 14, 0.15)
			}
		})
	}
}

// BenchmarkInterfaceFamilyChange is the cost of naming another family and shaping the
// line again: what a settings page pays once per choice.
func BenchmarkInterfaceFamilyChange(b *testing.B) {
	engine := PlatformTextEngine()
	if engine == nil || !SetInterfaceFontFamily("") {
		b.Skip("no engine that follows a family")
	}
	b.Cleanup(func() { SetInterfaceFontFamily("") })
	families := MonospaceFamilies()
	if len(families) == 0 {
		b.Skip("no installed fixed-pitch family")
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if i%2 == 0 {
			SetInterfaceFontFamily(families[0])
		} else {
			SetInterfaceFontFamily("")
		}
		engine.Width(interfaceFontSample, 14, 0.15)
	}
}
