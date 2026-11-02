// Package iconcompile turns SVG icon sources into ui.Icon Go values at
// build time. It understands the icon-dialect subset used by Lucide and
// similar stroke-icon sets: path, line, polyline, polygon, rect, circle and ellipse
// elements with fill="none" stroke-based geometry; groups and their
// transforms; and currentColor-aware defaults. An element with a concrete fill
// color (fill="#e65100", with fill-opacity, opacity and fill-rule) becomes a
// colored layer of the icon, which is how multi-color glyphs such as file-type
// icons compile. A fill of url(#id) naming a linearGradient with two stops and no
// gradientTransform becomes a ui.LinearGradient layer, in userSpaceOnUse units or,
// when it runs along an axis, in SVG's default objectBoundingBox units; radial
// gradients, more stops, slanted bounding-box gradients and other spread methods
// are rejected with an error. It is a build tool: nothing here links into
// applications.
package iconcompile

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/vivaui/viva/ui"
)

// Compile parses one SVG document into a ui.Icon. Width and height default
// to the viewBox; stroke metadata is taken from the document with Lucide
// defaults (stroke 2, round cap and join). A stroke-width of 0 on the root
// element means the icon's tinted shapes are filled instead.
func Compile(name string, source []byte) (ui.Icon, error) {
	document := &svgNode{}
	if err := xml.Unmarshal(source, document); err != nil {
		return ui.Icon{}, fmt.Errorf("%s: %w", name, err)
	}
	root := document
	if root.XMLName.Local != "svg" {
		for index := range document.Children {
			if document.Children[index].XMLName.Local == "svg" {
				root = &document.Children[index]
				break
			}
		}
	}
	if root.XMLName.Local != "svg" {
		return ui.Icon{}, fmt.Errorf("%s: no svg element", name)
	}

	viewBox, viewHeight := 24.0, 0.0
	origin := identityTransform()
	if raw := root.Attr("viewBox"); raw != "" {
		fields := strings.Fields(raw)
		if len(fields) == 4 {
			// A view box that does not start at 0,0 (Material Symbols use
			// "0 -960 960 960") is brought to the origin.
			minX, errX := strconv.ParseFloat(fields[0], 64)
			minY, errY := strconv.ParseFloat(fields[1], 64)
			if errX == nil && errY == nil {
				origin = transform{1, 0, 0, 1, -minX, -minY}
			}
			if width, err := strconv.ParseFloat(fields[2], 32); err == nil && width > 0 {
				viewBox = width
			}
			if height, err := strconv.ParseFloat(fields[3], 32); err == nil && height > 0 {
				viewHeight = height
			}
		}
	}

	state := iconState{
		stroke: 2,
		cap:    ui.CapRound,
		join:   ui.JoinRound,
	}
	if raw := root.Attr("stroke-width"); raw != "" {
		// A root stroke-width of 0 makes the tinted shapes filled, not stroked.
		if width, err := strconv.ParseFloat(raw, 32); err == nil && width >= 0 {
			state.stroke = width
		}
	}
	if raw := root.Attr("stroke-linecap"); raw != "" {
		state.cap = parseCap(raw)
	}
	if raw := root.Attr("stroke-linejoin"); raw != "" {
		state.join = parseJoin(raw)
	}

	sink := &iconSink{layer: -1}
	state.opacity, state.fillOpacity = 1, 1
	// Paint set on the root element (<svg fill="#5786fe">) is inherited by every
	// shape below it, as in SVG.
	state = state.inherit(root)
	state.gradients = map[string]*svgNode{}
	collectGradients(root, state.gradients)
	state.walk(root, origin, sink)
	if sink.err != nil {
		return ui.Icon{}, fmt.Errorf("%s: %w", name, sink.err)
	}
	if len(sink.commands) == 0 && len(sink.layers) == 0 {
		return ui.Icon{}, fmt.Errorf("%s: no drawable geometry", name)
	}
	if viewHeight == viewBox {
		viewHeight = 0 // square: the icon's ViewHeight stays unset
	}
	return ui.Icon{
		ViewBox:    float32(viewBox),
		ViewHeight: float32(viewHeight),
		Stroke:     float32(state.stroke),
		Cap:        state.cap,
		Join:       state.join,
		Commands:   sink.commands,
		Layers:     sink.layers,
	}, nil
}

