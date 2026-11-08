//go:build darwin || windows || linux

package main

import (
	"path/filepath"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// MetricsFile is the platform text engine's answer for a fixed grid of strings,
// sizes and weights, independent of any scene: the raw material of the text
// measurement comparison.
type MetricsFile struct {
	Platform string        `json:"platform"`
	Lines    []MetricsLine `json:"lines"`
	Mono     []MonoCell    `json:"mono"`
}

// MetricsLine is one string at one style.
type MetricsLine struct {
	Sample string   `json:"sample"`
	Size   float32  `json:"size"`
	Weight float32  `json:"weight"`
	M      Measured `json:"m"`
}

// MonoCell is the fixed-pitch cell of a face at a size.
type MonoCell struct {
	Face   uint8   `json:"face"`
	Size   float32 `json:"size"`
	Width  float32 `json:"width"`
	Height float32 `json:"height"`
}

// samples are grouped by script, because the font that draws them differs.
var samples = []struct{ name, text string }{
	{"latin-word", "Hamburgefonstiv"},
	{"latin-sentence", "The quick brown fox jumps over the lazy dog."},
	{"digits", "0123456789"},
	{"ui-label", "Connect"},
	{"ui-label2", "Cancel"},
	{"cjk-zh", "界面一致性测试"},
	{"cjk-ja", "日本語のテキスト"},
	{"cjk-ko", "한국어 텍스트"},
	{"emoji", "😀👍🏽✅"},
	{"mixed", "Motty 连接 host-01 ✅ 已连接"},
	{"accents", "ß é ñ ø Ж"},
	{"space", " "},
}

func writeMetrics(out string, measure text.MeasureFunc) error {
	file := MetricsFile{Platform: platformName}
	for _, s := range samples {
		for _, size := range []float32{11, 12, 13, 14, 16, 18, 20, 24} {
			for _, w := range weights {
				m, err := measure(s.text, size, w.weight)
				if err != nil {
					continue
				}
				file.Lines = append(file.Lines, MetricsLine{Sample: s.name, Size: size, Weight: w.weight, M: *measured(m)})
			}
		}
	}
	for _, face := range []ui.MonoFace{ui.MonoSystem, ui.MonoClassic} {
		for _, size := range []float32{11, 12, 13, 14, 16, 20} {
			if width, height, err := nativeMono(size, face); err == nil {
				file.Mono = append(file.Mono, MonoCell{Face: uint8(face), Size: size, Width: width, Height: height})
			}
		}
	}
	return writeJSON(filepath.Join(out, "metrics.json"), file)
}

func sampleText(name string) string {
	for _, s := range samples {
		if s.name == name {
			return s.text
		}
	}
	return name
}
