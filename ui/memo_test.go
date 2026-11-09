package ui_test

import (
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type memoRow struct {
	id      int
	name    string
	done    bool
	version uint64
}

type memoModel struct {
	rows  []memoRow
	calls int // how many times a row was declared
	top   string
}

func newMemoModel(n int) *memoModel {
	m := &memoModel{}
	for i := range n {
		m.rows = append(m.rows, memoRow{id: i, name: "host " + strconv.Itoa(i), version: 1})
	}
	return m
}

func (m *memoModel) row(c ui.C, r *memoRow) {
	m.calls++
	ui.Row(c).Pad(8).Gap(8).Children(func() {
		ui.Text(c, r.name)
		ui.Checkbox(c, &r.done, "")
		ui.Button(c, "Open")
	})
}

func (m *memoModel) view(c ui.C) {
	ui.Column(c).Fill().Gap(4).Children(func() {
		if m.top != "" {
			ui.Text(c, m.top).Height(30)
		}
		for i := range m.rows {
			r := &m.rows[i]
			ui.Memo(c.KeyInt(r.id), r.version, func() { m.row(c, r) })
		}
	})
}

type memoPage struct {
	page   *ui.Page
	canvas *gfx.Canvas
}

func newMemoPage() *memoPage { return newMemoPageWith(ui.PageConfig{}) }

func newMemoPageWith(config ui.PageConfig) *memoPage {
	canvas := benchCanvas()
	canvas.KeepPrevious(true)
	if config.Theme == nil {
		config.Theme = &pageTheme
	}
	return &memoPage{page: ui.NewPage(config), canvas: canvas}
}

func (mp *memoPage) draw(view func(ui.C)) { drawPage(mp.page, mp.canvas, view) }

type painted struct {
	Rectangles []gfx.Rectangle
	Texts      []gfx.Text
	Hits       []gfx.Hit
}

func (mp *memoPage) output() painted {
	var hits []gfx.Hit
	for _, hit := range mp.canvas.Hits() {
		hit.Placement = nil
		hits = append(hits, hit)
	}
	return painted{append([]gfx.Rectangle(nil), mp.canvas.Rectangles()...), append([]gfx.Text(nil), mp.canvas.Texts()...), hits}
}

// What a page paints with its rows copied is what it paints declaring them.
func TestMemoCopyEqualsDeclaring(t *testing.T) {
	m := newMemoModel(20)
	fresh := newMemoPage()
	fresh.draw(m.view)
	want := fresh.output()

	cached := newMemoPage()
	cached.draw(m.view)
	m.calls = 0
	cached.draw(m.view)
	if m.calls != 0 {
		t.Fatalf("%d rows were declared in a paint where nothing changed", m.calls)
	}
	if got := cached.output(); !reflect.DeepEqual(got, want) {
		t.Fatal("the copied page differs from the declared one")
	}
	cached.draw(m.view) // a copy of a copy
	if got := cached.output(); !reflect.DeepEqual(got, want) {
		t.Fatal("the page copied twice differs from the declared one")
	}
}

func TestMemoDeclaresAgainWhenTheVersionChanges(t *testing.T) {
	m := newMemoModel(5)
	mp := newMemoPage()
	mp.draw(m.view)
	mp.draw(m.view)
	m.rows[2].name, m.rows[2].version = "renamed", 2
	m.calls = 0
	mp.draw(m.view)
	if m.calls != 1 {
		t.Fatalf("%d rows declared after one row's version changed, want 1", m.calls)
	}
	if !strings.Contains(textsOf(mp.canvas), "renamed") {
		t.Fatal("the changed row does not show its new text")
	}
}

// A row added above moves the rows under it: they are copied moved, and what is
// painted is what declaring them gives.
func TestMemoRowsMoveWithoutBeingDeclared(t *testing.T) {
	m := newMemoModel(10)
	mp := newMemoPage()
	mp.draw(m.view)
	mp.draw(m.view)
	m.top = "a heading above the rows"
	m.calls = 0
	mp.draw(m.view)
	if m.calls != 0 {
		t.Fatalf("%d rows were declared after only moving", m.calls)
	}
	fresh := newMemoPage()
	fresh.draw(m.view)
	if !reflect.DeepEqual(mp.output(), fresh.output()) {
		t.Fatal("the moved copies differ from the rows declared there")
	}
}

func TestMemoDeclaresAgainWhenTheWindowResizes(t *testing.T) {
	m := newMemoModel(5)
	mp := newMemoPage()
	mp.draw(m.view)
	mp.draw(m.view)
	m.calls = 0
	mp.canvas.Reset()
	mp.canvas.BeginFrame()
	mp.page.Draw(mp.canvas, gfx.Rect{Width: 500, Height: 400}, m.view)
	mp.canvas.EndFrame()
	if m.calls != 5 {
		t.Fatalf("%d rows declared after a resize, want 5", m.calls)
	}
}

// Through a window: clicks, focus and typing reach the controls of a copied row,
// and its names stay the same across copies.
func TestMemoRowsStayInteractive(t *testing.T) {
	m := newMemoModel(3)
	opened := -1
	view := func(c ui.C) {
		ui.Column(c).Fill().Gap(4).Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				ui.Memo(c.KeyInt(r.id), r.version, func() {
					ui.Row(c).Gap(8).Children(func() {
						ui.Checkbox(c, &r.done, r.name)
						if ui.Button(c, "Open "+r.name).Clicked() {
							opened = r.id
						}
					})
				})
			}
		})
	}
	d := openPage(t, view)
	ids := map[string]gfx.ID{}
	for _, hit := range d.driver.Snapshot().Hits {
		ids[hit.Label] = hit.ID
	}
	for range 3 {
		d.repaint()
	}
	for _, hit := range d.driver.Snapshot().Hits {
		if id, ok := ids[hit.Label]; ok && id != hit.ID {
			t.Fatalf("%q changed its name across copies", hit.Label)
		}
	}
	d.click("Open host 1")
	if opened != 1 {
		t.Fatalf("opened %d", opened)
	}
	d.click("host 2")
	if !m.rows[2].done {
		t.Fatal("a checkbox in a copied row did not take its click")
	}
	d.key(gfx.KeySpace)
	if m.rows[2].done {
		t.Fatal("the focused checkbox of a row did not take the key")
	}
}

