//go:build js && wasm

package web

import (
	"fmt"
	"syscall/js"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type domListener struct {
	name     string
	function js.Func
}

type textInputHost struct {
	mirror        *accessibilityMirror
	active        *accessibilityDOMNode
	input         TextInput
	composing     bool
	syncing       bool
	suppressInput string
	onChange      func()
}

func newTextInputHost(mirror *accessibilityMirror) *textInputHost {
	document := js.Global().Get("document")
	const styleID = "viva-text-input-overlay-style"
	if document.Call("getElementById", styleID).IsNull() {
		style := document.Call("createElement", "style")
		style.Set("id", styleID)
		style.Set("textContent", `textarea[data-viva-hide-native-overlays="true"]::selection {
			background: transparent;
			color: transparent;
		}`)
		document.Get("head").Call("appendChild", style)
	}
	return &textInputHost{mirror: mirror}
}

func (host *textInputHost) bind(node *accessibilityDOMNode) {
	if len(node.listeners) != 0 {
		return
	}
	host.addListener(node, "input", host.inputEvent)
	host.addListener(node, "paste", host.pasteEvent)
	host.addListener(node, "select", host.selectEvent)
	host.addListener(node, "compositionstart", host.compositionStart)
	host.addListener(node, "compositionupdate", host.compositionUpdate)
	host.addListener(node, "compositionend", host.compositionEnd)
}

func (host *textInputHost) addListener(node *accessibilityDOMNode, name string, callback func(*accessibilityDOMNode, js.Value)) {
	function := js.FuncOf(func(_ js.Value, arguments []js.Value) any {
		if len(arguments) != 0 {
			callback(node, arguments[0])
		}
		return nil
	})
	node.listeners = append(node.listeners, domListener{name: name, function: function})
	node.element.Call("addEventListener", name, function)
}

func (host *textInputHost) unbind(node *accessibilityDOMNode) {
	if host.active == node {
		host.deactivate(false)
	}
	for _, listener := range node.listeners {
		node.element.Call("removeEventListener", listener.name, listener.function)
		listener.function.Release()
	}
	node.listeners = node.listeners[:0]
}

func (host *textInputHost) sync(id ui.ID, input *TextInput, onChange func()) {
	host.onChange = onChange
	if id.IsZero() || input == nil {
		host.deactivate(true)
		return
	}
	node := host.mirror.nodes[id]
	if node == nil || node.description.Role != "textbox" {
		host.deactivate(true)
		return
	}
	if host.active != node {
		host.deactivate(false)
		host.active = node
		node.editing = true
		host.composing = false
		host.suppressInput = ""
	}
	host.input = *input
	host.show(node.element)
	if !host.composing {
		host.syncValueAndSelection(node.element)
	}
	if !host.mirror.document.Get("activeElement").Equal(node.element) {
		host.syncing = true
		node.element.Call("focus", map[string]any{"preventScroll": true})
		host.syncing = false
	}
}

func (host *textInputHost) deactivate(focusRoot bool) {
	if host.active == nil {
		return
	}
	node := host.active
	host.syncing = true
	node.element.Set("value", host.input.State.Value)
	hideAccessibilityElement(node.element)
	node.editing = false
	if host.mirror.document.Get("activeElement").Equal(node.element) {
		node.element.Call("blur")
		if focusRoot {
			host.mirror.root.Call("focus", map[string]any{"preventScroll": true})
		}
	}
	host.syncing = false
	host.active = nil
	host.composing = false
	host.suppressInput = ""
}

func (host *textInputHost) activeElement(element js.Value) bool {
	return host.active != nil && host.active.element.Equal(element)
}

func (host *textInputHost) show(element js.Value) {
	style := element.Get("style")
	style.Set("position", "absolute")
	style.Set("left", pixel(host.input.X))
	style.Set("top", pixel(host.input.Y))
	style.Set("width", pixel(max(host.input.Width, 1)))
	style.Set("height", pixel(max(host.input.Height, 1)))
	style.Set("boxSizing", "border-box")
	style.Set("margin", "0")
	style.Set("padding", "0")
	style.Set("border", "0")
	style.Set("outline", "none")
	style.Set("background", "transparent")
	style.Set("color", cssPlatformColor(host.input.Color))
	caretColor := cssPlatformColor(host.input.Color)
	if host.input.HideNativeOverlays {
		caretColor = "transparent"
		element.Call("setAttribute", "data-viva-hide-native-overlays", "true")
	} else {
		element.Call("removeAttribute", "data-viva-hide-native-overlays")
	}
	style.Set("caretColor", caretColor)
	style.Set("font", cssFont(host.input.FontSize, 0))
	style.Set("opacity", "1")
	style.Set("overflow", "hidden")
	style.Set("clipPath", "none")
	style.Set("pointerEvents", "auto")
	style.Set("userSelect", "text")
	style.Set("zIndex", "1")
	if host.input.Multiline {
		style.Set("resize", "none")
		style.Set("whiteSpace", "pre-wrap")
		style.Set("lineHeight", "normal")
	} else {
		style.Set("lineHeight", pixel(max(host.input.Height, 1)))
	}
	element.Set("readOnly", false)
	element.Set("disabled", false)
}

func hideAccessibilityElement(element js.Value) {
	style := element.Get("style")
	style.Set("position", "absolute")
	style.Set("left", "0")
	style.Set("top", "0")
	style.Set("width", "1px")
	style.Set("height", "1px")
	style.Set("margin", "0")
	style.Set("padding", "0")
	style.Set("border", "0")
	style.Set("opacity", "0")
	style.Set("overflow", "hidden")
	style.Set("clipPath", "inset(50%)")
	style.Set("pointerEvents", "none")
	style.Set("userSelect", "none")
	style.Set("zIndex", "0")
}

func (host *textInputHost) syncValueAndSelection(element js.Value) {
	host.syncing = true
	if element.Get("value").String() != host.input.State.Value {
		element.Set("value", host.input.State.Value)
	}
	start, end := runeRangeToUTF16(host.input.State.Value, host.input.State.Selection)
	if element.Get("selectionStart").Int() != start || element.Get("selectionEnd").Int() != end {
		element.Call("setSelectionRange", start, end)
	}
	host.syncing = false
}

func (host *textInputHost) inputEvent(node *accessibilityDOMNode, event js.Value) {
	if host.syncing || host.active != node || host.composing || event.Get("isComposing").Bool() {
		return
	}
	value := node.element.Get("value").String()
	if host.suppressInput == value {
		host.suppressInput = ""
		return
	}
	changed := host.apply(inputTextEvent(host.input.State, value, event.Get("inputType").String()))
	if node.element.Get("value").String() == host.input.State.Value {
		changed = host.applySelection(node.element) || changed
	} else {
		// The editor kept something other than what the browser typed: it
		// restricted, capped or overwrote the edit. The editor is the authority,
		// so the element is made to mirror it again, caret included.
		host.syncValueAndSelection(node.element)
	}
	host.repaint(changed)
}

// pasteEvent reports a paste that carries no text. The browser inserts nothing
// for it and sends no input event, so the field would never hear of a picture
// that was pasted.
func (host *textInputHost) pasteEvent(node *accessibilityDOMNode, event js.Value) {
	if host.syncing || host.active != node || host.composing || host.input.Terminal {
		return
	}
	data := event.Get("clipboardData")
	if data.IsUndefined() || data.IsNull() || data.Call("getData", "text/plain").String() != "" {
		return
	}
	host.repaint(host.apply(platform.TextEvent{Kind: platform.TextPasteNoText}))
}

func (host *textInputHost) selectEvent(node *accessibilityDOMNode, _ js.Value) {
	if host.syncing || host.active != node || host.composing {
		return
	}
	host.repaint(host.applySelection(node.element))
}

func (host *textInputHost) compositionStart(node *accessibilityDOMNode, _ js.Value) {
	if host.syncing || host.active != node {
		return
	}
	host.composing = true
	host.suppressInput = ""
}

func (host *textInputHost) compositionUpdate(node *accessibilityDOMNode, event js.Value) {
	if host.syncing || host.active != node || !host.composing {
		return
	}
	value := event.Get("data").String()
	selection := host.domSelection(node.element)
	start := host.input.State.Selection.Start
	if host.input.State.Composing {
		start = host.input.State.Composition.Start
	}
	marked := normalizedTextRange(text.Range{Start: selection.Start - start, End: selection.End - start}, utf8.RuneCountInString(value))
	host.repaint(host.apply(platform.TextEvent{Kind: platform.TextSetMarked, Text: value, MarkedSelection: marked}))
}

func (host *textInputHost) compositionEnd(node *accessibilityDOMNode, event js.Value) {
	if host.syncing || host.active != node || !host.composing {
		return
	}
	changed := host.apply(platform.TextEvent{Kind: platform.TextInsert, Text: event.Get("data").String()})
	host.composing = false
	value := node.element.Get("value").String()
	if host.input.State.Value != value {
		changed = host.apply(replaceTextEvent(host.input.State.Value, value)) || changed
	}
	changed = host.applySelection(node.element) || changed
	host.suppressInput = value
	host.repaint(changed)
}

func (host *textInputHost) domSelection(element js.Value) text.Range {
	value := element.Get("value").String()
	return utf16RangeToRunes(value, element.Get("selectionStart").Int(), element.Get("selectionEnd").Int())
}

func (host *textInputHost) applySelection(element js.Value) bool {
	selection := host.domSelection(element)
	if selection == host.input.State.Selection {
		return false
	}
	return host.apply(platform.TextEvent{Kind: platform.TextSetSelection, Selection: selection})
}

func (host *textInputHost) apply(event platform.TextEvent) bool {
	if host.input.OnEvent == nil {
		return false
	}
	before := host.input.State
	next, err := host.input.OnEvent(event)
	if err != nil {
		return false
	}
	host.input.State = next
	return next != before
}

func (host *textInputHost) repaint(changed bool) {
	if changed && host.onChange != nil {
		host.onChange()
	}
}

func pixel(value float32) string {
	return fmt.Sprintf("%gpx", value)
}

func cssPlatformColor(color platform.Color) string {
	return cssColor(ui.Color{
		Red: float32(color.Red), Green: float32(color.Green),
		Blue: float32(color.Blue), Alpha: float32(color.Alpha),
	})
}
