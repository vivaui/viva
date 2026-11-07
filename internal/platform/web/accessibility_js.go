//go:build js && wasm

package web

import (
	"strconv"
	"syscall/js"

	"github.com/vivaui/viva/ui"
)

type accessibilityDOMNode struct {
	element         js.Value
	description     accessibilityDescription
	listeners       []domListener
	actionListeners []domListener
	seen            uint64
	initialized     bool
	editing         bool
}

type accessibilityMirror struct {
	document         js.Value
	root             js.Value
	container        js.Value
	nodes            map[ui.ID]*accessibilityDOMNode
	order            []ui.ID
	orderScratch     []ui.ID
	generation       uint64
	activeDescendant string
	text             *textInputHost
	onPress          func(ui.ID)
	onFocus          func(ui.ID)
	onSetNumber      func(ui.ID, float64)
}

func newAccessibilityMirror(document, root, container js.Value, label string) *accessibilityMirror {
	root.Set("tabIndex", 0)
	root.Call("setAttribute", "role", "application")
	if label != "" {
		root.Call("setAttribute", "aria-label", label)
	}
	mirror := &accessibilityMirror{
		document:  document,
		root:      root,
		container: container,
		nodes:     make(map[ui.ID]*accessibilityDOMNode),
	}
	mirror.text = newTextInputHost(mirror)
	return mirror
}

// setActions connects the ARIA nodes to the input router: a screen reader's
// activation arrives as a DOM click, and its focus move as a DOM focus.
func (mirror *accessibilityMirror) setActions(press, focus func(ui.ID), setNumber func(ui.ID, float64)) {
	mirror.onPress, mirror.onFocus, mirror.onSetNumber = press, focus, setNumber
	for _, node := range mirror.nodes {
		node.unbindActions()
		mirror.bindActions(node)
	}
}

func (mirror *accessibilityMirror) bindActions(node *accessibilityDOMNode) {
	if len(node.actionListeners) != 0 ||
		(mirror.onPress == nil && mirror.onFocus == nil && mirror.onSetNumber == nil) {
		return
	}
	if mirror.onPress != nil {
		click := js.FuncOf(func(js.Value, []js.Value) any {
			mirror.onPress(node.description.ID)
			return nil
		})
		node.element.Call("addEventListener", "click", click)
		node.actionListeners = append(node.actionListeners, domListener{name: "click", function: click})
	}
	if mirror.onFocus != nil {
		focus := js.FuncOf(func(js.Value, []js.Value) any {
			mirror.onFocus(node.description.ID)
			return nil
		})
		node.element.Call("addEventListener", "focus", focus)
		node.actionListeners = append(node.actionListeners, domListener{name: "focus", function: focus})
	}
	if mirror.onSetNumber != nil && node.description.Element == "input" && node.description.Role == "slider" {
		input := js.FuncOf(func(js.Value, []js.Value) any {
			value, err := strconv.ParseFloat(node.element.Get("value").String(), 64)
			if err == nil {
				mirror.onSetNumber(node.description.ID, value)
			}
			return nil
		})
		node.element.Call("addEventListener", "input", input)
		node.actionListeners = append(node.actionListeners, domListener{name: "input", function: input})
	}
}

func (node *accessibilityDOMNode) unbindActions() {
	for _, listener := range node.actionListeners {
		node.element.Call("removeEventListener", listener.name, listener.function)
		listener.function.Release()
	}
	node.actionListeners = node.actionListeners[:0]
}

