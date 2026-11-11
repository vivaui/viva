package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A TextField's OnKey claims navigation keys while the field has the keyboard,
// leaves typing to the field, and refusal leaves the key to the page.
func TestTextFieldOnKeyClaimsKeysAndCanRefuse(t *testing.T) {
	var query ui.Field
	var seen []gfx.Key
	submitted := false
	d := openPage(t, func(c ui.C) {
		field := ui.TextField(c, &query, "Search", "")
		if field.Submitted() {
			submitted = true
		}
		field.OnKey(func(event gfx.KeyEvent) gfx.EventResult {
			seen = append(seen, event.Key)
			if event.Key == gfx.KeyDown {
				return gfx.EventResult{Handled: true}
			}
			return gfx.EventResult{} // Up and Enter are not the field's
		})
	})
	d.click("Search")
	d.typeText("ab")
	d.key(gfx.KeyDown)
	d.key(gfx.KeyUp)
	d.key(gfx.KeyEnter)
	if query.Text() != "ab" {
		t.Fatalf("typing reached %q, want the field to keep its text", query.Text())
	}
	want := []gfx.Key{gfx.KeyDown, gfx.KeyUp, gfx.KeyEnter}
	if len(seen) != len(want) {
		t.Fatalf("the handler saw %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("the handler saw %v, want %v", seen, want)
		}
	}
	if !submitted {
		t.Fatal("refusing Enter was not left to the page, so Submitted never fired")
	}
}

func TestTextFieldOnKeyNeedsATextField(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("OnKey on a container did not panic")
		}
	}()
	openPage(t, func(c ui.C) {
		ui.Column(c).OnKey(func(gfx.KeyEvent) gfx.EventResult { return gfx.EventResult{} })
	})
}
