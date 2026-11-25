// Package inputnumber is the opt-in page control for a number field: a text
// field that holds a number, with a button on each side that steps it down and
// up. A page keeps an InputNumber in its model beside the number it edits:
//
//	m.port = inputnumber.New().Min(1).Max(65535)
//	// in the page:
//	m.port.View(c, "Port", &m.portValue).Width(160)
//	if m.port.Changed() {
//		// the user entered or stepped to a new number; m.portValue holds it
//	}
//
// The user types a number and it is taken when the field loses the focus or
// Enter is pressed; text that is not a number is put back to the number the
// field held. A number outside Min and Max is brought into range. Up and Down
// step the number by Step while the field has the focus, PageUp and PageDown by
// ten steps, and Home and End go to Min and Max when they are set. The buttons
// step it too, and are dimmed at the ends of the range.
//
// Precision is how many decimals the number is shown and kept with; it follows
// Step by default, so a step of 0.1 shows one decimal. The number is a float64:
// whole numbers up to 2^53 are exact.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an InputNumber in the model: it holds the field and the
// handlers, so a warm paint allocates nothing. Do not copy an InputNumber after
// it has been declared.
package inputnumber

import (
	"math"
	"strconv"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var (
	minusIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(5, 12), gfx.LineTo(19, 12),
	}}
	plusIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(5, 12), gfx.LineTo(19, 12), gfx.MoveTo(12, 5), gfx.LineTo(12, 19),
	}}
)

// InputNumber is one number field. Configure it with the chainable methods; they
// may be called before each View.
type InputNumber struct {
	field ui.Field
	keyFn func(gfx.KeyEvent) gfx.EventResult

	min, max  float64
	step      float64
	precision int // decimals; negative follows step
	hint      string
	readOnly  bool
	decrease  string
	increase  string
	fieldID   gfx.ID
	fieldOpt  [1]ui.Option

	value    *float64
	shown    float64 // the number the field's text was made from
	shownSet bool
	focused  bool // the field had the focus in the last paint
	changed  bool
	reported bool
}

// New returns a number field with no range, a step of one and no decimals.
func New() *InputNumber {
	n := &InputNumber{min: math.Inf(-1), max: math.Inf(1), step: 1, precision: -1, decrease: "Decrease", increase: "Increase"}
	n.keyFn = n.key
	return n
}

// Min is the least number the field takes.
func (n *InputNumber) Min(min float64) *InputNumber {
	n.min = min
	return n
}

// Max is the greatest number the field takes.
func (n *InputNumber) Max(max float64) *InputNumber {
	n.max = max
	return n
}

// Step is how much the buttons and the arrow keys change the number; one by
// default. It must be above zero.
func (n *InputNumber) Step(step float64) *InputNumber {
	if step > 0 {
		n.step = step
	}
	return n
}

// Precision is how many decimals the number is shown and kept with. By default it
// is as many as Step has.
func (n *InputNumber) Precision(decimals int) *InputNumber {
	n.precision = max(decimals, 0)
	return n
}

// Placeholder is the text shown while the field is empty.
func (n *InputNumber) Placeholder(text string) *InputNumber {
	n.hint = text
	return n
}

// ReadOnly shows the number and takes no input.
func (n *InputNumber) ReadOnly() *InputNumber {
	n.readOnly = true
	return n
}

// Editable clears ReadOnly, for a page that enables editing later.
func (n *InputNumber) Editable() *InputNumber {
	n.readOnly = false
	return n
}

// ButtonLabels are the accessible names of the two step buttons, "Decrease" and
// "Increase" by default, for a page in another language.
func (n *InputNumber) ButtonLabels(decrease, increase string) *InputNumber {
	n.decrease, n.increase = decrease, increase
	return n
}

// FieldID gives the text field an identity of the application's own instead of the
// one its place gives it. It is for a runner that resolves the platform text
// service by an application identity, such as a component preview.
func (n *InputNumber) FieldID(id gfx.ID) *InputNumber {
	n.fieldID, n.fieldOpt[0] = id, ui.WithID(id)
	return n
}

// Field is the text field behind the number, for a runner that binds the editor
// the platform edits to it with Field.Bind. A page does not need it.
func (n *InputNumber) Field() *ui.Field { return &n.field }

// Changed reports whether the user entered or stepped to a new number in the
// paint that delivered it.
func (n *InputNumber) Changed() bool { return n.reported }

