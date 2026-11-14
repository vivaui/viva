// Package iconcompile turns SVG icon sources into gfx.Icon Go values at
// build time. It understands the icon-dialect subset used by Lucide and
// similar stroke-icon sets: path, line, polyline, polygon, rect, circle and ellipse
// elements with fill="none" stroke-based geometry; groups and their
// transforms; and currentColor-aware defaults. An element with a concrete fill
// color (fill="#e65100", with fill-opacity, opacity and fill-rule) becomes a
// colored layer of the icon, which is how multi-color glyphs such as file-type
// icons compile. A fill of url(#id) naming a linearGradient with two stops and no
// gradientTransform becomes a gfx.LinearGradient layer, in userSpaceOnUse units or,
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

	"github.com/vivaui/viva/gfx"
)

// Compile parses one SVG document into a gfx.Icon. Width and height default
// to the viewBox; stroke metadata is taken from the document with Lucide
// defaults (stroke 2, round cap and join). A stroke-width of 0 on the root
// element means the icon's tinted shapes are filled instead.
func Compile(name string, source []byte) (gfx.Icon, error) {
	document := &svgNode{}
	if err := xml.Unmarshal(source, document); err != nil {
		return gfx.Icon{}, fmt.Errorf("%s: %w", name, err)
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
		return gfx.Icon{}, fmt.Errorf("%s: no svg element", name)
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
		cap:    gfx.CapRound,
		join:   gfx.JoinRound,
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
	state.ids = map[string]*svgNode{}
	collectIDs(root, state.ids)
	state.walk(root, origin, sink)
	if sink.err != nil {
		return gfx.Icon{}, fmt.Errorf("%s: %w", name, sink.err)
	}
	if len(sink.commands) == 0 && len(sink.layers) == 0 {
		return gfx.Icon{}, fmt.Errorf("%s: no drawable geometry", name)
	}
	if viewHeight == viewBox {
		viewHeight = 0 // square: the icon's ViewHeight stays unset
	}
	return gfx.Icon{
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
	commands []gfx.PathCommand
	layers   []gfx.IconLayer
	layer    int // the layer being filled, or -1 for the stroked commands
	err      error
}

func (sink *iconSink) add(command gfx.PathCommand) {
	if sink.layer >= 0 {
		sink.layers[sink.layer].Commands = append(sink.layers[sink.layer].Commands, command)
		return
	}
	sink.commands = append(sink.commands, command)
}

func (sink *iconSink) begin(fill gfx.Color, rule gfx.PathFillRule, gradient *gfx.LinearGradient) {
	sink.layers = append(sink.layers, gfx.IconLayer{Fill: fill, FillRule: rule, Gradient: gradient})
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
	at := func(offset float64) gfx.Point {
		return gfx.Point{
			X: float32(minX + (box.ends[0]+dx*offset)*width),
			Y: float32(minY + (box.ends[1]+dy*offset)*height),
		}
	}
	layer.Gradient.Start, layer.Gradient.End = at(box.offsets[0]), at(box.offsets[1])
}

// commandBounds is the bounding box of a path, with each curve sampled, which is
// within a small fraction of a unit of the exact one for icon-sized curves.
func commandBounds(commands []gfx.PathCommand) (minX, minY, maxX, maxY float64, ok bool) {
	minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(x, y float64) {
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
		ok = true
	}
	var cx, cy, sx, sy float64
	for _, command := range commands {
		switch command.Kind {
		case gfx.PathCommandMove:
			cx, cy = float64(command.Point.X), float64(command.Point.Y)
			sx, sy = cx, cy
			add(cx, cy)
		case gfx.PathCommandLine:
			cx, cy = float64(command.Point.X), float64(command.Point.Y)
			add(cx, cy)
		case gfx.PathCommandQuad:
			x1, y1 := float64(command.Control1.X), float64(command.Control1.Y)
			x2, y2 := float64(command.Point.X), float64(command.Point.Y)
			for step := 1; step <= 32; step++ {
				t := float64(step) / 32
				u := 1 - t
				add(u*u*cx+2*u*t*x1+t*t*x2, u*u*cy+2*u*t*y1+t*t*y2)
			}
			cx, cy = x2, y2
		case gfx.PathCommandCubic:
			x1, y1 := float64(command.Control1.X), float64(command.Control1.Y)
			x2, y2 := float64(command.Control2.X), float64(command.Control2.Y)
			x3, y3 := float64(command.Point.X), float64(command.Point.Y)
			for step := 1; step <= 32; step++ {
				t := float64(step) / 32
				u := 1 - t
				add(u*u*u*cx+3*u*u*t*x1+3*u*t*t*x2+t*t*t*x3, u*u*u*cy+3*u*u*t*y1+3*u*t*t*y2+t*t*t*y3)
			}
			cx, cy = x3, y3
		case gfx.PathCommandClose:
			cx, cy = sx, sy
		}
	}
	return minX, minY, maxX, maxY, ok
}

func parseCap(raw string) gfx.StrokeCap {
	switch strings.TrimSpace(raw) {
	case "round":
		return gfx.CapRound
	case "square":
		return gfx.CapSquare
	default:
		return gfx.CapButt
	}
}

func parseJoin(raw string) gfx.StrokeJoin {
	switch strings.TrimSpace(raw) {
	case "round":
		return gfx.JoinRound
	case "miter", "miter-clip", "arcs":
		return gfx.JoinMiter
	default:
		return gfx.JoinBevel
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
	cap    gfx.StrokeCap
	join   gfx.StrokeJoin
	// Inherited paint: the fill attribute ("" when none was given), its
	// opacities and the fill rule.
	fill        string
	strokePaint string // the stroke attribute, inherited ("" when none was given)
	fillOpacity float64
	opacity     float64
	fillRule    gfx.PathFillRule
	// gradients are the document's gradient elements by id.
	gradients map[string]*svgNode
	// ids are the document's id-bearing drawable elements (paths, groups,
	// markers) by id, for <use> and marker references.
	ids map[string]*svgNode
	// useStack holds the ids of the <use> elements being expanded, plus the
	// referenced target id, to reject circular references.
	useStack []string
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
			state.fillRule = gfx.PathFillEvenOdd
		} else {
			state.fillRule = gfx.PathFillNonZero
		}
	}
	return state
}

// color resolves the element's fill: colored is false when the element is a
// plain stroke-or-currentColor shape and the icon's tint applies.
func (state iconState) color() (gfx.Color, bool, error) {
	switch fill := state.fill; {
	case fill == "" || fill == "none" || strings.EqualFold(fill, "currentColor"):
		return gfx.Color{}, false, nil
	case strings.HasPrefix(fill, "#"):
		color, err := parseHexColor(fill)
		if err != nil {
			return gfx.Color{}, false, err
		}
		color.Alpha *= float32(state.fillOpacity * state.opacity)
		return color, true, nil
	case strings.EqualFold(fill, "black"):
		return gfx.Color{Alpha: float32(state.fillOpacity * state.opacity)}, true, nil
	case strings.EqualFold(fill, "white"):
		return gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: float32(state.fillOpacity * state.opacity)}, true, nil
	}
	return gfx.Color{}, false, fmt.Errorf("fill %q is not supported", state.fill)
}

