//go:build darwin || windows || linux

package main

import (
	"fmt"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var (
	tree = ui.Tree{}
	page = tree.Feature("parity")

	idPrimary   = tree.ID(page, "primary")
	idSecondary = tree.ID(page, "secondary")
	idOutline   = tree.ID(page, "outline")
	idGhost     = tree.ID(page, "ghost")
	idDanger    = tree.ID(page, "danger")
	idSwitchOn  = tree.ID(page, "switch-on")
	idSwitchOff = tree.ID(page, "switch-off")
	idCheckOn   = tree.ID(page, "check-on")
	idCheckOff  = tree.ID(page, "check-off")
	idName      = tree.ID(page, "name")
	idHost      = tree.ID(page, "host")
	idSlider    = tree.ID(page, "slider")
	idSlider2   = tree.ID(page, "slider2")
	idList      = tree.ID(page, "list")
	idCancel    = tree.ID(page, "cancel")
	idDelete    = tree.ID(page, "delete")
	idClose     = tree.ID(page, "close")
	idDismiss   = tree.ID(page, "dismiss")
	idDialog    = tree.ID(page, "dialog")

	scopeChoice  = tree.Sub(page, tree.Name("choice"))
	scopeSelect  = tree.Sub(page, tree.Name("select"))
	scopeSelect2 = tree.Sub(page, tree.Name("select2"))
	scopeRows    = tree.Sub(page, tree.Name("rows"))
	scopeHeads   = tree.Sub(page, tree.Name("heads"))
	scopeTabs    = tree.Sub(page, tree.Name("tabs"))
	scopeTabs2   = tree.Sub(page, tree.Name("tabs2"))
	scopeListRow = tree.Sub(page, tree.Name("list-rows"))
)

// Weights are the theme's four, by the names the specimen sheets use.
var weights = []struct {
	name   string
	weight float32
}{{"regular", ui.WeightRegular}, {"medium", ui.WeightMedium}, {"semibold", ui.WeightSemibold}, {"bold", ui.WeightBold}}

// scene is one page of the catalogue. build declares it; the scene keeps the
// state its widgets need.
type scene struct {
	name          string
	width, height int
	build         func(f *flex.Frame, theme ui.Theme)
	// before runs ahead of each paint with the view, for scroll offsets.
	before func(view *flex.View)
}

// catalogue returns fresh scenes: they hold widget state, so every run gets its own.
func catalogue() []*scene {
	return []*scene{
		formScene(),
		selectOpenScene(),
		tableScene(),
		tabsScene(),
		dialogScene(),
		listClipScene(),
		cardsScene(),
		progressSliderScene(),
		specimenLatin(),
		specimenCJK(),
		specimenEmoji(),
		specimenMixed(),
		specimenMono(),
		specimenWrap(),
		specimenScripts(),
		specimenBidi(),
		specimenWrapScripts(),
		baselineScene(),
	}
}

func formScene() *scene {
	var name, host flex.Field
	name.SetText("Ada Lovelace")
	slider := flex.SliderState{Value: 35}
	selectState := flex.SelectState{}
	return &scene{name: "form", width: 560, height: 640, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(24).Gap(14).Children(func() {
			f.Text("New connection").FontSize(theme.Metrics.FontSizeL).Bold()
			f.Text("Connect to a server over SSH with a key or a password.").Muted()
			f.Row().Gap(8).Children(func() {
				f.Button(idPrimary, "Connect")
				f.Button(idSecondary, "Save").Secondary()
				f.Button(idOutline, "Cancel").Outline()
				f.Button(idGhost, "Help").Ghost()
				f.Button(idDanger, "Delete").Destructive()
			})
			f.Row().Gap(8).Items(flex.Center).Children(func() {
				f.Button(idPrimary, "Small").Small()
				f.Button(idPrimary, "Default")
				f.Button(idPrimary, "Large").Large()
			})
			f.Row().Gap(24).Items(flex.Center).Children(func() {
				f.Row().Gap(8).Items(flex.Center).Children(func() {
					f.Switch(idSwitchOn, true)
					f.Text("Keep alive")
				})
				f.Row().Gap(8).Items(flex.Center).Children(func() {
					f.Switch(idSwitchOff, false)
					f.Text("Compression")
				})
				f.Row().Gap(8).Items(flex.Center).Children(func() {
					f.Checkbox(idCheckOn, true)
					f.Text("Agent forwarding")
				})
				f.Row().Gap(8).Items(flex.Center).Children(func() {
					f.Checkbox(idCheckOff, false)
					f.Text("X11")
				})
			})
			f.Field(&name, idName, "Name", "Name")
			f.Field(&host, idHost, "Host", "hostname or address")
			f.Choice(scopeChoice, []string{"Password", "Key", "Agent"}, 1)
			f.Select(scopeSelect, &selectState, []string{"OpenSSH", "PuTTY", "Mosh"}, 0).Placeholder("Client").Width(220)
			f.Slider(idSlider, &slider).Range(0, 100).FillWidth()
			f.Progress(0.6).FillWidth()
		})
	}}
}

