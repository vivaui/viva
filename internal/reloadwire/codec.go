package reloadwire

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/vivaui/viva/ui"
)

const (
	ProtocolVersion = 20
	MaxFrameBytes   = 32 << 20
	MaxStateBytes   = 8 << 20
	maxItems        = 200_000
)

type messageKind uint16

const (
	messageHello messageKind = iota + 1
	messageHelloAck
	messagePaint
	messageClick
	messageSnapshotState
	messageRestoreState
	messageCloseRequest
	messageCloseAllowed
	messageShutdown
	messagePointer
	messageKey
	messageShortcut
	messageScroll
	messageModifierTap
	messageError = 0x7fff
	responseFlag = 0x8000
)

type WindowConfig struct {
	Title      string
	Width      int
	Height     int
	MinWidth   int
	MinHeight  int
	WindowMode uint8
	// TitlebarHeight is Config.TitlebarHeight rounded to whole logical pixels.
	TitlebarHeight uint16
	ClearRed       float64
	ClearGreen     float64
	ClearBlue      float64
	ClearAlpha     float64
	HasClick       bool
	HasPointer     bool
	HasKey         bool
	HasShortcut    bool
	HasScroll      bool
	HasModifierTap bool
	HasClose       bool
	HasSnapshot    bool
	HasRestore     bool
}

type Hello struct {
	Token  string
	Config WindowConfig
}

type PaintRequest struct {
	Width        float32
	Height       float32
	Hovered      ui.ID
	Pressed      ui.ID
	Focused      ui.ID
	FocusVisible bool
	ScrollX      float32
	ScrollY      float32
	// Now is the host's monotonic frame time in nanoseconds, so transitions
	// advance on the host clock no matter when the worker process is asked.
	Now          uint64
	ReduceMotion bool
}

type CanvasSnapshot struct {
	Rectangles   []ui.Rectangle
	Shadows      []ui.Shadow
	Images       []ui.Image
	Texts        []ui.Text
	Runs         []ui.TextRun // the runs Texts refer to by RunFirst and RunCount
	Hits         []ui.Hit
	TrappedScope ui.ID
	EscapeTarget ui.ID
	InitialFocus ui.ID
	// Animating reports that a transition was still in flight, so the host
	// must schedule another frame.
	Animating bool
	// FrameDelayMs, when Animating is false, is how long the host may wait
	// before the next frame (Canvas.RequestFrameIn), in milliseconds; zero
	// asks for none.
	FrameDelayMs uint32
}

type StateSlot struct {
	Key     uint64
	Version uint32
	Data    []byte
}

type encoder struct {
	data []byte
	err  error
}

func (value *encoder) raw(data []byte) {
	if value.err != nil {
		return
	}
	if len(data) > MaxFrameBytes-len(value.data) {
		value.err = fmt.Errorf("reload protocol payload exceeds %d bytes", MaxFrameBytes)
		return
	}
	value.data = append(value.data, data...)
}

func (value *encoder) u8(number uint8) { value.raw([]byte{number}) }
func (value *encoder) bool(flag bool) {
	if flag {
		value.u8(1)
	} else {
		value.u8(0)
	}
}
func (value *encoder) u16(number uint16) {
	var data [2]byte
	binary.LittleEndian.PutUint16(data[:], number)
	value.raw(data[:])
}
func (value *encoder) u32(number uint32) {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], number)
	value.raw(data[:])
}
func (value *encoder) u64(number uint64) {
	var data [8]byte
	binary.LittleEndian.PutUint64(data[:], number)
	value.raw(data[:])
}

// id writes an identity as 64 bits.
func (value *encoder) id(id ui.ID) { value.u64(id.Bits()) }

func (value *encoder) f32(number float32) { value.u32(math.Float32bits(number)) }
func (value *encoder) f64(number float64) { value.u64(math.Float64bits(number)) }
func (value *encoder) string(text string) {
	if len(text) > MaxFrameBytes {
		value.err = fmt.Errorf("reload protocol string exceeds %d bytes", MaxFrameBytes)
		return
	}
	value.u32(uint32(len(text)))
	value.raw([]byte(text))
}
func (value *encoder) bytes(data []byte) {
	value.u32(uint32(len(data)))
	value.raw(data)
}

type decoder struct {
	data   []byte
	offset int
	err    error
}

