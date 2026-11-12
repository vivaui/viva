package windows

import (
	"fmt"
	"image"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// pageLabels is a page of proportional labels replaced together, as a settings
// page or a host page does when it appears: short lines of mixed script, one
// size, one color, at the positions layout gives them.
func pageLabels(generation int, color platform.Color) []platform.TextLabel {
	values := []string{
		"通用", "终端", "连接", "文件", "安全", "备份与同步", "AI 助手", "MCP",
		"键盘快捷键", "关于", "搜索设置", "公开命令库", "命令补全", "记住命令",
		"命令历史", "Option 键", "终端字号", "终端行高", "字体回退", "默认编辑器",
		"路径补全", "自动填充", "快捷键", "会话日志", "端口转发", "代理",
		"已知主机", "快捷命令", "笔记", "本地终端", "外观", "语言", "更新",
	}
	labels := make([]platform.TextLabel, len(values))
	for index, value := range values {
		labels[index] = platform.TextLabel{
			Value:    fmt.Sprintf("%s %d", value, generation),
			X:        24,
			Y:        float32(index) * 26,
			Width:    float32(80 + len(value)*14),
			Height:   24,
			FontSize: 13,
			Weight:   400,
			Color:    color,
			Identity: gfx.TextIdentity{Owner: 1, Item: uint64(index)},
		}
	}
	return labels
}

// renderPage draws the given frames and returns the offscreen that drew them
// and its last image. batch picks the batched path or the per-label path.
func renderPage(t *testing.T, width, height int, clear platform.Color, batch bool, frames ...[]platform.TextLabel) (*Offscreen, *image.RGBA) {
	t.Helper()
	offscreen, err := NewOffscreen(width, height, 2, clear)
	if err != nil {
		t.Fatal(err)
	}
	offscreen.renderer.noLabelBatch = !batch
	for _, labels := range frames {
		if err := offscreen.Set(nil, labels); err != nil {
			offscreen.Close()
			t.Fatal(err)
		}
	}
	rendered, err := offscreen.Image()
	if err != nil {
		offscreen.Close()
		t.Fatal(err)
	}
	if batch && offscreen.renderer.labelBatches == 0 {
		offscreen.Close()
		t.Fatal("the page was not drawn by the batch path")
	}
	return offscreen, rendered
}

func comparePixels(t *testing.T, want, got *image.RGBA) {
	t.Helper()
	if want.Bounds() != got.Bounds() {
		t.Fatalf("bounds differ: %v vs %v", want.Bounds(), got.Bounds())
	}
	differing := 0
	first := ""
	for offset := 0; offset+3 < len(want.Pix); offset += 4 {
		if want.Pix[offset] == got.Pix[offset] && want.Pix[offset+1] == got.Pix[offset+1] &&
			want.Pix[offset+2] == got.Pix[offset+2] && want.Pix[offset+3] == got.Pix[offset+3] {
			continue
		}
		differing++
		if first == "" {
			index := offset / 4
			x, y := index%(want.Stride/4), index/(want.Stride/4)
			first = fmt.Sprintf("(%d,%d): %v vs %v", x, y, want.Pix[offset:offset+4], got.Pix[offset:offset+4])
		}
	}
	if differing != 0 {
		t.Fatalf("%d of %d pixels differ; first %s", differing, len(want.Pix)/4, first)
	}
}

// A page drawn in one batched Direct2D session must be the page the per-label
// sessions drew: the switch is an implementation change, not a picture change.
func TestBatchedLabelsDrawTheSamePage(t *testing.T) {
	light := platform.Color{Red: 0.9, Green: 0.9, Blue: 0.9, Alpha: 1}
	dark := platform.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}
	for _, test := range []struct {
		name  string
		clear platform.Color
		color platform.Color
	}{
		{"dark theme, light text", platform.Color{Red: 0.05, Green: 0.05, Blue: 0.05, Alpha: 1}, light},
		{"light theme, dark text", platform.Color{Red: 0.95, Green: 0.95, Blue: 0.95, Alpha: 1}, dark},
		{"grayscale on a composed window", platform.Color{Alpha: 0.5}, light},
	} {
		t.Run(test.name, func(t *testing.T) {
			labels := pageLabels(0, test.color)
			perLabel, want := renderPage(t, 700, 900, test.clear, false, labels)
			defer perLabel.Close()
			batched, got := renderPage(t, 700, 900, test.clear, true, labels)
			defer batched.Close()
			comparePixels(t, want, got)
		})
	}
}

