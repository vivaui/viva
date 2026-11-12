//go:build darwin || windows || linux

package components_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestCodeMergeHeadlessAcceptButtonAndNativeResultInput(t *testing.T) {
	var tree gfx.Tree
	scope := tree.Feature("merge")
	resultID := tree.ID(scope, "result")
	actionID := tree.ID(scope, "accept-incoming")
	var editor text.Editor
	editor.Insert("head\nchoice\ntail")
	var merge components.CodeMerge
	var selected bool
	config := app.Config{
		Title: "Merge comparison", Width: 840, Height: 220,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.SetTree(&tree)
			value := editor.String()
			merge.Paint(canvas, gfx.DarkTheme(), gfx.Rect{Width: size.Width, Height: size.Height}, components.CodeMergeSnapshot{
				Current: strings.Split("head\nlocal\ntail", "\n"), Incoming: strings.Split("head\nremote\ntail", "\n"),
				Rows: []components.CodeMergeRow{
					{CurrentLine: 0, ResultLine: 0, IncomingLine: 0},
					{CurrentLine: 1, ResultLine: 1, IncomingLine: 1, CurrentChanged: true, IncomingChanged: true},
					{CurrentLine: 2, ResultLine: 2, IncomingLine: 2},
				},
				Editor: components.CodeDiffSnapshot{ID: resultID, Label: "Result", Value: value,
					Caret: editor.ByteOffset(editor.Caret()), Focused: canvas.Focused() == resultID, ShowCaret: true},
				Actions: []components.CodeMergeAction{{Row: 1, Incoming: true, ID: actionID, Label: "Accept"}},
			})
			if canvas.Clicked(actionID) {
				selected = true
			}
		},
		TextInput: func(context app.TextInputContext) *app.TextInput {
			if context.ID != resultID {
				return nil
			}
			return &app.TextInput{Editor: &editor}
		},
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1.25})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	var result, action gfx.Rect
	for _, hit := range session.Snapshot().Hits {
		switch hit.ID {
		case resultID:
			result = hit.Bounds
		case actionID:
			action = hit.Bounds
		}
	}
	if result.Width <= 0 || action.Width <= 0 {
		t.Fatalf("missing result or action: result=%+v action=%+v", result, action)
	}
	if err := session.Click(result.X+12, result.Y+result.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := session.Type("x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(editor.String(), "x") {
		t.Fatal("native input did not edit the center result")
	}
	if err := session.Click(action.X+action.Width/2, action.Y+action.Height/2); err != nil {
		t.Fatal(err)
	}
	if !selected {
		t.Fatal("incoming row action did not receive a click")
	}
	if dir := os.Getenv("VIVA_MERGE_SCREENSHOT_DIR"); dir != "" {
		if err := session.Screenshot(filepath.Join(dir, "merge.png")); err != nil {
			t.Fatal(err)
		}
	}
}