func selectOpenScene() *scene {
	state := flex.SelectState{Open: true}
	return &scene{name: "select-open", width: 420, height: 320, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(24).Gap(12).Children(func() {
			f.Text("Shell").Bold()
			f.Select(scopeSelect2, &state, []string{"zsh", "bash", "fish", "PowerShell", "cmd"}, 1).Placeholder("Shell").Width(240)
		})
	}}
}

func tableScene() *scene {
	var state flex.TableState
	state.Select(2)
	state.SortBy(1)
	data := flex.TableRows{
		{"web-01", "Running", "12 ms"}, {"web-02", "Running", "9 ms"}, {"db-primary", "Degraded", "48 ms"},
		{"db-replica", "Running", "15 ms"}, {"cache", "Stopped", "-"}, {"build-agent-long-name", "Running", "103 ms"},
		{"监控服务器", "运行中", "7 ms"},
	}
	columns := []components.TableColumn{{Width: 200}, {Width: 120}, {Alignment: components.TableAlignRight}}
	return &scene{name: "table", width: 560, height: 360, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(24).Gap(12).Children(func() {
			f.Text("Servers").FontSize(theme.Metrics.FontSizeL).Bold()
			f.Table(&state, flex.TableSpec{
				Rows: scopeRows, Header: scopeHeads, Columns: columns,
				Headers: []string{"Name", "State", "Latency"}, Sortable: []bool{true, true, true}, Source: &data,
			}).FillWidth()
		})
	}}
}

func tabsScene() *scene {
	return &scene{name: "tabs", width: 520, height: 300, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(24).Gap(16).Children(func() {
			f.Tabs(scopeTabs, []string{"General", "Keys", "Advanced"}, 1)
			f.Text("Keys are read from ~/.ssh and the agent.").Muted()
			f.Tabs(scopeTabs2, []string{"Overview", "Files", "Terminal", "Monitor"}, 2).Line()
			f.Text("A line variant with an underline.").Muted()
		})
	}}
}

func dialogScene() *scene {
	return &scene{name: "dialog", width: 640, height: 420, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(24).Gap(12).Children(func() {
			f.Text("Hosts").FontSize(theme.Metrics.FontSizeL).Bold()
			f.Text("A page behind a dialog, dimmed by the scrim.").Muted()
			f.Row().Gap(8).Children(func() {
				f.Button(idPrimary, "New host")
				f.Button(idOutline, "Import").Outline()
			})
		})
		d := f.Dialog(flex.DialogSpec{Title: "Delete host?", Description: "This removes the saved connection. It cannot be undone.", Width: 400, Scope: idDialog, DismissID: idDismiss, CloseID: idClose})
		d.Body(func() { f.Text("web-01 (10.0.0.12)") })
		d.Footer(func() {
			f.Button(idCancel, "Cancel").Outline()
			f.Button(idDelete, "Delete").Destructive()
		})
	}}
}

