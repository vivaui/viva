package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/carousel"
)

var carouselComponent = Component{
	ID: "carousel", Title: "Carousel", Category: "Data Display", Summary: "外部持有索引和偏移量的裁剪轮播视口。",
	paint: (*State).paintCarousel, initialize: (*State).initializeCarousel,
	Examples: []Example{
		{ID: "horizontal", Title: "水平轮播", Description: "前后按钮只改变调用方持有的当前索引。", Snippet: `previous, next := tree.ID(page, "previous"), tree.ID(page, "next")
carousel := components.BeginCarousel(canvas, theme, bounds, 3, state.carousel).
    Content(bounds.Width*3, float32(state.carousel)*bounds.Width).
    Controls(previous, "Previous slide", next, "Next slide")
carousel.BeginContent()
// Paint each slide at carousel.ItemGeometry(i, offset, bounds.Width).Bounds.
carousel.EndContent()
if canvas.Clicked(previous) && state.carousel > 0 {
    state.carousel--
} else if canvas.Clicked(next) && state.carousel < 2 {
    state.carousel++
}`, PageSnippet: `slides := []string{"Retained scene", "Localized update", "Idle means idle"}
car := &carousel.Carousel{
    ItemCount: 3, Index: m.slide,
    PreviousID: m.previous, PreviousLabel: "Previous slide",
    NextID: m.next, NextLabel: "Next slide",
}
car.Draw = func(d *ui.DrawContext, geometry carousel.Geometry) {
    for index := 0; index < car.ItemCount; index++ {
        item := car.ItemBounds(geometry, index)
        fill := d.Theme.Muted
        if index == geometry.Index {
            fill = d.Theme.Accent
        }
        d.Canvas.Rounded(item, d.Theme.Metrics.RadiusSurface, fill)
        d.Canvas.LabelCenter(slides[index], item, gfx.TextStyle{
            Size: d.Theme.Metrics.FontSizeM + 2, Weight: gfx.WeightSemibold,
            Color: d.Theme.Foreground,
        })
    }
}
carousel.View(c, "Product tour", car).FillWidth().Height(200).MarginX(32)
if car.Clicked(m.previous) && m.slide > 0 {
    m.slide--
} else if car.Clicked(m.next) && m.slide < 2 {
    m.slide++
}`, PageSnippetZh: `slides := []string{"保留场景", "本地化更新", "空闲即空闲"}
car := &carousel.Carousel{
    ItemCount: 3, Index: m.slide,
    PreviousID: m.previous, PreviousLabel: "上一张",
    NextID: m.next, NextLabel: "下一张",
}
car.Draw = func(d *ui.DrawContext, geometry carousel.Geometry) {
    for index := 0; index < car.ItemCount; index++ {
        item := car.ItemBounds(geometry, index)
        fill := d.Theme.Muted
        if index == geometry.Index {
            fill = d.Theme.Accent
        }
        d.Canvas.Rounded(item, d.Theme.Metrics.RadiusSurface, fill)
        d.Canvas.LabelCenter(slides[index], item, gfx.TextStyle{
            Size: d.Theme.Metrics.FontSizeM + 2, Weight: gfx.WeightSemibold,
            Color: d.Theme.Foreground,
        })
    }
}
carousel.View(c, "产品导览", car).FillWidth().Height(200).MarginX(32)
if car.Clicked(m.previous) && m.slide > 0 {
    m.slide--
} else if car.Clicked(m.next) && m.slide < 2 {
    m.slide++
}`},
		{ID: "vertical", Title: "垂直轮播", Description: "相同几何模型切换为纵向内容轴。", Snippet: `previous, next := tree.ID(page, "previous"), tree.ID(page, "next")
carousel := components.BeginCarousel(canvas, theme, bounds, 3, state.carousel).
    Content(bounds.Height*3, float32(state.carousel)*bounds.Height).
    Controls(previous, "Previous slide", next, "Next slide").Vertical()
carousel.BeginContent()
// Slides lay out down the content axis; ItemGeometry maps each one.
carousel.EndContent()
if canvas.Clicked(previous) && state.carousel > 0 {
    state.carousel--
} else if canvas.Clicked(next) && state.carousel < 2 {
    state.carousel++
}`, PageSnippet: `slides := []string{"Retained scene", "Localized update", "Idle means idle"}
car := &carousel.Carousel{
    ItemCount: 3, Index: m.slide,
    Orientation: carousel.Vertical,
    PreviousID: m.previous, PreviousLabel: "Previous slide",
    NextID: m.next, NextLabel: "Next slide",
}
car.Draw = func(d *ui.DrawContext, geometry carousel.Geometry) {
    for index := 0; index < car.ItemCount; index++ {
        item := car.ItemBounds(geometry, index)
        fill := d.Theme.Muted
        if index == geometry.Index {
            fill = d.Theme.Accent
        }
        d.Canvas.Rounded(item, d.Theme.Metrics.RadiusSurface, fill)
        d.Canvas.LabelCenter(slides[index], item, gfx.TextStyle{
            Size: d.Theme.Metrics.FontSizeM + 2, Weight: gfx.WeightSemibold,
            Color: d.Theme.Foreground,
        })
    }
}
carousel.View(c, "Product tour", car).FillWidth().Height(180).MarginY(44).MarginX(32)
if car.Clicked(m.previous) && m.slide > 0 {
    m.slide--
} else if car.Clicked(m.next) && m.slide < 2 {
    m.slide++
}`, PageSnippetZh: `slides := []string{"保留场景", "本地化更新", "空闲即空闲"}
car := &carousel.Carousel{
    ItemCount: 3, Index: m.slide,
    Orientation: carousel.Vertical,
    PreviousID: m.previous, PreviousLabel: "上一张",
    NextID: m.next, NextLabel: "下一张",
}
car.Draw = func(d *ui.DrawContext, geometry carousel.Geometry) {
    for index := 0; index < car.ItemCount; index++ {
        item := car.ItemBounds(geometry, index)
        fill := d.Theme.Muted
        if index == geometry.Index {
            fill = d.Theme.Accent
        }
        d.Canvas.Rounded(item, d.Theme.Metrics.RadiusSurface, fill)
        d.Canvas.LabelCenter(slides[index], item, gfx.TextStyle{
            Size: d.Theme.Metrics.FontSizeM + 2, Weight: gfx.WeightSemibold,
            Color: d.Theme.Foreground,
        })
    }
}
carousel.View(c, "产品导览", car).FillWidth().Height(180).MarginY(44).MarginX(32)
if car.Clicked(m.previous) && m.slide > 0 {
    m.slide--
} else if car.Clicked(m.next) && m.slide < 2 {
    m.slide++
}`},
	},
}

