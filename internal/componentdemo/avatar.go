package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/avatar"
)

var avatarComponent = Component{
	ID: "avatar", Title: "Avatar", Category: "Data Display", Summary: "带首字母回退、状态点和重叠组合的头像。",
	paint: (*State).paintAvatar, initialize: (*State).initializeAvatar,
	Examples: []Example{
		{ID: "default", Title: "状态头像", Description: "头像在没有图片时使用首字母回退。", Snippet: `components.Avatar(canvas, theme, bounds, components.AvatarData{
    Name: "Skyler", Initials: "SK", Tint: theme.Primary,
    Status: components.AvatarStatusOnline,
})`, PageSnippet: `// m.avatar* were made with avatar.New() in the model.
look := c.Theme()
person := m.avatar
person.Reset().Initials("SK").Name("Skyler").Tint(look.Primary).Diameter(64).Online()
ui.Row(c).Gap(20).Items(ui.Center).Children(func() {
    person.View(c, "Skyler").Size(64, 64)
    ui.Column(c).Gap(4).Children(func() {
        ui.Text(c, "Skyler").FontSize(look.Metrics.FontSizeM).Bold()
        ui.Text(c, "Online · Local development").FontSize(look.Metrics.FontSizeS).Muted()
    })
})`, PageSnippetZh: `// m.avatar* 在 model 中用 avatar.New() 创建。
look := c.Theme()
person := m.avatar
person.Reset().Initials("SK").Name("Skyler").Tint(look.Primary).Diameter(64).Online()
ui.Row(c).Gap(20).Items(ui.Center).Children(func() {
    person.View(c, "Skyler").Size(64, 64)
    ui.Column(c).Gap(4).Children(func() {
        ui.Text(c, "Skyler").FontSize(look.Metrics.FontSizeM).Bold()
        ui.Text(c, "在线 · 本地开发").FontSize(look.Metrics.FontSizeS).Muted()
    })
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
})`, PageSnippet: `// m.avatar* were made with avatar.New() in the model.
look := c.Theme()
small := m.smallAvatar
small.Reset().Initials("L").Name("Lin").Diameter(24).Online()
standard := m.standardAvatar
standard.Reset().Initials("SK").Name("Skyler").Tint(look.Primary).Diameter(32).Busy()
large := m.largeAvatar
large.Reset().Name("Guest").Diameter(40).Offline()
photo := m.photoAvatar
// m.photo is the app's asset image.
photo.Reset().Image(m.photo).Name("Photo").Diameter(40)
ui.Row(c).Gap(24).Items(ui.Start).Children(func() {
    ui.Column(c).Gap(8).Items(ui.Center).Children(func() {
        small.View(c, "Lin").Size(24, 24)
        ui.Text(c, "24 px").FontSize(look.Metrics.FontSizeS).Muted()
    })
    ui.Column(c).Gap(8).Items(ui.Center).Children(func() {
        standard.View(c, "Skyler").Size(32, 32)
        ui.Text(c, "32 px").FontSize(look.Metrics.FontSizeS).Muted()
    })
    ui.Column(c).Gap(8).Items(ui.Center).Children(func() {
        large.View(c, "Guest").Size(40, 40)
        ui.Text(c, "40 px").FontSize(look.Metrics.FontSizeS).Muted()
    })
    ui.Column(c).Gap(8).Items(ui.Center).Children(func() {
        photo.View(c, "Photo").Size(40, 40)
        ui.Text(c, "Image").FontSize(look.Metrics.FontSizeS).Muted()
    })
})`, PageSnippetZh: `// m.avatar* 在 model 中用 avatar.New() 创建。
look := c.Theme()
small := m.smallAvatar
small.Reset().Initials("L").Name("Lin").Diameter(24).Online()
standard := m.standardAvatar
standard.Reset().Initials("SK").Name("Skyler").Tint(look.Primary).Diameter(32).Busy()
large := m.largeAvatar
large.Reset().Name("Guest").Diameter(40).Offline()
photo := m.photoAvatar
// m.photo 是应用自带的图片资源。
photo.Reset().Image(m.photo).Name("Photo").Diameter(40)
ui.Row(c).Gap(24).Items(ui.Start).Children(func() {
    ui.Column(c).Gap(8).Items(ui.Center).Children(func() {
        small.View(c, "Lin").Size(24, 24)
        ui.Text(c, "24 px").FontSize(look.Metrics.FontSizeS).Muted()
    })
    ui.Column(c).Gap(8).Items(ui.Center).Children(func() {
        standard.View(c, "Skyler").Size(32, 32)
        ui.Text(c, "32 px").FontSize(look.Metrics.FontSizeS).Muted()
    })
    ui.Column(c).Gap(8).Items(ui.Center).Children(func() {
        large.View(c, "Guest").Size(40, 40)
        ui.Text(c, "40 px").FontSize(look.Metrics.FontSizeS).Muted()
    })
    ui.Column(c).Gap(8).Items(ui.Center).Children(func() {
        photo.View(c, "Photo").Size(40, 40)
        ui.Text(c, "图片").FontSize(look.Metrics.FontSizeS).Muted()
    })
})`},
		{ID: "group", Title: "头像组", Description: "使用生产 AvatarGroup 形成固定重叠间距。", Snippet: `group := components.BeginAvatarGroup(canvas, theme, bounds, components.AvatarSizeLarge)
group.Add(components.AvatarData{
    Name: "Ada", Initials: "AD", Tint: theme.Primary,
    Status: components.AvatarStatusOnline,
})
group.Add(components.AvatarData{Name: "Bo", Initials: "BO", Tint: theme.Success})
group.Add(components.AvatarData{Name: "Chen", Initials: "CH", Tint: theme.Danger})
group.Add(components.AvatarData{Name: "Photo", Image: photoBitmap})
group.AddCount("+8")`, PageSnippet: `// m.avatar* were made with avatar.New() in the model.
look := c.Theme()
group := m.avatarGroup
group.Reset().Diameter(40).Overlap(8)
group.Add().Initials("AD").Name("Ada").Tint(look.Primary).Online()
group.Add().Initials("BO").Name("Bo").Tint(look.Success)
group.Add().Initials("CH").Name("Chen").Tint(look.Danger)
// m.photo is the app's asset image.
group.Add().Image(m.photo).Name("Photo")
group.AddCount("+8")
ui.Column(c).Gap(8).Items(ui.Start).Children(func() {
    group.View(c, "Collaborators")
    ui.Text(c, "12 collaborators").FontSize(look.Metrics.FontSizeS).Muted()
})`, PageSnippetZh: `// m.avatar* 在 model 中用 avatar.New() 创建。
look := c.Theme()
group := m.avatarGroup
group.Reset().Diameter(40).Overlap(8)
group.Add().Initials("AD").Name("Ada").Tint(look.Primary).Online()
group.Add().Initials("BO").Name("Bo").Tint(look.Success)
group.Add().Initials("陈").Name("陈").Tint(look.Danger)
// m.photo 是应用自带的图片资源。
group.Add().Image(m.photo).Name("Photo")
group.AddCount("+8")
ui.Column(c).Gap(8).Items(ui.Start).Children(func() {
    group.View(c, "协作者")
    ui.Text(c, "12 位协作者").FontSize(look.Metrics.FontSizeS).Muted()
})`},
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

// avatarDemo is one avatar example's page controls. They are kept beside the
// State rather than rebuilt in each paint, so their handlers and draw functions
// are stable and a warm paint allocates nothing.
type avatarDemo struct {
	single, small, standard, large, photo *avatar.Avatar
	group                                 *avatar.Group
}

// avatarDemos is keyed by State because the constrained demo model has no field
// for these controls; a State is painted on one thread at a time.
var avatarDemos = map[*State]*avatarDemo{}

func (state *State) initializeAvatar() { avatarDemoFor(state) }

func avatarDemoFor(state *State) *avatarDemo {
	if demo := avatarDemos[state]; demo != nil {
		return demo
	}
	demo := &avatarDemo{
		single:   avatar.New(),
		small:    avatar.New(),
		standard: avatar.New(),
		large:    avatar.New(),
		photo:    avatar.New(),
		group:    avatar.NewGroup(),
	}
	avatarDemos[state] = demo
	return demo
}

func (state *State) paintAvatar(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := avatarDemoFor(state)
	state.drawPage(canvas, theme, size, 520, state.Text("Avatar · 团队成员", "Avatar · Team members"), func(c ui.C) {
		switch state.exampleID {
		case "sizes":
			demo.small.Reset().Initials("L").Name("Lin").Diameter(24).Online()
			demo.standard.Reset().Initials("SK").Name("Skyler").Tint(theme.Primary).Diameter(32).Busy()
			demo.large.Reset().Name("Guest").Diameter(40).Offline()
			demo.photo.Reset().Image(avatarPhoto).Name("Photo").Diameter(40)
			ui.Row(c).Gap(24).Items(ui.Start).Children(func() {
				avatarSample(c, theme, demo.small, "Lin", 24, "24 px")
				avatarSample(c, theme, demo.standard, "Skyler", 32, "32 px")
				avatarSample(c, theme, demo.large, "Guest", 40, "40 px")
				avatarSample(c, theme, demo.photo, "Photo", 40, state.Text("图片", "Image"))
			})
		case "group":
			group := demo.group
			group.Reset().Diameter(40).Overlap(8)
			group.Add().Initials("AD").Name("Ada").Tint(theme.Primary).Online()
			group.Add().Initials("BO").Name("Bo").Tint(theme.Success)
			group.Add().Initials(state.Text("陈", "CH")).Name(state.Text("陈", "Chen")).Tint(theme.Danger)
			group.Add().Image(avatarPhoto).Name("Photo")
			group.AddCount("+8")
			ui.Column(c).Gap(8).Items(ui.Start).Children(func() {
				group.View(c, state.Text("协作者", "Collaborators"))
				ui.Text(c, state.Text("12 位协作者", "12 collaborators")).FontSize(theme.Metrics.FontSizeS).Muted()
			})
		default:
			demo.single.Reset().Initials("SK").Name("Skyler").Tint(theme.Primary).Diameter(64).Online()
			ui.Row(c).Gap(20).Items(ui.Center).Children(func() {
				demo.single.View(c, "Skyler").Size(64, 64)
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Skyler").FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightSemibold)
					ui.Text(c, state.Text("在线 · 本地开发", "Online · Local development")).FontSize(theme.Metrics.FontSizeS).Muted()
				})
			})
		}
	})
}

// avatarSample declares one labelled avatar of the sizes example.
func avatarSample(c ui.C, theme gfx.Theme, person *avatar.Avatar, name string, side float32, label string) {
	ui.Column(c).Gap(8).Items(ui.Center).Children(func() {
		person.View(c, name).Size(side, side)
		ui.Text(c, label).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