// invisible reports whether an element paints nothing: its fill is none and no
// stroke was given. Icon sets use such elements as transparent bounding boxes.
// An element with no paint attributes at all is not invisible; it keeps the
// stroked-glyph default.
func (state iconState) invisible() bool {
	return state.fill == "none" && (state.strokePaint == "" || state.strokePaint == "none")
}

// parseHexColor reads #rgb, #rgba, #rrggbb or #rrggbbaa.
func parseHexColor(raw string) (gfx.Color, error) {
	digits := strings.TrimPrefix(raw, "#")
	if len(digits) == 3 || len(digits) == 4 {
		expanded := make([]byte, 0, 8)
		for index := 0; index < len(digits); index++ {
			expanded = append(expanded, digits[index], digits[index])
		}
		digits = string(expanded)
	}
	if len(digits) != 6 && len(digits) != 8 {
		return gfx.Color{}, fmt.Errorf("color %q is not #rgb, #rgba, #rrggbb or #rrggbbaa", raw)
	}
	value, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return gfx.Color{}, fmt.Errorf("color %q: %w", raw, err)
	}
	color := gfx.Color{Alpha: 1}
	if len(digits) == 8 {
		color.Alpha = float32(value&0xff) / 255
		value >>= 8
	}
	color.Red = float32(value>>16&0xff) / 255
	color.Green = float32(value>>8&0xff) / 255
	color.Blue = float32(value&0xff) / 255
	return color, nil
}

