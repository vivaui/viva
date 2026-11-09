package main

import (
	"fmt"

	"github.com/vivaui/viva/text"
)

func main() {
	editor := text.NewEditor("a\nb")
	editor.MoveTo(0, false)
	editor.Insert("x")
	editor.Undo()
	fmt.Println(editor.String())
}