func (value *decoder) raw(length int) []byte {
	if value.err != nil {
		return nil
	}
	if length < 0 || length > len(value.data)-value.offset {
		value.err = fmt.Errorf("truncated reload protocol payload")
		return nil
	}
	data := value.data[value.offset : value.offset+length]
	value.offset += length
	return data
}

func (value *decoder) u8() uint8 {
	data := value.raw(1)
	if len(data) == 0 {
		return 0
	}
	return data[0]
}
func (value *decoder) bool() bool {
	flag := value.u8()
	if flag > 1 && value.err == nil {
		value.err = fmt.Errorf("invalid reload protocol boolean %d", flag)
	}
	return flag == 1
}
func (value *decoder) u16() uint16 {
	data := value.raw(2)
	if len(data) != 2 {
		return 0
	}
	return binary.LittleEndian.Uint16(data)
}
func (value *decoder) u32() uint32 {
	data := value.raw(4)
	if len(data) != 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}
func (value *decoder) u64() uint64 {
	data := value.raw(8)
	if len(data) != 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(data)
}
func (value *decoder) id() ui.ID { return ui.IDFromBits(value.u64()) }

func (value *decoder) f32() float32 { return math.Float32frombits(value.u32()) }
func (value *decoder) f64() float64 { return math.Float64frombits(value.u64()) }
func (value *decoder) string() string {
	length := value.u32()
	if length > MaxFrameBytes {
		value.err = fmt.Errorf("reload protocol string exceeds %d bytes", MaxFrameBytes)
		return ""
	}
	return string(value.raw(int(length)))
}
func (value *decoder) bytes() []byte {
	length := value.u32()
	if length > MaxFrameBytes {
		value.err = fmt.Errorf("reload protocol byte string exceeds %d bytes", MaxFrameBytes)
		return nil
	}
	return append([]byte(nil), value.raw(int(length))...)
}
func (value *decoder) finish() error {
	if value.err != nil {
		return value.err
	}
	if value.offset != len(value.data) {
		return fmt.Errorf("reload protocol payload has %d trailing bytes", len(value.data)-value.offset)
	}
	return nil
}

func encodeHello(value Hello) ([]byte, error) {
	var result encoder
	result.string(value.Token)
	encodeWindowConfig(&result, value.Config)
	return result.data, result.err
}

func decodeHello(data []byte) (Hello, error) {
	value := decoder{data: data}
	result := Hello{Token: value.string(), Config: decodeWindowConfig(&value)}
	return result, value.finish()
}

func encodeWindowConfig(result *encoder, value WindowConfig) {
	result.string(value.Title)
	result.u32(uint32(value.Width))
	result.u32(uint32(value.Height))
	result.u32(uint32(value.MinWidth))
	result.u32(uint32(value.MinHeight))
	result.u8(value.WindowMode)
	result.u16(value.TitlebarHeight)
	result.f64(value.ClearRed)
	result.f64(value.ClearGreen)
	result.f64(value.ClearBlue)
	result.f64(value.ClearAlpha)
	result.bool(value.HasClick)
	result.bool(value.HasPointer)
	result.bool(value.HasKey)
	result.bool(value.HasShortcut)
	result.bool(value.HasScroll)
	result.bool(value.HasModifierTap)
	result.bool(value.HasClose)
	result.bool(value.HasSnapshot)
	result.bool(value.HasRestore)
}

func decodeWindowConfig(value *decoder) WindowConfig {
	return WindowConfig{
		Title: value.string(), Width: int(value.u32()), Height: int(value.u32()),
		MinWidth: int(value.u32()), MinHeight: int(value.u32()), WindowMode: value.u8(), TitlebarHeight: value.u16(),
		ClearRed: value.f64(), ClearGreen: value.f64(), ClearBlue: value.f64(), ClearAlpha: value.f64(),
		HasClick: value.bool(), HasPointer: value.bool(), HasKey: value.bool(), HasShortcut: value.bool(), HasScroll: value.bool(), HasModifierTap: value.bool(),
		HasClose: value.bool(), HasSnapshot: value.bool(), HasRestore: value.bool(),
	}
}

func encodePaintRequest(value PaintRequest) []byte {
	var result encoder
	result.f32(value.Width)
	result.f32(value.Height)
	result.id(value.Hovered)
	result.id(value.Pressed)
	result.id(value.Focused)
	result.bool(value.FocusVisible)
	result.f32(value.ScrollX)
	result.f32(value.ScrollY)
	result.u64(value.Now)
	result.bool(value.ReduceMotion)
	return result.data
}