func (mirror *accessibilityMirror) sync(descriptions []accessibilityDescription, focused ui.ID) {
	mirror.generation++
	mirror.orderScratch = mirror.orderScratch[:0]
	orderChanged := false
	for _, description := range descriptions {
		node := mirror.nodes[description.ID]
		if node != nil && node.description.Element != description.Element {
			mirror.text.unbind(node)
			node.unbindActions()
			mirror.container.Call("removeChild", node.element)
			delete(mirror.nodes, description.ID)
			node = nil
			orderChanged = true
		}
		if node == nil {
			element := mirror.document.Call("createElement", description.Element)
			element.Set("id", description.DOMID)
			element.Set("tabIndex", -1)
			hideAccessibilityElement(element)
			node = &accessibilityDOMNode{element: element}
			mirror.nodes[description.ID] = node
			mirror.container.Call("appendChild", element)
		}
		applyAccessibilityDescription(node, description)
		// A slider's range listener depends on the element and role the
		// description names, so binding follows the description, not creation.
		mirror.bindActions(node)
		if description.Role == "textbox" || description.Role == "combobox" {
			mirror.text.bind(node)
		} else {
			mirror.text.unbind(node)
		}
		node.seen = mirror.generation
		mirror.orderScratch = append(mirror.orderScratch, description.ID)
	}

	for id, node := range mirror.nodes {
		if node.seen == mirror.generation {
			continue
		}
		mirror.text.unbind(node)
		node.unbindActions()
		mirror.container.Call("removeChild", node.element)
		delete(mirror.nodes, id)
	}

	if orderChanged || !sameAccessibilityOrder(mirror.order, descriptions) {
		for _, id := range mirror.orderScratch {
			mirror.container.Call("appendChild", mirror.nodes[id].element)
		}
	}
	mirror.order, mirror.orderScratch = mirror.orderScratch, mirror.order[:0]

	activeDescendant := accessibilityActiveDescendant(descriptions, focused)
	if activeDescendant == mirror.activeDescendant {
		return
	}
	if activeDescendant == "" {
		mirror.root.Call("removeAttribute", "aria-activedescendant")
	} else {
		mirror.root.Call("setAttribute", "aria-activedescendant", activeDescendant)
	}
	mirror.activeDescendant = activeDescendant
}

func applyAccessibilityDescription(node *accessibilityDOMNode, description accessibilityDescription) {
	before := node.description
	initial := !node.initialized
	if initial || before.Role != description.Role {
		node.element.Call("setAttribute", "role", description.Role)
	}
	if initial || before.Label != description.Label {
		setOptionalAttribute(node.element, "aria-label", description.Label)
	}
	if initial || before.Role != description.Role || before.Value != description.Value {
		if description.Element == "input" || description.Element == "textarea" {
			if !node.editing && !(description.Role == "slider" && description.Range.Set) {
				node.element.Set("value", description.Value)
			}
		} else {
			node.element.Set("textContent", description.Value)
		}
		if description.Role == "slider" {
			setOptionalAttribute(node.element, "aria-valuetext", description.Value)
		} else {
			node.element.Call("removeAttribute", "aria-valuetext")
		}
	}
	if initial || before.Description != description.Description {
		setOptionalAttribute(node.element, "aria-description", description.Description)
	}
	if initial || before.Level != description.Level {
		setNumberAttribute(node.element, "aria-level", float64(description.Level), description.Level > 0)
	}
	if initial || before.PosInSet != description.PosInSet {
		setNumberAttribute(node.element, "aria-posinset", float64(description.PosInSet), description.PosInSet > 0)
	}
	if initial || before.SetSize != description.SetSize {
		setNumberAttribute(node.element, "aria-setsize", float64(description.SetSize), description.SetSize > 0)
	}
	if initial || before.Live != description.Live {
		setOptionalAttribute(node.element, "aria-live", description.Live)
	}
	if initial || before.Range != description.Range {
		rangeSet := description.Range.Set
		setNumberAttribute(node.element, "aria-valuenow", description.Range.Now, rangeSet)
		setNumberAttribute(node.element, "aria-valuemin", description.Range.Min, rangeSet)
		setNumberAttribute(node.element, "aria-valuemax", description.Range.Max, rangeSet)
		if description.Role == "slider" && rangeSet {
			applySliderRange(node.element, description.Range)
		}
	}
	if initial || before.Disabled != description.Disabled {
		setBooleanState(node.element, "aria-disabled", description.Disabled, description.Disabled)
	}
	if initial || before.Hidden != description.Hidden {
		setBooleanState(node.element, "aria-hidden", description.Hidden, description.Hidden)
	}

	if initial || before.Role != description.Role || before.Checked != description.Checked || before.Mixed != description.Mixed || before.Selected != description.Selected {
		applySelectionStates(node.element, description)
	}
	if initial || before.Role != description.Role || before.Expanded != description.Expanded || before.Expandable != description.Expandable {
		supported := description.Role == "combobox" || description.Expandable
		setBooleanState(node.element, "aria-expanded", description.Expanded, supported)
	}
	if description.Element == "input" && description.Role != "slider" && (initial || before.Secure != description.Secure) {
		// A password input masks what the browser shows and keeps the real
		// text out of autofill-less copy and the accessibility value.
		if description.Secure {
			node.element.Set("type", "password")
		} else {
			node.element.Set("type", "text")
		}
	}
	if initial || before.Role != description.Role || before.Multiline != description.Multiline {
		setBooleanState(node.element, "aria-multiline", description.Multiline, description.Role == "textbox")
	}
	if initial || before.Role != description.Role || before.ReadOnly != description.ReadOnly {
		supported := description.Role == "textbox" || description.Role == "combobox" || description.Role == "slider"
		setBooleanState(node.element, "aria-readonly", description.ReadOnly, supported)
	}

	node.description = description
	node.initialized = true
}

