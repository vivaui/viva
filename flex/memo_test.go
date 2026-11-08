package flex_test

import (
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/flex"
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

func (m *memoModel) row(c flex.C, r *memoRow) {
	m.calls++
	flex.Row(c).Pad(8).Gap(8).Children(func() {
		flex.Text(c, r.name)
		flex.Checkbox(c, &r.done, "")
		flex.Button(c, "Open")
	})
}

func (m *memoModel) view(c flex.C) {
	flex.Column(c).Fill().Gap(4).Children(func() {
		if m.top != "" {
			flex.Text(c, m.top).Height(30)
		}
		for i := range m.rows {
			r := &m.rows[i]
			flex.Memo(c.KeyInt(r.id), r.version, func() { m.row(c, r) })
		}
	})
}

type memoPage struct {
	page   *flex.Page
	canvas *ui.Canvas
}

func newMemoPage() *memoPage {
	canvas := benchCanvas()
	canvas.KeepPrevious(true)
	return &memoPage{page: flex.NewPage(flex.PageConfig{Theme: &pageTheme}), canvas: canvas}
}

func (mp *memoPage) draw(view func(flex.C)) { drawPage(mp.page, mp.canvas, view) }

type painted struct {
	Rectangles []ui.Rectangle
	Texts      []ui.Text
	Hits       []ui.Hit
}

func (mp *memoPage) output() painted {
	var hits []ui.Hit
	for _, hit := range mp.canvas.Hits() {
		hit.Placement = nil
		hits = append(hits, hit)
	}
	return painted{append([]ui.Rectangle(nil), mp.canvas.Rectangles()...), append([]ui.Text(nil), mp.canvas.Texts()...), hits}
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
	mp.page.Draw(mp.canvas, ui.Rect{Width: 500, Height: 400}, m.view)
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
	view := func(c flex.C) {
		flex.Column(c).Fill().Gap(4).Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				flex.Memo(c.KeyInt(r.id), r.version, func() {
					flex.Row(c).Gap(8).Children(func() {
						flex.Checkbox(c, &r.done, r.name)
						if flex.Button(c, "Open "+r.name).Clicked() {
							opened = r.id
						}
					})
				})
			}
		})
	}
	d := openPage(t, view)
	ids := map[string]ui.ID{}
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
	d.key(ui.KeySpace)
	if m.rows[2].done {
		t.Fatal("the focused checkbox of a row did not take the key")
	}
}

func TestMemoCheckCatchesAVersionThatMissesAChange(t *testing.T) {
	m := newMemoModel(3)
	mp := newMemoPage()
	mp.draw(m.view)
	flex.MemoCheck(true)
	defer flex.MemoCheck(false)
	m.rows[1].name = "changed without a new version"
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "MemoCheck") {
			t.Fatalf("recover() = %v, want MemoCheck to report the row", r)
		}
	}()
	mp.draw(m.view)
}

func TestMemoRejectsWhatItCannotKeep(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "in a Memo") {
			t.Fatalf("recover() = %v", r)
		}
	}()
	mp := newMemoPage()
	mp.draw(func(c flex.C) {
		flex.Memo(c, 1, func() { flex.Scroll(c) })
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
	view := func(c flex.C) {
		flex.Column(c).Fill().Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				flex.Memo(c.KeyInt(page*30+i), 1, func() { m.row(c, r) })
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

func textsOf(canvas *ui.Canvas) string {
	var b strings.Builder
	for _, text := range canvas.Texts() {
		b.WriteString(text.Value)
		b.WriteByte('\n')
	}
	return b.String()
}

func BenchmarkRows200Declared(b *testing.B) {
	m := newMemoModel(200)
	page, canvas := flex.NewPage(flex.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c flex.C) {
		flex.Column(c).Fill().Gap(4).Children(func() {
			for i := range m.rows {
				r := &m.rows[i]
				flex.Row(c.KeyInt(r.id)).Children(func() { m.row(c, r) })
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
	view := func(c flex.C) {
		flex.Column(c).Fill().Children(func() {
			flex.Memo(c, version, func() { m.view(c) })
		})
	}
	mp.draw(view)
	b.ReportAllocs()
	for b.Loop() {
		mp.draw(view)
	}
}
