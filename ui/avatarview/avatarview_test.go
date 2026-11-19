package avatarview_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/avatarview"
)

var testPhoto = func() gfx.Bitmap {
	pix := make([]byte, 4*4*4)
	for index := range pix {
		pix[index] = byte(index * 7)
	}
	return gfx.Bitmap{Pix: pix, Width: 4, Height: 4, Stride: 16, ID: 1, Version: 1}
}()

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 420, Height: 260}, view)
	canvas.EndFrame()
}

// hit returns the front-most hit with the label.
func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	hits := canvas.Hits()
	for index := len(hits) - 1; index >= 0; index-- {
		if hits[index].Label == label {
			return hits[index]
		}
	}
	t.Fatalf("no hit labelled %q was painted", label)
	return gfx.Hit{}
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := new(gfx.Canvas)
	draw(page, canvas, view)
	return page, canvas
}

// The page declares an avatar, and the component paints the caller's initials,
// status and exact diameter inside the control's square.
func TestViewPaintsConfiguredAvatar(t *testing.T) {
	var avatar avatarview.Avatar
	theme := gfx.LightTheme()
	view := func(c ui.C) {
		avatar.Reset().Initials("SK").Name("Skyler").Tint(theme.Primary).Diameter(64).Online()
		avatar.View(c, "Skyler").Size(64, 64)
	}
	_, canvas := open(t, view)

	control := hit(t, canvas, "Skyler")
	if control.Role != gfx.RoleGroup {
		t.Fatalf("control role = %v, want a group", control.Role)
	}
	if control.Bounds.Width != 64 || control.Bounds.Height != 64 {
		t.Fatalf("control bounds = %v, want 64x64", control.Bounds)
	}
	if !hasText(canvas, "SK") {
		t.Fatal("the avatar's initials were not painted")
	}
}

// An image avatar paints the caller's bitmap and does not paint the initials.
func TestImageAvatarPaintsBitmap(t *testing.T) {
	var avatar avatarview.Avatar
	view := func(c ui.C) {
		avatar.Reset().Image(testPhoto).Initials("NO").Diameter(40)
		avatar.View(c, "Photo").Size(40, 40)
	}
	_, canvas := open(t, view)
	if len(canvas.Images()) == 0 {
		t.Fatal("the image avatar painted no bitmap")
	}
	if hasText(canvas, "NO") {
		t.Fatal("the image avatar painted its initials over the image")
	}
}

// A group advances by the caller's overlap and paints every member and the
// count fallback in the control's box.
func TestGroupPaintsMembersAtTheCallerOverlap(t *testing.T) {
	var group avatarview.Group
	theme := gfx.LightTheme()
	view := func(c ui.C) {
		group.Reset().Diameter(40).Overlap(8)
		group.Add().Initials("AD").Name("Ada").Tint(theme.Primary).Online()
		group.Add().Initials("BO").Name("Bo")
		group.Add().Initials("CH").Name("Chen")
		group.AddCount("+8")
		group.View(c, "Collaborators")
	}
	_, canvas := open(t, view)

	control := hit(t, canvas, "Collaborators")
	// Four members, each advancing 32 pixels after the first 40 pixel avatar.
	if control.Bounds.Width != 40+3*32 || control.Bounds.Height != 40 {
		t.Fatalf("group bounds = %v, want %v", control.Bounds, gfx.Rect{Width: 136, Height: 40})
	}
	for _, label := range []string{"AD", "BO", "CH", "+8"} {
		if !hasText(canvas, label) {
			t.Fatalf("group did not paint %q", label)
		}
	}
}

// Overlap is a caller choice: zero spaces the members edge to edge.
func TestGroupOverlapChangesTheExtent(t *testing.T) {
	var group avatarview.Group
	view := func(c ui.C) {
		group.Reset().Diameter(40).Overlap(0)
		group.Add().Initials("A")
		group.Add().Initials("B")
		group.Add().Initials("C")
		group.View(c, "Collaborators")
	}
	_, canvas := open(t, view)
	if control := hit(t, canvas, "Collaborators"); control.Bounds.Width != 120 {
		t.Fatalf("group width = %v, want 120 at zero overlap", control.Bounds.Width)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	var avatar avatarview.Avatar
	var group avatarview.Group
	theme := gfx.LightTheme()
	view := func(c ui.C) {
		avatar.Reset().Initials("SK").Name("Skyler").Tint(theme.Primary).Diameter(64).Online()
		avatar.View(c, "Skyler").Size(64, 64)
		group.Reset().Diameter(40).Overlap(8)
		group.Add().Initials("AD").Name("Ada").Online()
		group.Add().Initials("BO").Name("Bo")
		group.Add().Image(testPhoto).Name("Photo")
		group.AddCount("+8")
		group.View(c, "Collaborators")
	}
	page, canvas := open(t, view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	var avatar avatarview.Avatar
	var group avatarview.Group
	theme := gfx.LightTheme()
	view := func(c ui.C) {
		avatar.Reset().Initials("SK").Name("Skyler").Tint(theme.Primary).Diameter(64).Online()
		avatar.View(c, "Skyler").Size(64, 64)
		group.Reset().Diameter(40).Overlap(8)
		group.Add().Initials("AD").Name("Ada").Online()
		group.Add().Initials("BO").Name("Bo")
		group.Add().Image(testPhoto).Name("Photo")
		group.AddCount("+8")
		group.View(c, "Collaborators")
	}
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
