//go:build darwin || windows || linux

package result_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/result"
)

func TestResultIsAStatusNamedByItsTitle(t *testing.T) {
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		result.View(c, result.Spec{Status: ui.StatusSuccess, Title: "Key imported"}, nil).Fill()
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Result", Width: 480, Height: 320}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if node, ok := session.AccessByName("Key imported"); !ok || node.Role != gfx.RoleStatus {
		t.Fatalf("access node = %+v, %v; want a status named by the title", node, ok)
	}
}