func decodePaintRequest(data []byte) (PaintRequest, error) {
	value := decoder{data: data}
	result := PaintRequest{
		Width: value.f32(), Height: value.f32(), Hovered: value.id(), Pressed: value.id(), Focused: value.id(),
		FocusVisible: value.bool(), ScrollX: value.f32(), ScrollY: value.f32(),
		Now: value.u64(), ReduceMotion: value.bool(),
	}
	return result, value.finish()
}

func encodePointerEvent(value ui.PointerEvent) []byte {
	var result encoder
	result.u8(uint8(value.Kind))
	result.u8(uint8(value.Button))
	result.u8(uint8(value.Modifiers))
	result.f32(value.X)
	result.f32(value.Y)
	result.f32(value.Viewport.Width)
	result.f32(value.Viewport.Height)
	result.id(value.Target)
	result.bool(value.Captured)
	return result.data
}

func decodePointerEvent(data []byte) (ui.PointerEvent, error) {
	value := decoder{data: data}
	result := ui.PointerEvent{
		Kind: ui.PointerEventKind(value.u8()), Button: ui.PointerButton(value.u8()), Modifiers: ui.Modifiers(value.u8()),
		X: value.f32(), Y: value.f32(), Viewport: ui.Size{Width: value.f32(), Height: value.f32()}, Target: value.id(), Captured: value.bool(),
	}
	if err := value.finish(); err != nil {
		return ui.PointerEvent{}, err
	}
	if result.Kind < ui.PointerDown || result.Kind > ui.PointerExit {
		return ui.PointerEvent{}, fmt.Errorf("invalid reload pointer event kind %d", result.Kind)
	}
	if result.Button > ui.PointerButtonForward {
		return ui.PointerEvent{}, fmt.Errorf("invalid reload pointer button %d", result.Button)
	}
	if !validModifiers(result.Modifiers) {
		return ui.PointerEvent{}, fmt.Errorf("invalid reload pointer modifiers %d", result.Modifiers)
	}
	return result, nil
}

func encodeKeyEvent(value ui.KeyEvent) []byte {
	var result encoder
	result.u16(uint16(value.Key))
	result.u8(uint8(value.Modifiers))
	result.id(value.Target)
	result.bool(value.Repeat)
	result.string(value.Text)
	return result.data
}

func decodeKeyEvent(data []byte) (ui.KeyEvent, error) {
	value := decoder{data: data}
	result := ui.KeyEvent{Key: ui.Key(value.u16()), Modifiers: ui.Modifiers(value.u8()), Target: value.id(), Repeat: value.bool(), Text: value.string()}
	if err := value.finish(); err != nil {
		return ui.KeyEvent{}, err
	}
	if result.Key > ui.KeyF12 {
		return ui.KeyEvent{}, fmt.Errorf("invalid reload key %d", result.Key)
	}
	if !validModifiers(result.Modifiers) {
		return ui.KeyEvent{}, fmt.Errorf("invalid reload key modifiers %d", result.Modifiers)
	}
	return result, nil
}

func encodeScrollEvent(value ui.ScrollEvent) []byte {
	var result encoder
	result.u8(uint8(value.Modifiers))
	result.f32(value.X)
	result.f32(value.Y)
	result.f32(value.DeltaX)
	result.f32(value.DeltaY)
	result.f32(value.Viewport.Width)
	result.f32(value.Viewport.Height)
	result.id(value.Target)
	result.bool(value.Precise)
	return result.data
}

func decodeScrollEvent(data []byte) (ui.ScrollEvent, error) {
	value := decoder{data: data}
	result := ui.ScrollEvent{
		Modifiers: ui.Modifiers(value.u8()), X: value.f32(), Y: value.f32(),
		DeltaX: value.f32(), DeltaY: value.f32(), Viewport: ui.Size{Width: value.f32(), Height: value.f32()}, Target: value.id(), Precise: value.bool(),
	}
	if err := value.finish(); err != nil {
		return ui.ScrollEvent{}, err
	}
	if !validModifiers(result.Modifiers) {
		return ui.ScrollEvent{}, fmt.Errorf("invalid reload scroll modifiers %d", result.Modifiers)
	}
	return result, nil
}

