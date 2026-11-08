package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestAlertDialogDefaultAndSmallGeometry(t *testing.T) {
	viewport := ui.Rect{Width: 900, Height: 700}
	var defaultCanvas, smallCanvas ui.Canvas
	defaultLayout := BeginAlertDialog(&defaultCanvas, ui.LightTheme(), viewport, AlertDialogOptions{
		Layer: 3, Scope: ui.Item(4), ActionID: ui.Item(5), CancelID: ui.Item(6),
	}).WithMedia().Paint("Delete account?", "This action cannot be undone.", "Delete", "Cancel")
	smallLayout := BeginAlertDialog(&smallCanvas, ui.LightTheme(), viewport, AlertDialogOptions{
		Layer: 3, Scope: ui.Item(4), ActionID: ui.Item(5), CancelID: ui.Item(6), Size: AlertDialogSmall,
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
	var canvas ui.Canvas
	BeginAlertDialog(&canvas, ui.DarkTheme(), ui.Rect{Width: 700, Height: 500}, AlertDialogOptions{
		Layer: 8, Scope: ui.Item(9), ActionID: ui.Item(10), CancelID: ui.Item(11),
	}).Paint("Continue?", "Review the consequences.", "Continue", "Cancel")
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Layer != 8 {
			t.Fatalf("rectangle layer = %d", rectangle.Layer)
		}
	}
	if canvas.TrappedFocusScope() != ui.Item(9) || canvas.EscapeTarget() != ui.Item(11) {
		t.Fatalf("modal routing = %d / %d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	if len(canvas.Hits()) != 2 {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	for _, hit := range canvas.Hits() {
		if hit.Scope != ui.Item(9) || hit.Role != ui.RoleButton || !hit.Focusable {
			t.Fatalf("button hit = %+v", hit)
		}
	}
}

func TestAlertDialogDisabledStateIsExplicit(t *testing.T) {
	var canvas ui.Canvas
	BeginAlertDialog(&canvas, ui.LightTheme(), ui.Rect{Width: 600, Height: 400}, AlertDialogOptions{
		Layer: 1, Scope: ui.Item(2), ActionID: ui.Item(3), CancelID: ui.Item(4),
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
	viewport := ui.Rect{Width: 900, Height: 700}
	theme := ui.DarkTheme()
	options := AlertDialogOptions{Layer: 3, Scope: ui.Item(4), ActionID: ui.Item(5), CancelID: ui.Item(6)}

	var canvas ui.Canvas
	plain := BeginAlertDialog(&canvas, theme, viewport, options).Paint("Clear history?", "Every saved conversation is deleted.", "Clear", "Cancel")
	if plain.Body.Height != 0 {
		t.Fatalf("a plain confirmation leaves a body of %v above its buttons", plain.Body.Height)
	}
	if plain.Content.Height >= 180 {
		t.Fatalf("plain height = %v, want a dialog that fits its text", plain.Content.Height)
	}

	var media ui.Canvas
	withMedia := BeginAlertDialog(&media, theme, viewport, options).WithMedia().Paint("Clear history?", "Every saved conversation is deleted.", "Clear", "Cancel")
	if withMedia.Content.Height <= plain.Content.Height || withMedia.Body.Height != 0 {
		t.Fatalf("media height = %v with body %v, want a dialog fitted to its 64px media: taller than %v, no empty body", withMedia.Content.Height, withMedia.Body.Height, plain.Content.Height)
	}

	options.ContentSize.Height = 300
	var tall ui.Canvas
	room := BeginAlertDialog(&tall, theme, viewport, options).Paint("Clear history?", "Every saved conversation is deleted.", "Clear", "Cancel")
	if room.Content.Height != 300 || room.Body.Height <= 0 {
		t.Fatalf("explicit height = %v with body %v, want 300 with room for body UI", room.Content.Height, room.Body.Height)
	}
}

func TestAlertDialogActionVariantStylesOnlyTheConfirmingButton(t *testing.T) {
	theme := ui.DarkTheme()
	viewport := ui.Rect{Width: 900, Height: 700}
	paint := func(variant ButtonVariant) (danger, primary int) {
		var canvas ui.Canvas
		BeginAlertDialog(&canvas, theme, viewport, AlertDialogOptions{
			Layer: 3, Scope: ui.Item(4), ActionID: ui.Item(5), CancelID: ui.Item(6), ActionVariant: variant,
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
	viewport := ui.Rect{Width: 900, Height: 700}
	options := AlertDialogOptions{Layer: 3, Scope: ui.Item(4), ActionID: ui.Item(5), CancelID: ui.Item(6), Size: AlertDialogSmall}
	paint := func(description string) AlertDialogLayout {
		var canvas ui.Canvas
		return BeginAlertDialog(&canvas, ui.LightTheme(), viewport, options).Paint("Clear history?", description, "Clear", "Cancel")
	}
	short := paint("Deleted for good.")
	long := paint("Every saved conversation is deleted from this computer and from the synced backup, and it cannot be recovered afterwards.")
	if long.Content.Height <= short.Content.Height || long.Footer.Y+long.Footer.Height > long.Content.Y+long.Content.Height {
		t.Fatalf("short %+v, long %+v: the dialog did not grow with its text or the footer left it", short.Content, long.Content)
	}
}
