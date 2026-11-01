package linux

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

const pangoScale = 1024

type pangoRectangle struct {
	X, Y, Width, Height int32
}

type textServices struct {
	pango, pangoCairo, cairo, gobject uintptr
	context, layout                   uintptr

	// spans are the colors of the last LCD grid label rasterized, valid until the next call.
	spans []colorSpan

	fontMapGetDefault        func() uintptr
	fontMapCreateContext     func(uintptr) uintptr
	layoutNew                func(uintptr) uintptr
	layoutSetText            func(uintptr, *byte, int32)
	layoutSetFont            func(uintptr, uintptr)
	layoutSetWidth           func(uintptr, int32)
	layoutSetSingleParagraph func(uintptr, int32)
	layoutGetExtents         func(uintptr, *pangoRectangle, *pangoRectangle)
	layoutGetBaseline        func(uintptr) int32
	layoutGetCursorPos       func(uintptr, int32, *pangoRectangle, *pangoRectangle)
	layoutIndexToPos         func(uintptr, int32, *pangoRectangle)
	fontFromString           func(*byte) uintptr
	// contextSetRoundGlyphs is pango_context_set_round_glyph_positions (Pango 1.44); nil
	// before it.
	contextSetRoundGlyphs func(uintptr, int32)
	fontNew               func() uintptr
	fontFree              func(uintptr)
	fontSetFamily         func(uintptr, *byte)
	fontSetAbsoluteSize   func(uintptr, float64)
	fontSetWeight         func(uintptr, int32)
	fontSetStyle          func(uintptr, int32)
	objectUnref           func(uintptr)

	imageSurfaceCreateForData func(*byte, int32, int32, int32, int32) uintptr
	surfaceStatus             func(uintptr) int32
	surfaceFlush              func(uintptr)
	surfaceDestroy            func(uintptr)
	cairoCreate               func(uintptr) uintptr
	cairoStatus               func(uintptr) int32
	cairoDestroy              func(uintptr)
	cairoSetSourceRGBA        func(uintptr, float64, float64, float64, float64)
	cairoMoveTo               func(uintptr, float64, float64)
	cairoRectangle            func(uintptr, float64, float64, float64, float64)
	cairoClip                 func(uintptr)
	cairoSave                 func(uintptr)
	cairoRestore              func(uintptr)
	cairoSetLineWidth         func(uintptr, float64)
	cairoLineTo               func(uintptr, float64, float64)
	cairoStroke               func(uintptr)
	cairoScale                func(uintptr, float64, float64)
	cairoTranslate            func(uintptr, float64, float64)
	updateLayout              func(uintptr, uintptr)
	showLayout                func(uintptr, uintptr)

	fontOptionsCreate      func() uintptr
	fontOptionsDestroy     func(uintptr)
	fontOptionsAntialias   func(uintptr, int32)
	fontOptionsHintStyle   func(uintptr, int32)
	fontOptionsHintMetrics func(uintptr, int32)
	fontOptionsSubpixel    func(uintptr, int32)
	contextSetFontOptions  func(uintptr, uintptr)
	cairoPaint             func(uintptr)
	appliedFonts           fontSettings
	hasAppliedFonts        bool
	// hintMetrics is the cairo_hint_metrics_t of this context's font options.
	hintMetrics int32
}

var systemTextAPI struct {
	once sync.Once
	api  *textServices
	err  error
}