// A clipped label rasterizes only its visible window; the batch must keep that
// window and not let the shifted glyphs paint into the next label's cells.
func TestBatchedClippedLabelsMatch(t *testing.T) {
	color := platform.Color{Red: 0.9, Green: 0.9, Blue: 0.9, Alpha: 1}
	frames := make([][]platform.TextLabel, 2)
	for frame := range frames {
		frames[frame] = pageLabels(0, color)[:8]
		for index := range frames[frame] {
			frames[frame][index].Clipped = true
			frames[frame][index].Clip = gfx.Rect{X: 60, Y: 0, Width: 90, Height: 24}
			if frame == 1 {
				frames[frame][index].Value += " changed"
			}
		}
	}
	perLabel, want := renderPage(t, 500, 260, platform.Color{Alpha: 1}, false, frames...)
	defer perLabel.Close()
	batched, got := renderPage(t, 500, 260, platform.Color{Alpha: 1}, true, frames...)
	defer batched.Close()
	comparePixels(t, want, got)
}

// Labels the batch cannot draw exactly — color glyphs, a color whose channels
// need different ClearType backgrounds, italics and weights — must still come
// out as the per-label path draws them, whether batched or left to that path.
func TestBatchedSpecialLabelsMatch(t *testing.T) {
	labels := []platform.TextLabel{
		{Value: "部署 🚀 完成", X: 10, Y: 0, Width: 220, Height: 30, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}},
		{Value: "切换到主机", X: 10, Y: 34, Width: 220, Height: 30, FontSize: 14, Color: platform.Color{Red: 0.9, Green: 0.1, Blue: 0.1, Alpha: 1}},
		{Value: "italic source code", X: 10, Y: 68, Width: 260, Height: 30, FontSize: 14, Italic: true, Color: platform.Color{Red: 0.8, Green: 0.8, Blue: 0.2, Alpha: 1}},
		{Value: "semibold heading", X: 10, Y: 102, Width: 260, Height: 30, FontSize: 15, Weight: 600, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}},
		{Value: "muted note", X: 10, Y: 136, Width: 160, Height: 28, FontSize: 13, Color: platform.Color{Red: 0.6, Green: 0.6, Blue: 0.6, Alpha: 0.6}},
	}
	for _, clear := range []platform.Color{{Alpha: 1}, {Red: 1, Green: 1, Blue: 1, Alpha: 1}} {
		perLabel, want := renderPage(t, 400, 200, clear, false, labels)
		defer perLabel.Close()
		batched, got := renderPage(t, 400, 200, clear, true, labels)
		defer batched.Close()
		comparePixels(t, want, got)
	}
}

// A second frame of the same page must not rasterize its labels again: the
// batch path keeps the retained textures the per-label path kept.
func TestBatchedPageKeepsItsTextures(t *testing.T) {
	offscreen, err := NewOffscreen(700, 900, 2, platform.Color{Alpha: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer offscreen.Close()
	white := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	if err := offscreen.Set(nil, pageLabels(0, white)); err != nil {
		t.Fatal(err)
	}
	if _, err := offscreen.Image(); err != nil {
		t.Fatal(err)
	}
	built := offscreen.renderer.labelRebuilds
	if err := offscreen.Set(nil, pageLabels(0, white)); err != nil {
		t.Fatal(err)
	}
	if _, err := offscreen.Image(); err != nil {
		t.Fatal(err)
	}
	if got := offscreen.renderer.labelRebuilds - built; got != 0 {
		t.Fatalf("an unchanged page rasterized %d labels, want 0", got)
	}
}

// BenchmarkPageSwitch is a page of thirty-three labels replaced in one frame,
// as choosing a settings category does.
func BenchmarkPageSwitch(b *testing.B) {
	for _, test := range []struct {
		name  string
		batch bool
	}{{"batched", true}, {"per-label", false}} {
		b.Run(test.name, func(b *testing.B) {
			offscreen, err := NewOffscreen(900, 900, 2, platform.Color{Alpha: 1})
			if err != nil {
				b.Fatal(err)
			}
			defer offscreen.Close()
			offscreen.renderer.noLabelBatch = !test.batch
			width, height := offscreen.size()
			white := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
			draw := func(labels []platform.TextLabel) error {
				if err := offscreen.Set(nil, labels); err != nil {
					return err
				}
				return offscreen.renderer.draw(width, height, 2)
			}
			if err := draw(pageLabels(0, white)); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				if err := draw(pageLabels(index+1, white)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