// iconSink collects what the walk emits: stroked commands, and the colored
// layers of elements that name a fill.
type iconSink struct {
	commands []ui.PathCommand
	layers   []ui.IconLayer
	layer    int // the layer being filled, or -1 for the stroked commands
	err      error
}

func (sink *iconSink) add(command ui.PathCommand) {
	if sink.layer >= 0 {
		sink.layers[sink.layer].Commands = append(sink.layers[sink.layer].Commands, command)
		return
	}
	sink.commands = append(sink.commands, command)
}

func (sink *iconSink) begin(fill ui.Color, rule ui.PathFillRule, gradient *ui.LinearGradient) {
	sink.layers = append(sink.layers, ui.IconLayer{Fill: fill, FillRule: rule, Gradient: gradient})
	sink.layer = len(sink.layers) - 1
}

func (sink *iconSink) end() { sink.layer = -1 }

// fitBoxGradient places the bounding-box gradient of the layer just drawn on the
// box of its path.
func (sink *iconSink) fitBoxGradient(box *boxGradient) {
	layer := &sink.layers[sink.layer]
	minX, minY, maxX, maxY, ok := commandBounds(layer.Commands)
	if !ok || maxX <= minX || maxY <= minY {
		if sink.err == nil {
			sink.err = fmt.Errorf("a bounding-box gradient fills a shape with no area")
		}
		return
	}
	width, height := maxX-minX, maxY-minY
	dx, dy := box.ends[2]-box.ends[0], box.ends[3]-box.ends[1]
	at := func(offset float64) ui.Point {
		return ui.Point{
			X: float32(minX + (box.ends[0]+dx*offset)*width),
			Y: float32(minY + (box.ends[1]+dy*offset)*height),
		}
	}
	layer.Gradient.Start, layer.Gradient.End = at(box.offsets[0]), at(box.offsets[1])
}

// commandBounds is the bounding box of a path, with each curve sampled, which is
// within a small fraction of a unit of the exact one for icon-sized curves.
func commandBounds(commands []ui.PathCommand) (minX, minY, maxX, maxY float64, ok bool) {
	minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(x, y float64) {
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
		ok = true
	}
	var cx, cy, sx, sy float64
	for _, command := range commands {
		switch command.Kind {
		case ui.PathCommandMove:
			cx, cy = float64(command.Point.X), float64(command.Point.Y)
			sx, sy = cx, cy
			add(cx, cy)
		case ui.PathCommandLine:
			cx, cy = float64(command.Point.X), float64(command.Point.Y)
			add(cx, cy)
		case ui.PathCommandQuad:
			x1, y1 := float64(command.Control1.X), float64(command.Control1.Y)
			x2, y2 := float64(command.Point.X), float64(command.Point.Y)
			for step := 1; step <= 32; step++ {
				t := float64(step) / 32
				u := 1 - t
				add(u*u*cx+2*u*t*x1+t*t*x2, u*u*cy+2*u*t*y1+t*t*y2)
			}
			cx, cy = x2, y2
		case ui.PathCommandCubic:
			x1, y1 := float64(command.Control1.X), float64(command.Control1.Y)
			x2, y2 := float64(command.Control2.X), float64(command.Control2.Y)
			x3, y3 := float64(command.Point.X), float64(command.Point.Y)
			for step := 1; step <= 32; step++ {
				t := float64(step) / 32
				u := 1 - t
				add(u*u*u*cx+3*u*u*t*x1+3*u*t*t*x2+t*t*t*x3, u*u*u*cy+3*u*u*t*y1+3*u*t*t*y2+t*t*t*y3)
			}
			cx, cy = x3, y3
		case ui.PathCommandClose:
			cx, cy = sx, sy
		}
	}
	return minX, minY, maxX, maxY, ok
}

func parseCap(raw string) ui.StrokeCap {
	switch strings.TrimSpace(raw) {
	case "round":
		return ui.CapRound
	case "square":
		return ui.CapSquare
	default:
		return ui.CapButt
	}
}

func parseJoin(raw string) ui.StrokeJoin {
	switch strings.TrimSpace(raw) {
	case "round":
		return ui.JoinRound
	case "miter", "miter-clip", "arcs":
		return ui.JoinMiter
	default:
		return ui.JoinBevel
	}
}

type svgNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []svgNode  `xml:",any"`
}