func loadTextAPI() (*textServices, error) {
	services := &textServices{}
	opened := []struct {
		name   string
		handle *uintptr
	}{
		{"libpango-1.0.so.0", &services.pango},
		{"libpangocairo-1.0.so.0", &services.pangoCairo},
		{"libcairo.so.2", &services.cairo},
		{"libgobject-2.0.so.0", &services.gobject},
	}
	for _, library := range opened {
		handle, err := purego.Dlopen(library.name, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			services.closeLibraries()
			return nil, fmt.Errorf("linux: load system text library %s: %w", library.name, err)
		}
		*library.handle = handle
	}

	purego.RegisterLibFunc(&services.fontMapGetDefault, services.pangoCairo, "pango_cairo_font_map_get_default")
	purego.RegisterLibFunc(&services.fontMapCreateContext, services.pango, "pango_font_map_create_context")
	purego.RegisterLibFunc(&services.layoutNew, services.pango, "pango_layout_new")
	purego.RegisterLibFunc(&services.layoutSetText, services.pango, "pango_layout_set_text")
	purego.RegisterLibFunc(&services.layoutSetFont, services.pango, "pango_layout_set_font_description")
	purego.RegisterLibFunc(&services.layoutSetWidth, services.pango, "pango_layout_set_width")
	purego.RegisterLibFunc(&services.layoutSetSingleParagraph, services.pango, "pango_layout_set_single_paragraph_mode")
	purego.RegisterLibFunc(&services.layoutGetExtents, services.pango, "pango_layout_get_extents")
	purego.RegisterLibFunc(&services.layoutGetBaseline, services.pango, "pango_layout_get_baseline")
	if symbol, err := purego.Dlsym(services.pango, "pango_context_set_round_glyph_positions"); err == nil && symbol != 0 {
		purego.RegisterLibFunc(&services.contextSetRoundGlyphs, services.pango, "pango_context_set_round_glyph_positions")
	}
	purego.RegisterLibFunc(&services.layoutGetCursorPos, services.pango, "pango_layout_get_cursor_pos")
	purego.RegisterLibFunc(&services.layoutIndexToPos, services.pango, "pango_layout_index_to_pos")
	purego.RegisterLibFunc(&services.fontFromString, services.pango, "pango_font_description_from_string")
	purego.RegisterLibFunc(&services.fontNew, services.pango, "pango_font_description_new")
	purego.RegisterLibFunc(&services.fontFree, services.pango, "pango_font_description_free")
	purego.RegisterLibFunc(&services.fontSetFamily, services.pango, "pango_font_description_set_family")
	purego.RegisterLibFunc(&services.fontSetAbsoluteSize, services.pango, "pango_font_description_set_absolute_size")
	purego.RegisterLibFunc(&services.fontSetWeight, services.pango, "pango_font_description_set_weight")
	purego.RegisterLibFunc(&services.fontSetStyle, services.pango, "pango_font_description_set_style")
	purego.RegisterLibFunc(&services.objectUnref, services.gobject, "g_object_unref")

	purego.RegisterLibFunc(&services.imageSurfaceCreateForData, services.cairo, "cairo_image_surface_create_for_data")
	purego.RegisterLibFunc(&services.surfaceStatus, services.cairo, "cairo_surface_status")
	purego.RegisterLibFunc(&services.surfaceFlush, services.cairo, "cairo_surface_flush")
	purego.RegisterLibFunc(&services.surfaceDestroy, services.cairo, "cairo_surface_destroy")
	purego.RegisterLibFunc(&services.cairoCreate, services.cairo, "cairo_create")
	purego.RegisterLibFunc(&services.cairoStatus, services.cairo, "cairo_status")
	purego.RegisterLibFunc(&services.cairoDestroy, services.cairo, "cairo_destroy")
	purego.RegisterLibFunc(&services.cairoSetSourceRGBA, services.cairo, "cairo_set_source_rgba")
	purego.RegisterLibFunc(&services.cairoMoveTo, services.cairo, "cairo_move_to")
	purego.RegisterLibFunc(&services.cairoRectangle, services.cairo, "cairo_rectangle")
	purego.RegisterLibFunc(&services.cairoClip, services.cairo, "cairo_clip")
	purego.RegisterLibFunc(&services.cairoSave, services.cairo, "cairo_save")
	purego.RegisterLibFunc(&services.cairoRestore, services.cairo, "cairo_restore")
	purego.RegisterLibFunc(&services.cairoSetLineWidth, services.cairo, "cairo_set_line_width")
	purego.RegisterLibFunc(&services.cairoLineTo, services.cairo, "cairo_line_to")
	purego.RegisterLibFunc(&services.cairoStroke, services.cairo, "cairo_stroke")
	purego.RegisterLibFunc(&services.cairoScale, services.cairo, "cairo_scale")
	purego.RegisterLibFunc(&services.cairoTranslate, services.cairo, "cairo_translate")
	purego.RegisterLibFunc(&services.fontOptionsCreate, services.cairo, "cairo_font_options_create")
	purego.RegisterLibFunc(&services.fontOptionsDestroy, services.cairo, "cairo_font_options_destroy")
	purego.RegisterLibFunc(&services.fontOptionsAntialias, services.cairo, "cairo_font_options_set_antialias")
	purego.RegisterLibFunc(&services.fontOptionsHintStyle, services.cairo, "cairo_font_options_set_hint_style")
	purego.RegisterLibFunc(&services.fontOptionsHintMetrics, services.cairo, "cairo_font_options_set_hint_metrics")
	purego.RegisterLibFunc(&services.fontOptionsSubpixel, services.cairo, "cairo_font_options_set_subpixel_order")
	purego.RegisterLibFunc(&services.contextSetFontOptions, services.pangoCairo, "pango_cairo_context_set_font_options")
	purego.RegisterLibFunc(&services.updateLayout, services.pangoCairo, "pango_cairo_update_layout")
	purego.RegisterLibFunc(&services.showLayout, services.pangoCairo, "pango_cairo_show_layout")
	return services, nil
}

