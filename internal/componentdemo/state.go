package componentdemo

import (
	"fmt"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type Example struct {
	ID, Title, Description string
	// Snippet is the call-site code shown beside the example on the
	// documentation site. Empty means the page omits the usage block.
	Snippet string
	// Height is the preview height in pixels. Zero uses the page default.
	Height int
}

type Component struct {
	ID, Title, Category, Summary string
	Examples                     []Example
	paint                        func(*State, *ui.Canvas, ui.Theme, ui.Size)
	pointer                      func(*State, ui.PointerEvent) ui.EventResult
	key                          func(*State, ui.KeyEvent) ui.EventResult
	scroll                       func(*State, ui.ScrollEvent) ui.EventResult
	initialize                   func(*State)
	// editable names the one text field of an example that a platform text
	// service may edit (see EnableTextEditing); examples that are fixed
	// snapshots report false.
	editable func(exampleID string) (editableField, bool)
	editor   func(*State) *text.Editor
}

// editableField describes an example's editable text field. Accept and
// MaxRunes, when set, restrict what can be typed into it.
type editableField struct {
	ID       ui.ID
	Initial  string
	Accept   func(rune) bool
	MaxRunes int
	// Overwrite makes typing replace the text after the caret, as a fixed-width
	// field does (see text.Editor.SetOverwrite).
	Overwrite bool
}

var coreComponents = []Component{
	buttonComponent, badgeComponent, checkboxComponent, colorSwatchComponent, colorPickerComponent,
	switchComponent, inputComponent, dialogComponent,
	codeEditorComponent, codeDiffComponent, splitButtonComponent, imageViewComponent,
	reorderListComponent,
}

var catalog = buildCatalog()

func buildCatalog() []Component {
	result := make([]Component, 0, len(coreComponents)+len(groupAComponents)+len(groupBComponents)+len(groupCComponents))
	result = append(result, coreComponents...)
	result = append(result, groupAComponents...)
	result = append(result, groupBComponents...)
	result = append(result, groupCComponents...)
	for index := 1; index < len(result); index++ {
		for current := index; current > 0 && componentLess(result[current], result[current-1]); current-- {
			result[current], result[current-1] = result[current-1], result[current]
		}
	}
	return result
}

func componentLess(left, right Component) bool {
	if left.Category != right.Category {
		return left.Category < right.Category
	}
	return left.ID < right.ID
}

func Catalog() []Component { return catalog }

type State struct {
	componentID string
	exampleID   string
	descriptor  *Component
	dark        bool
	english     bool
	density     ui.Density
	button      buttonState
	checkbox    checkboxState
	switches    switchState
	// editor is the Go-owned text behind the example's editable field once a
	// runner has called EnableTextEditing; fieldID is the hit that edits it.
	editor       *text.Editor
	multiSelect  *components.MultiSelectState
	singleSelect *components.SingleSelectState
	fieldID      ui.ID
	editing      bool
	// textarea keeps the rows and scroll of the Textarea example.
	textarea    *components.TextareaLayout
	codeEditor  *components.CodeEditor
	codeDiff    *codeDiffDemoState
	reorder     *reorderDemoState
	imageBitmap *ui.Bitmap
	dialog      dialogState
	slider      sliderDemoState
	picker      *colorPickerDemoState
	// chart keeps the hover and label caches of the chart examples.
	chart *chartDemoState
	// view keeps the memory the flex pages of this demo are laid out in (see page).
	// It is a pointer so a State stays comparable.
	view  *flex.View
	flags uint64
	// markdown keeps the Markdown example's arrays, so painting it again allocates nothing.
	markdown *markdownScratch
	values   [8]int32
	points   [4]float32
}

func New(componentID, exampleID string) (*State, error) {
	return newState(componentID, exampleID, false)
}

// NewEnglish creates the same state with its painted copy in English. The
// language must be known before the example initializes its caller-owned text.
func NewEnglish(componentID, exampleID string) (*State, error) {
	return newState(componentID, exampleID, true)
}

func newState(componentID, exampleID string, english bool) (*State, error) {
	for index := range catalog {
		component := &catalog[index]
		if component.ID != componentID {
			continue
		}
		if exampleID == "" {
			exampleID = component.Examples[0].ID
		}
		for _, example := range component.Examples {
			if example.ID != exampleID {
				continue
			}
			state := &State{componentID: componentID, exampleID: exampleID, descriptor: component, english: english, view: new(flex.View)}
			if component.initialize != nil {
				component.initialize(state)
			}
			return state, nil
		}
		return nil, fmt.Errorf("componentdemo: unknown example %q for %q", exampleID, componentID)
	}
	return nil, fmt.Errorf("componentdemo: unknown component %q", componentID)
}

// Text returns the Chinese copy in the default preview and the English copy in
// the English preview.
func (state *State) Text(chinese, english string) string {
	if state.english {
		return english
	}
	return chinese
}

func (state *State) SetDark(dark bool) { state.dark = dark }

// SetDensity selects the control density the example is painted at. The zero
// value is the regular density every runner used before this setter existed.
func (state *State) SetDensity(density ui.Density) { state.density = density }

// theme is the appearance and density the example is painted at.
func (state *State) theme() ui.Theme {
	theme := ui.LightTheme()
	if state.dark {
		theme = ui.DarkTheme()
	}
	theme.Metrics = theme.Metrics.WithDensity(state.density)
	return theme
}

func (state *State) Paint(canvas *ui.Canvas, size ui.Size) {
	if canvas == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	canvas.SetTree(&tree)
	theme := state.theme()
	viewport := ui.Rect{Width: size.Width, Height: size.Height}
	canvas.Clip(viewport)
	canvas.Fill(viewport, theme.Background)
	if state.descriptor != nil && state.descriptor.paint != nil {
		state.descriptor.paint(state, canvas, theme, size)
	}
	canvas.Unclip()
}

func (state *State) Pointer(event ui.PointerEvent) ui.EventResult {
	if state.descriptor == nil || state.descriptor.pointer == nil {
		return ui.EventResult{}
	}
	return state.descriptor.pointer(state, event)
}

func (state *State) Key(event ui.KeyEvent) ui.EventResult {
	if state.descriptor == nil || state.descriptor.key == nil {
		return ui.EventResult{}
	}
	return state.descriptor.key(state, event)
}

func (state *State) Scroll(event ui.ScrollEvent) ui.EventResult {
	if state.descriptor == nil || state.descriptor.scroll == nil {
		return ui.EventResult{}
	}
	return state.descriptor.scroll(state, event)
}

func pointerViewport(event ui.PointerEvent) ui.Size {
	if event.Viewport.Width > 0 && event.Viewport.Height > 0 {
		return event.Viewport
	}
	return ui.Size{Width: 760, Height: 380}
}

func textRange(start, end int) text.Range { return text.Range{Start: start, End: end} }

// page lays a demo out with flex: the title over the content, centered on the
// stage and at most maxWidth wide. content declares the specimens; a Slot it
// declares has its Rect once page returns, for a specimen a component paints
// itself. Clicks are read in content, where the control is declared.
func (state *State) page(canvas *ui.Canvas, theme ui.Theme, size ui.Size, maxWidth float32, title string, content func(f *flex.Frame)) {
	if state.view == nil {
		state.view = new(flex.View)
	}
	state.view.Draw(canvas, theme, ui.Rect{Width: size.Width, Height: size.Height}, func(f *flex.Frame) {
		f.Column().Fill().Pad(theme.Metrics.PadSurface).Items(flex.Center).Children(func() {
			f.Column().FillWidth().MaxWidth(maxWidth).Gap(theme.Metrics.GapItem).Children(func() {
				f.Text(title).FontSize(theme.Metrics.FontSizeM).Weight(ui.WeightSemibold)
				content(f)
			})
		})
	})
}