func (node *svgNode) Attr(name string) string {
	for _, attr := range node.Attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

type transform struct{ a, b, c, d, e, f float64 }

func identityTransform() transform { return transform{1, 0, 0, 1, 0, 0} }

func (t transform) apply(x, y float64) (float64, float64) {
	return t.a*x + t.c*y + t.e, t.b*x + t.d*y + t.f
}

func (t transform) then(next transform) transform {
	return transform{
		a: t.a*next.a + t.c*next.b,
		b: t.b*next.a + t.d*next.b,
		c: t.a*next.c + t.c*next.d,
		d: t.b*next.c + t.d*next.d,
		e: t.a*next.e + t.c*next.f + t.e,
		f: t.b*next.e + t.d*next.f + t.f,
	}
}

func parseTransform(raw string) transform {
	result := identityTransform()
	// A transform list is name(arguments) items; the arguments are separated by
	// commas, spaces, or both.
	for rest := raw; ; {
		open := strings.IndexByte(rest, '(')
		if open < 0 {
			break
		}
		closing := strings.IndexByte(rest[open:], ')')
		if closing < 0 {
			break
		}
		name := strings.TrimSpace(strings.Trim(rest[:open], ", "))
		arguments := rest[open+1 : open+closing]
		rest = rest[open+closing+1:]
		numbers := make([]float64, 0, 6)
		for _, part := range strings.FieldsFunc(arguments, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
			if value, err := strconv.ParseFloat(part, 64); err == nil {
				numbers = append(numbers, value)
			}
		}
		switch name {
		case "translate":
			if len(numbers) >= 1 {
				ty := 0.0
				if len(numbers) >= 2 {
					ty = numbers[1]
				}
				result = result.then(transform{1, 0, 0, 1, numbers[0], ty})
			}
		case "scale":
			if len(numbers) >= 1 {
				sy := numbers[0]
				if len(numbers) >= 2 {
					sy = numbers[1]
				}
				result = result.then(transform{numbers[0], 0, 0, sy, 0, 0})
			}
		case "matrix":
			if len(numbers) == 6 {
				result = result.then(transform{numbers[0], numbers[1], numbers[2], numbers[3], numbers[4], numbers[5]})
			}
		case "rotate":
			if len(numbers) >= 1 {
				angle := numbers[0] * math.Pi / 180
				cos, sin := math.Cos(angle), math.Sin(angle)
				rotation := transform{cos, sin, -sin, cos, 0, 0}
				if len(numbers) >= 3 {
					cx, cy := numbers[1], numbers[2]
					result = result.then(transform{1, 0, 0, 1, cx, cy}).
						then(rotation).
						then(transform{1, 0, 0, 1, -cx, -cy})
				} else {
					result = result.then(rotation)
				}
			}
		}
	}
	return result
}

type iconState struct {
	stroke float64
	cap    ui.StrokeCap
	join   ui.StrokeJoin
	// Inherited paint: the fill attribute ("" when none was given), its
	// opacities and the fill rule.
	fill        string
	strokePaint string // the stroke attribute, inherited ("" when none was given)
	fillOpacity float64
	opacity     float64
	fillRule    ui.PathFillRule
	// gradients are the document's gradient elements by id.
	gradients map[string]*svgNode
}

// inherit applies a node's own paint attributes over its parent's.
func (state iconState) inherit(node *svgNode) iconState {
	if raw := node.Attr("fill"); raw != "" {
		state.fill = strings.TrimSpace(raw)
	}
	if raw := node.Attr("stroke"); raw != "" {
		state.strokePaint = strings.TrimSpace(raw)
	}
	if raw := node.Attr("fill-opacity"); raw != "" {
		if value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
			state.fillOpacity = value
		}
	}
	if raw := node.Attr("opacity"); raw != "" {
		if value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
			state.opacity *= value
		}
	}
	if raw := node.Attr("fill-rule"); raw != "" {
		if strings.TrimSpace(raw) == "evenodd" {
			state.fillRule = ui.PathFillEvenOdd
		} else {
			state.fillRule = ui.PathFillNonZero
		}
	}
	return state
}