func newTextServices() (*textServices, error) {
	systemTextAPI.once.Do(func() {
		systemTextAPI.api, systemTextAPI.err = loadTextAPI()
	})
	if systemTextAPI.err != nil {
		return nil, systemTextAPI.err
	}
	services := *systemTextAPI.api

	fontMap := services.fontMapGetDefault()
	if fontMap == 0 {
		services.close()
		return nil, fmt.Errorf("linux: system Pango font map is unavailable")
	}
	services.context = services.fontMapCreateContext(fontMap)
	if services.context == 0 {
		services.close()
		return nil, fmt.Errorf("linux: create Pango font context")
	}
	services.layout = services.layoutNew(services.context)
	if services.layout == 0 {
		services.close()
		return nil, fmt.Errorf("linux: create Pango text layout")
	}
	services.layoutSetSingleParagraph(services.layout, 1)
	services.hintMetrics = textHintMetrics
	if services.contextSetRoundGlyphs != nil {
		services.contextSetRoundGlyphs(services.context, 0)
	}
	services.useFonts(systemFonts())
	return &services, nil
}

// useFonts gives the Pango context the font options of the desktop, as a GTK application does:
// the antialiasing, hint style and subpixel order that are set, with hinted metrics, and the
// fontconfig default for what is not.
func (services *textServices) useFonts(settings fontSettings) {
	if services.hasAppliedFonts && services.appliedFonts == settings {
		return
	}
	options := services.fontOptionsCreate()
	if options == 0 {
		return
	}
	defer services.fontOptionsDestroy(options)
	services.fontOptionsAntialias(options, settings.Antialias)
	services.fontOptionsHintStyle(options, settings.HintStyle)
	services.fontOptionsHintMetrics(options, services.hintMetrics)
	services.fontOptionsSubpixel(options, settings.SubpixelOrder)
	services.contextSetFontOptions(services.context, options)
	services.appliedFonts, services.hasAppliedFonts = settings, true
}

func (services *textServices) close() {
	if services.layout != 0 && services.objectUnref != nil {
		services.objectUnref(services.layout)
	}
	if services.context != 0 && services.objectUnref != nil {
		services.objectUnref(services.context)
	}
	services.layout, services.context = 0, 0
}

func (services *textServices) closeLibraries() {
	for _, library := range []*uintptr{&services.gobject, &services.cairo, &services.pangoCairo, &services.pango} {
		if *library != 0 {
			purego.Dlclose(*library)
			*library = 0
		}
	}
}

