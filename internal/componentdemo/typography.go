package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var typographyComponent = Component{
	ID: "typography", Title: "Typography", Category: "Display", Summary: "主题驱动的文档文字角色。",
	paint: (*State).paintTypography,
	Examples: []Example{
		{ID: "hierarchy", Title: "标题层级", Description: "H1 至 H4 的尺寸与字重层级。", Snippet: `h1 := ui.Slot(c).Height(40)
h2 := ui.Slot(c).Height(36)
h3 := ui.Slot(c).Height(32)
h4 := ui.Slot(c).Height(30)
components.Typography(canvas, theme, h1.Rect(), components.TypographyH1, "Heading 1")
components.Typography(canvas, theme, h2.Rect(), components.TypographyH2, "Heading 2")
components.Typography(canvas, theme, h3.Rect(), components.TypographyH3, "Heading 3")
components.Typography(canvas, theme, h4.Rect(), components.TypographyH4, "Heading 4")`},
		{ID: "content", Title: "正文角色", Description: "段落、引导、大号与小号正文。", Snippet: `lead := ui.Slot(c).Height(32)
body := ui.Slot(c).Height(28)
large := ui.Slot(c).Height(28)
small := ui.Slot(c).Height(24)
components.Typography(canvas, theme, lead.Rect(),
    components.TypographyLead, "A complete type system for native desktop interfaces")
components.Typography(canvas, theme, body.Rect(),
    components.TypographyParagraph, "Paragraph text uses the theme's font size and foreground color.")
components.Typography(canvas, theme, large.Rect(),
    components.TypographyLarge, "Large · Key note")
components.Typography(canvas, theme, small.Rect(),
    components.TypographySmall, "Small · Compact labels and supporting copy")`},
		{ID: "muted", Title: "强调与弱化", Description: "正文、强调文字与弱化说明。", Snippet: `body := ui.Slot(c).Height(32)
strong := ui.Slot(c).Height(30)
note := ui.Slot(c).Height(26)
components.Typography(canvas, theme, body.Rect(),
    components.TypographyParagraph, "Body copy stays clear, stable, and readable.")
components.Typography(canvas, theme, strong.Rect(),
    components.TypographyLarge, "Key results use a heavier weight.")
components.Typography(canvas, theme, note.Rect(),
    components.TypographyMuted, "Secondary notes use the theme's muted foreground.")`},
	},
}

// typographyLine is one line of the specimen: the role the component paints it in,
// the copy in both preview languages, and the room it needs.
type typographyLine struct {
	role    components.TypographyVariant
	text    string
	english string
	height  float32
}

var (
	typographyHierarchy = [...]typographyLine{
		{components.TypographyH1, "一级标题", "Heading 1", 40},
		{components.TypographyH2, "二级标题", "Heading 2", 36},
		{components.TypographyH3, "三级标题", "Heading 3", 32},
		{components.TypographyH4, "四级标题", "Heading 4", 30},
	}
	typographyContent = [...]typographyLine{
		{components.TypographyLead, "原生桌面界面的完整文字系统", "A complete type system for native desktop interfaces", 32},
		{components.TypographyParagraph, "段落文字使用主题字号与前景色。", "Paragraph text uses the theme's font size and foreground color.", 28},
		{components.TypographyLarge, "Large · 关键说明", "Large · Key note", 28},
		{components.TypographySmall, "Small · 紧凑标签与辅助内容", "Small · Compact labels and supporting copy", 24},
	}
	typographyMuted = [...]typographyLine{
		{components.TypographyParagraph, "正文保持清晰、稳定且可读。", "Body copy stays clear, stable, and readable.", 32},
		{components.TypographyLarge, "关键结果使用更高字重。", "Key results use a heavier weight.", 30},
		{components.TypographyMuted, "次要说明使用主题的弱化前景色。", "Secondary notes use the theme's muted foreground.", 26},
	}
)

// The specimens are the component's roles themselves, painted in slots: ui.Text
// would only imitate their size and weight.
func (state *State) paintTypography(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	lines := typographyHierarchy[:]
	switch state.exampleID {
	case "content":
		lines = typographyContent[:]
	case "muted":
		lines = typographyMuted[:]
	}
	var slots [len(typographyHierarchy)]ui.Box
	state.drawPage(canvas, theme, size, 600, state.Text("Typography · 文字角色", "Typography · Text roles"), func(c ui.C) {
		ui.Column(c).Gap(6).Children(func() {
			for index, line := range lines {
				slots[index] = ui.Slot(c).Height(line.height)
			}
		})
	})
	for index, line := range lines {
		components.Typography(canvas, theme, slots[index].Rect(), line.role, state.Text(line.text, line.english))
	}
}