// color resolves the element's fill: colored is false when the element is a
// plain stroke-or-currentColor shape and the icon's tint applies.
func (state iconState) color() (ui.Color, bool, error) {
	switch fill := state.fill; {
	case fill == "" || fill == "none" || strings.EqualFold(fill, "currentColor"):
		return ui.Color{}, false, nil
	case strings.HasPrefix(fill, "#"):
		color, err := parseHexColor(fill)
		if err != nil {
			return ui.Color{}, false, err
		}
		color.Alpha *= float32(state.fillOpacity * state.opacity)
		return color, true, nil
	case strings.EqualFold(fill, "black"):
		return ui.Color{Alpha: float32(state.fillOpacity * state.opacity)}, true, nil
	case strings.EqualFold(fill, "white"):
		return ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: float32(state.fillOpacity * state.opacity)}, true, nil
	}
	return ui.Color{}, false, fmt.Errorf("fill %q is not supported", state.fill)
}

// invisible reports whether an element paints nothing: its fill is none and no
// stroke was given. Icon sets use such elements as transparent bounding boxes.
// An element with no paint attributes at all is not invisible; it keeps the
// stroked-glyph default.
func (state iconState) invisible() bool {
	return state.fill == "none" && (state.strokePaint == "" || state.strokePaint == "none")
}

// parseHexColor reads #rgb, #rgba, #rrggbb or #rrggbbaa.
func parseHexColor(raw string) (ui.Color, error) {
	digits := strings.TrimPrefix(raw, "#")
	if len(digits) == 3 || len(digits) == 4 {
		expanded := make([]byte, 0, 8)
		for index := 0; index < len(digits); index++ {
			expanded = append(expanded, digits[index], digits[index])
		}
		digits = string(expanded)
	}
	if len(digits) != 6 && len(digits) != 8 {
		return ui.Color{}, fmt.Errorf("color %q is not #rgb, #rgba, #rrggbb or #rrggbbaa", raw)
	}
	value, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return ui.Color{}, fmt.Errorf("color %q: %w", raw, err)
	}
	color := ui.Color{Alpha: 1}
	if len(digits) == 8 {
		color.Alpha = float32(value&0xff) / 255
		value >>= 8
	}
	color.Red = float32(value>>16&0xff) / 255
	color.Green = float32(value>>8&0xff) / 255
	color.Blue = float32(value&0xff) / 255
	return color, nil
}

func (state iconState) walk(node *svgNode, at transform, sink *iconSink) {
	emit := sink.add
	for _, child := range node.Children {
		name := child.XMLName.Local
		childState := state.inherit(&child)
		if name == "g" {
			next := at
			if raw := child.Attr("transform"); raw != "" {
				next = at.then(parseTransform(raw))
			}
			childState.walk(&child, next, sink)
			continue
		}
		elementAt := at
		if raw := child.Attr("transform"); raw != "" {
			elementAt = at.then(parseTransform(raw))
		}
		var (
			fill     ui.Color
			colored  bool
			gradient *ui.LinearGradient
			box      *boxGradient
			err      error
		)
		if strings.HasPrefix(childState.fill, "url(") {
			gradient, box, err = childState.linearGradient(elementAt)
			if gradient != nil {
				fill, colored = gradient.From, true
			}
		} else {
			fill, colored, err = childState.color()
		}
		if err != nil && sink.err == nil {
			sink.err = err
		}
		if !colored && childState.invisible() {
			continue
		}
		if colored {
			sink.begin(fill, childState.fillRule, gradient)
		}
		switch name {
		case "path":
			state.emitPathData(child.Attr("d"), elementAt, emit)
		case "line":
			state.emitLine(&child, elementAt, emit)
		case "polyline", "polygon":
			state.emitPoly(&child, elementAt, name == "polygon", emit)
		case "rect":
			state.emitRect(&child, elementAt, emit)
		case "circle":
			state.emitCircle(&child, elementAt, emit)
		case "ellipse":
			state.emitEllipse(&child, elementAt, emit)
		case "defs", "title", "desc", "metadata", "style", "linearGradient", "radialGradient", "clipPath", "mask", "filter":
			// Structure and metadata: not drawn by themselves.
		default:
			// An element the compiler does not know is never skipped silently: a
			// dropped <ellipse> once left half of Lucide's cylinder on screen.
			if sink.err == nil {
				sink.err = fmt.Errorf("unsupported element <%s>", name)
			}
		}
		if box != nil {
			sink.fitBoxGradient(box)
		}
		if colored {
			sink.end()
		}
	}
}

func (state iconState) point(at transform, x, y float64) (float32, float32) {
	px, py := at.apply(x, y)
	return float32(px), float32(py)
}