func listClipScene() *scene {
	return &scene{name: "list-clip", width: 360, height: 300,
		before: func(view *flex.View) { view.ScrollState(idList).ScrollTo(37) },
		build: func(f *flex.Frame, theme ui.Theme) {
			f.Column().Pad(24).Gap(12).Children(func() {
				f.Text("Recent").Bold()
				f.ScrollArea(idList).Height(180).FillWidth().Background(theme.Card).Radius(theme.Metrics.RadiusControl).Border(1, theme.Border).Children(func() {
					for i := 0; i < 9; i++ {
						f.Row().Pad(8).Gap(8).Items(flex.Center).Children(func() {
							f.Column().Size(16, 16).Background(theme.Muted).Radius(4).Children(func() {})
							f.Text(fmt.Sprintf("session-%02d.log", i))
						})
					}
				})
			})
		}}
}

func cardsScene() *scene {
	return &scene{name: "cards", width: 560, height: 460, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(24).Gap(16).Children(func() {
			f.Column().Pad(16).Gap(8).Background(theme.Card).Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).Children(func() {
				f.Text("Card title").Bold()
				f.Text("Card body in muted text.").Muted()
			})
			f.Row().Gap(12).Children(func() {
				for _, radius := range []float32{0, 2, 4, 7.5, 11, 16} {
					f.Column().Size(64, 64).Background(theme.Muted).Radius(radius).Border(1, theme.Border).Children(func() {})
				}
			})
			f.Row().Gap(12).Children(func() {
				for _, width := range []float32{0.5, 1, 1.5, 2, 3} {
					f.Column().Size(64, 48).Background(theme.Card).Radius(10).Border(width, theme.Primary).Children(func() {})
				}
			})
			f.Row().Gap(12).Children(func() {
				for _, size := range []float32{17, 33, 41, 63} {
					f.Column().Size(size, size).Background(theme.Primary).Radius(size / 2).Children(func() {})
				}
				f.Column().Size(120, 1).Background(theme.Border).Children(func() {})
				f.Column().Size(121, 33).Background(theme.Accent).Radius(5).Children(func() {})
			})
		})
	}}
}

func progressSliderScene() *scene {
	sliders := []*flex.SliderState{{Value: 0}, {Value: 25}, {Value: 50}, {Value: 100}}
	return &scene{name: "progress-slider", width: 480, height: 420, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(24).Gap(14).Children(func() {
			for _, fraction := range []float32{0, 0.33, 0.66, 1} {
				f.Progress(fraction).FillWidth()
			}
			for i, state := range sliders {
				f.Slider(tree.ID(page, fmt.Sprintf("slider-%d", i)), state).Range(0, 100).FillWidth()
			}
			f.Progress(0.5).Height(12).FillWidth()
			f.Row().Gap(12).Children(func() {
				f.Slider(idSlider2, &flex.SliderState{Value: 40}).Vertical().Height(120)
			})
		})
	}}
}

var sizes = []float32{11, 12, 13, 14, 16, 18, 20, 24}

func specimen(name string, height int, sample string, sizes []float32, mono bool) *scene {
	return &scene{name: name, width: 880, height: height, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(16).Gap(2).Children(func() {
			for _, size := range sizes {
				for _, w := range weights {
					text := f.Text(fmt.Sprintf("%g %s  %s", size, w.name, sample)).FontSize(size).Weight(w.weight)
					if mono {
						text.Mono()
					}
				}
			}
		})
	}}
}

func specimenLatin() *scene {
	return specimen("text-latin", 880, "Hamburgefonstiv 0123456789 The quick brown fox jumps over the lazy dog.", sizes, false)
}

func specimenCJK() *scene {
	return specimen("text-cjk", 560, "界面一致性测试。你好，世界！日本語のテキスト 한국어 텍스트", []float32{12, 14, 16, 20, 24}, false)
}

func specimenEmoji() *scene {
	return specimen("text-emoji", 560, "😀 👍🏽 🇨🇳 ✅ ❤️ 🧑‍💻 🚀", []float32{12, 14, 16, 20, 24}, false)
}

func specimenMixed() *scene {
	return specimen("text-mixed", 560, "Motty 连接 host-01 ✅ 已连接 — 3 sessions · ß é ñ ø Ж", []float32{12, 14, 16, 20, 24}, false)
}