var (
	carouselUI       = tree.Feature("carousel")
	carouselPrevious = tree.ID(carouselUI, "previous")
	carouselNext     = tree.ID(carouselUI, "next")
)

// carouselDemo is one carousel example's page control. The wrapper's handlers
// are made once and point at this value, so it is kept beside the State rather
// than rebuilt in each paint.
type carouselDemo struct {
	state *State
	view  carousel.Carousel
}

// carouselDemos is keyed by State because the constrained demo model has no
// field for this example; a State is painted on one thread at a time.
var carouselDemos = map[*State]*carouselDemo{}

func (state *State) initializeCarousel() {
	demo := &carouselDemo{state: state}
	demo.view.Draw = demo.draw
	carouselDemos[state] = demo
}

func carouselDemoFor(state *State) *carouselDemo {
	if demo := carouselDemos[state]; demo != nil {
		return demo
	}
	demo := &carouselDemo{state: state}
	demo.view.Draw = demo.draw
	carouselDemos[state] = demo
	return demo
}

func (state *State) paintCarousel(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	vertical := state.exampleID == "vertical"
	demo := carouselDemoFor(state)
	demo.view.ItemCount = 3
	demo.view.Index = int(state.values[0])
	demo.view.Offset, demo.view.Extent = 0, 0
	demo.view.Orientation = carousel.Horizontal
	if vertical {
		demo.view.Orientation = carousel.Vertical
	}
	demo.view.PreviousID, demo.view.NextID = carouselPrevious, carouselNext
	demo.view.PreviousLabel = state.Text("上一张", "Previous slide")
	demo.view.NextLabel = state.Text("下一张", "Next slide")
	// The previous and next buttons sit just outside the viewport, so the view is
	// kept clear of the page edge and of the title by a margin.
	state.drawPage(canvas, theme, size, 584, state.Text("轮播 · 产品导览", "Carousel · Product tour"), func(c ui.C) {
		view := carousel.View(c, state.Text("轮播", "Carousel"), &demo.view).FillWidth()
		if vertical {
			view.Height(180).MarginY(44).MarginX(32)
		} else {
			view.Height(200).MarginX(32)
		}
	})
	if demo.view.Clicked(carouselPrevious) && state.values[0] > 0 {
		state.values[0]--
	} else if demo.view.Clicked(carouselNext) && state.values[0] < int32(demo.view.ItemCount-1) {
		state.values[0]++
	}
}

// draw paints each caller-owned slide into the clipped viewport. The wrapper
// hands it the complete geometry, so it maps an evenly divided content extent
// with ItemBounds.
func (d *carouselDemo) draw(ctx *ui.DrawContext, geometry carousel.Geometry) {
	labels := [3]string{
		d.state.Text("保留场景", "Retained scene"),
		d.state.Text("本地化更新", "Localized update"),
		d.state.Text("空闲即空闲", "Idle means idle"),
	}
	vertical := d.view.Orientation == carousel.Vertical
	for index := range labels {
		bounds := d.view.ItemBounds(geometry, index)
		if !carouselSlideVisible(bounds, geometry.Viewport) {
			continue
		}
		if vertical {
			bounds.Y += 6
			bounds.Height -= 12
		} else {
			bounds.X += 6
			bounds.Width -= 12
		}
		fill := ctx.Theme.Muted
		if index == geometry.Index {
			fill = ctx.Theme.Accent
		}
		ctx.Canvas.Rounded(bounds, ctx.Theme.Metrics.RadiusSurface, fill)
		ctx.Canvas.LabelCenter(labels[index], bounds, gfx.TextStyle{
			Size: ctx.Theme.Metrics.FontSizeM + 2, Weight: gfx.WeightSemibold, Color: ctx.Theme.Foreground,
		})
	}
}

func carouselSlideVisible(bounds, viewport gfx.Rect) bool {
	return bounds.Width > 0 && bounds.Height > 0 &&
		bounds.X < viewport.X+viewport.Width && bounds.X+bounds.Width > viewport.X &&
		bounds.Y < viewport.Y+viewport.Height && bounds.Y+bounds.Height > viewport.Y
}