func (state iconState) emitPathData(data string, at transform, emit func(ui.PathCommand)) {
	var currentX, currentY float64
	var lastCX, lastCY float64 // reflected control points for S/T
	var lastCommand byte
	startX, startY := 0.0, 0.0
	parser := &pathParser{input: data}
	for {
		command, relative, ok := parser.nextCommand()
		if !ok {
			return
		}
		switch command {
		case 'M':
			first := true
			for {
				x, y, ok := parser.coords()
				if !ok {
					break
				}
				if relative && !first {
					x, y = x+currentX, y+currentY
				} else if relative && first && lastCommand != 0 {
					x, y = x+currentX, y+currentY
				}
				currentX, currentY = x, y
				x32, y32 := state.point(at, x, y)
				if first {
					startX, startY = x, y
					emit(ui.MoveTo(x32, y32))
					first = false
					if !parser.moreNumbers() {
						break
					}
					continue
				}
				// Subsequent coordinate pairs after a move are implicit
				// lineto commands in the same relative/absolute mode.
				emit(ui.LineTo(x32, y32))
				if !parser.moreNumbers() {
					break
				}
			}
		case 'L', 'l':
			for {
				x, y, ok := parser.coords()
				if !ok {
					break
				}
				if relative {
					x, y = x+currentX, y+currentY
				}
				currentX, currentY = x, y
				x32, y32 := state.point(at, x, y)
				emit(ui.LineTo(x32, y32))
				if !parser.moreNumbers() {
					break
				}
			}
		case 'H', 'h':
			for {
				x, ok := parser.number()
				if !ok {
					break
				}
				if relative {
					x += currentX
				}
				currentX = x
				x32, y32 := state.point(at, x, currentY)
				emit(ui.LineTo(x32, y32))
				if !parser.moreNumbers() {
					break
				}
			}
		case 'V', 'v':
			for {
				y, ok := parser.number()
				if !ok {
					break
				}
				if relative {
					y += currentY
				}
				currentY = y
				x32, y32 := state.point(at, currentX, y)
				emit(ui.LineTo(x32, y32))
				if !parser.moreNumbers() {
					break
				}
			}
		case 'C', 'c':
			for {
				c1x, c1y, c2x, c2y, x, y, ok := parser.six()
				if !ok {
					break
				}
				if relative {
					c1x, c1y = c1x+currentX, c1y+currentY
					c2x, c2y = c2x+currentX, c2y+currentY
					x, y = x+currentX, y+currentY
				}
				c1x32, c1y32 := state.point(at, c1x, c1y)
				c2x32, c2y32 := state.point(at, c2x, c2y)
				x32, y32 := state.point(at, x, y)
				emit(ui.CubicTo(c1x32, c1y32, c2x32, c2y32, x32, y32))
				lastCX, lastCY = c2x, c2y
				currentX, currentY = x, y
				if !parser.moreNumbers() {
					break
				}
			}
		case 'S', 's':
			for {
				c2x, c2y, x, y, ok := parser.four()
				if !ok {
					break
				}
				if relative {
					c2x, c2y = c2x+currentX, c2y+currentY
					x, y = x+currentX, y+currentY
				}
				c1x, c1y := 2*currentX-lastCX, 2*currentY-lastCY
				if lastCommand != 'C' && lastCommand != 'c' && lastCommand != 'S' && lastCommand != 's' {
					c1x, c1y = currentX, currentY
				}
				c1x32, c1y32 := state.point(at, c1x, c1y)
				c2x32, c2y32 := state.point(at, c2x, c2y)
				x32, y32 := state.point(at, x, y)
				emit(ui.CubicTo(c1x32, c1y32, c2x32, c2y32, x32, y32))
				lastCX, lastCY = c2x, c2y
				currentX, currentY = x, y
				if !parser.moreNumbers() {
					break
				}
			}
		case 'Q', 'q':
			for {
				cx, cy, x, y, ok := parser.four()
				if !ok {
					break
				}
				if relative {
					cx, cy = cx+currentX, cy+currentY
					x, y = x+currentX, y+currentY
				}
				cx32, cy32 := state.point(at, cx, cy)
				x32, y32 := state.point(at, x, y)
				emit(ui.QuadTo(cx32, cy32, x32, y32))
				lastCX, lastCY = cx, cy
				currentX, currentY = x, y
				if !parser.moreNumbers() {
					break
				}
			}
		case 'T', 't':
			for {
				x, y, ok := parser.coords()
				if !ok {
					break
				}
				if relative {
					x, y = x+currentX, y+currentY
				}
				cx, cy := 2*currentX-lastCX, 2*currentY-lastCY
				if lastCommand != 'Q' && lastCommand != 'q' && lastCommand != 'T' && lastCommand != 't' {
					cx, cy = currentX, currentY
				}
				cx32, cy32 := state.point(at, cx, cy)
				x32, y32 := state.point(at, x, y)
				emit(ui.QuadTo(cx32, cy32, x32, y32))
				lastCX, lastCY = cx, cy
				currentX, currentY = x, y
				if !parser.moreNumbers() {
					break
				}
			}
		case 'A', 'a':
			for {
				args, ok := parser.seven()
				if !ok {
					break
				}
				// emitArc resolves the relative target itself; track the
				// absolute endpoint for the following command.
				absoluteX, absoluteY := args[5], args[6]
				if relative {
					absoluteX += currentX
					absoluteY += currentY
				}
				args[5], args[6] = absoluteX, absoluteY
				state.emitArc(currentX, currentY, args, false, at, emit)
				currentX, currentY = absoluteX, absoluteY
				if !parser.moreNumbers() {
					break
				}
			}
		case 'Z', 'z':
			emit(ui.ClosePath())
			currentX, currentY = startX, startY
		}
		lastCommand = command
	}
}