func encodeModifierTapEvent(value ui.ModifierTapEvent) []byte {
	var result encoder
	result.u8(uint8(value.Key))
	result.id(value.Target)
	return result.data
}

func decodeModifierTapEvent(data []byte) (ui.ModifierTapEvent, error) {
	value := decoder{data: data}
	result := ui.ModifierTapEvent{Key: ui.Key(value.u8()), Target: value.id()}
	if err := value.finish(); err != nil {
		return ui.ModifierTapEvent{}, err
	}
	if !result.Key.IsModifier() {
		return ui.ModifierTapEvent{}, fmt.Errorf("invalid reload modifier tap key %d", result.Key)
	}
	return result, nil
}

func encodeEventResult(value ui.EventResult) []byte {
	var result encoder
	result.id(value.Focus)
	result.bool(value.Handled)
	result.bool(value.Repaint)
	result.bool(value.Blur)
	result.bool(value.FocusVisible)
	result.string(value.ClipboardText)
	return result.data
}

func decodeEventResult(data []byte) (ui.EventResult, error) {
	value := decoder{data: data}
	result := ui.EventResult{
		Focus: value.id(), Handled: value.bool(), Repaint: value.bool(), Blur: value.bool(), FocusVisible: value.bool(),
		ClipboardText: value.string(),
	}
	return result, value.finish()
}

func validModifiers(value ui.Modifiers) bool {
	const supported = ui.ModifierShift | ui.ModifierControl | ui.ModifierAlt | ui.ModifierMeta
	return value&^supported == 0
}

func encodeCanvas(value CanvasSnapshot) ([]byte, error) {
	var result encoder
	result.u32(uint32(len(value.Rectangles)))
	for _, rectangle := range value.Rectangles {
		encodeRect(&result, rectangle.Bounds)
		encodeColor(&result, rectangle.Color)
		result.f32(rectangle.Radius)
		result.u16(rectangle.Layer)
		encodeRect(&result, rectangle.ClipOf)
		result.bool(rectangle.HasClip)
	}
	result.u32(uint32(len(value.Shadows)))
	for _, shadow := range value.Shadows {
		encodeRect(&result, shadow.Bounds)
		result.f32(shadow.Radius)
		encodeColor(&result, shadow.Color)
		result.f32(shadow.Style.OffsetX)
		result.f32(shadow.Style.OffsetY)
		result.f32(shadow.Style.Blur)
		result.f32(shadow.Style.Spread)
		result.u16(shadow.Layer)
		encodeRect(&result, shadow.ClipOf)
		result.bool(shadow.HasClip)
		result.u32(shadow.RectsBefore)
		result.u32(shadow.TrianglesBefore)
	}
	result.u32(uint32(len(value.Images)))
	for _, image := range value.Images {
		result.bytes(image.Bitmap.Pix)
		result.u32(uint32(image.Bitmap.Width))
		result.u32(uint32(image.Bitmap.Height))
		result.u32(uint32(image.Bitmap.Stride))
		result.u64(image.Bitmap.ID)
		result.u32(image.Bitmap.Version)
		encodeRect(&result, image.Source)
		encodeRect(&result, image.Bounds)
		result.u16(image.Layer)
		encodeRect(&result, image.ClipOf)
		result.bool(image.HasClip)
	}
	result.u32(uint32(len(value.Runs)))
	for _, run := range value.Runs {
		result.u32(uint32(run.Column))
		result.u32(uint32(run.Bytes))
		result.u32(uint32(run.Cells))
		encodeColor(&result, run.Color)
		result.f32(run.Weight)
		result.bool(run.Italic)
		result.bool(run.Underline)
		result.bool(run.Strike)
	}
	result.u32(uint32(len(value.Texts)))
	for _, text := range value.Texts {
		result.string(text.Value)
		encodeRect(&result, text.Bounds)
		result.f32(text.Style.Size)
		result.f32(text.Style.Weight)
		encodeColor(&result, text.Style.Color)
		result.bool(text.Style.Mono)
		result.u16(uint16(text.Style.MonoFace))
		result.f32(text.CellWidth)
		result.u32(uint32(text.RunFirst))
		result.u32(uint32(text.RunCount))
		result.u16(text.Layer)
		encodeRect(&result, text.ClipOf)
		result.bool(text.HasClip)
	}
	result.u32(uint32(len(value.Hits)))
	for _, hit := range value.Hits {
		result.id(hit.ID)
		encodeRect(&result, hit.Bounds)
		result.string(hit.Label)
		result.string(hit.Value)
		result.string(hit.Hint)
		result.id(hit.Scope)
		result.u16(hit.Layer)
		result.u8(uint8(hit.Role))
		result.bool(hit.Disabled)
		result.bool(hit.Checked)
		result.bool(hit.Selected)
		result.bool(hit.Expanded)
		result.bool(hit.Multiline)
		result.bool(hit.ReadOnly)
		result.bool(hit.Secure)
		result.bool(hit.Focusable)
		result.bool(hit.Autofocus)
		result.bool(hit.Block)
		result.bool(hit.MovesWindow)
		encodeRect(&result, hit.TextBounds)
		result.f32(hit.TextSize)
		encodeColor(&result, hit.TextColor)
		result.f32(hit.TextWeight)
	}
	result.id(value.TrappedScope)
	result.id(value.EscapeTarget)
	result.id(value.InitialFocus)
	result.bool(value.Animating)
	result.u32(value.FrameDelayMs)
	return result.data, result.err
}

