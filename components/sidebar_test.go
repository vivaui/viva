package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSidebarPaintsOfficialSurfaceAndMenuVariants(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	sidebar := BeginSidebarWithOptions(&canvas, theme, gfx.Rect{Width: 220, Height: 240}, "Acme", SidebarOptions{Side: SidebarRight, Variant: SidebarFloating, Collapsible: SidebarIcon})
	sidebar.AddGroup("Workspace")
	sidebar.AddMenuItem(SidebarMenuItem{ID: gfx.Item(1), Label: "Overview", Icon: "O", Active: true, Variant: SidebarMenuOutline})
	sidebar.AddMenuItem(SidebarMenuItem{ID: gfx.Item(2), Label: "Archive", Trailing: "12", Disabled: true, Size: SidebarMenuSizeSmall})
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].ID != gfx.Item(1) {
		t.Fatalf("sidebar hits = %+v", canvas.Hits())
	}
	if hit := canvas.Hits()[0]; hit.Role != gfx.RoleMenuItem || !hit.Selected {
		t.Fatalf("sidebar semantics = %+v", hit)
	}
	if want := gfx.DarkTheme().Metrics.RadiusPopup(); canvas.Rectangles()[0].Radius != want {
		t.Fatalf("floating sidebar radius = %f, want %f", canvas.Rectangles()[0].Radius, want)
	}
}

func TestSidebarCollapseStatesAndCommandsAreStable(t *testing.T) {
	var hidden gfx.Canvas
	sidebar := BeginSidebarWithOptions(&hidden, gfx.DarkTheme(), gfx.Rect{Width: 200, Height: 180}, "Product", SidebarOptions{Collapsed: true, Collapsible: SidebarOffcanvas})
	sidebar.AddItem(gfx.Item(1), "Inbox", "4", false, false)
	if len(hidden.Rectangles()) != 0 || len(hidden.Texts()) != 0 {
		t.Fatal("collapsed off-canvas sidebar emitted commands")
	}

	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		sidebar := BeginSidebarWithOptions(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 200, Height: 180}, "Product", SidebarOptions{Collapsed: true, Collapsible: SidebarIcon})
		sidebar.AddMenuItem(SidebarMenuItem{ID: gfx.Item(1), Label: "Inbox", Icon: "I", Active: true})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if canvas.Texts()[0].Value != "I" || canvas.Hits()[0].Label != "Inbox" {
		t.Fatalf("icon sidebar commands = %+v / %+v", canvas.Texts(), canvas.Hits())
	}
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical sidebars produced different command streams")
	}
}

func TestSidebarPaintsVectorIconsAndReturnsFooterAnchor(t *testing.T) {
	icon := gfx.Icon{ViewBox: 10, Stroke: 1, Commands: []gfx.PathCommand{gfx.MoveTo(1, 5), gfx.LineTo(9, 5)}}
	var canvas gfx.Canvas
	sidebar := BeginSidebarWithOptions(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 200, Height: 180}, "Product", SidebarOptions{Collapsed: true, Collapsible: SidebarIcon})
	item := sidebar.AddMenuItem(SidebarMenuItem{ID: gfx.Item(1), Label: "Hosts", VectorIcon: icon})
	footer := sidebar.AddFooterMenuItem(SidebarMenuItem{ID: gfx.Item(2), Label: "Settings", VectorIcon: icon})
	if item == (gfx.Rect{}) || footer == (gfx.Rect{}) || footer.Y <= item.Y {
		t.Fatalf("anchors = %+v / %+v", item, footer)
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("vector icons did not paint geometry")
	}
	if hits := canvas.Hits(); len(hits) != 2 || hits[0].Label != "Hosts" || hits[1].Label != "Settings" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestSidebarSubMenuIsIndentedUnderItsParentAndClickable(t *testing.T) {
	var canvas gfx.Canvas
	sidebar := BeginSidebarWithOptions(&canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 300}, "", SidebarOptions{Collapsible: SidebarFixed})
	parent := sidebar.AddMenuItem(SidebarMenuItem{ID: gfx.Item(1), Label: "Settings"})
	menu := sidebar.AddSubMenu([]SidebarSubItem{
		{ID: gfx.Item(2), Label: "General", Active: true},
		{ID: gfx.Item(3), Label: "Keys", Size: SidebarSubSizeSmall},
		{Label: "Inert"},
	})
	next := sidebar.AddMenuItem(SidebarMenuItem{ID: gfx.Item(4), Label: "Hosts"})
	hits := canvas.Hits()
	if len(hits) != 4 || hits[1].ID != gfx.Item(2) || !hits[1].Selected || hits[2].ID != gfx.Item(3) {
		t.Fatalf("sub menu hits = %+v", hits)
	}
	if hits[1].Bounds.X <= parent.X+10 || hits[1].Bounds.Y < menu.Y || hits[2].Bounds.Y <= hits[1].Bounds.Y {
		t.Fatalf("sub buttons %+v / %+v are not indented under %+v inside %+v", hits[1].Bounds, hits[2].Bounds, parent, menu)
	}
	if next.Y < menu.Y+menu.Height {
		t.Fatalf("next item %+v overlaps the sub menu %+v", next, menu)
	}

	var collapsed gfx.Canvas
	icons := BeginSidebarWithOptions(&collapsed, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 300}, "", SidebarOptions{Collapsible: SidebarIcon, Collapsed: true})
	if got := icons.AddSubMenu([]SidebarSubItem{{ID: gfx.Item(2), Label: "General"}}); got != (gfx.Rect{}) || len(collapsed.Hits()) != 0 {
		t.Fatalf("icon-collapsed sidebar painted a sub menu: %+v", got)
	}
}

