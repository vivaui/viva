package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestAlertDialogDefaultAndSmallGeometry(t *testing.T) {
	viewport := gfx.Rect{Width: 900, Height: 700}
	var defaultCanvas, smallCanvas gfx.Canvas
	defaultLayout := BeginAlertDialog(&defaultCanvas, gfx.LightTheme(), viewport, AlertDialogOptions{
		Layer: 3, Scope: gfx.Item(4), ActionID: gfx.Item(5), CancelID: gfx.Item(6),
	}).WithMedia().Paint("Delete account?", "This action cannot be undone.", "Delete", "Cancel")
	smallLayout := BeginAlertDialog(&smallCanvas, gfx.LightTheme(), viewport, AlertDialogOptions{
		Layer: 3, Scope: gfx.Item(4), ActionID: gfx.Item(5), CancelID: gfx.Item(6), Size: AlertDialogSmall,
	}).WithMedia().Paint("Delete account?", "This action cannot be undone.", "Delete", "Cancel")
	if defaultLayout.Content.Width != 512 || smallLayout.Content.Width != 320 {
		t.Fatalf("content widths = %v / %v", defaultLayout.Content.Width, smallLayout.Content.Width)
	}
	if defaultLayout.Media.Width != 64 || smallLayout.Media.Width != 64 {
		t.Fatalf("media slots = %+v / %+v", defaultLayout.Media, smallLayout.Media)
	}
	if smallLayout.Cancel.Width != smallLayout.Action.Width {
		t.Fatalf("small action widths = %v / %v", smallLayout.Cancel.Width, smallLayout.Action.Width)
	}
	if defaultLayout.Cancel.X >= defaultLayout.Action.X || smallLayout.Cancel.X >= smallLayout.Action.X {
		t.Fatal("cancel action must precede primary action")
	}
}

func TestAlertDialogUsesModalScopeLayerAndTypedButtons(t *testing.T) {
	var canvas gfx.Canvas
	BeginAlertDialog(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 700, Height: 500}, AlertDialogOptions{
		Layer: 8, Scope: gfx.Item(9), ActionID: gfx.Item(10), CancelID: gfx.Item(11),
	}).Paint("Continue?", "Review the consequences.", "Continue", "Cancel")
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Layer != 8 {
			t.Fatalf("rectangle layer = %d", rectangle.Layer)
		}
	}
	if canvas.TrappedFocusScope() != gfx.Item(9) || canvas.EscapeTarget() != gfx.Item(11) {
		t.Fatalf("modal routing = %d / %d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	if len(canvas.Hits()) != 2 {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	for _, hit := range canvas.Hits() {
		if hit.Scope != gfx.Item(9) || hit.Role != gfx.RoleButton || !hit.Focusable {
			t.Fatalf("button hit = %+v", hit)
		}
	}
}

func TestAlertDialogDisabledStateIsExplicit(t *testing.T) {
	var canvas gfx.Canvas
	BeginAlertDialog(&canvas, gfx.LightTheme(), gfx.Rect{Width: 600, Height: 400}, AlertDialogOptions{
		Layer: 1, Scope: gfx.Item(2), ActionID: gfx.Item(3), CancelID: gfx.Item(4),
	}).Disabled().Paint("Unavailable", "Try again later.", "Retry", "Cancel")
	if !canvas.EscapeTarget().IsZero() {
		t.Fatalf("escape target = %d", canvas.EscapeTarget())
	}
	for _, hit := range canvas.Hits() {
		if !hit.Disabled {
			t.Fatalf("enabled hit = %+v", hit)
		}
	}
}

func TestAlertDialogWithoutHeightFitsItsText(t *testing.T) {
	viewport := gfx.Rect{Width: 900, Height: 700}
	theme := gfx.DarkTheme()
	options := AlertDialogOptions{Layer: 3, Scope: gfx.Item(4), ActionID: gfx.Item(5), CancelID: gfx.Item(6)}

	var canvas gfx.Canvas
	plain := BeginAlertDialog(&canvas, theme, viewport, options).Paint("Clear history?", "Every saved conversation is deleted.", "Clear", "Cancel")
	if plain.Body.Height != 0 {
		t.Fatalf("a plain confirmation leaves a body of %v above its buttons", plain.Body.Height)
	}
	if plain.Content.Height >= 180 {
		t.Fatalf("plain height = %v, want a dialog that fits its text", plain.Content.Height)
	}

	var media gfx.Canvas
	withMedia := BeginAlertDialog(&media, theme, viewport, options).WithMedia().Paint("Clear history?", "Every saved conversation is deleted.", "Clear", "Cancel")
	if withMedia.Content.Height <= plain.Content.Height || withMedia.Body.Height != 0 {
		t.Fatalf("media height = %v with body %v, want a dialog fitted to its 64px media: taller than %v, no empty body", withMedia.Content.Height, withMedia.Body.Height, plain.Content.Height)
	}

	options.ContentSize.Height = 300
	var tall gfx.Canvas
	room := BeginAlertDialog(&tall, theme, viewport, options).Paint("Clear history?", "Every saved conversation is deleted.", "Clear", "Cancel")
	if room.Content.Height != 300 || room.Body.Height <= 0 {
		t.Fatalf("explicit height = %v with body %v, want 300 with room for body UI", room.Content.Height, room.Body.Height)
	}
}

func TestAlertDialogActionVariantStylesOnlyTheConfirmingButton(t *testing.T) {
	theme := gfx.DarkTheme()
	viewport := gfx.Rect{Width: 900, Height: 700}
	paint := func(variant ButtonVariant) (danger, primary int) {
		var canvas gfx.Canvas
		BeginAlertDialog(&canvas, theme, viewport, AlertDialogOptions{
			Layer: 3, Scope: gfx.Item(4), ActionID: gfx.Item(5), CancelID: gfx.Item(6), ActionVariant: variant,
		}).Paint("Delete?", "This cannot be undone.", "Delete", "Cancel")
		for _, rectangle := range canvas.Rectangles() {
			switch rectangle.Color {
			case buttonStyleFor(&theme, ButtonDestructive, false, false).background:
				danger++
			case theme.Primary:
				primary++
			}
		}
		return danger, primary
	}
	if danger, primary := paint(ButtonDefault); danger != 0 || primary != 1 {
		t.Fatalf("default action: %d danger / %d primary rectangles", danger, primary)
	}
	if danger, primary := paint(ButtonDestructive); danger != 1 || primary != 0 {
		t.Fatalf("destructive action: %d danger / %d primary rectangles", danger, primary)
	}
}

func TestAlertDialogGrowsWithAWrappedDescription(t *testing.T) {
	viewport := gfx.Rect{Width: 900, Height: 700}
	options := AlertDialogOptions{Layer: 3, Scope: gfx.Item(4), ActionID: gfx.Item(5), CancelID: gfx.Item(6), Size: AlertDialogSmall}
	paint := func(description string) AlertDialogLayout {
		var canvas gfx.Canvas
		return BeginAlertDialog(&canvas, gfx.LightTheme(), viewport, options).Paint("Clear history?", description, "Clear", "Cancel")
	}
	short := paint("Deleted for good.")
	long := paint("Every saved conversation is deleted from this computer and from the synced backup, and it cannot be recovered afterwards.")
	if long.Content.Height <= short.Content.Height || long.Footer.Y+long.Footer.Height > long.Content.Y+long.Content.Height {
		t.Fatalf("short %+v, long %+v: the dialog did not grow with its text or the footer left it", short.Content, long.Content)
	}
}