func TestMemoCheckCatchesAValueThatMissesAChange(t *testing.T) {
	m := newMemoModel(3)
	mp := newMemoPageWith(ui.PageConfig{MemoCheck: true})
	mp.draw(m.view)
	m.rows[1].name = "changed without a new value"
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "MemoCheck") {
			t.Fatalf("recover() = %v, want MemoCheck to report the row", r)
		}
	}()
	mp.draw(m.view)
}

func TestMemoValueSnapshotDeclaresAgainOnChange(t *testing.T) {
	m := newMemoModel(5)
	mp := newMemoPage()
	view := func(c ui.C) {
		ui.Column(c).Fill().Gap(4).Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				ui.Memo(c.KeyInt(r.id), *r, func() { m.row(c, r) })
			}
		})
	}
	mp.draw(view)
	mp.draw(view)
	m.rows[2].name = "renamed"
	m.calls = 0
	mp.draw(view)
	if m.calls != 1 {
		t.Fatalf("%d rows declared after one row's value changed, want 1", m.calls)
	}
	if !strings.Contains(textsOf(mp.canvas), "renamed") {
		t.Fatal("the changed row does not show its new text")
	}
}

func TestMemoValueSnapshotSteadyPaintAllocatesNothing(t *testing.T) {
	m := newMemoModel(100)
	mp := newMemoPage()
	view := func(c ui.C) {
		ui.Column(c).Fill().Gap(4).Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				ui.Memo(c.KeyInt(r.id), *r, func() { m.row(c, r) })
			}
		})
	}
	for range 3 {
		mp.draw(view)
	}
	if allocs := testing.AllocsPerRun(30, func() { mp.draw(view) }); allocs != 0 {
		t.Fatalf("a steady value-snapshot page allocates %v times", allocs)
	}
}

func TestMemoDeclaresAgainWhenTheThemeChanges(t *testing.T) {
	m := newMemoModel(3)
	theme := gfx.LightTheme()
	mp := newMemoPageWith(ui.PageConfig{Theme: &theme})
	mp.draw(m.view)
	mp.draw(m.view)
	m.calls = 0
	theme = gfx.DarkTheme()
	mp.draw(m.view)
	if m.calls != 3 {
		t.Fatalf("%d rows declared after the theme changed, want 3", m.calls)
	}
}

func TestMemoDeclaresAgainWhenTheScaleChanges(t *testing.T) {
	m := newMemoModel(3)
	mp := newMemoPage()
	mp.draw(m.view)
	mp.draw(m.view)
	m.calls = 0
	mp.canvas.SetPixelScale(3)
	mp.draw(m.view)
	if m.calls != 3 {
		t.Fatalf("%d rows declared after the scale changed, want 3", m.calls)
	}
}