func cString(value string) []byte {
	result := make([]byte, len(value)+1)
	copy(result, value)
	return result
}

func validFontSize(size float32) bool {
	return size > 0 && !math.IsNaN(float64(size)) && !math.IsInf(float64(size), 0)
}

func pangoWeight(weight float32) int32 {
	switch {
	case weight >= 0.35:
		return 700
	case weight >= 0.20:
		return 600
	case weight >= 0.10:
		return 500
	default:
		return 400
	}
}

// proportionalFamily is the family of every proportional label: Pango's sans alias, which
// is the desktop's interface font when the desktop says what it is (fontSettings.Family).
const proportionalFamily = "sans"

// description is the Pango font description of family at an absolute size in device pixels.
// The proportional family is the desktop's interface font as XSETTINGS names it, with the
// size and weight of the text; another family is used as it is.
func (services *textServices) description(family string, size, weight float32, italic bool) (uintptr, error) {
	var description uintptr
	if family == proportionalFamily {
		if name := systemFonts().Family; name != "" {
			nameBytes := cString(name)
			description = services.fontFromString(&nameBytes[0])
			runtime.KeepAlive(nameBytes)
		}
	}
	if description == 0 {
		description = services.fontNew()
		if description == 0 {
			return 0, fmt.Errorf("linux: create Pango font description")
		}
		familyBytes := cString(family)
		services.fontSetFamily(description, &familyBytes[0])
		runtime.KeepAlive(familyBytes)
	}
	services.fontSetAbsoluteSize(description, float64(size)*pangoScale)
	services.fontSetWeight(description, pangoWeight(weight))
	if italic {
		services.fontSetStyle(description, 2)
	} else {
		services.fontSetStyle(description, 0)
	}
	return description, nil
}

func (services *textServices) configureLayout(value, family string, size, weight float32, italic bool) (text.Metrics, error) {
	if !validFontSize(size) {
		return text.Metrics{}, fmt.Errorf("linux: invalid font size")
	}
	if !utf8.ValidString(value) {
		return text.Metrics{}, fmt.Errorf("linux: text is not valid UTF-8")
	}
	description, err := services.description(family, size, weight, italic)
	if err != nil {
		return text.Metrics{}, err
	}
	defer services.fontFree(description)
	valueBytes := cString(value)
	services.layoutSetWidth(services.layout, -1)
	services.layoutSetFont(services.layout, description)
	services.layoutSetText(services.layout, &valueBytes[0], int32(len(value)))
	var logical pangoRectangle
	services.layoutGetExtents(services.layout, nil, &logical)
	baseline := services.layoutGetBaseline(services.layout)
	runtime.KeepAlive(valueBytes)
	metrics := text.Metrics{
		Width:   float32(logical.Width) / pangoScale,
		Height:  float32(logical.Height) / pangoScale,
		Ascent:  float32(baseline) / pangoScale,
		Descent: float32(logical.Height-baseline) / pangoScale,
	}
	return metrics, nil
}

func (services *textServices) measure(value string, size, weight float32) (text.Metrics, error) {
	return services.configureLayout(value, proportionalFamily, size, weight, false)
}

func MeasureText(value string, size, weight float32) (text.Metrics, error) {
	measure, close, err := NewTextMeasurer()
	if err != nil {
		return text.Metrics{}, err
	}
	defer close()
	return measure(value, size, weight)
}

func MeasureMono(size float32, face ui.MonoFace) (width, height float32, err error) {
	services, err := newTextServices()
	if err != nil {
		return 0, 0, err
	}
	defer services.close()
	metrics, err := services.configureLayout("M", monoFamily(face), size, 0, false)
	if err != nil {
		return 0, 0, err
	}
	return metrics.Width, metrics.Height, nil
}