// walk draws node's children: groups recurse with their transform, use
// elements expand their referenced target, and drawable elements paint.
func (state iconState) walk(node *svgNode, at transform, sink *iconSink) {
	for index := range node.Children {
		child := &node.Children[index]
		name := child.XMLName.Local
		if name == "g" {
			next := at
			if raw := child.Attr("transform"); raw != "" {
				next = at.then(parseTransform(raw))
			}
			state.inherit(child).walk(child, next, sink)
			continue
		}
		if name == "use" {
			state.expandUse(child, at, sink)
			continue
		}
		state.emitOne(child, at, sink)
	}
}

// emitOne paints one drawable element in its parent's coordinate system: its
// own transform and paint compose onto the incoming state, colored shapes open
// a layer, and the flag icon sets' vertex markers follow their path.
func (state iconState) emitOne(node *svgNode, at transform, sink *iconSink) {
	childState := state.inherit(node)
	name := node.XMLName.Local
	elementAt := at
	if raw := node.Attr("transform"); raw != "" {
		elementAt = at.then(parseTransform(raw))
	}
	switch name {
	case "g", "use", "defs", "title", "desc", "metadata", "style", "linearGradient", "radialGradient", "clipPath", "mask", "filter", "marker":
		// Handled by walk or not drawn by themselves.
		return
	}
	var (
		fill     gfx.Color
		colored  bool
		gradient *gfx.LinearGradient
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
		// An unpainted path still places its markers: that is exactly how the
		// US star field is drawn — a fill-none outline whose vertices carry
		// the stars.
		state.emitMarkers(node, at, sink)
		return
	}
	if colored {
		sink.begin(fill, childState.fillRule, gradient)
	}
	emit := sink.add
	switch name {
	case "path":
		state.emitPathData(node.Attr("d"), elementAt, emit)
		// Star fields of flag icons place a marker at every intermediate
		// vertex of the outline (US-SVG's marker-mid).
		state.emitMarkers(node, at, sink)
	case "line":
		state.emitLine(node, elementAt, emit)
	case "polyline", "polygon":
		state.emitPoly(node, elementAt, name == "polygon", emit)
	case "rect":
		state.emitRect(node, elementAt, emit)
	case "circle":
		state.emitCircle(node, elementAt, emit)
	case "ellipse":
		state.emitEllipse(node, elementAt, emit)
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

// expandUse draws one <use> element: the referenced target is expanded as if
// deeply cloned here, so its shapes inherit paint from the use site unless
// they carry their own, and the use element's transform and x/y offset place
// the clone. A target that is itself a <use> dereferences again, up to a
// bounded depth: a cycle is a compile error, not an infinite icon.
func (state iconState) expandUse(use *svgNode, at transform, sink *iconSink) {
	id := referencedID(use)
	if id == "" {
		if sink.err == nil {
			sink.err = fmt.Errorf("<use> without a target")
		}
		return
	}
	target := state.ids[id]
	if target == nil {
		// An invalid reference draws nothing, as in SVG 1.1.
		return
	}
	for _, active := range state.useStack {
		if active == id {
			if sink.err == nil {
				sink.err = fmt.Errorf("<use> cycle through #%s", id)
			}
			return
		}
	}
	if len(state.useStack) >= maxUseDepth {
		if sink.err == nil {
			sink.err = fmt.Errorf("<use> nested more than %d deep through #%s", maxUseDepth, id)
		}
		return
	}
	next := at
	if raw := use.Attr("transform"); raw != "" {
		next = next.then(parseTransform(raw))
	}
	if raw := use.Attr("x"); raw != "" {
		if x, err := strconv.ParseFloat(raw, 64); err == nil {
			next = next.then(transform{1, 0, 0, 1, x, 0})
		}
	}
	if raw := use.Attr("y"); raw != "" {
		// y="44" on a use of the Korean flag is a plain shift of the bars.
		if y, err := strconv.ParseFloat(raw, 64); err == nil {
			next = next.then(transform{1, 0, 0, 1, 0, y})
		}
	}
	// width/height would scale the target of a <use>, but SVG only applies
	// them to <svg> and <symbol> targets; on shapes and groups (every real
	// icon source, including the flag sets) they are inert.
	useState := state.inherit(use)
	useState.useStack = append(state.useStack, id)
	switch target.XMLName.Local {
	case "g":
		nextG := next
		if raw := target.Attr("transform"); raw != "" {
			nextG = next.then(parseTransform(raw))
		}
		// A group target is cloned: its own paint and transform compose like
		// a <g> in flow at the use site, then its children walk underneath.
		useState.inherit(target).walk(target, nextG, sink)
	case "use":
		// A use pointing at a use dereferences through the same rules.
		useState.expandUse(target, next, sink)
	default:
		// A single shape target paints like the same node sitting in flow
		// where the use is; x/y and transform above carried it so far.
		useState.emitOne(target, next, sink)
	}
}

// referencedID reads the href (or xlink:href, whose parsed name is also
// href) attribute of a reference and strips the leading #.
func referencedID(node *svgNode) string {
	return strings.TrimPrefix(strings.TrimSpace(node.Attr("href")), "#")
}

// maxUseDepth bounds use dereferences, including through nested groups.
const maxUseDepth = 32

func (state iconState) point(at transform, x, y float64) (float32, float32) {
	px, py := at.apply(x, y)
	return float32(px), float32(py)
}

func (state iconState) emitPathData(data string, at transform, emit func(gfx.PathCommand)) {
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
					emit(gfx.MoveTo(x32, y32))
					first = false
					if !parser.moreNumbers() {
						break
					}
					continue
				}
				// Subsequent coordinate pairs after a move are implicit
				// lineto commands in the same relative/absolute mode.
				emit(gfx.LineTo(x32, y32))
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
				emit(gfx.LineTo(x32, y32))
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
				emit(gfx.LineTo(x32, y32))
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
				emit(gfx.LineTo(x32, y32))
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
				emit(gfx.CubicTo(c1x32, c1y32, c2x32, c2y32, x32, y32))
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
				emit(gfx.CubicTo(c1x32, c1y32, c2x32, c2y32, x32, y32))
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
				emit(gfx.QuadTo(cx32, cy32, x32, y32))
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
				emit(gfx.QuadTo(cx32, cy32, x32, y32))
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
			emit(gfx.ClosePath())
			currentX, currentY = startX, startY
		}
		lastCommand = command
	}
}