// applySliderRange makes the mirrored input range carry the same bounds and
// step as the slider the canvas paints. The ARIA attributes are still set by
// applyAccessibilityDescription; these are what the browser's native range
// control and screen readers use to step and set the value.
func applySliderRange(element js.Value, value ui.ValueRange) {
	element.Set("type", "range")
	element.Set("min", value.Min)
	element.Set("max", value.Max)
	step := value.Step
	if step <= 0 {
		step = 1
	}
	element.Set("step", step)
	element.Set("value", min(max(value.Now, value.Min), value.Max))
}

func applySelectionStates(element js.Value, description accessibilityDescription) {
	checked := description.Checked || description.Selected
	switch description.Role {
	case "button":
		setBooleanState(element, "aria-pressed", checked, checked)
		setBooleanState(element, "aria-checked", false, false)
		setBooleanState(element, "aria-selected", false, false)
		setBooleanState(element, "aria-current", false, false)
	case "checkbox", "radio", "switch":
		if description.Mixed {
			element.Set("aria-checked", "mixed")
		} else {
			setBooleanState(element, "aria-checked", checked, true)
		}
		setBooleanState(element, "aria-pressed", false, false)
		setBooleanState(element, "aria-selected", false, false)
		setBooleanState(element, "aria-current", false, false)
	case "tab":
		setBooleanState(element, "aria-selected", description.Selected, true)
		setBooleanState(element, "aria-pressed", false, false)
		setBooleanState(element, "aria-checked", false, false)
		setBooleanState(element, "aria-current", false, false)
	case "menuitem":
		setBooleanState(element, "aria-checked", description.Checked, description.Checked)
		setBooleanState(element, "aria-current", description.Selected, description.Selected)
		setBooleanState(element, "aria-pressed", false, false)
		setBooleanState(element, "aria-selected", false, false)
	default:
		setBooleanState(element, "aria-current", description.Selected, description.Selected)
		setBooleanState(element, "aria-pressed", false, false)
		setBooleanState(element, "aria-checked", false, false)
		setBooleanState(element, "aria-selected", false, false)
	}
}

func setOptionalAttribute(element js.Value, name, value string) {
	if value == "" {
		element.Call("removeAttribute", name)
		return
	}
	element.Call("setAttribute", name, value)
}

func setNumberAttribute(element js.Value, name string, value float64, present bool) {
	if !present {
		element.Call("removeAttribute", name)
		return
	}
	element.Call("setAttribute", name, strconv.FormatFloat(value, 'f', -1, 64))
}

func setBooleanState(element js.Value, name string, value, present bool) {
	if !present {
		element.Call("removeAttribute", name)
		return
	}
	element.Call("setAttribute", name, boolString(value))
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