// emitArc converts one SVG arc from endpoint to center parameterization
// and emits it as cubic segments, following the W3C SVG implementation
// notes.
func (state iconState) emitArc(currentX, currentY float64, args [7]float64, relative bool, at transform, emit func(ui.PathCommand)) {
	rx, ry, rotation, largeArc, sweep, x, y := args[0], args[1], args[2], args[3], args[4], args[5], args[6]
	if relative {
		x, y = x+currentX, y+currentY
	}
	if rx == 0 || ry == 0 || (x == currentX && y == currentY) {
		x32, y32 := state.point(at, x, y)
		emit(ui.LineTo(x32, y32))
		return
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	angle := rotation * math.Pi / 180
	cos, sin := math.Cos(angle), math.Sin(angle)

	// Step 1: translate so the midpoint of the chord is the origin, then
	// undo the ellipse rotation.
	dx := (currentX - x) / 2
	dy := (currentY - y) / 2
	x1 := cos*dx + sin*dy
	y1 := -sin*dx + cos*dy

	// Step 2: scale the radii up if the chord does not fit.
	lambda := x1*x1/(rx*rx) + y1*y1/(ry*ry)
	if lambda > 1 {
		scale := math.Sqrt(lambda)
		rx *= scale
		ry *= scale
	}

	// Step 3: compute the ellipse center in the rotated frame.
	numerator := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	denominator := rx*rx*y1*y1 + ry*ry*x1*x1
	coefficient := 1.0
	if denominator > 0 {
		coefficient = math.Sqrt(max64(0, numerator/denominator))
	}
	if largeArc == sweep {
		coefficient = -coefficient
	}
	cxPrime := coefficient * rx * y1 / ry
	cyPrime := -coefficient * ry * x1 / rx

	// Step 4: center back in user coordinates.
	cxUser := cos*cxPrime - sin*cyPrime + (currentX+x)/2
	cyUser := sin*cxPrime + cos*cyPrime + (currentY+y)/2

	startAngle := math.Atan2((y1-cyPrime)/ry, (x1-cxPrime)/rx)
	endAngle := math.Atan2((-y1-cyPrime)/ry, (-x1-cxPrime)/rx)
	sweepAngle := endAngle - startAngle
	if sweep > 0.5 && sweepAngle < 0 {
		sweepAngle += 2 * math.Pi
	}
	if sweep < 0.5 && sweepAngle > 0 {
		sweepAngle -= 2 * math.Pi
	}

	segments := int(math.Ceil(math.Abs(sweepAngle) / (math.Pi / 2)))
	if segments < 1 {
		segments = 1
	}
	delta := sweepAngle / float64(segments)
	handle := 4.0 / 3.0 * math.Tan(delta/4)

	pointOn := func(angle float64) (float64, float64) {
		px := rx * math.Cos(angle)
		py := ry * math.Sin(angle)
		return cos*px - sin*py + cxUser, sin*px + cos*py + cyUser
	}
	controlOn := func(angle, tangent float64) (float64, float64) {
		px := rx*math.Cos(angle) - tangent*ry*math.Sin(angle)
		py := ry*math.Sin(angle) + tangent*rx*math.Cos(angle)
		return cos*px - sin*py + cxUser, sin*px + cos*py + cyUser
	}

	for index := 0; index < segments; index++ {
		a0 := startAngle + delta*float64(index)
		a1 := a0 + delta
		c1x, c1y := controlOn(a0, handle)
		c2x, c2y := controlOn(a1, -handle)
		toX, toY := pointOn(a1)
		c1x32, c1y32 := state.point(at, c1x, c1y)
		c2x32, c2y32 := state.point(at, c2x, c2y)
		toX32, toY32 := state.point(at, toX, toY)
		emit(ui.CubicTo(c1x32, c1y32, c2x32, c2y32, toX32, toY32))
	}
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// collectGradients indexes every gradient element of the document by id.
func collectGradients(node *svgNode, into map[string]*svgNode) {
	for index := range node.Children {
		child := &node.Children[index]
		if name := child.XMLName.Local; name == "linearGradient" || name == "radialGradient" {
			if id := child.Attr("id"); id != "" {
				into[id] = child
			}
		}
		collectGradients(child, into)
	}
}

// boxGradient is a gradient given in objectBoundingBox units, SVG's default: its
// endpoints are fractions of the bounding box of the shape it fills, which is only
// known once the shape has been drawn. fitBoxGradient finishes it.
type boxGradient struct {
	ends    [4]float64 // x1, y1, x2, y2 as fractions of the box
	offsets [2]float64 // where the first and last stop sit along them
}

// linearGradient resolves a fill of url(#id) for an element drawn with at. In
// userSpaceOnUse units the gradient's endpoints go through the same transform as
// the element's path, which is only exact when that transform keeps angles
// (rotation, uniform scale, flip); anything else would shear the gradient and is
// rejected. In objectBoundingBox units, the default, the gradient comes back
// unplaced with a boxGradient to fit once the shape's path is known; the
// transform may then only move and scale it, and the gradient must run along an
// axis, because only then do the lines of equal color stay perpendicular to it
// after the box is stretched to the shape.
func (state iconState) linearGradient(at transform) (*ui.LinearGradient, *boxGradient, error) {
	reference := strings.TrimSpace(state.fill)
	id := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(reference, "url("), ")"), " '\"")
	id = strings.TrimPrefix(id, "#")
	node := state.gradients[id]
	if node == nil {
		return nil, nil, fmt.Errorf("fill %q names no gradient in the document", reference)
	}
	if node.XMLName.Local != "linearGradient" {
		return nil, nil, fmt.Errorf("gradient %q is a %s; only linearGradient is supported", id, node.XMLName.Local)
	}
	var box bool
	switch units := node.Attr("gradientUnits"); units {
	case "userSpaceOnUse":
	case "", "objectBoundingBox":
		box = true
	default:
		return nil, nil, fmt.Errorf("gradient %q: gradientUnits %q is not supported", id, units)
	}
	if node.Attr("gradientTransform") != "" || node.Attr("href") != "" {
		return nil, nil, fmt.Errorf("gradient %q: gradientTransform and href are not supported", id)
	}
	if spread := node.Attr("spreadMethod"); spread != "" && spread != "pad" {
		return nil, nil, fmt.Errorf("gradient %q: spreadMethod %q is not supported", id, spread)
	}
	var ends [4]float64
	defaults := [4]float64{0, 0, 1, 0} // SVG's, in box units
	for index, name := range [4]string{"x1", "y1", "x2", "y2"} {
		raw := strings.TrimSpace(node.Attr(name))
		if raw == "" && box {
			ends[index] = defaults[index]
			continue
		}
		fraction := strings.HasSuffix(raw, "%")
		if fraction && !box {
			return nil, nil, fmt.Errorf("gradient %q: %s is a percentage of the viewport, which is not supported", id, name)
		}
		value, err := strconv.ParseFloat(strings.TrimSuffix(raw, "%"), 64)
		if err != nil {
			return nil, nil, fmt.Errorf("gradient %q: %s must be a number", id, name)
		}
		if fraction {
			value /= 100
		}
		ends[index] = value
	}
	var offsets [2]float64
	var colors [2]ui.Color
	stops := 0
	for index := range node.Children {
		stop := &node.Children[index]
		if stop.XMLName.Local != "stop" {
			continue
		}
		if stops == 2 {
			return nil, nil, fmt.Errorf("gradient %q has more than two stops, which is not supported", id)
		}
		offset, color, err := parseStop(stop)
		if err != nil {
			return nil, nil, fmt.Errorf("gradient %q: %w", id, err)
		}
		offsets[stops], colors[stops] = offset, color
		stops++
	}
	if stops != 2 {
		return nil, nil, fmt.Errorf("gradient %q has %d stops; exactly two are supported", id, stops)
	}
	if offsets[0] >= offsets[1] {
		return nil, nil, fmt.Errorf("gradient %q: stop offsets %v and %v must increase", id, offsets[0], offsets[1])
	}
	opacity := float32(state.fillOpacity * state.opacity)
	colors[0].Alpha *= opacity
	colors[1].Alpha *= opacity
	if box {
		if at.b != 0 || at.c != 0 || at.a <= 0 || at.d <= 0 {
			return nil, nil, fmt.Errorf("gradient %q is in bounding-box units and drawn under a transform that rotates or flips", id)
		}
		if math.Abs(ends[1]-ends[3]) > 1e-9 && math.Abs(ends[0]-ends[2]) > 1e-9 {
			return nil, nil, fmt.Errorf("gradient %q is in bounding-box units and slanted; only horizontal and vertical ones are supported", id)
		}
		return &ui.LinearGradient{From: colors[0], To: colors[1]}, &boxGradient{ends: ends, offsets: offsets}, nil
	}
	a, b, c, d := at.a, at.b, at.c, at.d
	scale := a*a + b*b
	if math.Abs(scale-(c*c+d*d)) > 1e-6*scale || math.Abs(a*c+b*d) > 1e-6*scale {
		return nil, nil, fmt.Errorf("gradient %q is drawn under a transform that skews or scales unevenly", id)
	}
	dx, dy := ends[2]-ends[0], ends[3]-ends[1]
	startX, startY := at.apply(ends[0]+dx*offsets[0], ends[1]+dy*offsets[0])
	endX, endY := at.apply(ends[0]+dx*offsets[1], ends[1]+dy*offsets[1])
	return &ui.LinearGradient{
		Start: ui.Point{X: float32(startX), Y: float32(startY)},
		End:   ui.Point{X: float32(endX), Y: float32(endY)},
		From:  colors[0], To: colors[1],
	}, nil, nil
}