// emitArc converts one SVG arc from endpoint to center parameterization
// and emits it as cubic segments, following the W3C SVG implementation
// notes.
func (state iconState) emitArc(currentX, currentY float64, args [7]float64, relative bool, at transform, emit func(gfx.PathCommand)) {
	rx, ry, rotation, largeArc, sweep, x, y := args[0], args[1], args[2], args[3], args[4], args[5], args[6]
	if relative {
		x, y = x+currentX, y+currentY
	}
	if rx == 0 || ry == 0 || (x == currentX && y == currentY) {
		x32, y32 := state.point(at, x, y)
		emit(gfx.LineTo(x32, y32))
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
		emit(gfx.CubicTo(c1x32, c1y32, c2x32, c2y32, toX32, toY32))
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

// collectIDs indexes every id-bearing element that <use> and marker
// references can point at: shapes, groups and markers, wherever they are
// defined, including inside <defs>. Flag icon sets reference a shared path or
// group from several <use> elements, each with its own transform.
func collectIDs(node *svgNode, into map[string]*svgNode) {
	for index := range node.Children {
		child := &node.Children[index]
		switch child.XMLName.Local {
		case "path", "line", "polyline", "polygon", "rect", "circle", "ellipse", "g", "marker", "use":
			if id := child.Attr("id"); id != "" {
				if _, taken := into[id]; !taken {
					into[id] = child
				}
			}
		}
		collectIDs(child, into)
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
func (state iconState) linearGradient(at transform) (*gfx.LinearGradient, *boxGradient, error) {
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
	var colors [2]gfx.Color
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
		return &gfx.LinearGradient{From: colors[0], To: colors[1]}, &boxGradient{ends: ends, offsets: offsets}, nil
	}
	a, b, c, d := at.a, at.b, at.c, at.d
	scale := a*a + b*b
	if math.Abs(scale-(c*c+d*d)) > 1e-6*scale || math.Abs(a*c+b*d) > 1e-6*scale {
		return nil, nil, fmt.Errorf("gradient %q is drawn under a transform that skews or scales unevenly", id)
	}
	dx, dy := ends[2]-ends[0], ends[3]-ends[1]
	startX, startY := at.apply(ends[0]+dx*offsets[0], ends[1]+dy*offsets[0])
	endX, endY := at.apply(ends[0]+dx*offsets[1], ends[1]+dy*offsets[1])
	return &gfx.LinearGradient{
		Start: gfx.Point{X: float32(startX), Y: float32(startY)},
		End:   gfx.Point{X: float32(endX), Y: float32(endY)},
		From:  colors[0], To: colors[1],
	}, nil, nil
}

// parseStop reads a stop's offset (0 to 1, or a percentage) and its color, taken
// from the stop-color and stop-opacity attributes or the style attribute.
func parseStop(stop *svgNode) (float64, gfx.Color, error) {
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
			return 0, gfx.Color{}, fmt.Errorf("stop offset %q is not a number", stop.Attr("offset"))
		}
		offset = min(max(value*scale, 0), 1)
	}
	color := gfx.Color{Alpha: 1}
	switch text := property("stop-color"); {
	case text == "" || strings.EqualFold(text, "black"):
		// SVG's default stop color is black.
	case strings.EqualFold(text, "white"):
		color = gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	case strings.HasPrefix(text, "#"):
		parsed, err := parseHexColor(text)
		if err != nil {
			return 0, gfx.Color{}, err
		}
		color = parsed
	default:
		return 0, gfx.Color{}, fmt.Errorf("stop-color %q is not supported", text)
	}
	if text := property("stop-opacity"); text != "" {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, gfx.Color{}, fmt.Errorf("stop-opacity %q is not a number", text)
		}
		color.Alpha *= float32(min(max(value, 0), 1))
	}
	return offset, color, nil
}

