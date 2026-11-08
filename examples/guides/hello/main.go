package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

type model struct {
	clicks int
}

// page declares the window's content. It runs on every repaint and reads the
// model; a click changes the model, and the repaint that follows shows it.
func (m *model) page(s flex.Screen) {
	s.Column().Fill().Pad(24).Gap(12).Children(func() {
		if s.Button(fmt.Sprintf("Clicked %d times", m.clicks)).Clicked() {
			m.clicks++
		}
	})
}

func main() {
	m := &model{}
	if err := flex.Run(app.Config{Title: "Hello", Width: 480, Height: 240}, m.page); err != nil {
		log.Fatal(err)
	}
}