func decodeCanvas(data []byte) (CanvasSnapshot, error) {
	value := decoder{data: data}
	var result CanvasSnapshot
	rectangleCount := decodeCount(&value)
	result.Rectangles = make([]ui.Rectangle, rectangleCount)
	for index := range result.Rectangles {
		result.Rectangles[index] = ui.Rectangle{
			Bounds: decodeRect(&value), Color: decodeColor(&value), Radius: value.f32(), Layer: value.u16(),
			ClipOf: decodeRect(&value), HasClip: value.bool(),
		}
	}
	shadowCount := decodeCount(&value)
	result.Shadows = make([]ui.Shadow, shadowCount)
	for index := range result.Shadows {
		result.Shadows[index] = ui.Shadow{
			Bounds: decodeRect(&value), Radius: value.f32(), Color: decodeColor(&value),
			Style: ui.ShadowStyle{OffsetX: value.f32(), OffsetY: value.f32(), Blur: value.f32(), Spread: value.f32()},
			Layer: value.u16(), ClipOf: decodeRect(&value), HasClip: value.bool(),
			RectsBefore: value.u32(), TrianglesBefore: value.u32(),
		}
	}
	imageCount := decodeCount(&value)
	result.Images = make([]ui.Image, imageCount)
	for index := range result.Images {
		pixels := value.bytes()
		bitmap := ui.Bitmap{
			Pix: pixels, Width: int(value.u32()), Height: int(value.u32()), Stride: int(value.u32()),
			ID: value.u64(), Version: value.u32(),
		}
		result.Images[index] = ui.Image{
			Bitmap: bitmap, Source: decodeRect(&value), Bounds: decodeRect(&value), Layer: value.u16(),
			ClipOf: decodeRect(&value), HasClip: value.bool(),
		}
		invalid := bitmap.Width <= 0 || bitmap.Height <= 0 || bitmap.Width > MaxFrameBytes/4 || bitmap.Stride < bitmap.Width*4
		if !invalid {
			invalid = len(bitmap.Pix) < bitmap.Width*4 || bitmap.Height-1 > len(bitmap.Pix)/bitmap.Stride ||
				(bitmap.Height-1)*bitmap.Stride > len(bitmap.Pix)-bitmap.Width*4
		}
		if invalid {
			if value.err == nil {
				value.err = fmt.Errorf("reload protocol bitmap %d is invalid", index)
			}
		}
	}
	runCount := decodeCount(&value)
	result.Runs = make([]ui.TextRun, runCount)
	for index := range result.Runs {
		result.Runs[index] = ui.TextRun{
			Column: int32(value.u32()), Bytes: int32(value.u32()), Cells: int32(value.u32()),
			Color: decodeColor(&value), Weight: value.f32(),
			Italic: value.bool(), Underline: value.bool(), Strike: value.bool(),
		}
	}
	textCount := decodeCount(&value)
	result.Texts = make([]ui.Text, textCount)
	for index := range result.Texts {
		text := ui.Text{
			Value: value.string(), Bounds: decodeRect(&value),
			Style: ui.TextStyle{Size: value.f32(), Weight: value.f32(), Color: decodeColor(&value)},
		}
		text.Style.Mono = value.bool()
		text.Style.MonoFace = ui.MonoFace(value.u16())
		text.CellWidth = value.f32()
		text.RunFirst, text.RunCount = int32(value.u32()), int32(value.u32())
		if text.RunFirst < 0 || text.RunCount < 0 || int64(text.RunFirst)+int64(text.RunCount) > int64(runCount) {
			if value.err == nil {
				value.err = fmt.Errorf("reload protocol text refers to runs %d+%d of %d", text.RunFirst, text.RunCount, runCount)
			}
			text.RunFirst, text.RunCount = 0, 0
		}
		text.Layer, text.ClipOf, text.HasClip = value.u16(), decodeRect(&value), value.bool()
		result.Texts[index] = text
	}
	hitCount := decodeCount(&value)
	result.Hits = make([]ui.Hit, hitCount)
	for index := range result.Hits {
		result.Hits[index] = ui.Hit{
			ID: value.id(), Bounds: decodeRect(&value), Label: value.string(), Value: value.string(), Hint: value.string(),
			Scope: value.id(), Layer: value.u16(), Role: ui.Role(value.u8()), Disabled: value.bool(), Checked: value.bool(),
			Selected: value.bool(), Expanded: value.bool(), Multiline: value.bool(), ReadOnly: value.bool(), Secure: value.bool(), Focusable: value.bool(), Autofocus: value.bool(), Block: value.bool(), MovesWindow: value.bool(),
			TextBounds: decodeRect(&value), TextSize: value.f32(), TextColor: decodeColor(&value), TextWeight: value.f32(),
		}
	}
	result.TrappedScope = value.id()
	result.EscapeTarget = value.id()
	result.InitialFocus = value.id()
	result.Animating = value.bool()
	result.FrameDelayMs = value.u32()
	return result, value.finish()
}

