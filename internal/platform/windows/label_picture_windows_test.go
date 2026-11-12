package windows

import (
	"fmt"
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func pictureLabels(color platform.Color) []platform.TextLabel {
	values := []string{"设置：自动连接与字体回退", "Italic fj Ág e\u0301", "العربية עברית", "emoji 😀 👩‍💻", "   ", "剪裁内容 Clipped fj"}
	labels := make([]platform.TextLabel, len(values))
	for index, value := range values {
		labels[index] = platform.TextLabel{
			Value: value, X: 24.25, Y: float32(index)*36 + 10.5,
			Width: 600, Height: 30, FontSize: 17, Weight: 400, Color: color,
			Identity: gfx.TextIdentity{Owner: 1, Item: uint64(index)},
		}
	}
	labels[1].Italic = true
	labels[2].Weight = 700
	labels[5].Clipped = true
	labels[5].Clip = gfx.Rect{X: 45.5, Y: 0, Width: 108.25, Height: 400}
	return labels
}

func TestLabelPicturesPreserveNativePixelsAcrossPageVisits(t *testing.T) {
	for _, scale := range []float64{1, 1.25, 1.5, 2} {
		for _, theme := range []struct {
			name             string
			background, text platform.Color
		}{
			{"dark", platform.Color{Alpha: 1}, platform.Color{Red: 0.9, Green: 0.9, Blue: 0.9, Alpha: 1}},
			{"light", platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, platform.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}},
			{"composed", platform.Color{Alpha: 0.5}, platform.Color{Red: 0.8, Green: 0.4, Blue: 0.6, Alpha: 0.75}},
		} {
			t.Run(fmt.Sprintf("%s/%gx", theme.name, scale), func(t *testing.T) {
				reference, err := NewOffscreen(680, 250, scale, theme.background)
				if err != nil {
					t.Fatal(err)
				}
				defer reference.Close()
				reference.renderer.noLabelPictures = true
				candidate, err := NewOffscreen(680, 250, scale, theme.background)
				if err != nil {
					t.Fatal(err)
				}
				defer candidate.Close()
				labels := pictureLabels(theme.text)
				if err := reference.Set(nil, labels); err != nil {
					t.Fatal(err)
				}
				want, err := reference.Image()
				if err != nil {
					t.Fatal(err)
				}
				for visit := range 2 {
					if err := candidate.Set(nil, labels); err != nil {
						t.Fatal(err)
					}
					built := candidate.renderer.labelRebuilds
					got, err := candidate.Image()
					if err != nil {
						t.Fatal(err)
					}
					comparePixels(t, want, got)
					if visit > 0 && candidate.renderer.labelRebuilds != built {
						t.Fatal("returning to a cached page rebuilt its native pictures")
					}
					if err := candidate.Set(nil, pictureLabels(platform.Color{Red: 1, Alpha: 1})); err != nil {
						t.Fatal(err)
					}
					if _, err := candidate.Image(); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestLabelPictureSharingAndInvalidation(t *testing.T) {
	offscreen, err := NewOffscreen(680, 250, 2, platform.Color{Alpha: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer offscreen.Close()
	labels := pictureLabels(platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})[:1]
	labels = append(labels, labels[0])
	labels[1].Identity.Item++
	labels[1].Y += 40
	if err := offscreen.Set(nil, labels); err != nil {
		t.Fatal(err)
	}
	if _, err := offscreen.Image(); err != nil {
		t.Fatal(err)
	}
	if offscreen.renderer.labelRebuilds != 1 {
		t.Fatal("equal live labels did not share one upload")
	}
	picture := offscreen.renderer.labels[0].picture
	if picture == nil || picture.refs != 2 {
		t.Fatal("live picture borrowers were not tracked")
	}
	if err := offscreen.Set(nil, labels[1:]); err != nil {
		t.Fatal(err)
	}
	if _, err := offscreen.Image(); err != nil {
		t.Fatal(err)
	}
	if picture.refs != 1 || picture.view == 0 {
		t.Fatal("removing a borrower destroyed another label's picture")
	}
	offscreen.renderer.trim()
	if picture.view == 0 {
		t.Fatal("idle trim destroyed a visible picture")
	}
	for _, change := range []func(*platform.TextLabel){
		func(label *platform.TextLabel) { label.Value += " changed" },
		func(label *platform.TextLabel) { label.FontSize++ },
		func(label *platform.TextLabel) { label.Color.Red = 0.5 },
		func(label *platform.TextLabel) { label.Baseline += 2 },
		func(label *platform.TextLabel) { label.FontGeneration++ },
		func(label *platform.TextLabel) { label.Italic = true },
		func(label *platform.TextLabel) {
			label.Clipped = true
			label.Clip = gfx.Rect{X: 60, Y: 0, Width: 90, Height: 250}
		},
	} {
		change(&labels[1])
		built := offscreen.renderer.labelRebuilds
		if err := offscreen.Set(nil, labels[1:]); err != nil {
			t.Fatal(err)
		}
		if _, err := offscreen.Image(); err != nil {
			t.Fatal(err)
		}
		if offscreen.renderer.labelRebuilds <= built {
			t.Fatal("changed native text reused an obsolete picture")
		}
	}
	built := offscreen.renderer.labelRebuilds
	sharedEngine.forget()
	if _, err := offscreen.Image(); err != nil {
		t.Fatal(err)
	}
	if offscreen.renderer.labelRebuilds <= built {
		t.Fatal("installed font changes did not invalidate a live picture")
	}
	built = offscreen.renderer.labelRebuilds
	offscreen.renderer.color.Alpha = 0.5
	if _, err := offscreen.Image(); err != nil {
		t.Fatal(err)
	}
	if offscreen.renderer.labelRebuilds <= built {
		t.Fatal("compositing changes kept an incompatible ClearType picture")
	}
	built = offscreen.renderer.labelRebuilds
	offscreen.renderer.invalidateText()
	if _, err := offscreen.Image(); err != nil {
		t.Fatal(err)
	}
	if offscreen.renderer.labelRebuilds <= built {
		t.Fatal("native rendering settings did not invalidate text")
	}
	current := offscreen.renderer.labels[offscreen.renderer.labelFrame[0]].picture
	offscreen.renderer.releaseDevice()
	if picture.view != 0 || picture.texture != 0 || current.view != 0 || current.texture != 0 || offscreen.renderer.labelPictures.bytes != 0 {
		t.Fatal("device teardown retained a borrowed native resource")
	}
	if offscreen.renderer.labels[offscreen.renderer.labelFrame[0]].label.Value != labels[1].Value {
		t.Fatal("device teardown lost the retained scene needed for recovery")
	}
}

func TestLabelPictureCacheBoundsIdleMemoryAndPinsBorrowers(t *testing.T) {
	var cache labelPictureCache
	pinned := &labelPicture{key: labelPictureKey{value: "visible"}, width: 100, height: 20}
	cache.store(pinned)
	for index := range labelPictureIdleBudget/(400*40*4) + 20 {
		picture := &labelPicture{key: labelPictureKey{value: fmt.Sprint(index)}, width: 400, height: 40}
		cache.store(picture)
		picture.release()
		if cache.idleBytes > labelPictureIdleBudget {
			t.Fatal("off-frame memory exceeded its bound")
		}
	}
	if cache.entries[labelPictureKey{value: "0"}] != nil {
		t.Fatal("memory pressure did not evict the least recent idle picture")
	}
	if cache.acquire(pinned.key) != pinned {
		t.Fatal("memory pressure evicted a visible picture")
	}
	pinned.release()
	key := cache.newest.key
	borrowed := cache.acquire(key)
	cache.evict(0)
	if cache.bytes != pinned.bytes+borrowed.bytes {
		t.Fatal("trim retained resources without borrowers")
	}
	pinned.release()
	borrowed.release()
	cache.evict(0)
	if cache.bytes != 0 || cache.idleBytes != 0 || len(cache.entries) != 0 {
		t.Fatal("empty cache retained pictures")
	}
}

func TestPictureBoundsIncludeAllNativeCoverageChannels(t *testing.T) {
	pixels := make([]byte, 8*6*4)
	pixels[(1*8+2)*4] = 1
	pixels[(4*8+6)*4+3] = 1
	left, top, right, bottom := picturePixelBounds(pixels, 8, 6)
	if left > 2 || top > 1 || right <= 6 || bottom <= 4 {
		t.Fatal("tight picture lost native coverage")
	}
}

func TestLabelPicturesDoNotRetainUnremovableMapKeys(t *testing.T) {
	var renderer renderer
	label := labelTexture{label: pictureLabels(platform.Color{Alpha: 1})[0]}
	label.label.Color.Red = math.NaN()
	if _, eligible := renderer.pictureKey(&label, 2, 0, 100, 40); eligible {
		t.Fatal("NaN color admitted a map key that eviction cannot remove")
	}
	label.label.Color.Red = 0
	label.label.Weight = float32(math.NaN())
	if _, eligible := renderer.pictureKey(&label, 2, 0, 100, 40); eligible {
		t.Fatal("NaN weight admitted a map key that eviction cannot remove")
	}
}

func BenchmarkLabelPicturePageSwitch(b *testing.B) {
	for _, cached := range []bool{false, true} {
		for _, warm := range []bool{false, true} {
			b.Run(fmt.Sprintf("cache=%t/warm=%t", cached, warm), func(b *testing.B) {
				offscreen, err := NewOffscreen(1400, 900, 2, platform.Color{Alpha: 1})
				if err != nil {
					b.Fatal(err)
				}
				defer offscreen.Close()
				offscreen.renderer.noLabelPictures = !cached
				color := platform.Color{Red: 0.9, Green: 0.9, Blue: 0.9, Alpha: 1}
				pages := [2][]platform.TextLabel{pageLabels(0, color), pageLabels(1, color)}
				for _, labels := range pages {
					for index := range labels {
						labels[index].Width = 1100
					}
				}
				width, height := offscreen.size()
				draw := func(labels []platform.TextLabel) {
					if err := offscreen.Set(nil, labels); err != nil {
						b.Fatal(err)
					}
					if err := offscreen.renderer.draw(width, height, 2); err != nil {
						b.Fatal(err)
					}
				}
				for _, labels := range pages {
					draw(labels)
				}
				built := offscreen.renderer.labelRebuilds
				b.ReportAllocs()
				b.ResetTimer()
				for index := 0; index < b.N; index++ {
					if !warm {
						offscreen.renderer.labelPictures.evict(0)
					}
					draw(pages[index%len(pages)])
				}
				b.StopTimer()
				b.ReportMetric(float64(offscreen.renderer.labelRebuilds-built)/float64(b.N), "uploads/op")
				b.ReportMetric(float64(offscreen.renderer.labelPictures.bytes), "picture-bytes")
				b.ReportMetric(float64(offscreen.renderer.labelPictures.idleBytes), "idle-bytes")
			})
		}
	}
}
