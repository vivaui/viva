//go:build js && wasm

package main

import (
	"net/url"
	"strings"
	"syscall/js"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/componentdemo"
	"github.com/vivaui/viva/internal/previewmeasure"
)

func main() {
	parameters, err := url.ParseQuery(strings.TrimPrefix(js.Global().Get("location").Get("search").String(), "?"))
	if err != nil {
		reportPreview("viva-preview-error", "Invalid preview parameters")
		return
	}
	component := parameters.Get("component")
	if component == "" {
		component = "button"
	}
	locale := parameters.Get("locale")
	if locale != "" && locale != "zh" && locale != "en" {
		reportPreview("viva-preview-error", "Unknown preview language")
		return
	}
	var state *componentdemo.State
	if locale == "en" {
		state, err = componentdemo.NewEnglish(component, parameters.Get("example"))
	} else {
		state, err = componentdemo.New(component, parameters.Get("example"))
	}
	if err != nil {
		reportPreview("viva-preview-error", err.Error())
		return
	}
	theme := parameters.Get("theme")
	if theme != "" && theme != "light" && theme != "dark" {
		reportPreview("viva-preview-error", "Unknown appearance")
		return
	}
	state.SetDark(theme == "dark")
	state.EnableTextEditing()
	appearance := gfx.LightTheme()
	if theme == "dark" {
		appearance = gfx.DarkTheme()
	}
	var previewInput app.TextInput
	resolveInput := func(context app.TextInputContext) *app.TextInput {
		editor, supported := state.Editor(context.ID)
		if !supported {
			return nil
		}
		previewInput = app.TextInput{Editor: editor}
		return &previewInput
	}
	ready := false
	lastHeight := 0
	var forward func(deltaX, deltaY float32)
	embedded := !js.Global().Get("parent").Equal(js.Global())
	if embedded {
		forward = forwardWheel
	}
	err = app.RunCanvas(app.Config{
		Title: "Viva · " + component + " · Web preview", Width: 760, Height: 380,
		ClearColor: app.Color{
			Red: float64(appearance.Background.Red), Green: float64(appearance.Background.Green),
			Blue: float64(appearance.Background.Blue), Alpha: 1,
		},
		Tree: componentdemo.Tree(),
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			state.Paint(canvas, size)
			if height := previewmeasure.Content(canvas, size); embedded && height > 0 && height != lastHeight {
				lastHeight = height
				postPreviewHeight(height)
			}
			if !ready {
				ready = true
				reportPreview("viva-preview-ready", "")
			}
		},
		OnPointer: state.Pointer, OnKey: state.Key, OnScroll: state.Scroll,
		ForwardWheel: forward,
		TextInput:    resolveInput,
	}, nil)
	if err != nil {
		reportPreview("viva-preview-error", err.Error())
	}
}

func postPreviewHeight(height int) {
	window := js.Global()
	payload := window.Get("Object").New()
	payload.Set("type", "viva-preview-height")
	payload.Set("height", height)
	window.Get("parent").Call("postMessage", payload, window.Get("location").Get("origin").String())
}

// forwardWheel lets the documentation page scroll when the component did not
// use a wheel gesture.
func forwardWheel(deltaX, deltaY float32) {
	window := js.Global()
	payload := window.Get("Object").New()
	payload.Set("type", "viva-preview-wheel")
	payload.Set("deltaX", float64(deltaX))
	payload.Set("deltaY", float64(deltaY))
	window.Get("parent").Call("postMessage", payload, window.Get("location").Get("origin").String())
}

func reportPreview(kind, message string) {
	window := js.Global()
	payload := window.Get("Object").New()
	payload.Set("type", kind)
	payload.Set("message", message)
	origin := window.Get("location").Get("origin").String()
	window.Get("parent").Call("postMessage", payload, origin)
	status := window.Get("document").Call("getElementById", "preview-status")
	if !status.IsNull() {
		status.Set("hidden", kind == "viva-preview-ready")
	}
	if kind == "viva-preview-error" {
		window.Get("document").Get("body").Set("textContent", message)
	}
}
