//go:build vivahead && (darwin || windows || linux)

package textview_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/textview"
)

// TestVisualCapture is an opt-in frame for reviewing the same textview states in
// both themes. VIVA_TEXTVIEW_SHOTS names the output directory.
func TestVisualCapture(t *testing.T) {
	dir := os.Getenv("VIVA_TEXTVIEW_SHOTS")
	if dir == "" {
		t.Skip("set VIVA_TEXTVIEW_SHOTS to capture textview frames")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		name, theme := "light", ui.LightTheme()
		if dark {
			name, theme = "dark", ui.DarkTheme()
		}
		for _, siblings := range []bool{false, true} {
			scene := "unicode"
			if siblings {
				scene = "siblings-menu"
			}
			t.Run(name+"/"+scene, func(t *testing.T) {
				var views [4]textview.View
				page := ui.NewPage(ui.PageConfig{Theme: &theme, View: func(c ui.C) {
					ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
						if !siblings {
							ui.Text(c, "Selectable text / 可选择文本").Bold()
							views[0].Declare(c, textview.Spec{
								Label: "Unicode output", Source: textview.Strings{
									"Combining accent: cafe\u0301",
									"Emoji sequence: a👩‍💻b",
									"Flag: a🇨🇳b",
									"Indic conjunct: क्ष",
									"中文输出：选择并复制完整字符。",
								},
								Font:       gfx.TextStyle{Size: 22, Color: theme.Foreground},
								LineHeight: 46, Selection: theme.Selection,
							})
							return
						}
						ui.Text(c, "Sibling views / 同级视图").Bold()
						for i, line := range []string{"First plain output / 第一段", "Second plain output / 第二段"} {
							views[i].Declare(c, textview.Spec{
								Label: line, Source: textview.Strings{line},
								Font:       gfx.TextStyle{Size: 17, Color: theme.Foreground},
								LineHeight: 32, Selection: theme.Selection,
							})
						}
						for i := 2; i < len(views); i++ {
							views[i].Declare(c, textview.Spec{
								Label:      []string{"First scroll output", "Second scroll output"}[i-2],
								Source:     textview.Strings{"Process output / 程序输出", "Context menu target / 菜单目标"},
								Font:       gfx.TextStyle{Size: 17, Color: theme.Foreground},
								LineHeight: 30, Scroll: true, Selection: theme.Selection,
								Menu: func(menu *ui.MenuItems) { menu.Item("Copy line / 复制行") },
							}).Height(68)
						}
						ui.Button(c, "After the views / 后续按钮")
					})
				}})
				session, err := headless.Open(ui.Configure(page, app.Config{
					Title: "Textview", Width: 800, Height: 420, WindowMode: app.WindowFrameless,
				}), nil, headless.Options{Scale: 2, Strict: true})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(session.Close)
				if siblings {
					at, ok := views[1].PointOf(textview.Point{Byte: 2})
					if !ok {
						t.Fatal("second plain view is not on screen")
					}
					if err := session.DoubleClick(at.X, at.Y); err != nil {
						t.Fatal(err)
					}
					if got := views[1].Text(); got != "Second" {
						t.Fatalf("second view selected %q, want Second", got)
					}
					at, ok = views[3].PointOf(textview.Point{Line: 1, Byte: 2})
					if !ok {
						t.Fatal("second scroll view is not on screen")
					}
					if err := session.RightClick(at.X, at.Y); err != nil {
						t.Fatal(err)
					}
					if line, ok := views[3].MenuLine(); !ok || line != 1 {
						t.Fatalf("menu targets line %d (%v), want 1", line, ok)
					}
				}
				if err := session.CheckStrict(); err != nil {
					t.Fatal(err)
				}
				if err := session.Screenshot(filepath.Join(dir, scene+"-"+name+".png")); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
