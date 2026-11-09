package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var carouselComponent = Component{
	ID: "carousel", Title: "Carousel", Category: "Data Display", Summary: "外部持有索引和偏移量的裁剪轮播视口。",
	paint: (*State).paintCarousel,
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
}`},
	},
}

var (
	carouselUI       = tree.Feature("carousel")
	carouselPrevious = tree.ID(carouselUI, "previous")
	carouselNext     = tree.ID(carouselUI, "next")
)

func (state *State) paintCarousel(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	vertical := state.exampleID == "vertical"
	// The previous and next buttons sit just outside the viewport, so the slot is
	// kept clear of the page edge and of the title by a margin.
	var slot ui.Box
	state.page(canvas, theme, size, 584, state.Text("Carousel · 产品导览", "Carousel · Product tour"), func(f *ui.Frame) {
		if vertical {
			slot = f.Slot().FillWidth().Height(180).MarginY(44).MarginX(32)
		} else {
			slot = f.Slot().FillWidth().Height(200).MarginX(32)
		}
	})
	bounds := slot.Rect()
	index := int(state.values[0])
	itemExtent := bounds.Width
	if vertical {
		itemExtent = bounds.Height
	}
	carousel := components.BeginCarousel(canvas, theme, bounds, 3, index).
		Content(itemExtent*3, float32(index)*itemExtent).
		Controls(carouselPrevious, state.Text("上一张", "Previous slide"), carouselNext, state.Text("下一张", "Next slide"))
	if vertical {
		carousel = carousel.Vertical()
	}
	carousel.BeginContent()
	paintCarouselSlides(canvas, theme, &carousel, itemExtent, vertical)
	carousel.EndContent()
	if canvas.Clicked(carouselPrevious) && state.values[0] > 0 {
		state.values[0]--
	} else if canvas.Clicked(carouselNext) && state.values[0] < 2 {
		state.values[0]++
	}
}

func paintCarouselSlides(canvas *gfx.Canvas, theme gfx.Theme, carousel *components.Carousel, itemExtent float32, vertical bool) {
	labels := [...]string{"Retained scene", "Localized update", "Idle means idle"}
	for index := range labels {
		item := carousel.ItemGeometry(index, float32(index)*itemExtent, itemExtent)
		if !item.Visible {
			continue
		}
		bounds := item.Bounds
		if vertical {
			bounds.Y += 6
			bounds.Height -= 12
		} else {
			bounds.X += 6
			bounds.Width -= 12
		}
		fill := theme.Muted
		if item.Selected {
			fill = theme.Accent
		}
		canvas.Rounded(bounds, theme.Metrics.RadiusSurface, fill)
		canvas.LabelCenter(labels[index], bounds, gfx.TextStyle{Size: theme.Metrics.FontSizeM + 2, Weight: gfx.WeightSemibold, Color: theme.Foreground})
	}
}
