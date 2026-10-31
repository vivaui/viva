package gallery

import (
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// These tests operate the gallery through app.Driver: the real painter, input
// router and focus rules, fed explicit events. They prove that what a user does
// reaches the example that owns the control, not just that the canvas looks right.

// galleryConfig is the application as the launchers run it.
func galleryConfig(state *State) app.Config {
	var nativeInput app.TextInput
	return app.Config{
		Title: "gallery test", Width: DefaultWidth, Height: DefaultHeight,
		Tree:  Tree(),
		Paint: state.Paint, OnPointer: state.Pointer,
		OnScroll: state.Scroll, OnKey: state.Key,
		TextInput: func(context app.TextInputContext) *app.TextInput {
			editor, ok := state.TextEditor(context.ID)
			if !ok {
				return nil
			}
			nativeInput = app.TextInput{Editor: editor}
			return &nativeInput
		},
	}
}

func newDriver(t *testing.T, state *State) *app.Driver {
	t.Helper()
	driver, err := app.NewDriver(galleryConfig(state), nil)
	if err != nil {
		t.Fatalf("driver: %v", err)
	}
	t.Cleanup(driver.Close)
	return driver
}

// open navigates to a component the way a user does: scrolling the catalog
// until its card is on screen, then clicking it.
func open(t *testing.T, driver *app.Driver, title string) {
	t.Helper()
	var hit ui.Hit
	for step := 0; ; step++ {
		for _, candidate := range driver.Snapshot().Hits {
			if !candidate.Block && candidate.Label == title && isCard(candidate.ID) {
				hit = candidate
			}
		}
		if !hit.ID.IsZero() {
			break
		}
		if step > 60 {
			t.Fatalf("the %q card never came into view", title)
		}
		if err := driver.Scroll(ui.ScrollEvent{X: 2, Y: toolbarHeight + 4, DeltaY: 240, Precise: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatalf("click %q: %v", title, err)
	}
}

func findLabel(t *testing.T, driver *app.Driver, label string) ui.Hit {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if !hit.Block && hit.Label == label {
			return hit
		}
	}
	t.Fatalf("no control labelled %q on screen", label)
	return ui.Hit{}
}

func labelsNamed(driver *app.Driver, label string) []ui.Hit {
	var found []ui.Hit
	for _, hit := range driver.Snapshot().Hits {
		if !hit.Block && hit.Label == label {
			found = append(found, hit)
		}
	}
	return found
}

func clickHit(t *testing.T, driver *app.Driver, hit ui.Hit) {
	t.Helper()
	if err := driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatalf("click %q: %v", hit.Label, err)
	}
}

func countText(driver *app.Driver, value string) int {
	count := 0
	for _, text := range driver.Snapshot().Texts {
		if text.Value == value {
			count++
		}
	}
	return count
}

func TestClickingACardOpensTheComponentWithAllItsExamples(t *testing.T) {
	state := New()
	driver := newDriver(t, state)
	open(t, driver, "Button")
	if state.page == 0 || catalog[state.page-1].ID != "button" {
		t.Fatalf("the Button card opened page %d", state.page)
	}
	for _, example := range catalog[state.page-1].Examples {
		if countText(driver, example.Title) == 0 {
			t.Errorf("example %q is not on screen", example.Title)
		}
	}
	// Every example is live from the start.
	if countText(driver, "点击：0") != 3 {
		t.Fatalf("expected three live counters, found %d", countText(driver, "点击：0"))
	}
}

func TestClickReachesOnlyTheExampleThatOwnsTheControl(t *testing.T) {
	state := New()
	driver := newDriver(t, state)
	open(t, driver, "Button")

	clickHit(t, driver, findLabel(t, driver, "Click me"))
	if countText(driver, "点击：1") != 1 || countText(driver, "点击：0") != 2 {
		t.Fatalf("one click changed %d counters (want exactly the first example's)", countText(driver, "点击：1"))
	}

	// The variants example reuses the very same local control IDs.
	variants := labelsNamed(driver, "Default")
	if len(variants) == 0 {
		t.Fatal("the variants example has no Default button")
	}
	clickHit(t, driver, variants[0])
	if countText(driver, "点击：1") != 2 || countText(driver, "点击：0") != 1 {
		t.Fatalf("clicking the second example's button: %d at 1, %d at 0", countText(driver, "点击：1"), countText(driver, "点击：0"))
	}
	if countText(driver, "点击：2") != 0 {
		t.Fatal("a click was delivered to more than one example")
	}
}

func TestHoverReachesTheExampleUnderThePointer(t *testing.T) {
	state := New()
	driver := newDriver(t, state)
	open(t, driver, "Button")
	button := findLabel(t, driver, "Click me")

	fill := func() ui.Color {
		for _, primitive := range driver.Snapshot().Primitives {
			if primitive.Bounds == button.Bounds && primitive.Kind == app.RectanglePrimitive {
				return primitive.Color
			}
		}
		t.Fatal("the button surface was not painted")
		return ui.Color{}
	}
	idle := fill()
	if err := driver.Move(button.Bounds.X+4, button.Bounds.Y+4); err != nil {
		t.Fatal(err)
	}
	if hovered := driver.Snapshot().Hovered; hovered != button.ID {
		t.Fatalf("hovered %v, want the button %v", hovered, button.ID)
	}
	if fill() == idle {
		t.Fatal("the example did not paint its hover state")
	}
}

func TestKeyboardActivatesTheFocusedControlOfAnExample(t *testing.T) {
	state := New()
	driver := newDriver(t, state)
	open(t, driver, "Button")

	reached := false
	for press := 0; press < 30 && !reached; press++ {
		if err := driver.Key(ui.KeyTab, 0); err != nil {
			t.Fatal(err)
		}
		slot, _, ok := state.decodeExample(driver.Snapshot().Focused)
		reached = ok && slot == 0
	}
	if !reached {
		t.Fatal("Tab never reached the first example's control")
	}
	if err := driver.Key(ui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if countText(driver, "点击：1") != 1 {
		t.Fatalf("Enter on the focused control produced %d activations", countText(driver, "点击：1"))
	}
}

func TestEditingATextFieldGoesThroughTheNativeTextPath(t *testing.T) {
	state := New()
	state.EnableTextEditing()
	driver := newDriver(t, state)
	open(t, driver, "Input")

	var field ui.Hit
	for _, hit := range driver.Snapshot().Hits {
		if hit.Role == ui.RoleTextField && !hit.Disabled && !hit.ReadOnly {
			field = hit
			break
		}
	}
	if field.ID.IsZero() {
		t.Fatal("the input page has no editable field")
	}
	clickHit(t, driver, field)
	if driver.Snapshot().Focused != field.ID {
		t.Fatalf("clicking the field focused %v, want %v", driver.Snapshot().Focused, field.ID)
	}
	editor, ok := state.TextEditor(field.ID)
	if !ok {
		t.Fatal("the focused field has no editor behind it")
	}
	if err := driver.Type("你好 world"); err != nil {
		t.Fatalf("type: %v", err)
	}
	if !strings.Contains(editor.String(), "你好 world") {
		t.Fatalf("the editor holds %q after typing", editor.String())
	}
	if !strings.Contains(editor.String(), "你好 world") || countText(driver, editor.String()) == 0 && driver.Snapshot().Editor == nil {
		t.Fatal("the typed text is neither painted nor handed to the platform editor")
	}

	// A second input example is a separate editor, not the same one.
	slot, local, _ := state.decodeExample(field.ID)
	other, ok := state.TextEditor(tree.MountID(exampleMount(slot+1), local))
	if ok && other == editor {
		t.Fatal("two examples share one editor")
	}
}

func TestDraggingASliderTranslatesThePointerIntoTheExample(t *testing.T) {
	state := New()
	driver := newDriver(t, state)
	open(t, driver, "Slider")

	// The price-range example sits in the page's second column, hundreds of
	// pixels from the origin, so a pointer that was not translated into the
	// example's own coordinates lands far past the end of its track.
	lower, upper := findLabel(t, driver, "最低价格"), findLabel(t, driver, "最高价格")
	center := func(hit ui.Hit) (float32, float32) {
		return hit.Bounds.X + hit.Bounds.Width/2, hit.Bounds.Y + hit.Bounds.Height/2
	}
	lowerX, y := center(lower)
	upperX, _ := center(upper)
	if lowerX < 500 {
		t.Fatalf("the range example is at x=%v; expected it far from the page origin", lowerX)
	}
	midway := (lowerX + upperX) / 2 // the middle of the track: 50 on a 0-100 slider

	if err := driver.Press(lowerX, y, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(midway, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(midway, y, ui.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	got := findLabel(t, driver, "最低价格").Value
	if got != "45" && got != "50" && got != "55" {
		t.Fatalf("dragging the lower thumb to the middle of the track set it to %q, want about 50", got)
	}
	if upperValue := findLabel(t, driver, "最高价格").Value; upperValue != upper.Value {
		t.Fatalf("dragging one thumb moved the other: %q -> %q", upper.Value, upperValue)
	}
}

func TestWheelScrollsTheExampleUnderThePointerBeforeThePage(t *testing.T) {
	state := New()
	driver := newDriver(t, state)
	open(t, driver, "Scroll Area")

	viewport := findLabel(t, driver, "Scrollable activity")
	rowY := func() float32 {
		for _, text := range driver.Snapshot().Texts {
			if text.Value == "Window created" {
				return text.Bounds.Y
			}
		}
		t.Fatal("the first activity row is not painted")
		return 0
	}
	before := rowY()
	if err := driver.Move(viewport.Bounds.X+viewport.Bounds.Width/2, viewport.Bounds.Y+viewport.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	pageBefore := state.viewScroll
	if err := driver.Scroll(ui.ScrollEvent{
		X:      viewport.Bounds.X + viewport.Bounds.Width/2,
		Y:      viewport.Bounds.Y + viewport.Bounds.Height/2,
		DeltaY: 40, Precise: true,
	}); err != nil {
		t.Fatal(err)
	}
	if after := rowY(); after >= before {
		t.Fatalf("the wheel over a scroll area left its first row at y=%v (was %v)", after, before)
	}
	if state.viewScroll != pageBefore {
		t.Fatalf("the wheel moved the page (%v -> %v) although the example consumed it", pageBefore, state.viewScroll)
	}

	// Over blank page, the same gesture moves the page.
	if err := driver.Move(2, toolbarHeight+4); err != nil {
		t.Fatal(err)
	}
	if err := driver.Scroll(ui.ScrollEvent{X: 2, Y: toolbarHeight + 4, DeltaY: 40, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if state.scrollMax > 0 && state.viewScroll == pageBefore {
		t.Fatal("the wheel over empty page did not scroll the page")
	}
}

func TestAnOpenDialogDoesNotLockTheRestOfThePage(t *testing.T) {
	state := New()
	driver := newDriver(t, state)
	open(t, driver, "Dialog")

	// The confirm and destructive examples start with their dialogs open.
	if len(labelsNamed(driver, "Confirm"))+len(labelsNamed(driver, "Cancel")) == 0 {
		t.Fatal("no example shows an open dialog; the lock is not being exercised")
	}
	clickHit(t, driver, findLabel(t, driver, "Components"))
	if state.page != 0 {
		t.Fatalf("the toolbar was unreachable behind an open dialog: still on page %d", state.page)
	}
}

func TestDialogButtonsStillWorkInsideTheirStage(t *testing.T) {
	state := New()
	driver := newDriver(t, state)
	open(t, driver, "Dialog")
	confirm := labelsNamed(driver, "Confirm")
	if len(confirm) == 0 {
		t.Fatal("no open dialog with a Confirm button")
	}
	before := len(labelsNamed(driver, "Confirm"))
	clickHit(t, driver, confirm[0])
	if after := len(labelsNamed(driver, "Confirm")); after >= before {
		t.Fatalf("confirming did not close that dialog: %d -> %d", before, after)
	}
}

func TestEveryComponentPageSurvivesBeingOperated(t *testing.T) {
	// Click the center of every control of every example, Tab through each page,
	// and move the pointer over everything: no demo may panic when its events
	// arrive through the gallery.
	state := New()
	state.EnableTextEditing()
	driver := newDriver(t, state)
	for index := range catalog {
		state.openPage(index + 1)
		if err := driver.Resize(DefaultWidth, DefaultHeight); err != nil {
			t.Fatal(err)
		}
		for _, hit := range driver.Snapshot().Hits {
			if hit.Block || hit.Bounds.Width <= 0 {
				continue
			}
			if _, _, ok := state.decodeExample(hit.ID); !ok {
				continue
			}
			x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
			if y < toolbarHeight {
				continue
			}
			if err := driver.Move(x, y); err != nil {
				t.Fatalf("%s: move over %q: %v", catalog[index].ID, hit.Label, err)
			}
			if err := driver.Click(x, y); err != nil {
				t.Fatalf("%s: click %q: %v", catalog[index].ID, hit.Label, err)
			}
		}
		for press := 0; press < 6; press++ {
			if err := driver.Key(ui.KeyTab, 0); err != nil {
				t.Fatalf("%s: tab: %v", catalog[index].ID, err)
			}
		}
		if state.page != index+1 {
			t.Fatalf("%s: operating the page navigated away to page %d", catalog[index].ID, state.page)
		}
	}
}

// editableComponents are the components whose examples take keyboard input
// through the platform text service. The rest of the text-entry family is a
// fixed snapshot; this list failing means one of them stopped (or started)
// being editable and the gallery page should say so.
var editableComponents = []string{"code-diff", "field", "form", "input", "input-group", "input-otp", "textarea"}

func editableHits(driver *app.Driver) []ui.Hit {
	var fields []ui.Hit
	seen := map[ui.ID]bool{}
	for _, hit := range driver.Snapshot().Hits {
		if !hit.Block && hit.Role == ui.RoleTextField && !hit.Disabled && !hit.ReadOnly && !seen[hit.ID] {
			seen[hit.ID] = true
			fields = append(fields, hit)
		}
	}
	return fields
}

func TestEveryEditableFieldAcceptsKeyboardInput(t *testing.T) {
	state := New()
	state.EnableTextEditing()
	driver := newDriver(t, state)

	editable := map[string]bool{}
	for index := range catalog {
		state.openPage(index + 1)
		if err := driver.Resize(DefaultWidth, DefaultHeight); err != nil {
			t.Fatal(err)
		}
		for _, field := range editableHits(driver) {
			if field.Bounds.Y < toolbarHeight || field.Bounds.Y+field.Bounds.Height > DefaultHeight {
				continue // below the fold; its siblings cover the component
			}
			editable[catalog[index].ID] = true
			clickHit(t, driver, field)
			editor, ok := state.TextEditor(field.ID)
			if !ok {
				t.Fatalf("%s: %q looks editable but has no editor", catalog[index].ID, field.Label)
			}
			before := editor.String()
			if err := driver.Type("7"); err != nil {
				t.Fatalf("%s: typing into %q: %v", catalog[index].ID, field.Label, err)
			}
			if editor.String() == before {
				t.Fatalf("%s: %q ignored the keystroke (still %q)", catalog[index].ID, field.Label, before)
			}
		}
	}
	for _, id := range editableComponents {
		if !editable[id] {
			t.Errorf("%s has no field that takes keyboard input", id)
		}
	}
	for id := range editable {
		if !slices.Contains(editableComponents, id) {
			t.Errorf("%s is editable but not listed in editableComponents", id)
		}
	}
}

func TestReadOnlyAndDisabledFieldsRefuseKeyboardInput(t *testing.T) {
	state := New()
	state.EnableTextEditing()
	driver := newDriver(t, state)
	open(t, driver, "Input")
	refused := 0
	for _, hit := range driver.Snapshot().Hits {
		if hit.Block || hit.Role != ui.RoleTextField || (!hit.Disabled && !hit.ReadOnly) {
			continue
		}
		if hit.Bounds.Y < toolbarHeight || hit.Bounds.Y+hit.Bounds.Height > DefaultHeight {
			continue
		}
		clickHit(t, driver, hit)
		if err := driver.Type("x"); err == nil {
			t.Fatalf("%q (disabled=%v readonly=%v) accepted a keystroke", hit.Label, hit.Disabled, hit.ReadOnly)
		}
		refused++
	}
	if refused == 0 {
		t.Fatal("the input page shows no disabled or read-only field to check")
	}
}

func fieldNamed(t *testing.T, driver *app.Driver, label string) ui.Hit {
	t.Helper()
	for _, hit := range editableHits(driver) {
		if hit.Label == label {
			return hit
		}
	}
	t.Fatalf("no editable field labelled %q", label)
	return ui.Hit{}
}

func TestOTPTakesOnlyDigitsAndAtMostSix(t *testing.T) {
	state := New()
	state.EnableTextEditing()
	driver := newDriver(t, state)
	open(t, driver, "Input OTP")

	field := fieldNamed(t, driver, "Verification code")
	editor, _ := state.TextEditor(field.ID)
	if editor.String() != "482" {
		t.Fatalf("the code starts as %q, want 482", editor.String())
	}
	typeAndExpect := func(input, want string) {
		t.Helper()
		if err := driver.Type(input); err != nil {
			t.Fatalf("type %q: %v", input, err)
		}
		if got := editor.String(); got != want {
			t.Fatalf("after typing %q the code is %q, want %q", input, got, want)
		}
	}

	// Click in the empty slot after the code: the caret goes to the end.
	layout := slotCenters(driver, field)
	if err := driver.Click(layout[5], field.Bounds.Y+field.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	typeAndExpect("9", "4829")
	typeAndExpect("ab -!", "4829") // not digits
	typeAndExpect("12345678", "482912")

	typeAndExpect("5", "482915") // a complete code: the last digit is replaced

	if err := driver.Key(ui.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "48291" {
		t.Fatalf("Backspace left %q", editor.String())
	}
	if err := driver.Key(ui.KeyLeft, 0); err != nil {
		t.Fatal(err)
	}
	typeAndExpect("0", "48290") // typed over the last digit, which Left put the caret on

	// The slots paint what the editor holds, one digit each.
	for _, digit := range []string{"4", "8", "2", "9", "0"} {
		if countText(driver, digit) == 0 {
			t.Errorf("slot digit %q is not painted", digit)
		}
	}
}

// slotCenters returns the window x of each of the six slot centers of an OTP
// field, from its hit.
func slotCenters(driver *app.Driver, field ui.Hit) [6]float32 {
	const slot, separator = 32, 32
	var centers [6]float32
	for index := range centers {
		centers[index] = field.Bounds.X + float32(index)*slot + float32(index/3)*separator + slot/2
	}
	return centers
}

func TestTypingInTheMiddleOfAnOTPFillsSlotsInTurnAndAdvances(t *testing.T) {
	state := New()
	state.EnableTextEditing()
	driver := newDriver(t, state)
	open(t, driver, "Input OTP")
	field := fieldNamed(t, driver, "Verification code")
	editor, _ := state.TextEditor(field.ID)

	// Fill the code, then go back to the first slot.
	if err := driver.Click(slotCenters(driver, field)[5], field.Bounds.Y+field.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("901"); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "482901" {
		t.Fatalf("setup left %q", editor.String())
	}
	y := field.Bounds.Y + field.Bounds.Height/2
	if err := driver.Click(slotCenters(driver, field)[0], y); err != nil {
		t.Fatal(err)
	}
	if selection := editor.Selection(); selection.Start != 0 || selection.End != 0 {
		t.Fatalf("clicking slot 0 left the selection at %+v, want a caret there", selection)
	}

	// Each digit fills the slot the caret is on and the caret moves to the next
	// one, so a run of digits lands in consecutive slots instead of pushing the
	// rest of the code along.
	for step, digit := range []string{"1", "2", "3"} {
		if err := driver.Type(digit); err != nil {
			t.Fatal(err)
		}
		if got, want := editor.Selection().Start, step+1; got != want {
			t.Fatalf("after digit number %d the caret is on slot %d, want %d", step+1, got, want)
		}
	}
	if editor.String() != "123901" {
		t.Fatalf("three digits typed from the first slot gave %q, want 123901", editor.String())
	}
	// The focus border on screen follows: the accent-colored border now sits on slot 3.
	theme := state.Theme()
	accent := theme.Primary.Over(theme.Background)
	ring := 0
	for _, primitive := range driver.Snapshot().Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == accent && primitive.Layer == 0 {
			slot := slotCenters(driver, field)[3]
			if primitive.Bounds.X <= slot && slot <= primitive.Bounds.X+primitive.Bounds.Width && primitive.Bounds.Y <= y && y <= primitive.Bounds.Y+primitive.Bounds.Height {
				ring++
			}
		}
	}
	if ring == 0 {
		t.Fatal("the focus border did not follow the caret to slot 3")
	}
}

func TestTextareaTakesMultilineInput(t *testing.T) {
	state := New()
	state.EnableTextEditing()
	driver := newDriver(t, state)
	open(t, driver, "Textarea")
	field := fieldNamed(t, driver, "项目说明")
	clickHit(t, driver, field)
	editor, _ := state.TextEditor(field.ID)
	if err := driver.Key(ui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("第三行"); err != nil {
		t.Fatal(err)
	}
	if got := editor.String(); !strings.HasSuffix(got, "\n第三行") || strings.Count(got, "\n") != 2 {
		t.Fatalf("Enter did not start a new line in the textarea: %q", got)
	}
}

func TestInputGroupClearButtonEmptiesAndRefillsTheEditor(t *testing.T) {
	state := New()
	state.EnableTextEditing()
	driver := newDriver(t, state)
	open(t, driver, "Input Group")
	field := fieldNamed(t, driver, "Project domain")
	editor, _ := state.TextEditor(field.ID)
	if editor.String() != "viva.dev" {
		t.Fatalf("the domain starts as %q", editor.String())
	}
	clickHit(t, driver, findLabel(t, driver, "Clear domain"))
	if editor.String() != "" {
		t.Fatalf("Clear left %q", editor.String())
	}
	clickHit(t, driver, findLabel(t, driver, "Clear domain"))
	if editor.String() != "viva.dev" {
		t.Fatalf("Clear on an empty field did not restore the example text: %q", editor.String())
	}
}

func TestValidationReactsToWhatIsTyped(t *testing.T) {
	state := New()
	state.EnableTextEditing()
	driver := newDriver(t, state)

	open(t, driver, "Field")
	if countText(driver, "Enter a valid work email.") != 1 {
		t.Fatal("the invalid Field example shows no error before anything is typed")
	}
	// The invalid example is the third one on the page.
	var invalid ui.Hit
	for _, hit := range editableHits(driver) {
		if hit.Label == "Work email" && hit.Value == "team" {
			invalid = hit
		}
	}
	if invalid.ID.IsZero() {
		t.Fatal("the invalid Field example's input was not found")
	}
	clickHit(t, driver, invalid)
	if err := driver.Type("@viva.dev"); err != nil {
		t.Fatal(err)
	}
	if countText(driver, "Enter a valid work email.") != 0 {
		t.Fatal("a valid address left the error on screen")
	}

	click(state, hitPageCatalog)
	open(t, driver, "Form")
	if countText(driver, "Use at least three characters.") != 1 {
		t.Fatal("the invalid Form example shows no error before anything is typed")
	}
	for _, hit := range editableHits(driver) {
		if hit.Value == "go" {
			clickHit(t, driver, hit)
		}
	}
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if countText(driver, "Use at least three characters.") != 0 {
		t.Fatal("a long enough username left the error on screen")
	}
}

func isCard(id ui.ID) bool { _, ok := cardOf(id); return ok }
