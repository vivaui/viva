package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/avatar"
	"github.com/vivaui/viva/ui/comment"
)

var commentComponent = Component{
	ID: "comment", Title: "Comment", Category: "Data display", Summary: "评论：头像、作者与时间、正文、操作按钮和嵌套回复。",
	Source: "ui/comment/comment.go", paint: (*State).paintComment,
	Examples: []Example{
		{ID: "default", Title: "带回复的评论", Description: "操作是页面自己的按钮，回复是页面在 Replies 里继续声明的评论，任意深度都是同一个函数。", Snippet: `comment.View(c, comment.Spec{
    Avatar: &m.avatar, Author: "Ada", Time: "2h ago", Content: "Looks good to me.",
    Actions: m.actions, // declares the Reply and Like buttons
    Replies: m.replies, // declares comments in turn
}).FillWidth()`, PageSnippet: `comment.View(c, comment.Spec{
    Avatar: &m.avatar, Author: "Ada", Time: "2h ago", Content: "Looks good to me.",
    Actions: m.actions, // declares the Reply and Like buttons
    Replies: m.replies, // declares comments in turn
}).FillWidth()`, PageSnippetZh: `comment.View(c, comment.Spec{
    Avatar: &m.avatar, Author: "Ada", Time: "2h ago", Content: "Looks good to me.",
    Actions: m.actions, // 声明“回复”“赞”按钮
    Replies: m.replies, // 继续声明回复的评论
}).FillWidth()`},
	},
}

type commentDemo struct {
	avatars [2]*avatar.State
	liked   int
	c       ui.C
	state   *State
	actions func()
	reply   func()
}

var commentDemos = map[*State]*commentDemo{}

func commentDemoFor(state *State) *commentDemo {
	if demo := commentDemos[state]; demo != nil {
		return demo
	}
	demo := &commentDemo{state: state}
	demo.avatars[0] = avatar.New().Initials("AL")
	demo.avatars[1] = avatar.New().Initials("GH")
	demo.actions = func() {
		ui.Button(demo.c, state.Text("回复", "Reply")).Ghost().Small()
		if ui.Button(demo.c, state.Text("赞", "Like")).Ghost().Small().Clicked() {
			demo.liked++
		}
	}
	demo.reply = func() {
		comment.View(demo.c, comment.Spec{
			Avatar: demo.avatars[1], Author: "Grace", Time: state.Text("1 小时前", "1h ago"),
			Content: state.Text("同意，合并前再补一条测试就好。", "Agreed. One more test before it merges and it is good."),
		}).FillWidth()
	}
	commentDemos[state] = demo
	return demo
}

func (state *State) paintComment(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := commentDemoFor(state)
	state.drawPage(canvas, theme, size, 520, state.Text("Comment · 评论", "Comment"), func(c ui.C) {
		demo.c = c
		comment.View(c, comment.Spec{
			Avatar: demo.avatars[0], Author: "Ada", Time: state.Text("2 小时前", "2h ago"),
			Content: state.Text("这个改动把重复的接线收进了一个组件，读起来清楚多了。", "This change folds the repeated wiring into one component, and it reads much clearer."),
			Actions: demo.actions, Replies: demo.reply,
		}).FillWidth()
	})
}