// monoFamily is the Pango family list of a face: its primary font, then the
// families that draw what the primary lacks, in the order the application gave,
// then the generic fixed-pitch family fontconfig chooses for the rest.
func monoFamily(face ui.MonoFace) string {
	name := ui.MonoFamilyName(face)
	fallbacks := ui.MonoFallbacks(face)
	switch {
	case name == "" && len(fallbacks) == 0:
		if ui.MonoIsClassic(face) {
			return "DejaVu Sans Mono"
		}
		return "monospace"
	case name == "":
		if ui.MonoIsClassic(face) {
			name = "DejaVu Sans Mono"
		}
	}
	list := name
	for _, fallback := range fallbacks {
		if list != "" {
			list += ", "
		}
		list += fallback
	}
	return list + ", monospace"
}

func NewTextMeasurer() (text.MeasureFunc, func(), error) {
	services, err := newTextServices()
	if err != nil {
		return nil, nil, err
	}
	return services.measure, services.close, nil
}

// rasterize draws the label. With lcd allowed, and the desktop asking for LCD antialiasing,
// text is drawn white on black into an RGB surface, where Cairo antialiases per
// channel: the pixels are then the coverage of each channel (alpha their luminance) for
// the renderer to blend with the label's color, and lcd is true. Otherwise the pixels are
// premultiplied BGRA in the label's color.
func (services *textServices) rasterize(label platform.TextLabel, scale float32, width, height int, allowLCD bool) (pixels []byte, lcd bool, err error) {
	settings := systemFonts()
	services.spans = services.spans[:0]
	// A fixed-pitch label is LCD text too, unless it holds a color glyph, which has no
	// coverage to blend per channel.
	grid := label.Mono || len(label.Runs) > 0
	lcd = allowLCD && settings.subpixel() && label.Value != "" && !(grid && platform.ColorGlyphs(label.Value))
	if !lcd {
		settings = settings.grayscale()
	}
	services.useFonts(settings)
	pixels, err = services.draw(label, scale, width, height, lcd)
	return pixels, lcd && err == nil, err
}

func (services *textServices) draw(label platform.TextLabel, scale float32, width, height int, lcd bool) ([]byte, error) {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || width*height > 16*1024*1024 {
		return nil, fmt.Errorf("linux: label texture exceeds bounded size")
	}
	if !validFontSize(label.FontSize) || scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return nil, fmt.Errorf("linux: invalid label font size or scale")
	}
	pixels := make([]byte, width*height*4)
	format := int32(0) // CAIRO_FORMAT_ARGB32
	if lcd {
		format = 1 // CAIRO_FORMAT_RGB24: the only content Cairo antialiases per channel
	}
	surface := services.imageSurfaceCreateForData(&pixels[0], format, int32(width), int32(height), int32(width*4))
	if surface == 0 || services.surfaceStatus(surface) != 0 {
		if surface != 0 {
			services.surfaceDestroy(surface)
		}
		return nil, fmt.Errorf("linux: create Cairo text surface")
	}
	defer services.surfaceDestroy(surface)
	context := services.cairoCreate(surface)
	if context == 0 || services.cairoStatus(context) != 0 {
		if context != 0 {
			services.cairoDestroy(context)
		}
		return nil, fmt.Errorf("linux: create Cairo text context")
	}
	defer services.cairoDestroy(context)
	if label.Value != "" {
		if len(label.Runs) == 0 {
			family := proportionalFamily
			if label.Mono {
				family = monoFamily(label.MonoFace)
			}
			if lcd {
				services.cairoSetSourceRGBA(context, 1, 1, 1, 1)
			} else {
				services.setSourceColor(context, label.Color)
			}
			target := float64(platform.LabelBaseline(height, float64(label.FontSize*scale)))
			if label.Mono {
				// A fixed-pitch label is placed on its grid, not fitted.
				metrics, err := services.configureLayout(label.Value, family, label.FontSize*scale, label.Weight, false)
				if err != nil {
					return nil, err
				}
				services.cairoMoveTo(context, 0, target-float64(metrics.Ascent))
				services.updateLayout(context, services.layout)
				services.showLayout(context, services.layout)
			} else {
				// One Pango layout for the whole line, the one the engine measured it with, so
				// the glyphs stand where layout, the caret and a click put them.
				if err := services.drawLine(context, label, scale, target, lcd); err != nil {
					return nil, err
				}
			}
		} else if err := services.drawRuns(context, label, scale, width, height, lcd); err != nil {
			return nil, err
		}
	}
	if status := services.cairoStatus(context); status != 0 {
		return nil, fmt.Errorf("linux: Cairo text drawing failed with status %d", status)
	}
	services.surfaceFlush(surface)
	runtime.KeepAlive(pixels)
	if lcd {
		for index := 0; index+3 < len(pixels); index += 4 {
			pixels[index+3] = byte((uint32(pixels[index+2])*77 + uint32(pixels[index+1])*151 + uint32(pixels[index])*28 + 128) >> 8)
		}
	}
	return pixels, nil
}

