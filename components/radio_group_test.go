package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/ui"
)

var radioGroupTestOptions = [...]RadioOption{
	{ID: ui.Item(21), Label: "Default"},
	{ID: ui.Item(22), Label: "Comfortable"},
	{ID: ui.Item(23), Label: "Compact", Disabled: true},
}

func TestRadioGroupPaintsOfficialSelectionAndTypedDisabledHit(t *testing.T) {
	theme := ui.LightTheme()
	var canvas ui.Canvas
	BeginRadioGroup(&canvas, theme, ui.Rect{Width: 260}, radioGroupTestOptions[:], ui.Item(22)).Paint()
	if len(canvas.Rectangles()) != 9 || len(canvas.Texts()) != 3 {
		t.Fatalf("radio commands = %d rectangles, %d texts", len(canvas.Rectangles()), len(canvas.Texts()))
	}
	if canvas.Rectangles()[2].Color.Alpha != 0 {
		t.Fatalf("unselected indicator is visible: %+v", canvas.Rectangles()[2])
	}
	if canvas.Rectangles()[3].Color != theme.Primary || canvas.Rectangles()[5].Color != theme.PrimaryForeground {
		t.Fatalf("selected radio colors = %+v", canvas.Rectangles()[3:6])
	}
	if len(canvas.Hits()) != 3 || canvas.Hits()[1].Role != ui.RoleRadio || !canvas.Hits()[1].Checked || !canvas.Hits()[1].Selected || !canvas.Hits()[2].Disabled {
		t.Fatalf("radio hits = %+v", canvas.Hits())
	}
}

func TestRadioGroupHorizontalStableAndAllocationFree(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		BeginRadioGroup(&canvas, theme, ui.Rect{Width: 360, Height: 32}, radioGroupTestOptions[:], ui.Item(21)).Horizontal().Paint()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical radio groups produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("radio group paint allocations = %v", allocations)
	}
}

func TestRadioGroupInvalidUsesDangerTreatment(t *testing.T) {
	theme := ui.LightTheme()
	options := [...]RadioOption{{ID: ui.Item(24), Label: "Invalid", Invalid: true}}
	var canvas ui.Canvas
	BeginRadioGroup(&canvas, theme, ui.Rect{Width: 160}, options[:], ui.ID{}).Paint()
	rectangles := canvas.Rectangles()
	if rectangles[0].Color != invalidRing(&theme) || rectangles[1].Color != invalidBorder(&theme) {
		t.Fatalf("invalid radio commands = %+v, want the invalid ring and a danger border", rectangles)
	}
}

func TestRadioGroupClickedReportsTheClickedOption(t *testing.T) {
	var canvas ui.Canvas
	canvas.BeginEventPass(radioGroupTestOptions[1].ID)
	defer canvas.EndEventPass()
	id, ok := BeginRadioGroup(&canvas, ui.LightTheme(), ui.Rect{Width: 260}, radioGroupTestOptions[:], radioGroupTestOptions[0].ID).Paint().Clicked()
	if !ok || id != radioGroupTestOptions[1].ID {
		t.Fatalf("Clicked() = (%v, %v)", id, ok)
	}
}

func TestRadioGroupKeyDown(t *testing.T) {
	options := [...]RadioOption{
		{ID: ui.Item(101)},
		{ID: ui.Item(102), Disabled: true},
		{},
		{ID: ui.Item(104)},
		{ID: ui.Item(105)},
	}
	tests := []struct {
		name      string
		current   uint32
		direction Direction
		key       input.Key
		want      uint32
		handled   bool
	}{
		{name: "right skips disabled and zero", current: 101, key: input.KeyRight, want: 104, handled: true},
		{name: "left wraps", current: 101, key: input.KeyLeft, want: 105, handled: true},
		{name: "rtl right moves backward", current: 101, direction: RTL, key: input.KeyRight, want: 105, handled: true},
		{name: "rtl left moves forward", current: 101, direction: RTL, key: input.KeyLeft, want: 104, handled: true},
		{name: "up moves backward", current: 104, key: input.KeyUp, want: 101, handled: true},
		{name: "down wraps", current: 105, key: input.KeyDown, want: 101, handled: true},
		{name: "disabled current advances", current: 102, key: input.KeyDown, want: 104, handled: true},
		{name: "home is the page's", current: 105, key: input.KeyHome, want: 105},
		{name: "end is the page's", current: 101, key: input.KeyEnd, want: 101},
		{name: "missing current forward", current: 999, key: input.KeyRight, want: 101, handled: true},
		{name: "missing current backward", current: 999, key: input.KeyLeft, want: 105, handled: true},
		{name: "unowned key", current: 104, key: input.KeyEnter, want: 104},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, handled := RadioGroupKeyDown(ui.Item(int(test.current)), options[:], test.direction, test.key)
			if got != ui.Item(int(test.want)) || handled != test.handled {
				t.Fatalf("RadioGroupKeyDown() = (%d, %v), want (%d, %v)", got, handled, test.want, test.handled)
			}
		})
	}
}

func TestRadioGroupKeyDownNoAvailableOptionAndNoAllocations(t *testing.T) {
	options := [...]RadioOption{{ID: ui.Item(101), Disabled: true}, {}, {ID: ui.Item(103), Disabled: true}}
	if got, handled := RadioGroupKeyDown(ui.Item(77), options[:], LTR, input.KeyHome); got != ui.Item(77) || handled {
		t.Fatalf("all-disabled Home = (%d, %v), want (77, false)", got, handled)
	}
	if got, handled := RadioGroupKeyDown(ui.Item(77), nil, LTR, input.KeyDown); got != ui.Item(77) || !handled {
		t.Fatalf("empty Down = (%d, %v), want (77, true)", got, handled)
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		RadioGroupKeyDown(ui.Item(101), radioGroupTestOptions[:], RTL, input.KeyLeft)
	}); allocations != 0 {
		t.Fatalf("RadioGroupKeyDown allocations = %v", allocations)
	}
}

var radioGroupKeyDownResult ui.ID

func BenchmarkRadioGroupKeyDown(b *testing.B) {
	var result ui.ID
	b.ReportAllocs()
	for range b.N {
		result, _ = RadioGroupKeyDown(ui.Item(21), radioGroupTestOptions[:], LTR, input.KeyRight)
	}
	radioGroupKeyDownResult = result
}