// emitMarkers draws the markers a path element names at its vertices. Only the
// star fields of the flag icon sets use markers today, all through marker-mid,
// which places a marker at every vertex apart from the first of each subpath
// (a closed outline's last vertex keeps its marker, as browsers draw it).
// marker-start and marker-end are placed by the same machinery, one at a
// subpath's first, respectively last, vertex, but no yielded source exercises
// them yet. A marker with no orient attribute — every real source — stays at
// angle 0, matching the browsers (SVG's default; the US star is drawn upright
// at every vertex). orient="auto" turns the content to the bisector of the
// two segments at the vertex.
func (state iconState) emitMarkers(path *svgNode, at transform, sink *iconSink) {
	mid := path.Attr("marker-mid")
	start, end := path.Attr("marker-start"), path.Attr("marker-end")
	if mid == "" && start == "" && end == "" {
		return
	}
	// markerUnits defaults to strokeWidth, so the marker content scales with
	// the path's stroke (1 in the flag sets, where paths carry no stroke).
	scale := 1.0
	if raw := strings.TrimSpace(path.Attr("stroke-width")); raw != "" {
		if width, err := strconv.ParseFloat(raw, 64); err == nil && width > 0 {
			scale = width
		}
	}
	for _, subpath := range skeletonVertices(path.Attr("d")) {
		for index, vertex := range subpath.vertices {
			switch {
			case index == 0 && start != "":
				state.placeMarker(start, vertex, at, scale, sink)
			case index == len(subpath.vertices)-1 && end != "":
				// An end marker belongs to the last vertex; on a closed
				// subpath that vertex is also where the closepath lands.
				state.placeMarker(end, vertex, at, scale, sink)
			case index == 0 || index == len(subpath.vertices)-1 && !subpath.closed:
				// The first vertex, and the last of an open subpath, are the
				// start and end property's, not marker-mid's.
				continue
			case mid != "":
				state.placeMarker(mid, vertex, at, scale, sink)
			}
		}
	}
}