func TestSidebarMenuActionAppearsOnHoverAndDoesNotOverlapTheItem(t *testing.T) {
	icon := gfx.Icon{ViewBox: 10, Stroke: 1, Commands: []gfx.PathCommand{gfx.MoveTo(1, 5), gfx.LineTo(9, 5)}}
	item := SidebarMenuItem{ID: gfx.Item(1), Label: "Project", Action: SidebarMenuAction{ID: gfx.Item(2), Icon: icon, Label: "More", ShowOnHover: true}}
	paint := func(hovered gfx.ID) *gfx.Canvas {
		canvas := new(gfx.Canvas)
		canvas.SetInteraction(hovered, gfx.ID{}, gfx.ID{}, false)
		sidebar := BeginSidebarWithOptions(canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 200}, "", SidebarOptions{Collapsible: SidebarFixed})
		sidebar.AddMenuItem(item)
		return canvas
	}
	idle := paint(gfx.ID{})
	if len(idle.Triangles()) != 0 {
		t.Fatal("action icon is visible without hover")
	}
	hits := idle.Hits()
	if len(hits) != 2 || hits[1].ID != gfx.Item(2) || hits[1].Label != "More" {
		t.Fatalf("hidden action must stay reachable after the item: %+v", hits)
	}
	if hovered := paint(gfx.Item(1)); len(hovered.Triangles()) == 0 {
		t.Fatal("action icon is hidden while its item is hovered")
	}
	if hits[0].Bounds.X+hits[0].Bounds.Width > hits[1].Bounds.X {
		t.Fatalf("item hit %+v overlaps the action %+v", hits[0].Bounds, hits[1].Bounds)
	}
}

func TestSidebarFooterReservesRoomFromContent(t *testing.T) {
	var canvas gfx.Canvas
	sidebar := BeginSidebarWithOptions(&canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 140}, "", SidebarOptions{Collapsible: SidebarFixed})
	footer := sidebar.AddFooter(40)
	if footer.Y+footer.Height > 140 || footer.Height != 40 {
		t.Fatalf("footer = %+v", footer)
	}
	var last gfx.Rect
	for range 6 {
		if item := sidebar.AddMenuItem(SidebarMenuItem{ID: gfx.Item(1), Label: "Item"}); item != (gfx.Rect{}) {
			last = item
		}
	}
	if last == (gfx.Rect{}) || last.Y+last.Height > footer.Y {
		t.Fatalf("content reaches %+v, below the footer at %+v", last, footer)
	}
	stacked := sidebar.AddFooterMenuItem(SidebarMenuItem{ID: gfx.Item(9), Label: "Account"})
	if stacked.Y+stacked.Height > footer.Y {
		t.Fatalf("second footer %+v does not sit above the first %+v", stacked, footer)
	}
}