func specimenMono() *scene {
	mono := func(face ui.MonoFace, title string) func(f *flex.Frame, theme ui.Theme) {
		return func(f *flex.Frame, theme ui.Theme) {
			f.Text(title).Bold()
			for _, size := range []float32{11, 12, 13, 14, 16} {
				f.Text("ls -la ~/.ssh | grep id_  0Oo1lI |{}[]  界面 ✅").FontSize(size).MonoFace(face)
			}
		}
	}
	system, classic := mono(ui.MonoSystem, "mono system"), mono(ui.MonoClassic, "mono classic")
	return &scene{name: "text-mono", width: 640, height: 360, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(16).Gap(4).Children(func() {
			system(f, theme)
			classic(f, theme)
		})
	}}
}

func specimenWrap() *scene {
	latin := "Viva lays text out in lines: a paragraph breaks at spaces, between East Asian characters, after the separators of a long path such as /usr/local/share/applications/example.desktop, and as a last resort inside a word like pneumonoultramicroscopicsilicovolcanoconiosis."
	cjk := "文本按宽度折行：东亚文字可以在任意两个字符之间断开，但是句号、逗号等标点不会出现在行首。日本語の文章も同じように折り返されます。"
	return &scene{name: "text-wrap", width: 960, height: 560, build: func(f *flex.Frame, theme ui.Theme) {
		f.Row().Pad(16).Gap(16).Children(func() {
			for _, width := range []float32{160, 240, 360} {
				f.Column().Width(width).Gap(12).Pad(8).Border(1, theme.Border).Children(func() {
					f.Paragraph(latin, 12).FontSize(13)
					f.Paragraph(cjk, 12).FontSize(13)
					f.Paragraph(latin, 3).FontSize(16).Weight(ui.WeightSemibold)
				})
			}
		})
	}}
}

// scriptSamples are sentences of the scripts whose widths come from shaped tables,
// each prefixed with its name in Latin so a reader can tell the rows apart.
var scriptSamples = []struct{ name, text string }{
	{"Arabic", "السلام عليكم ورحمة الله وبركاته. هذه تجربة لعرض النص العربي"},
	{"Persian", "سلام دنیا! این یک متن آزمایشی فارسی است"},
	{"Hebrew", "שלום עולם! זוהי דוגמה להצגת טקסט בעברית בממשק המשתמש"},
	{"Thai", "สวัสดีครับ นี่คือตัวอย่างข้อความภาษาไทยในอินเทอร์เฟซผู้ใช้"},
	{"Hindi", "नमस्ते दुनिया! यह यूज़र इंटरफ़ेस में हिन्दी पाठ दिखाने का उदाहरण है"},
	{"Bengali", "ওহে বিশ্ব! এটি ব্যবহারকারী ইন্টারফেসে বাংলা লেখা দেখানোর একটি উদাহরণ"},
	{"Tamil", "வணக்கம் உலகம்! இது பயனர் இடைமுகத்தில் தமிழ் உரையைக் காட்டும் எடுத்துக்காட்டு"},
	{"Telugu", "నమస్కారం ప్రపంచం! ఇది తెలుగు పాఠ్యానికి ఒక ఉదాహరణ"},
	{"Korean", "안녕하세요 세계! 사용자 인터페이스에서 한국어 텍스트를 표시하는 예제입니다"},
	{"Khmer", "សួស្តី​ពិភពលោក ភាសាខ្មែរ ជាឧទាហរណ៍"},
	{"Myanmar", "မင်္ဂလာပါ ကမ္ဘာ မြန်မာဘာသာ စာသား နမူနာ"},
}

// specimenScripts is every complex script at several sizes and weights.
func specimenScripts() *scene {
	return &scene{name: "text-scripts", width: 880, height: 900, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(16).Gap(3).Children(func() {
			for _, sample := range scriptSamples {
				f.Text(sample.name).FontSize(11).Muted()
				f.Text(sample.text).FontSize(16)
				f.Text(sample.text).FontSize(13).Weight(ui.WeightBold)
			}
		})
	}}
}