// drawLine draws a proportional label as the shared engine's layout of its text.
func (services *textServices) drawLine(context uintptr, label platform.TextLabel, scale float32, baseline float64, lcd bool) error {
	engine := sharedTextEngine()
	if engine == nil {
		return fmt.Errorf("linux: no text engine")
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line, err := engine.lineLocked(label.Value, label.FontSize*scale, label.Weight)
	if err != nil {
		return err
	}
	settings := systemFonts()
	if !lcd {
		settings = settings.grayscale()
	}
	engine.services.useFonts(settings)
	services.cairoMoveTo(context, 0, baseline-float64(line.ascent))
	engine.services.updateLayout(context, line.layout)
	engine.services.showLayout(context, line.layout)
	return nil
}

func (services *textServices) setSourceColor(context uintptr, color platform.Color) {
	services.cairoSetSourceRGBA(context, clampUnit64(color.Red), clampUnit64(color.Green), clampUnit64(color.Blue), clampUnit64(color.Alpha))
}

func (services *textServices) setSourceUIColor(context uintptr, color ui.Color) {
	services.cairoSetSourceRGBA(context, clampUnit64(float64(color.Red)), clampUnit64(float64(color.Green)), clampUnit64(float64(color.Blue)), clampUnit64(float64(color.Alpha)))
}

func clampUnit64(value float64) float64 {
	return min(max(value, 0), 1)
}

// colorSpan is the columns of an LCD grid label's texture, left to right in device pixels, that
// are text of one color. Its coverage is the same for every color, so the renderer draws the
// one texture once per span, with the span's color, scissored to its columns.
type colorSpan struct {
	left, right int
	color       [4]float32
}

// drawRuns draws a fixed-pitch label's runs. With lcd they are drawn white, for their coverage,
// and the color of each run is recorded in services.spans instead.
func (services *textServices) drawRuns(context uintptr, label platform.TextLabel, scale float32, width, height int, lcd bool) error {
	pitch := float64(label.CellWidth * scale)
	if pitch <= 0 || math.IsNaN(pitch) || math.IsInf(pitch, 0) {
		return fmt.Errorf("linux: invalid fixed-pitch cell width")
	}
	family := monoFamily(label.MonoFace)
	baseMetrics, err := services.configureLayout("M", family, label.FontSize*scale, label.Weight, false)
	if err != nil {
		return err
	}
	baseline := float64(platform.LabelBaseline(height, float64(label.FontSize*scale)))
	offset := 0
	for _, run := range label.Runs {
		end := offset + int(run.Bytes)
		if run.Bytes < 0 || run.Cells < 0 || end < offset || end > len(label.Value) {
			return fmt.Errorf("linux: invalid fixed-pitch text run")
		}
		piece := label.Value[offset:end]
		offset = end
		if !utf8.ValidString(piece) {
			return fmt.Errorf("linux: text run splits a UTF-8 sequence")
		}
		left := float64(run.Column) * pitch
		span := float64(run.Cells) * pitch
		services.cairoSave(context)
		services.cairoRectangle(context, left, 0, span, float64(height))
		services.cairoClip(context)
		source := func() {
			if lcd {
				services.cairoSetSourceRGBA(context, 1, 1, 1, 1)
			} else {
				services.setSourceUIColor(context, run.Color)
			}
		}
		source()
		if lcd && run.Cells > 0 && piece != "" {
			services.addSpan(int(math.Floor(left)), int(math.Ceil(left+span)), [4]float32{run.Color.Red, run.Color.Green, run.Color.Blue, run.Color.Alpha})
		}
		metrics := baseMetrics
		if run.Ligatures && piece != "" {
			// Shape the run as one line and scale it to the cells it spans, so
			// the font's ligatures survive without moving later runs off their
			// columns.
			metrics, err = services.configureLayout(piece, family, label.FontSize*scale, run.Weight, run.Italic)
			if err != nil {
				services.cairoRestore(context)
				return err
			}
			if metrics.Width > 0 && span > 0 {
				services.cairoSave(context)
				services.cairoTranslate(context, left, baseline)
				services.cairoScale(context, span/float64(metrics.Width), 1)
				services.cairoMoveTo(context, 0, -float64(metrics.Ascent))
				// Draw the measured layout: updating it under this transform reshapes
				// its metrics and leaks the scaled matrix into subsequent runs.
				services.showLayout(context, services.layout)
				services.cairoRestore(context)
			}
		} else if printableASCII(piece) {
			for index := range len(piece) {
				metrics, err = services.configureLayout(piece[index:index+1], family, label.FontSize*scale, run.Weight, run.Italic)
				if err != nil {
					services.cairoRestore(context)
					return err
				}
				services.cairoMoveTo(context, left+float64(index)*pitch, baseline-float64(metrics.Ascent))
				services.updateLayout(context, services.layout)
				services.showLayout(context, services.layout)
			}
		} else if piece != "" {
			metrics, err = services.configureLayout(piece, family, label.FontSize*scale, run.Weight, run.Italic)
			if err != nil {
				services.cairoRestore(context)
				return err
			}
			x := left
			if run.Cells > 0 && isSingleCharacter(piece) {
				x += (span - float64(metrics.Width)) / 2
			}
			services.cairoMoveTo(context, x, baseline-float64(metrics.Ascent))
			services.updateLayout(context, services.layout)
			services.showLayout(context, services.layout)
		}
		services.cairoRestore(context)
		if run.Underline || run.Strike {
			source()
			services.cairoSetLineWidth(context, max(1, float64(label.FontSize*scale)/16))
			if run.Underline {
				y := min(float64(height)-0.5, baseline+max(1, float64(metrics.Descent)*0.45))
				services.cairoMoveTo(context, left, y)
				services.cairoLineTo(context, left+span, y)
				services.cairoStroke(context)
			}
			if run.Strike {
				y := baseline - float64(metrics.Ascent)*0.42
				services.cairoMoveTo(context, left, y)
				services.cairoLineTo(context, left+span, y)
				services.cairoStroke(context)
			}
		}
	}
	if offset != len(label.Value) {
		return fmt.Errorf("linux: fixed-pitch text runs do not cover the label")
	}
	_ = width
	return nil
}

// addSpan records a run's columns and color, joined to the span before it when they touch and
// the color is the same.
func (services *textServices) addSpan(left, right int, color [4]float32) {
	if n := len(services.spans); n > 0 && services.spans[n-1].color == color && services.spans[n-1].right >= left {
		services.spans[n-1].right = max(services.spans[n-1].right, right)
		return
	}
	services.spans = append(services.spans, colorSpan{left: left, right: right, color: color})
}

func printableASCII(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		if value[index] < 0x20 || value[index] >= 0x7f {
			return false
		}
	}
	return true
}

func isSingleCharacter(value string) bool {
	count := 0
	for _, character := range value {
		if count > 0 && unicode.In(character, unicode.Mn, unicode.Me, unicode.Cf) {
			continue
		}
		count++
	}
	return count == 1
}
