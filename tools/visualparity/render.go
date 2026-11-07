//go:build darwin || windows || linux

package main

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type runOptions struct {
	out       string
	scales    []float64
	only      string
	images    bool
	themeOnly string
}

func runCommand(options runOptions) error {
	measure, closeMeasure, err := nativeMeasurer()
	if err != nil {
		return fmt.Errorf("platform text measurer: %w", err)
	}
	defer closeMeasure()
	if err := os.MkdirAll(options.out, 0o755); err != nil {
		return err
	}
	if err := writePlatform(options); err != nil {
		return err
	}
	if err := writeMetrics(options.out, measure); err != nil {
		return err
	}
	imagesWanted := options.images
	for _, dark := range []bool{false, true} {
		themeName := "light"
		if dark {
			themeName = "dark"
		}
		if options.themeOnly != "" && options.themeOnly != themeName {
			continue
		}
		for _, sc := range catalogue() {
			if options.only != "" && !strings.Contains(sc.name, options.only) {
				continue
			}
			id := sc.name + "." + themeName
			for index, scale := range options.scales {
				wantImage := imagesWanted
				facts, picture, err := renderScene(sc, dark, scale, wantImage, true, measure)
				if err != nil && noRenderer(err) {
					fmt.Fprintln(os.Stderr, "no offscreen renderer here (macOS needs -tags vivahead, Linux needs a Vulkan driver): layout facts only")
					imagesWanted = false
					facts, picture, err = renderScene(sc, dark, scale, false, true, measure)
				}
				if err != nil {
					return fmt.Errorf("%s @%gx: %w", id, scale, err)
				}
				if index == 0 {
					if err := writeJSON(filepath.Join(options.out, id+".json"), facts); err != nil {
						return err
					}
				}
				if picture != nil {
					if err := writePNG(filepath.Join(options.out, fmt.Sprintf("%s@%gx.png", id, scale)), picture); err != nil {
						return err
					}
				}
			}
			fmt.Printf("%s ok\n", id)
		}
	}
	return nil
}

func appColor(c ui.Color) app.Color {
	return app.Color{Red: float64(c.Red), Green: float64(c.Green), Blue: float64(c.Blue), Alpha: float64(c.Alpha)}
}

// noRenderer reports whether err means this machine cannot produce pixels: a build
// without an offscreen renderer, or one whose graphics driver is missing (Linux
// without Vulkan). The layout facts need neither.
func noRenderer(err error) bool {
	return errors.Is(err, headless.ErrNoRenderer) || errors.Is(err, app.ErrNoGraphics)
}

// renderScene opens the scene in a headless session, captures its layout facts
// and, when asked, its pixels. measure is the platform's own text engine, asked only
// to record next to each text how wide the platform would have drawn it naturally
// (what Viva's layout does not use); nil leaves that out.
func renderScene(sc *scene, dark bool, scale float64, wantImage, reference bool, measure text.MeasureFunc) (*Facts, *image.RGBA, error) {
	theme := ui.LightTheme()
	if dark {
		theme = ui.DarkTheme()
	}
	view := &flex.View{}
	config := app.Config{
		Title: sc.name, Width: sc.width, Height: sc.height, ClearColor: appColor(theme.Background), Tree: &tree,
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			if sc.before != nil {
				sc.before(view)
			}
			view.Draw(canvas, theme, ui.Rect{Width: size.Width, Height: size.Height}, func(f *flex.Frame) { sc.build(f, theme) })
		},
	}
	canvas := &ui.Canvas{}
	session, err := headless.Open(config, canvas, headless.Options{Scale: scale, ReferenceText: reference})
	if err != nil {
		return nil, nil, err
	}
	defer session.Close()
	// A dialog opens with an entrance; the facts are those of the scene at rest.
	if err := session.Rest(); err != nil {
		return nil, nil, err
	}
	facts := &Facts{Scene: sc.name, Platform: platformName, Width: sc.width, Height: sc.height, Dark: dark}
	facts.Texts, facts.Rectangles, facts.Triangles, facts.Shadows, facts.Hits = capture(canvas, &tree, measure)
	var picture *image.RGBA
	if wantImage {
		if picture, err = session.Image(); err != nil {
			return nil, nil, err
		}
	}
	return facts, picture, nil
}

func writePNG(path string, picture *image.RGBA) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encodeErr := png.Encode(file, picture)
	if closeErr := file.Close(); encodeErr == nil {
		encodeErr = closeErr
	}
	return encodeErr
}

// Platform describes the machine a run came from.
type Platform struct {
	Name   string            `json:"name"`
	GOOS   string            `json:"goos"`
	GOARCH string            `json:"goarch"`
	Scales []float64         `json:"scales"`
	Notes  map[string]string `json:"notes,omitempty"`
}

func writePlatform(options runOptions) error {
	p := Platform{Name: platformName, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Scales: options.scales, Notes: map[string]string{}}
	if runtime.GOOS == "linux" {
		for _, query := range [][]string{{"sans"}, {"sans:weight=bold"}, {"sans:lang=zh-cn"}, {"sans:lang=ja"}, {"monospace"}, {"emoji"}} {
			if output, err := exec.Command("fc-match", append(query, "-f", "%{family} | %{style} | %{file}")...).Output(); err == nil {
				p.Notes["fc-match "+query[0]] = string(output)
			}
		}
	}
	return writeJSON(filepath.Join(options.out, "platform.json"), p)
}
