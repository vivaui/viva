// Package statistic is the opt-in page control for one number shown as a
// reading: a muted title, then the value in a large size with its prefix and
// suffix, formatted to a number of decimals with digit groups. A page keeps a
// Statistic in its model and gives it the number each paint:
//
//	m.sessions = statistic.New()
//	// in the page:
//	m.sessions.View(c, statistic.Spec{Title: "Active sessions", Value: m.count, Group: true})
//	m.latency.View(c, statistic.Spec{Title: "Latency", Value: m.ms, Precision: 1, Suffix: "ms"})
//
// Precision is the number of decimals; Group puts a comma between thousands
// (GroupSeparator changes it). Status colors the value as an alert or a tag
// does: success, warning or danger take the theme's color for it. NaN and the
// infinities are shown as an em dash, the usual mark of a reading that is not
// there.
//
// Unlike metriccard, which is a card with a trend and a description, this is the
// number alone, to place in a row, a toolbar or a card of the page's own. The
// formatted text is made again only when the number or its format changes, so a
// warm paint of an unchanged reading allocates nothing. Keep a Statistic in the
// model; do not copy one after it has been declared.
package statistic

import (
	"math"
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Spec declares one reading.
type Spec struct {
	// Title is the muted caption above the value; not drawn when empty.
	Title string
	// Value is the number.
	Value float64
	// Precision is the number of decimals shown.
	Precision int
	// Group puts GroupSeparator between the thousands of the integer part.
	Group bool
	// GroupSeparator is the rune between groups; ',' when zero.
	GroupSeparator rune
	// Prefix and Suffix are written around the number, such as a currency sign or a
	// unit. They take the value's color.
	Prefix, Suffix string
	// Status colors the value; the zero value keeps the foreground.
	Status ui.Status
	// ValueSize is the value's font size; 24 when zero.
	ValueSize float32
}

// Statistic is one reading and the text made from it.
type Statistic struct {
	last  Spec
	text  string
	label string // the title and the text, the reading's name for assistive technology
	made  bool
}

// New returns a reading.
func New() *Statistic { return &Statistic{} }

// Format writes value with precision decimals, and with sep between groups of
// three digits of the integer part when sep is not zero. NaN and infinities are
// an em dash.
func Format(value float64, precision int, sep rune) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "—"
	}
	precision = max(precision, 0)
	digits := strconv.FormatFloat(math.Abs(value), 'f', precision, 64)
	whole, fraction := digits, ""
	for i := 0; i < len(digits); i++ {
		if digits[i] == '.' {
			whole, fraction = digits[:i], digits[i:]
			break
		}
	}
	out := make([]byte, 0, len(digits)+len(digits)/3+2)
	// A value that rounds to zero is not negative.
	if value < 0 && (precision == 0 && math.Round(value) != 0 || precision > 0 && !allZero(digits)) {
		out = append(out, '-')
	}
	for i := 0; i < len(whole); i++ {
		if sep != 0 && i > 0 && (len(whole)-i)%3 == 0 {
			out = append(out, string(sep)...)
		}
		out = append(out, whole[i])
	}
	return string(append(out, fraction...))
}

func allZero(digits string) bool {
	for i := 0; i < len(digits); i++ {
		if digits[i] != '0' && digits[i] != '.' {
			return false
		}
	}
	return true
}

// Text is the number as View shows it, with the prefix and suffix.
func (s *Statistic) Text(spec Spec) string {
	if s.made && s.last.Value == spec.Value && s.last.Precision == spec.Precision && s.last.Group == spec.Group &&
		s.last.GroupSeparator == spec.GroupSeparator && s.last.Prefix == spec.Prefix && s.last.Suffix == spec.Suffix && s.last.Title == spec.Title ||
		s.made && math.IsNaN(spec.Value) && math.IsNaN(s.last.Value) && s.last.Precision == spec.Precision &&
			s.last.Prefix == spec.Prefix && s.last.Suffix == spec.Suffix && s.last.Title == spec.Title {
		return s.text
	}
	var sep rune
	if spec.Group {
		if sep = spec.GroupSeparator; sep == 0 {
			sep = ','
		}
	}
	s.text = spec.Prefix + Format(spec.Value, spec.Precision, sep) + spec.Suffix
	s.label = spec.Title + " " + s.text
	s.last, s.made = spec, true
	return s.text
}

// View declares the reading at c. The reading is as wide as its text and as tall
// as its title and value. Declare it on its own, not inside a Memo.
func (s *Statistic) View(c ui.C, spec Spec) ui.Box {
	if s == nil {
		panic("statistic: View needs its Statistic")
	}
	text := s.Text(spec)
	theme := c.Theme()
	metrics := theme.Metrics
	size := spec.ValueSize
	if size <= 0 {
		size = 24
	}
	ink := theme.Foreground
	switch spec.Status {
	case ui.StatusSuccess:
		ink = theme.Success
	case ui.StatusWarning:
		ink = theme.WarningColor()
	case ui.StatusDanger:
		ink = theme.Danger
	}
	root := ui.Column(c).Gap(4).HitWith(ui.ID(c), s.label, gfx.HitOptions{Role: gfx.RoleStatus, SkipFocus: true})
	root.Children(func() {
		if spec.Title != "" {
			ui.Text(c, spec.Title).FontSize(metrics.FontSizeS).Muted()
		}
		ui.Text(c, text).FontSize(size).Weight(gfx.WeightMedium).Color(ink)
	})
	return root
}
