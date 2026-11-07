package app

import (
	"math"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/interaction"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// accessibilityHandler is the window's answer to what assistive technology asks
// of a node. A window with nothing to paint has no nodes to ask about.
func (painter *painter) accessibilityHandler(config *Config) platform.AccessibilityHandler {
	if config.Paint == nil {
		return nil
	}
	return painter.accessibilityRequest
}

// accessibleHit finds the control a request names. Labels share their control's
// ID and are not controls, and a region declared by Container has no ID.
func (painter *painter) accessibleHit(id ui.ID) (ui.Hit, bool) {
	return interaction.Control(painter.canvas.Hits(), id)
}

// accessibilityRequest carries out one request the way the keyboard or the
// mouse would, through the same paths, so a control cannot tell assistive
// technology from a person.
func (painter *painter) accessibilityRequest(request platform.AccessibilityRequest, present func(platform.Frame) error) (bool, error) {
	hit, ok := painter.accessibleHit(request.Node)
	if !ok || (hit.Disabled && !(request.Action == platform.AccessibilityFocus && hit.FocusableDisabled)) {
		return false, nil
	}
	showFrame := func() error { return painter.presentRepaint(present) }
	switch request.Action {
	case platform.AccessibilityFocus:
		if !hit.Focusable {
			return false, nil
		}
		painter.focusHit(hit.ID)
		return true, showFrame()

	case platform.AccessibilityPress:
		if hit.Focusable {
			painter.focusHit(hit.ID)
		}
		if hit.Role == ui.RoleTextField || hit.Role == ui.RoleTerminal {
			// Pressing a field is putting the caret in it.
			return true, showFrame()
		}
		if !painter.deliverClick(hit.ID) {
			return false, nil
		}
		return true, showFrame()

	case platform.AccessibilityIncrement, platform.AccessibilityDecrement:
		if hit.Role != ui.RoleSlider || !hit.Focusable {
			return false, nil
		}
		key := input.KeyUp
		if request.Action == platform.AccessibilityDecrement {
			key = input.KeyDown
		}
		return true, painter.pressKeys(hit.ID, key, 1, present)

	case platform.AccessibilitySetNumber:
		if hit.Role != ui.RoleSlider || !hit.Focusable || !hit.Range.Set || math.IsNaN(request.Number) {
			return false, nil
		}
		return true, painter.moveSlider(hit, request.Number, present)

	case platform.AccessibilitySetText, platform.AccessibilitySetSelection:
		if (hit.Role != ui.RoleTextField && hit.Role != ui.RoleCombobox) || hit.Secure || !hit.Focusable {
			return false, nil
		}
		if request.Action == platform.AccessibilitySetText && hit.ReadOnly {
			return false, nil
		}
		return painter.editText(hit, request, present)
	}
	return false, nil
}

// focusHit gives a control the keyboard, as a Tab onto it would.
func (painter *painter) focusHit(id ui.ID) {
	painter.applyEventResult(ui.EventResult{Focus: id, FocusVisible: true})
}

// pressKeys focuses a control and presses a key on it count times, presenting
// each repaint.
func (painter *painter) pressKeys(id ui.ID, key input.Key, count int, present func(platform.Frame) error) error {
	painter.focusHit(id)
	for index := 0; index < count; index++ {
		frame, err := painter.key(platform.KeyEvent{Key: key})
		if err != nil {
			return err
		}
		if err := present(frame); err != nil {
			return err
		}
	}
	if count == 0 {
		return painter.presentRepaint(present)
	}
	return nil
}

// moveSlider brings a slider to a number with the keys the slider answers:
// Home and End for the ends and arrow steps between.
func (painter *painter) moveSlider(hit ui.Hit, target float64, present func(platform.Frame) error) error {
	key, steps := interaction.SliderSteps(hit.Range, target)
	if key == ui.KeyUnknown {
		return nil
	}
	if steps == 0 {
		return painter.presentRepaint(present)
	}
	return painter.pressKeys(hit.ID, input.Key(key), steps, present)
}

// editText replaces the text of the focused field, or moves its selection, with
// the events the platform's own text service would send.
func (painter *painter) editText(hit ui.Hit, request platform.AccessibilityRequest, present func(platform.Frame) error) (bool, error) {
	painter.focusHit(hit.ID)
	if err := painter.presentRepaint(present); err != nil {
		return false, err
	}
	if painter.activeText == nil || painter.focus.Focused() != hit.ID {
		return false, nil // the application gave the field no native editor
	}
	length := utf8.RuneCountInString(painter.activeText.State.Value)
	event := platform.TextEvent{Kind: platform.TextSetSelection}
	switch request.Action {
	case platform.AccessibilitySetText:
		event = platform.TextEvent{
			Kind: platform.TextInsert, Text: request.Text,
			Replacement: text.Range{Start: 0, End: length}, HasReplacement: true,
		}
	default:
		start, end := int(request.Start), int(request.End)
		start, end = max(0, min(start, length)), max(0, min(end, length))
		event.Selection = text.Range{Start: start, End: end}
	}
	if _, err := painter.applyTextEvent(event); err != nil {
		return false, err
	}
	// The platform editor did not make this edit and so does not know of it:
	// have the next frame hand it the new state.
	painter.textEnabled = false
	return true, painter.presentRepaint(present)
}

func (painter *painter) presentRepaint(present func(platform.Frame) error) error {
	frame, err := painter.repaint()
	if err != nil {
		return err
	}
	return present(frame)
}