func TestMemoDeclaresAgainWhenTheInheritedOpacityChanges(t *testing.T) {
	m := newMemoModel(3)
	mp := newMemoPage()
	opacity := float32(1)
	view := func(c ui.C) {
		ui.Column(c).Opacity(opacity).Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				ui.Memo(c.KeyInt(r.id), r.version, func() { m.row(c, r) })
			}
		})
	}
	mp.draw(view)
	mp.draw(view)
	m.calls = 0
	opacity = 0.5
	mp.draw(view)
	if m.calls != 3 {
		t.Fatalf("%d rows declared after the inherited opacity changed, want 3", m.calls)
	}
}

// fontGenerationEngine is a reference text engine with a generation a test moves.
type fontGenerationEngine struct {
	gfx.TextEngine
	generation uint32
}

func (e *fontGenerationEngine) SetFontFamily(string) {}

func (e *fontGenerationEngine) FontGeneration() uint32 { return e.generation }

func TestMemoDeclaresAgainWhenTheFontGenerationChanges(t *testing.T) {
	m := newMemoModel(3)
	mp := newMemoPage()
	engine := &fontGenerationEngine{TextEngine: gfx.ReferenceTextEngine(), generation: 1}
	mp.canvas.SetTextEngine(engine)
	mp.draw(m.view)
	mp.draw(m.view)
	m.calls = 0
	engine.generation++
	mp.draw(m.view)
	if m.calls != 3 {
		t.Fatalf("%d rows declared after the font generation changed, want 3", m.calls)
	}
}

func TestMemoRejectsWhatItCannotKeep(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "in a Memo") {
			t.Fatalf("recover() = %v", r)
		}
	}()
	mp := newMemoPage()
	mp.draw(func(c ui.C) {
		ui.Memo(c, 1, func() { ui.Scroll(c) })
	})
}

func TestMemoSteadyPaintAllocatesNothing(t *testing.T) {
	m := newMemoModel(100)
	mp := newMemoPage()
	for range 3 {
		mp.draw(m.view)
	}
	if allocs := testing.AllocsPerRun(30, func() { mp.draw(m.view) }); allocs != 0 {
		t.Fatalf("a steady memoized page allocates %v times", allocs)
	}
}

// Rows that come and go leave nothing behind.
func TestMemoRetainedHeapStaysFlat(t *testing.T) {
	mp := newMemoPage()
	m := newMemoModel(30)
	heap := func() uint64 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return stats.HeapAlloc
	}
	page := 0
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				ui.Memo(c.KeyInt(page*30+i), 1, func() { m.row(c, r) })
			}
		})
	}
	for page = range 100 {
		mp.draw(view)
	}
	before := heap()
	for page = 100; page < 2_100; page++ {
		mp.draw(view)
	}
	if after := heap(); after > before+256<<10 {
		t.Fatalf("the heap grew from %d to %d bytes over 60,000 rows", before, after)
	}
}

func textsOf(canvas *gfx.Canvas) string {
	var b strings.Builder
	for _, text := range canvas.Texts() {
		b.WriteString(text.Value)
		b.WriteByte('\n')
	}
	return b.String()
}

func BenchmarkRows200Declared(b *testing.B) {
	m := newMemoModel(200)
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.Column(c).Fill().Gap(4).Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				ui.Row(c.KeyInt(r.id)).Children(func() { m.row(c, r) })
			}
		})
	}
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, view)
	}
}

func BenchmarkRows200Memo(b *testing.B) {
	m := newMemoModel(200)
	mp := newMemoPage()
	mp.draw(m.view)
	b.ReportAllocs()
	for b.Loop() {
		mp.draw(m.view)
	}
}

// The same list, each row memoized by the value itself rather than a counter.
func BenchmarkRows200MemoValue(b *testing.B) {
	m := newMemoModel(200)
	mp := newMemoPage()
	view := func(c ui.C) {
		ui.Column(c).Fill().Gap(4).Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				ui.Memo(c.KeyInt(r.id), *r, func() { m.row(c, r) })
			}
		})
	}
	mp.draw(view)
	b.ReportAllocs()
	for b.Loop() {
		mp.draw(view)
	}
}

// One row of two hundred changes in every paint.
func BenchmarkRows200MemoOneChanging(b *testing.B) {
	m := newMemoModel(200)
	mp := newMemoPage()
	mp.draw(m.view)
	b.ReportAllocs()
	for b.Loop() {
		m.rows[7].version++
		mp.draw(m.view)
	}
}

// The whole list in one Memo, each row in its own: an unchanged list is one copy.
func BenchmarkRows200MemoList(b *testing.B) {
	m := newMemoModel(200)
	mp := newMemoPage()
	var version uint64 = 1
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.Memo(c, version, func() { m.view(c) })
		})
	}
	mp.draw(view)
	b.ReportAllocs()
	for b.Loop() {
		mp.draw(view)
	}
}
