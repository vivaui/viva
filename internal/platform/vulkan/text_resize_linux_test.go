//go:build linux

package vulkan

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func BenchmarkVulkanTextLiveResize(b *testing.B) {
	for _, count := range []int{256, 1024} {
		b.Run(fmt.Sprintf("ClippedWidths/%d", count), func(b *testing.B) {
			narrow := resizeTestLabels(count, 60)
			wide := append([]platform.TextLabel(nil), narrow...)
			for i := range wide {
				wide[i].Width++
			}
			var state vulkanText
			if err := state.update(nil, narrow); err != nil {
				b.Fatal(err)
			}
			for _, labels := range [][]platform.TextLabel{wide, narrow} {
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				labels := narrow
				if i&1 == 0 {
					labels = wide
				}
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("Unchanged/%d", count), func(b *testing.B) {
			labels := resizeTestLabels(count, 800)
			var state vulkanText
			if err := state.update(nil, labels); err != nil {
				b.Fatal(err)
			}
			if err := state.update(nil, labels); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("RepeatedClippedWidths/%d", count), func(b *testing.B) {
			narrow := resizeTestLabels(count, 60)
			for i := range narrow {
				narrow[i].Value, narrow[i].Runs = narrow[0].Value, narrow[0].Runs
			}
			wide := append([]platform.TextLabel(nil), narrow...)
			for i := range wide {
				wide[i].Width++
			}
			var state vulkanText
			for _, labels := range [][]platform.TextLabel{narrow, wide, narrow} {
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// The old textures were clipped at the previous width. Growing
				// beyond them requires new pictures, even for duplicate rows.
				for j := range state.labels {
					state.labels[j].view = uintptr(j + 1)
					state.labels[j].scale = 1
					state.labels[j].width = int(state.labels[j].label.Width)
				}
				labels := narrow
				if i&1 == 0 {
					labels = wide
				}
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestVulkanTextResizeKeepsDistinctDuplicateTextures(t *testing.T) {
	labels := resizeTestLabels(3, 800)
	labels[1].Value, labels[1].Runs = labels[0].Value, labels[0].Runs
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	for i := range state.labels {
		state.labels[i].view = uintptr(i + 1)
	}
	next := []platform.TextLabel{labels[2], labels[0], labels[1]}
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	seen := make(map[uintptr]bool)
	for i, label := range state.labels {
		if label.view == 0 || seen[label.view] {
			t.Fatalf("slot %d retained invalid or duplicate image view %d", i, label.view)
		}
		seen[label.view] = true
	}
	if state.labels[0].view != 3 {
		t.Fatalf("moved unique label view = %d, want 3", state.labels[0].view)
	}
}

func TestVulkanTextResizeInvalidLabelKeepsPreviousFrame(t *testing.T) {
	labels := resizeTestLabels(3, 800)
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	state.labels[0].view = 42
	next := append([]platform.TextLabel(nil), labels...)
	next[0].Value = "changed"
	next[2].Width = -1
	if err := state.update(nil, next); err == nil {
		t.Fatal("accepted negative width")
	}
	if state.labels[0].view != 42 || state.labels[0].label.Value != labels[0].Value {
		t.Fatal("invalid update discarded the previous frame")
	}
}

func TestVulkanGridResizeReusesReservedColumnsAcrossIndexShift(t *testing.T) {
	labels := resizeTestLabels(2, 70)
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	for i := range state.labels {
		state.labels[i].view = uintptr(i + 1)
		state.labels[i].width = 128
		state.labels[i].scale = 1
	}
	next := []platform.TextLabel{labels[1], labels[0]}
	next[0].Width, next[1].Width = 105, 35
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	for i, want := range []uintptr{2, 1} {
		if state.labels[i].view != want || state.labels[i].label.Width != next[i].Width {
			t.Fatalf("row %d lost its texture or new visible width: %+v", i, state.labels[i])
		}
	}
	next[0].Width = 129
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[0].view != 0 || state.labels[1].view != 1 {
		t.Fatal("growing beyond the reserved columns did not invalidate only the wider row")
	}
	next[1].Runs = append([]gfx.TextRun(nil), next[1].Runs...)
	next[1].Runs[0].Color = gfx.RGB(255, 0, 0)
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[1].view != 0 {
		t.Fatal("a changed run color kept its old texture")
	}
}

func TestVulkanGridTextureReservationIsBounded(t *testing.T) {
	label := resizeTestLabels(1, 70)[0]
	if got := LabelRasterWidth(label, 1, 20); got != 128 {
		t.Fatalf("reserved width = %d, want 128", got)
	}
	label.Value = "hello"
	label.Runs = []gfx.TextRun{{Bytes: 5, Cells: 5}}
	if got := LabelRasterWidth(label, 1, 20); got != 50 {
		t.Fatalf("short row width = %d, want its ink and overhang only", got)
	}
	label.Value = strings.Repeat("x", 100000)
	label.Runs = []gfx.TextRun{{Bytes: int32(len(label.Value)), Cells: int32(len(label.Value))}}
	for _, test := range []struct {
		width  float32
		height int
		want   int
	}{{70, 20, 128}, {16360, 20, 16384}, {1000, 16384, 1024}} {
		label.Width = test.width
		if got := LabelRasterWidth(label, 1, test.height); got != test.want {
			t.Fatalf("width %g height %d reserves %d pixels, want %d", test.width, test.height, got, test.want)
		}
	}
}

func TestVulkanTextResizeMatchesUnclippedWidthAndRejectsClippedWidth(t *testing.T) {
	labels := resizeTestLabels(2, 80)
	labels[1].Value = "short"
	labels[1].Runs = []gfx.TextRun{{Bytes: 5, Cells: 5}}
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	state.labels[0].view, state.labels[1].view = 41, 42
	next := []platform.TextLabel{labels[1], labels[0]}
	next[0].Width, next[1].Width = 200, 85
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[0].view != 42 {
		t.Fatalf("unclipped moved text view = %d, want 42", state.labels[0].view)
	}
	if state.labels[1].view != 0 {
		t.Fatal("reused texture that was clipped at a narrower width")
	}
	state.labels[1].view = 99
	next[1].Width = 80
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[0].view != 42 || state.labels[1].view != 0 {
		t.Fatal("shrinking a clipped label did not retain only the unaffected texture")
	}
}

func TestVulkanTextResizeBitmapMatchesSourceAndKeepsDrawSize(t *testing.T) {
	one, two := []byte{0, 0, 255, 255}, []byte{0, 0, 255, 255}
	labels := []platform.TextLabel{
		{Width: 10, Height: 10, Bitmap: one, BitmapWidth: 1, BitmapHeight: 1, BitmapStride: 4, BitmapID: 1},
		{Width: 10, Height: 10, Bitmap: two, BitmapWidth: 1, BitmapHeight: 1, BitmapStride: 4, BitmapID: 1},
	}
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	state.labels[0].view, state.labels[1].view = 41, 42
	next := []platform.TextLabel{labels[1], labels[0]}
	next[0].Width, next[0].Height = 20, 30
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[0].view != 42 || state.labels[1].view != 41 {
		t.Fatal("bitmap source identity was lost when the labels moved")
	}
	if state.labels[0].label.Width != 20 || state.labels[0].label.Height != 30 {
		t.Fatal("retained bitmap did not take its new drawn dimensions")
	}
}