// parseStop reads a stop's offset (0 to 1, or a percentage) and its color, taken
// from the stop-color and stop-opacity attributes or the style attribute.
func parseStop(stop *svgNode) (float64, ui.Color, error) {
	property := func(name string) string {
		if value := stop.Attr(name); value != "" {
			return strings.TrimSpace(value)
		}
		for _, declaration := range strings.Split(stop.Attr("style"), ";") {
			if key, value, ok := strings.Cut(declaration, ":"); ok && strings.TrimSpace(key) == name {
				return strings.TrimSpace(value)
			}
		}
		return ""
	}
	offsetText := strings.TrimSpace(stop.Attr("offset"))
	scale := 1.0
	if strings.HasSuffix(offsetText, "%") {
		offsetText, scale = strings.TrimSuffix(offsetText, "%"), 0.01
	}
	offset := 0.0
	if offsetText != "" {
		value, err := strconv.ParseFloat(offsetText, 64)
		if err != nil {
			return 0, ui.Color{}, fmt.Errorf("stop offset %q is not a number", stop.Attr("offset"))
		}
		offset = min(max(value*scale, 0), 1)
	}
	color := ui.Color{Alpha: 1}
	switch text := property("stop-color"); {
	case text == "" || strings.EqualFold(text, "black"):
		// SVG's default stop color is black.
	case strings.EqualFold(text, "white"):
		color = ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	case strings.HasPrefix(text, "#"):
		parsed, err := parseHexColor(text)
		if err != nil {
			return 0, ui.Color{}, err
		}
		color = parsed
	default:
		return 0, ui.Color{}, fmt.Errorf("stop-color %q is not supported", text)
	}
	if text := property("stop-opacity"); text != "" {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, ui.Color{}, fmt.Errorf("stop-opacity %q is not a number", text)
		}
		color.Alpha *= float32(min(max(value, 0), 1))
	}
	return offset, color, nil
}
