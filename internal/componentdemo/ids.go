package componentdemo

import "github.com/vivaui/viva/gfx"

// tree is the identity tree every demo names its controls in. A demo declares
// its places once, in package variables beside its Component:
//
//	var switchUI = tree.Feature("switch")                    // the demo's page
//	var switchToggle = tree.Sub(switchUI, tree.Name("toggle")) // a list: switchToggle.At(i)
//
// and reads its clicks where it paints (canvas.Clicked, a component's Clicked,
// a ui.Box's Clicked). A runner gives the same tree to its window
// (app.Config.Tree = componentdemo.Tree()); the gallery names its own places in
// it too and mounts each live demo at a scope of its own (gfx.Tree.Mount), so the
// same demo painted in several places has distinct identities and needs no
// numbering by hand.
var tree gfx.Tree

// Tree returns the identity tree the demos name their controls in. A runner
// that paints a State directly sets it as the window's Config.Tree; the gallery
// builds its own identities in it.
func Tree() *gfx.Tree { return &tree }
