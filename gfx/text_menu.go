package gfx

// TextMenuLabels are the words of the menu a text field opens on a secondary
// press. A label left empty is English. A page, a window and its menu share one
// pointer to it, so a language change is seen the next time the menu opens.
type TextMenuLabels struct {
	Cut, Copy, Paste, SelectAll string
}

// WithDefaults returns labels with every empty one English.
func (labels TextMenuLabels) WithDefaults() TextMenuLabels {
	if labels.Cut == "" {
		labels.Cut = "Cut"
	}
	if labels.Copy == "" {
		labels.Copy = "Copy"
	}
	if labels.Paste == "" {
		labels.Paste = "Paste"
	}
	if labels.SelectAll == "" {
		labels.SelectAll = "Select All"
	}
	return labels
}
