package upload_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/upload"
)

type model struct {
	up       *upload.Upload
	spec     upload.Spec
	added    []string
	rejected []string
	removed  []int
	retried  []int
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Children(func() {
		m.up.View(c, "Attachments", m.spec).FillWidth()
		for _, f := range m.up.Added() {
			m.added = append(m.added, f.Name())
		}
		for _, f := range m.up.Rejected() {
			m.rejected = append(m.rejected, f.Name())
		}
		if i, ok := m.up.Removed(); ok {
			m.removed = append(m.removed, i)
		}
		if i, ok := m.up.Retried(); ok {
			m.retried = append(m.retried, i)
		}
	})
}

type driver struct {
	t *testing.T
	*app.Driver
}

func open(t *testing.T, m *model) driver {
	t.Helper()
	m.up = upload.New()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	d, err := app.NewDriver(ui.Configure(page, app.Config{Width: 480, Height: 480, Invalidator: new(app.Invalidator)}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return driver{t, d}
}

func (d driver) areaPoint() (float32, float32) {
	d.t.Helper()
	for _, text := range d.Snapshot().Texts {
		if text.Value == "Drag files here" {
			return text.Bounds.X + text.Bounds.Width/2, text.Bounds.Y + text.Bounds.Height/2
		}
	}
	d.t.Fatal("the area's title is not painted")
	return 0, 0
}

func (d driver) drop(paths ...string) {
	d.t.Helper()
	x, y := d.areaPoint()
	if _, err := d.Drop(gfx.FileDropEvent{Phase: gfx.FileDropOver, X: x, Y: y}); err != nil {
		d.t.Fatal(err)
	}
	if taken, err := d.Drop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, X: x, Y: y, Paths: paths}); err != nil || !taken {
		d.t.Fatalf("the area did not take the drop: %v %v", taken, err)
	}
	if err := d.Pump(); err != nil {
		d.t.Fatal(err)
	}
}

func (d driver) click(label string) {
	d.t.Helper()
	hits := d.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		if h := hits[i]; h.Label == label && !h.IsLabel && !h.ID.IsZero() {
			if err := d.Click(h.Bounds.X+h.Bounds.Width/2, h.Bounds.Y+h.Bounds.Height/2); err != nil {
				d.t.Fatal(err)
			}
			return
		}
	}
	d.t.Fatalf("no control labeled %q", label)
}

func files(t *testing.T, names ...string) []string {
	t.Helper()
	if runtime.GOOS == "js" {
		t.Skip("a browser gives dropped files without paths, so a test cannot drop them by path")
	}
	dir := t.TempDir()
	var paths []string
	for _, name := range names {
		path := filepath.Join(dir, name)
		if filepath.Ext(name) == "" && name[len(name)-1] == '/' {
			path = filepath.Join(dir, name)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDroppedFilesAreAddedOrRejectedByAccept(t *testing.T) {
	m := &model{spec: upload.Spec{Multiple: true, Accept: []string{".png", ".jpg"}}}
	d := open(t, m)
	d.drop(files(t, "a.PNG", "b.txt", "c.jpg")...)
	if want := []string{"a.PNG", "c.jpg"}; !equal(m.added, want) {
		t.Errorf("added %v, want %v", m.added, want)
	}
	if want := []string{"b.txt"}; !equal(m.rejected, want) {
		t.Errorf("rejected %v, want %v", m.rejected, want)
	}
}

func TestSingleBatchTakesOnlyTheFirstFile(t *testing.T) {
	m := &model{}
	d := open(t, m)
	d.drop(files(t, "a.txt", "b.txt")...)
	if !equal(m.added, []string{"a.txt"}) || !equal(m.rejected, []string{"b.txt"}) {
		t.Errorf("added %v rejected %v", m.added, m.rejected)
	}
}

func TestLimitCountsTheItemsAlreadyInTheList(t *testing.T) {
	m := &model{spec: upload.Spec{Multiple: true, Limit: 3, Items: []upload.Item{{Name: "x"}, {Name: "y"}}}}
	d := open(t, m)
	d.drop(files(t, "a.txt", "b.txt")...)
	if !equal(m.added, []string{"a.txt"}) || !equal(m.rejected, []string{"b.txt"}) {
		t.Errorf("added %v rejected %v", m.added, m.rejected)
	}
}

func TestFoldersAreRejected(t *testing.T) {
	m := &model{spec: upload.Spec{Multiple: true}}
	d := open(t, m)
	d.drop(files(t, "dir/", "a.txt")...)
	if !equal(m.added, []string{"a.txt"}) || !equal(m.rejected, []string{"dir"}) {
		t.Errorf("added %v rejected %v", m.added, m.rejected)
	}
}

func TestDisabledAreaTakesNothing(t *testing.T) {
	m := &model{spec: upload.Spec{Disabled: true}}
	d := open(t, m)
	x, y := d.areaPoint()
	if taken, err := d.Drop(gfx.FileDropEvent{Phase: gfx.FileDropOver, X: x, Y: y}); err != nil || taken {
		t.Fatalf("a disabled area took files: %v %v", taken, err)
	}
}

func TestRemoveAndRetryReportTheRow(t *testing.T) {
	m := &model{spec: upload.Spec{Items: []upload.Item{
		{Name: "a.txt", Size: 10, Status: upload.Done},
		{Name: "b.txt", Size: 2048, Status: upload.Failed, Error: "Network error"},
		{Name: "c.txt", Status: upload.Uploading, Progress: 0.5},
	}}}
	d := open(t, m)
	d.click("Retry b.txt")
	d.click("Remove c.txt")
	if len(m.retried) != 1 || m.retried[0] != 1 {
		t.Errorf("retried %v, want [1]", m.retried)
	}
	if len(m.removed) != 1 || m.removed[0] != 2 {
		t.Errorf("removed %v, want [2]", m.removed)
	}
}

func TestOnlyFailedItemsOfferRetry(t *testing.T) {
	m := &model{spec: upload.Spec{Items: []upload.Item{{Name: "a.txt", Status: upload.Done}}}}
	d := open(t, m)
	for _, h := range d.Snapshot().Hits {
		if h.Label == "Retry a.txt" {
			t.Fatal("a finished item offers retry")
		}
	}
}

func warmView() (func(), *model) {
	m := &model{up: upload.New(), spec: upload.Spec{Multiple: true, Hint: "PNG", Items: []upload.Item{
		{Name: "a.txt", Size: 10, Status: upload.Done},
		{Name: "b.txt", Size: 2048, Status: upload.Failed, Error: "Network error"},
		{Name: "c.txt", Size: 1 << 20, Status: upload.Uploading, Progress: 0.5},
	}}}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	return func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 480, Height: 480}, m.page)
		canvas.EndFrame()
	}, m
}

func TestWarmPaintDoesNotAllocate(t *testing.T) {
	draw, _ := warmView()
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	draw, _ := warmView()
	draw()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		draw()
	}
}

func TestFormatSizeGrowsByUnit(t *testing.T) {
	m := &model{spec: upload.Spec{Items: []upload.Item{{Name: "a", Size: 512}, {Name: "b", Size: 1536}, {Name: "c", Size: 5 << 20}}}}
	d := open(t, m)
	seen := map[string]bool{}
	for _, text := range d.Snapshot().Texts {
		seen[text.Value] = true
	}
	for _, want := range []string{"512 B", "1.5 KB", "5.0 MB"} {
		if !seen[want] {
			t.Errorf("no %q among the painted texts", want)
		}
	}
}
