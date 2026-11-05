//go:build linux

package linux

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/vivaui/viva/internal/fontfile"
	"github.com/vivaui/viva/ui"
)

// fontConfigApp is fontconfig's application font set and Pango's way of being told it changed.
// Fontconfig can add a file to the set but not take one away, so a removal clears the set and adds
// the files that remain.
type fontConfigApp struct {
	addFile       func(config uintptr, path *byte) int32
	clear         func(config uintptr)
	configChanged func(fontMap uintptr)
}

var (
	fontApp     fontConfigApp
	fontAppErr  error
	fontAppOnce sync.Once
	fonts       fontfile.Registry
	// fontFiles are the files in the application set, in the order they were added. Guarded by
	// the engine's lock.
	fontFiles []string
)

func loadFontConfigApp() (*fontConfigApp, error) {
	fontAppOnce.Do(func() {
		config, err := purego.Dlopen("libfontconfig.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			fontAppErr = fmt.Errorf("linux: load fontconfig: %w", err)
			return
		}
		pangoFT, err := purego.Dlopen("libpangoft2-1.0.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			fontAppErr = fmt.Errorf("linux: load Pango's fontconfig font map: %w", err)
			return
		}
		purego.RegisterLibFunc(&fontApp.addFile, config, "FcConfigAppFontAddFile")
		purego.RegisterLibFunc(&fontApp.clear, config, "FcConfigAppFontClear")
		purego.RegisterLibFunc(&fontApp.configChanged, pangoFT, "pango_fc_font_map_config_changed")
	})
	return &fontApp, fontAppErr
}

var _ ui.FontRegistrar = (*textEngine)(nil)

// RegisterFont implements ui.FontRegistrar: the font joins fontconfig's application fonts, which
// only this process sees, and Pango is told its font set changed.
func (engine *textEngine) RegisterFont(data []byte) (string, func(), error) {
	app, err := loadFontConfigApp()
	if err != nil {
		return "", nil, err
	}
	family, unregister, err := fonts.Register(data, func(path string) error {
		return engine.changeFonts(app, func() error {
			name := append([]byte(path), 0)
			if app.addFile(0, &name[0]) == 0 {
				return errors.New("fontconfig refused the font")
			}
			fontFiles = append(fontFiles, path)
			return nil
		})
	}, func(path string) {
		engine.changeFonts(app, func() error {
			if index := slices.Index(fontFiles, path); index >= 0 {
				fontFiles = slices.Delete(fontFiles, index, index+1)
			}
			app.clear(0)
			for _, file := range fontFiles {
				name := append([]byte(file), 0)
				app.addFile(0, &name[0])
			}
			return nil
		})
	})
	if err != nil {
		return "", nil, err
	}
	return family, unregister, nil
}

// changeFonts runs edit on fontconfig's application fonts and makes Pango and the engine start
// over with the result.
func (engine *textEngine) changeFonts(app *fontConfigApp, edit func() error) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if err := edit(); err != nil {
		return err
	}
	app.configChanged(engine.services.fontMapGetDefault())
	engine.cache.Clear()
	engine.generation.Add(1)
	return nil
}