// placeMarker expands one marker element at a vertex. The content is cloned
// at the vertex, shifted by negated refX/refY, scaled by the path's stroke
// width under the default markerUnits, and turned per orient: a fixed angle
// (absent is 0) or the bisector of the vertex's two directions for auto.
func (state iconState) placeMarker(reference string, vertex pathVertex, at transform, scale float64, sink *iconSink) {
	id := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(reference), "url("), ")"), " '\"")
	id = strings.TrimPrefix(id, "#")
	marker := state.ids[id]
	if marker == nil || marker.XMLName.Local != "marker" {
		return // An invalid reference draws nothing, as in the browsers.
	}
	if strings.TrimSpace(marker.Attr("markerUnits")) == "userSpaceOnUse" {
		scale = 1
	}
	placed := transform{scale, 0, 0, scale, 0, 0}
	angle := 0.0
	if orient := strings.TrimSpace(marker.Attr("orient")); orient == "auto" {
		// Weighted with the segment lengths normalized: the bisector of the
		// unit tangents, as SVG specifies.
		inX, inY := vertex.in[0], vertex.in[1]
		outX, outY := vertex.out[0], vertex.out[1]
		lengthIn := math.Hypot(inX, inY)
		lengthOut := math.Hypot(outX, outY)
		if lengthIn > 0 && lengthOut > 0 {
			bisX, bisY := inX/lengthIn+outX/lengthOut, inY/lengthIn+outY/lengthOut
			angle = math.Atan2(bisY, bisX)
		} else if lengthOut > 0 {
			angle = math.Atan2(outY, outX)
		} else if lengthIn > 0 {
			angle = math.Atan2(inY, inX)
		}
	} else if orient != "" && orient != "0" {
		if parsed, err := strconv.ParseFloat(orient, 64); err == nil {
			angle = parsed * math.Pi / 180
		}
	}
	cos, sin := math.Cos(angle), math.Sin(angle)
	rotation := transform{cos, sin, -sin, cos, 0, 0}
	view := markerToView(marker)
	translate := transform{1, 0, 0, 1, vertex.point[0], vertex.point[1]}
	ref := transform{1, 0, 0, 1, -markerNumber(marker.Attr("refX")) * view, -markerNumber(marker.Attr("refY")) * view}
	place := at.then(translate).then(rotation).then(placed).then(ref)
	// The marker's children are cloned with the paint in scope here.
	state.walk(marker, place, sink)
}

// markerToView converts marker content coordinates into marker viewport units
// when the marker declares a viewBox; without one — the flag sets' case — the
// content is already in viewport units.
func markerToView(marker *svgNode) float64 {
	raw := marker.Attr("viewBox")
	if raw == "" {
		return 1
	}
	fields := strings.Fields(raw)
	if len(fields) != 4 {
		return 1
	}
	width, err := strconv.ParseFloat(fields[2], 64)
	viewport := markerNumber(marker.Attr("markerWidth"))
	if err != nil || width <= 0 || viewport <= 0 {
		return 1
	}
	return viewport / width
}

// markerNumber reads a marker geometry attribute, treating SVG's bare-value
// form; anything unparsable is SVG's zero default.
func markerNumber(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}

// pathVertex is one vertex of a path's straight-line skeleton: the point plus
// the directions of the segments entering and leaving it (a curve flattens to
// its chord).
type pathVertex struct {
	point [2]float64
	in    [2]float64 // the incoming segment direction; zero at a subpath's first vertex
	out   [2]float64 // the outgoing segment direction; set by the next vertex or closepath
}

// pathSkeleton is one subpath of the vertex skeleton.
type pathSkeleton struct {
	vertices []pathVertex
	closed   bool
}

