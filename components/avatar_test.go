package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestAvatarPaintsOfficialSizesFallbackBorderAndBadge(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	Avatar(&canvas, theme, gfx.Rect{}, AvatarData{Name: "Ada Lovelace", Initials: "AL", Status: AvatarStatusOnline, Size: AvatarSizeSmall})
	if canvas.Rectangles()[0].Bounds.Width != 24 || canvas.Rectangles()[0].Color != theme.Border {
		t.Fatalf("small avatar border = %+v", canvas.Rectangles()[0])
	}
	if len(canvas.Texts()) != 1 || canvas.Texts()[0].Value != "AL" {
		t.Fatalf("avatar texts = %+v", canvas.Texts())
	}
	if got := canvas.Rectangles()[3].Color; got != theme.Success {
		t.Fatalf("presence color = %+v", got)
	}
}

func TestAvatarGroupAndImagePlaceholderAreStable(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		group := BeginAvatarGroup(&canvas, gfx.DarkTheme(), gfx.Rect{X: 4, Y: 4}, AvatarSizeDefault)
		group.Add(AvatarData{Name: "No Image", Initials: "N"})
		group.Add(AvatarData{Name: "Bea", Initials: "B"})
		group.AddCount("+3")
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	paint()
	if len(canvas.Texts()) != 3 || canvas.Texts()[2].Value != "+3" {
		t.Fatalf("avatar group texts = %+v", canvas.Texts())
	}
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical avatar groups produced different commands")
	}
}

func TestAvatarImageStaysInsideCircleAndFallsBackWhenInvalid(t *testing.T) {
	theme := gfx.LightTheme()
	bitmap := gfx.Bitmap{Pix: make([]byte, 16*8*4), Width: 16, Height: 8, Stride: 16 * 4, ID: 7}
	var canvas gfx.Canvas
	bounds := gfx.Rect{X: 10, Y: 20, Width: 32, Height: 32}
	Avatar(&canvas, theme, bounds, AvatarData{Initials: "AL", Image: bitmap})
	if len(canvas.Images()) == 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("image avatar: %d images, %d texts", len(canvas.Images()), len(canvas.Texts()))
	}
	for _, image := range canvas.Images() {
		if image.Bounds.X < bounds.X || image.Bounds.Y < bounds.Y ||
			image.Bounds.X+image.Bounds.Width > bounds.X+bounds.Width || image.Bounds.Y+image.Bounds.Height > bounds.Y+bounds.Height {
			t.Fatalf("image strip %+v leaves the avatar", image.Bounds)
		}
		if image.Source.Width > 8 || image.Source.Height > 8 {
			t.Fatalf("image source %+v is not the cropped square", image.Source)
		}
	}
	canvas.Reset()
	bitmap.Pix = nil
	Avatar(&canvas, theme, bounds, AvatarData{Initials: "AL", Image: bitmap})
	if len(canvas.Images()) != 0 || len(canvas.Texts()) != 1 {
		t.Fatalf("invalid image should fall back to initials: %d images, %d texts", len(canvas.Images()), len(canvas.Texts()))
	}
}