// decimals is how many decimals the number carries.
func (n *InputNumber) decimals() int {
	if n.precision >= 0 {
		return n.precision
	}
	text := strconv.FormatFloat(n.step, 'f', -1, 64)
	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		return len(text) - dot - 1
	}
	return 0
}

// tidy is x in range and rounded to the decimals the field keeps.
func (n *InputNumber) tidy(x float64) float64 {
	scale := math.Pow10(n.decimals())
	x = math.Round(x*scale) / scale
	return math.Min(math.Max(x, n.min), n.max)
}

func (n *InputNumber) format(x float64) string {
	return strconv.FormatFloat(x, 'f', n.decimals(), 64)
}

// parse is the number a text holds. A text with no digits, or one that is not a
// number, holds none.
func parse(text string) (float64, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	x, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
		return 0, false
	}
	return x, true
}

// show puts x in the field's text.
func (n *InputNumber) show(x float64) {
	n.field.SetText(n.format(x))
	n.shown, n.shownSet = x, true
	n.field.Invalid = false
}

// take makes x the number of the field and reports whether it changed.
func (n *InputNumber) take(x float64) {
	x = n.tidy(x)
	if x != *n.value {
		*n.value, n.changed = x, true
	}
	n.show(*n.value)
}

// commit takes the number the field's text holds, or puts the held one back.
func (n *InputNumber) commit() {
	if x, ok := parse(n.field.Text()); ok {
		n.take(x)
		return
	}
	n.show(*n.value)
}

// stepBy changes the number by delta steps, from what the field's text says.
func (n *InputNumber) stepBy(delta float64) {
	from := *n.value
	if x, ok := parse(n.field.Text()); ok {
		from = n.tidy(x)
	}
	n.take(from + delta*n.step)
}

func (n *InputNumber) key(event gfx.KeyEvent) gfx.EventResult {
	if n.readOnly || n.value == nil {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyUp:
		n.stepBy(1)
	case gfx.KeyDown:
		n.stepBy(-1)
	case gfx.KeyPageUp:
		n.stepBy(10)
	case gfx.KeyPageDown:
		n.stepBy(-10)
	case gfx.KeyHome:
		if math.IsInf(n.min, 0) {
			return gfx.EventResult{}
		}
		n.take(n.min)
	case gfx.KeyEnd:
		if math.IsInf(n.max, 0) {
			return gfx.EventResult{}
		}
		n.take(n.max)
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// View declares the field at c, showing and editing *value. label names it for
// assistive technology. The row is one control high; give it a Width. Declare the
// view on its own, not inside a Memo.
func (n *InputNumber) View(c ui.C, label string, value *float64) ui.Box {
	if n == nil {
		panic("inputnumber: View needs its InputNumber")
	}
	if value == nil {
		panic("inputnumber: View needs the number")
	}
	n.reported, n.changed = n.changed, false
	n.value = value
	if math.IsNaN(*value) {
		*value = 0
	}
	*value = n.tidy(*value)
	focused := n.focused
	// Out of the field, the text follows the number the page holds.
	if !n.shownSet || (!focused && *value != n.shown) {
		n.show(*value)
	}
	n.field.ReadOnly = n.readOnly
	row := ui.Row(c).Gap(4).Items(ui.Center)
	row.Children(func() {
		step := ui.Button(c, n.decrease).Icon(minusIcon).IconOnly().Outline().Disabled(n.readOnly || *value <= n.min)
		if step.Clicked() {
			n.stepBy(-1)
		}
		var options []ui.Option
		if !n.fieldID.IsZero() {
			options = n.fieldOpt[:]
		}
		field := ui.TextField(c, &n.field, label, n.hint, options...).Grow(1).TextCenter()
		field.OnKey(n.keyFn)
		if field.Submitted() {
			n.commit()
		}
		step = ui.Button(c, n.increase).Icon(plusIcon).IconOnly().Outline().Disabled(n.readOnly || *value >= n.max)
		if step.Clicked() {
			n.stepBy(1)
		}
		now := c.Canvas().Focused() == n.field.ID && !n.field.ID.IsZero()
		if n.focused && !now {
			n.commit()
		} else if now {
			// While it is edited, text that is not a number is marked.
			_, ok := parse(n.field.Text())
			n.field.Invalid = !ok && n.field.Text() != ""
		}
		n.focused = now
	})
	return row
}