// skeletonVertices parses path data into per-subpath vertex lists with segment
// directions: every move and every explicit endpoint (line, curve or arc) is a
// vertex; the direction of a curve is its chord. Marker sources are polyline
// outlines, where both are exact.
func skeletonVertices(data string) []pathSkeleton {
	parser := &pathParser{input: data}
	var currentX, currentY, startX, startY float64
	var subpaths []pathSkeleton
	var current []pathVertex
	closed := false
	var inX, inY float64 // direction of the segment ending at the current point
	flush := func() {
		if len(current) > 0 {
			subpaths = append(subpaths, pathSkeleton{vertices: current, closed: closed})
			current = nil
		}
		closed = false
	}
	appendVertex := func(x, y, iX, iY float64) {
		if len(current) > 0 {
			previous := &current[len(current)-1]
			previous.out = [2]float64{x - previous.point[0], y - previous.point[1]}
		}
		current = append(current, pathVertex{point: [2]float64{x, y}, in: [2]float64{iX, iY}})
	}
	for {
		command, relative, ok := parser.nextCommand()
		if !ok {
			break
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
					// Coordinate pairs after the first of an 'm' are implicit
					// relative linetos of the same subpath.
					x, y = x+currentX, y+currentY
					inX, inY = x-currentX, y-currentY
					currentX, currentY = x, y
					appendVertex(x, y, inX, inY)
				} else {
					flush()
					currentX, currentY = x, y
					startX, startY = x, y
					appendVertex(x, y, 0, 0)
				}
				first = false
				if !parser.moreNumbers() {
					break
				}
			}
		case 'L':
			for {
				x, y, ok := parser.coords()
				if !ok {
					break
				}
				if relative {
					x, y = x+currentX, y+currentY
				}
				inX, inY = x-currentX, y-currentY
				currentX, currentY = x, y
				appendVertex(x, y, inX, inY)
				if !parser.moreNumbers() {
					break
				}
			}
		case 'H':
			for {
				x, ok := parser.number()
				if !ok {
					break
				}
				if relative {
					x += currentX
				}
				inX, inY = x-currentX, 0
				currentX = x
				appendVertex(x, currentY, inX, inY)
				if !parser.moreNumbers() {
					break
				}
			}
		case 'V':
			for {
				y, ok := parser.number()
				if !ok {
					break
				}
				if relative {
					y += currentY
				}
				inX, inY = 0, y-currentY
				currentY = y
				appendVertex(currentX, y, inX, inY)
				if !parser.moreNumbers() {
					break
				}
			}
		case 'C', 'S', 'Q', 'T':
			var pairs int
			if command == 'C' || command == 'S' {
				pairs = 3
			} else {
				pairs = 2
			}
			for {
				var last [2]float64
				ok := true
				for pair := 0; pair < pairs; pair++ {
					x, y, got := parser.coords()
					if !got {
						ok = false
						break
					}
					last = [2]float64{x, y}
				}
				if !ok {
					break
				}
				if relative {
					last = [2]float64{last[0] + currentX, last[1] + currentY}
				}
				inX, inY = last[0]-currentX, last[1]-currentY
				currentX, currentY = last[0], last[1]
				appendVertex(last[0], last[1], inX, inY)
				if !parser.moreNumbers() {
					break
				}
			}
		case 'A':
			for {
				args, ok := parser.seven()
				if !ok {
					break
				}
				x, y := args[5], args[6]
				if relative {
					x, y = x+currentX, y+currentY
				}
				inX, inY = x-currentX, y-currentY
				currentX, currentY = x, y
				appendVertex(x, y, inX, inY)
				if !parser.moreNumbers() {
					break
				}
			}
		case 'Z':
			// A close emits no vertex; the closing line's direction is stored
			// as the last vertex's outgoing segment, and the subpath is marked
			// closed: its initial vertex is this vertex too, which browsers
			// honour by drawing the mid marker on the word's last vertex.
			closed = true
			if len(current) > 0 {
				last := &current[len(current)-1]
				last.out = [2]float64{startX - last.point[0], startY - last.point[1]}
			}
			currentX, currentY = startX, startY
		}
	}
	flush()
	return subpaths
}