// bidiSamples are lines whose characters are not all displayed in the order they are stored.
var bidiSamples = []string{
	"The word שלום means peace in Hebrew.",
	"מחיר: 42.50 USD (כולל מע״מ) לכל יחידה",
	"اسم المستخدم: admin و كلمة المرور: 12345",
	"Connect to שרת-01 (192.168.1.1) failed: חיבור נכשל!",
	"(שלום) [עולם] {טוב} «abc» <x>",
	"בדיקה 123 של מספרים 4.5% ו-$7",
	"النسخة 2.0.1 من التطبيق متوفرة الآن",
	"\u200Fhello world! (RLM first)",
}

// specimenBidi shows mixed direction lines, in the order UAX #9 displays them, aligned to
// their start, and the same lines aligned to their end.
func specimenBidi() *scene {
	return &scene{name: "text-bidi", width: 880, height: 640, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(16).Gap(4).Children(func() {
			for _, size := range []float32{13, 16} {
				for _, line := range bidiSamples {
					f.Text(line).FontSize(size).Width(560)
				}
			}
			f.Text("end-aligned").FontSize(11).Muted()
			for _, line := range bidiSamples[:3] {
				f.Text(line).FontSize(14).Width(560).TextEnd()
			}
		})
	}}
}

// specimenWrapScripts wraps long paragraphs of the complex scripts at three widths: where
// a line breaks is the same on every platform.
func specimenWrapScripts() *scene {
	paragraphs := []string{
		"هذه فقرة طويلة من النص العربي تُعرض في عمود ضيق لاختبار الالتفاف الأسطري واتجاه الكتابة من اليمين إلى اليسار في واجهة المستخدم.",
		"זוהי פסקה ארוכה של טקסט בעברית המוצגת בעמודה צרה כדי לבדוק את שבירת השורות ואת כיוון הכתיבה מימין לשמאל בממשק המשתמש.",
		"ประเทศไทยมีภาษาไทยเป็นภาษาราชการ ข้อความนี้ไม่มีช่องว่างระหว่างคำจึงต้องตัดบรรทัดตามพยางค์เพื่อให้พอดีกับความกว้างของคอลัมน์",
		"यह हिन्दी भाषा में एक लंबा अनुच्छेद है जो संकरे स्तंभ में पंक्ति विभाजन और संयुक्ताक्षरों की जाँच के लिए दिखाया गया है।",
		"한국어 문단은 단어 사이의 공백에서 줄바꿈이 됩니다. 아주 긴 단어는 폭에 맞지 않으면 음절 사이에서 나뉩니다 대한민국만세대한민국만세.",
	}
	return &scene{name: "text-wrap-scripts", width: 960, height: 720, build: func(f *flex.Frame, theme ui.Theme) {
		f.Row().Pad(16).Gap(16).Children(func() {
			for _, width := range []float32{160, 240, 360} {
				f.Column().Width(width).Gap(10).Pad(8).Border(1, theme.Border).Children(func() {
					for _, paragraph := range paragraphs {
						f.Paragraph(paragraph, 12).FontSize(13)
					}
				})
			}
		})
	}}
}

// baselineScene is the probe for where a platform puts the glyphs inside the
// line box Viva gives a text: caps sit on the baseline, x-height letters show
// the x-height, and the descender shows the line's bottom. Every box has a
// fixed width, so the page does not depend on text metrics.
func baselineScene() *scene {
	return &scene{name: "baseline", width: 760, height: 700, build: func(f *flex.Frame, theme ui.Theme) {
		f.Column().Pad(12).Gap(4).Children(func() {
			for _, size := range sizes {
				for _, w := range weights[:3:3] {
					f.Row().Gap(8).Children(func() {
						f.Text("HHHH").FontSize(size).Weight(w.weight).Width(110)
						f.Text("xxxx").FontSize(size).Weight(w.weight).Width(110)
						f.Text("Hpgy").FontSize(size).Weight(w.weight).Width(110)
						f.Text("HHHH").FontSize(size).Weight(w.weight).Mono().Width(110)
					})
				}
			}
		})
	}}
}
