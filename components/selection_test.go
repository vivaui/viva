package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSelectionColorsFallBackForOlderThemes(t *testing.T) {
	theme := gfx.DarkTheme()
	theme.Selection = gfx.Color{}
	theme.SelectionForeground = gfx.Color{}
	background, foreground := selectionColors(theme)
	if background != theme.Primary || foreground != theme.PrimaryForeground {
		t.Fatalf("selection fallback = %+v / %+v, want Primary pair %+v / %+v", background, foreground, theme.Primary, theme.PrimaryForeground)
	}

	theme.Selection = gfx.RGB(1, 2, 3)
	theme.SelectionForeground = gfx.RGB(4, 5, 6)
	background, foreground = selectionColors(theme)
	if background != theme.Selection || foreground != theme.SelectionForeground {
		t.Fatalf("selection tokens = %+v / %+v, want %+v / %+v", background, foreground, theme.Selection, theme.SelectionForeground)
	}
}