func encodeState(slots []StateSlot) ([]byte, error) {
	if err := validateState(slots); err != nil {
		return nil, err
	}
	var result encoder
	result.u32(uint32(len(slots)))
	for _, slot := range slots {
		result.u64(slot.Key)
		result.u32(slot.Version)
		result.bytes(slot.Data)
	}
	return result.data, result.err
}

func decodeState(data []byte) ([]StateSlot, error) {
	value := decoder{data: data}
	count := decodeCount(&value)
	result := make([]StateSlot, count)
	for index := range result {
		result[index] = StateSlot{Key: value.u64(), Version: value.u32(), Data: value.bytes()}
	}
	if err := value.finish(); err != nil {
		return nil, err
	}
	if err := validateState(result); err != nil {
		return nil, err
	}
	return result, nil
}

func validateState(slots []StateSlot) error {
	if len(slots) > maxItems {
		return fmt.Errorf("reload state has too many slots: %d", len(slots))
	}
	total := 0
	keys := make(map[uint64]struct{}, len(slots))
	for _, slot := range slots {
		if slot.Key == 0 {
			return fmt.Errorf("reload state slot key 0 is reserved")
		}
		if _, exists := keys[slot.Key]; exists {
			return fmt.Errorf("duplicate reload state slot key %d", slot.Key)
		}
		keys[slot.Key] = struct{}{}
		total += len(slot.Data)
		if total > MaxStateBytes {
			return fmt.Errorf("reload state exceeds %d bytes", MaxStateBytes)
		}
	}
	return nil
}

func decodeCount(value *decoder) int {
	count := value.u32()
	if count > maxItems && value.err == nil {
		value.err = fmt.Errorf("reload protocol item count %d exceeds %d", count, maxItems)
		return 0
	}
	return int(count)
}

func encodeRect(result *encoder, value ui.Rect) {
	result.f32(value.X)
	result.f32(value.Y)
	result.f32(value.Width)
	result.f32(value.Height)
}
func decodeRect(value *decoder) ui.Rect {
	return ui.Rect{X: value.f32(), Y: value.f32(), Width: value.f32(), Height: value.f32()}
}
func encodeColor(result *encoder, value ui.Color) {
	result.f32(value.Red)
	result.f32(value.Green)
	result.f32(value.Blue)
	result.f32(value.Alpha)
}
func decodeColor(value *decoder) ui.Color {
	return ui.Color{Red: value.f32(), Green: value.f32(), Blue: value.f32(), Alpha: value.f32()}
}
