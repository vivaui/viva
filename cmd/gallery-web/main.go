//go:build js && wasm

package main

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/internal/gallery"
)

func main() {
	showcase := gallery.New()
	showcase.EnableTextEditing()
	theme := showcase.Theme()
	var nativeInput app.TextInput
	resolveTextInput := func(context app.TextInputContext) *app.TextInput {
		editor, ok := showcase.TextEditor(context.ID)
		if !ok {
			return nil
		}
		nativeInput = app.TextInput{Editor: editor}
		return &nativeInput
	}
	_ = app.RunCanvas(app.Config{
		Title:  "Viva · component gallery",
		Width:  gallery.DefaultWidth,
		Height: gallery.DefaultHeight,
		ClearColor: app.Color{
			Red: float64(theme.Background.Red), Green: float64(theme.Background.Green),
			Blue: float64(theme.Background.Blue), Alpha: float64(theme.Background.Alpha),
		},
		Tree:      gallery.Tree(),
		Paint:     showcase.Paint,
		OnScroll:  showcase.Scroll,
		OnPointer: showcase.Pointer,
		OnKey:     showcase.Key,
		TextInput: resolveTextInput,
	}, nil)
}
