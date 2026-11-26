// Package indicator is the opt-in page layout for a count or a dot pinned to the
// corner of another control, the red mark of an unread count on an icon or an
// avatar. A page declares the control it marks inside Child, and says what the
// mark shows:
//
//	indicator.View(c, "Inbox", indicator.Spec{Count: m.unread, Max: 99}, func() {
//		ui.Button(c, "Inbox").Icon(inboxIcon).IconOnly().Ghost()
//	})
//
// A Count above zero shows the number, and above Max shows Max and a plus sign
// ("99+"); a Count of zero shows nothing unless ShowZero is set. Dot shows a plain
// dot in place of the number. Status colors the mark as an alert does, the danger
// color by default. The mark is pinned to the top right corner of the control,
// reaches outside it by half its size, and takes no room of its own, so adding it
// does not move the control. label names the group for assistive technology.
//
// Keep no state: the page gives the count each paint. Counts up to 99 and their capped
// forms have their words made once, so a warm paint allocates nothing.
// It is a separate package so an application that does not import it pays
// nothing for it.
package indicator

import (
	"strconv"

	"github.com/vivaui/viva/ui"
)

// Spec declares the mark.
type Spec struct {
	// Count is the number shown.
	Count int
	// Max is the largest number shown as it is; above it the mark reads "Max+". 99
	// when zero.
	Max int
	// ShowZero shows the mark for a Count of zero.
	ShowZero bool
	// Dot shows a dot, whatever the count, while Count is above zero or ShowZero
	// is set.
	Dot bool
	// Status colors the mark; the danger color by default.
	Status ui.Status
}

// texts are the words of the small counts, made once.
var texts = func() (t [100]string) {
	for i := range t {
		t[i] = strconv.Itoa(i)
	}
	return t
}()

// pluses are the words of a capped count, made once.
var pluses = func() (t [100]string) {
	for i := range t {
		t[i] = strconv.Itoa(i) + "+"
	}
	return t
}()

// text is the words of a count, cached for the small ones.
func text(count, max int) string {
	if max <= 0 {
		max = 99
	}
	if count > max {
		if max < len(pluses) {
			return pluses[max]
		}
		return strconv.Itoa(max) + "+"
	}
	if count >= 0 && count < len(texts) {
		return texts[count]
	}
	return strconv.Itoa(count)
}

// View declares the control and its mark. child declares the control, one
// element. The result is the box that holds them.
func View(c ui.C, label string, spec Spec, child func()) ui.Box {
	theme := c.Theme()
	metrics := theme.Metrics
	root := ui.Stack(c).Shrink(0).Label(label)
	root.Children(func() {
		child()
		shown := spec.Count > 0 || spec.ShowZero
		if !shown {
			return
		}
		fill := theme.Danger
		switch spec.Status {
		case ui.StatusSuccess:
			fill = theme.Success
		case ui.StatusWarning:
			fill = theme.WarningColor()
		}
		if spec.Dot {
			ui.Row(c).Absolute().Top(-4).Right(-4).Size(10, 10).Radius(5).Background(fill).Border(2, theme.Background)
			return
		}
		words := text(spec.Count, spec.Max)
		size := metrics.FontSizeS
		mark := ui.Row(c).Absolute().Top(-size*0.6).Right(-size*0.6).Height(size+6).Center().PadSides(0, 5, 0, 5)
		mark = mark.Radius((size + 6) / 2).Background(fill)
		mark.Children(func() {
			ui.Text(c, words).FontSize(size).Color(theme.PrimaryForeground).Shrink(0)
		})
	})
	return root
}
