package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestSelectionColorsFallBackForOlderThemes(t *testing.T) {
	theme := ui.DarkTheme()
	theme.Selection = ui.Color{}
	theme.SelectionForeground = ui.Color{}
	background, foreground := selectionColors(theme)
	if background != theme.Primary || foreground != theme.PrimaryForeground {
		t.Fatalf("selection fallback = %+v / %+v, want Primary pair %+v / %+v", background, foreground, theme.Primary, theme.PrimaryForeground)
	}

	theme.Selection = ui.RGB(1, 2, 3)
	theme.SelectionForeground = ui.RGB(4, 5, 6)
	background, foreground = selectionColors(theme)
	if background != theme.Selection || foreground != theme.SelectionForeground {
		t.Fatalf("selection tokens = %+v / %+v, want %+v / %+v", background, foreground, theme.Selection, theme.SelectionForeground)
	}
}
