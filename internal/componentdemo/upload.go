package componentdemo

import (
	"slices"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/upload"
)

var uploadComponent = Component{
	ID: "upload", Title: "Upload", Category: "Forms", Summary: "文件上传：拖入或选择文件，列表显示进度、失败重试与移除。",
	Source: "ui/upload/upload.go", paint: (*State).paintUpload,
	Examples: []Example{
		{ID: "default", Title: "拖入或选择文件", Description: "控件只管选择与展示；传输由页面负责，页面通过 Status、Progress 告诉控件每个文件传到哪了。", Snippet: `m.upload = upload.New()
m.upload.View(c, "Attachments", upload.Spec{
    Items: m.files, Multiple: true, Accept: []string{".png", ".jpg"},
}).FillWidth()
for _, file := range m.upload.Added() {
    m.files = append(m.files, upload.Item{Name: file.Name(), Size: file.Size()})
}
if i, ok := m.upload.Removed(); ok {
    m.files = slices.Delete(m.files, i, i+1)
}`, PageSnippet: `m.upload = upload.New()
m.upload.View(c, "Attachments", upload.Spec{
    Items: m.files, Multiple: true, Accept: []string{".png", ".jpg"},
}).FillWidth()
for _, file := range m.upload.Added() {
    m.files = append(m.files, upload.Item{Name: file.Name(), Size: file.Size()})
}
if i, ok := m.upload.Removed(); ok {
    m.files = slices.Delete(m.files, i, i+1)
}`, PageSnippetZh: `m.upload = upload.New()
m.upload.View(c, "Attachments", upload.Spec{
    Items: m.files, Multiple: true, Accept: []string{".png", ".jpg"},
}).FillWidth()
for _, file := range m.upload.Added() {
    m.files = append(m.files, upload.Item{Name: file.Name(), Size: file.Size()})
}
if i, ok := m.upload.Removed(); ok {
    m.files = slices.Delete(m.files, i, i+1) // 并停止传输
}`},
	},
}

var uploadAccept = []string{".png", ".jpg", ".md", ".mov", ".gz"}

type uploadDemo struct {
	up    *upload.Upload
	items []upload.Item
}

var uploadDemos = map[*State]*uploadDemo{}

func uploadDemoFor(state *State) *uploadDemo {
	if demo := uploadDemos[state]; demo != nil {
		return demo
	}
	demo := &uploadDemo{up: upload.New(), items: []upload.Item{
		{Name: "release-notes.md", Size: 18 << 10, Status: upload.Done},
		{Name: "product-demo.mov", Size: 42 << 20, Status: upload.Uploading, Progress: 0.64},
		{Name: "backup.tar.gz", Size: 1 << 30, Status: upload.Failed, Error: state.Text("网络中断", "Connection lost")},
	}}
	uploadDemos[state] = demo
	return demo
}

func (state *State) paintUpload(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := uploadDemoFor(state)
	state.drawPage(canvas, theme, size, 520, state.Text("Upload · 文件上传", "Upload"), func(c ui.C) {
		demo.up.View(c, state.Text("附件", "Attachments"), upload.Spec{
			Items: demo.items, Multiple: true, Accept: uploadAccept,
			Title: state.Text("把文件拖到这里", "Drag files here"), Choose: state.Text("选择文件", "Choose files"),
			Hint:        state.Text("PNG、JPG、MD、MOV 或 GZ", "PNG, JPG, MD, MOV or GZ"),
			RemoveLabel: state.Text("移除", "Remove"), RetryLabel: state.Text("重试", "Retry"),
		}).FillWidth()
		for _, file := range demo.up.Added() {
			demo.items = append(demo.items, upload.Item{Name: file.Name(), Size: file.Size()})
		}
		if i, ok := demo.up.Removed(); ok {
			demo.items = slices.Delete(demo.items, i, i+1)
		}
		if i, ok := demo.up.Retried(); ok {
			demo.items[i].Status, demo.items[i].Error = upload.Pending, ""
		}
	})
}
