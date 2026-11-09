//go:build darwin || windows || linux

package headless

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func strictSession(t *testing.T, strict bool, paint func(*gfx.Canvas)) *Session {
	t.Helper()
	var tree gfx.Tree
	config := app.Config{Title: "strict", Width: 200, Height: 100, Tree: &tree}
	config.OnWake = func() {}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) { paint(canvas) }
	session, err := Open(config, nil, Options{Strict: strict})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

func TestStrictReportsDuplicateLabels(t *testing.T) {
	session := strictSession(t, true, func(canvas *gfx.Canvas) {
		tree := canvas.Tree()
		home := tree.Feature("home")
		canvas.HitWithOptions(tree.ID(home, "a"), gfx.Rect{X: 10, Y: 10, Width: 50, Height: 20}, "Same", gfx.HitOptions{Role: gfx.RoleButton})
		canvas.HitWithOptions(tree.ID(home, "b"), gfx.Rect{X: 10, Y: 40, Width: 50, Height: 20}, "Same", gfx.HitOptions{Role: gfx.RoleButton})
	})
	if err := session.CheckStrict(); err == nil || !strings.Contains(err.Error(), "share the label") {
		t.Fatalf("CheckStrict() = %v, want a duplicate label", err)
	}
}

func TestStrictReportsDuplicateIdentities(t *testing.T) {
	session := strictSession(t, true, func(canvas *gfx.Canvas) {
		tree := canvas.Tree()
		home := tree.Feature("home")
		id := tree.ID(home, "same")
		canvas.HitWithOptions(id, gfx.Rect{X: 10, Y: 10, Width: 50, Height: 20}, "One", gfx.HitOptions{Role: gfx.RoleButton})
		canvas.HitWithOptions(id, gfx.Rect{X: 10, Y: 40, Width: 50, Height: 20}, "Two", gfx.HitOptions{Role: gfx.RoleButton})
	})
	if err := session.CheckStrict(); err == nil || !strings.Contains(err.Error(), "share the identity") {
		t.Fatalf("CheckStrict() = %v, want a duplicate identity", err)
	}
}

func TestStrictIsOffByDefault(t *testing.T) {
	session := strictSession(t, false, func(canvas *gfx.Canvas) {
		tree := canvas.Tree()
		home := tree.Feature("home")
		canvas.HitWithOptions(tree.ID(home, "a"), gfx.Rect{X: 10, Y: 10, Width: 50, Height: 20}, "Same", gfx.HitOptions{Role: gfx.RoleButton})
		canvas.HitWithOptions(tree.ID(home, "b"), gfx.Rect{X: 10, Y: 40, Width: 50, Height: 20}, "Same", gfx.HitOptions{Role: gfx.RoleButton})
	})
	if err := session.CheckStrict(); err != nil {
		t.Fatalf("CheckStrict() = %v with strict off", err)
	}
}
