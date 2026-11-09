//go:build darwin || windows

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/componentdemo"
)

func main() {
	component := flag.String("component", "button", "component ID")
	example := flag.String("example", "", "example ID; empty selects the first")
	dark := flag.Bool("dark", false, "use dark appearance")
	flag.Parse()
	if os.Getenv("VIVA_RELOAD_ADDRESS") != "" || os.Getenv("VIVA_RELOAD_TOKEN") != "" {
		log.Fatal("component previews require standalone native execution; text events are not transported by the development worker")
	}
	state, err := componentdemo.New(*component, *example)
	if err != nil {
		log.Fatal(err)
	}
	state.SetDark(*dark)
	state.EnableTextEditing()
	theme := gfx.LightTheme()
	if *dark {
		theme = gfx.DarkTheme()
	}
	var canvas gfx.Canvas
	var nativeInput app.TextInput
	resolveInput := func(context app.TextInputContext) *app.TextInput {
		editor, supported := state.Editor(context.ID)
		if !supported {
			return nil
		}
		nativeInput = app.TextInput{Editor: editor}
		return &nativeInput
	}
	config := app.Config{
		Title: fmt.Sprintf("Viva · %s · native component preview", *component), Width: 760, Height: 380,
		ClearColor: app.Color{Red: float64(theme.Background.Red), Green: float64(theme.Background.Green), Blue: float64(theme.Background.Blue), Alpha: 1},
		Tree:       componentdemo.Tree(),
		Paint:      state.Paint,
		OnPointer:  state.Pointer,
		OnKey:      state.Key, OnScroll: state.Scroll, TextInput: resolveInput,
	}
	if err := app.RunCanvas(config, &canvas); err != nil {
		log.Fatal(err)
	}
}
