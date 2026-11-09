package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var avatarComponent = Component{
	ID: "avatar", Title: "Avatar", Category: "Data Display", Summary: "带首字母回退、状态点和重叠组合的头像。",
	paint: (*State).paintAvatar,
	Examples: []Example{
		{ID: "default", Title: "状态头像", Description: "头像在没有图片时使用首字母回退。", Snippet: `components.Avatar(canvas, theme, bounds, components.AvatarData{
    Name: "Skyler", Initials: "SK", Tint: theme.Primary,
    Status: components.AvatarStatusOnline,
})`},
		{ID: "sizes", Title: "尺寸、回退与图片", Description: "展示小型、默认和大型尺寸，以及带图片的头像。", Snippet: `components.Avatar(canvas, theme, small, components.AvatarData{
    Name: "Lin", Initials: "L", Size: components.AvatarSizeSmall,
})
components.Avatar(canvas, theme, standard, components.AvatarData{
    Name: "Skyler", Initials: "SK", Tint: theme.Primary,
})
components.Avatar(canvas, theme, large, components.AvatarData{
    Name: "Guest", Size: components.AvatarSizeLarge,
})
components.Avatar(canvas, theme, photo, components.AvatarData{
    Name: "Photo", Image: photoBitmap, Size: components.AvatarSizeLarge,
})`},
		{ID: "group", Title: "头像组", Description: "使用生产 AvatarGroup 形成固定重叠间距。", Snippet: `group := components.BeginAvatarGroup(canvas, theme, bounds, components.AvatarSizeLarge)
group.Add(components.AvatarData{
    Name: "Ada", Initials: "AD", Tint: theme.Primary,
    Status: components.AvatarStatusOnline,
})
group.Add(components.AvatarData{Name: "Bo", Initials: "BO", Tint: theme.Success})
group.Add(components.AvatarData{Name: "Chen", Initials: "CH", Tint: theme.Danger})
group.Add(components.AvatarData{Name: "Photo", Image: photoBitmap})
group.AddCount("+8")`},
	},
}

// avatarPhoto is a generated 48x32 gradient standing in for a decoded picture.
var avatarPhoto = func() gfx.Bitmap {
	const width, height = 48, 32
	pix := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 4
			pix[offset], pix[offset+1], pix[offset+2], pix[offset+3] = byte(x*5), byte(y*7), 180, 255
		}
	}
	return gfx.Bitmap{Pix: pix, Width: width, Height: height, Stride: width * 4, ID: 0xa7a7, Version: 1}
}()

func (state *State) paintAvatar(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	// The avatars are drawn by components into the slots the page reserves.
	var slots [4]ui.Box
	state.page(canvas, theme, size, 520, state.Text("Avatar · 团队成员", "Avatar · Team members"), func(f *ui.Frame) {
		switch state.exampleID {
		case "sizes":
			f.Row().Gap(24).Items(ui.Start).Children(func() {
				for index, side := range [...]float32{24, 32, 40, 40} {
					f.Column().Gap(8).Items(ui.Center).Children(func() {
						slots[index] = f.Slot().Size(side, side)
						f.Text([...]string{"24 px", "32 px", "40 px", state.Text("图片", "Image")}[index]).FontSize(theme.Metrics.FontSizeS).Muted()
					})
				}
			})
		case "group":
			// Five overlapping 40 px avatars, each advancing by 32 px.
			slots[0] = f.Slot().Size(5*32+8, 40)
			f.Text(state.Text("12 位协作者", "12 collaborators")).FontSize(theme.Metrics.FontSizeS).Muted()
		default:
			f.Row().Gap(20).Items(ui.Center).Children(func() {
				slots[0] = f.Slot().Size(64, 64)
				f.Column().Gap(4).Children(func() {
					f.Text("Skyler").FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightSemibold)
					f.Text(state.Text("在线 · 本地开发", "Online · Local development")).FontSize(theme.Metrics.FontSizeS).Muted()
				})
			})
		}
	})
	switch state.exampleID {
	case "sizes":
		components.Avatar(canvas, theme, slots[0].Rect(), components.AvatarData{Name: "Lin", Initials: "L", Size: components.AvatarSizeSmall, Status: components.AvatarStatusOnline})
		components.Avatar(canvas, theme, slots[1].Rect(), components.AvatarData{Name: "Skyler", Initials: "SK", Status: components.AvatarStatusBusy, Tint: theme.Primary})
		components.Avatar(canvas, theme, slots[2].Rect(), components.AvatarData{Name: "Guest", Size: components.AvatarSizeLarge, Status: components.AvatarStatusOffline})
		components.Avatar(canvas, theme, slots[3].Rect(), components.AvatarData{Name: "Photo", Image: avatarPhoto, Size: components.AvatarSizeLarge})
	case "group":
		group := components.BeginAvatarGroup(canvas, theme, slots[0].Rect(), components.AvatarSizeLarge)
		group.Add(components.AvatarData{Name: "Ada", Initials: "AD", Tint: theme.Primary, Status: components.AvatarStatusOnline})
		group.Add(components.AvatarData{Name: "Bo", Initials: "BO", Tint: theme.Success})
		group.Add(components.AvatarData{Name: "Chen", Initials: state.Text("陈", "CH"), Tint: theme.Danger})
		group.Add(components.AvatarData{Name: "Photo", Image: avatarPhoto})
		group.AddCount("+8")
	default:
		components.Avatar(canvas, theme, slots[0].Rect(), components.AvatarData{
			Name: "Skyler", Initials: "SK", Tint: theme.Primary, Status: components.AvatarStatusOnline,
		})
	}
}
