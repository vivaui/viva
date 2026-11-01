package iconcompile

import (
	"fmt"
	"go/format"
	"math"
	"strconv"
	"strings"

	"github.com/vivaui/viva/ui"
)

// pathParser walks one SVG path data string. Numbers may be separated by
// commas, spaces, or nothing when unambiguous; command letters repeat.
type pathParser struct {
	input string
	pos   int
}

func (parser *pathParser) skipSpace() {
	for parser.pos < len(parser.input) {
		switch parser.input[parser.pos] {
		case ' ', '\t', '\n', '\r', ',':
			parser.pos++
		default:
			return
		}
	}
}

// nextCommand returns the next command letter, whether it is the relative
// form, and whether one was found.
func (parser *pathParser) nextCommand() (byte, bool, bool) {
	parser.skipSpace()
	if parser.pos >= len(parser.input) {
		return 0, false, false
	}
	letter := parser.input[parser.pos]
	if !isPathCommand(letter) {
		return 0, false, false
	}
	parser.pos++
	if letter >= 'a' && letter <= 'z' {
		return letter - 'a' + 'A', true, true
	}
	return letter, false, true
}

func isPathCommand(letter byte) bool {
	switch letter {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
		return true
	}
	return false
}

// number parses one path number, including the compact "1.5.5" form.
func (parser *pathParser) number() (float64, bool) {
	parser.skipSpace()
	start := parser.pos
	if parser.pos < len(parser.input) && (parser.input[parser.pos] == '-' || parser.input[parser.pos] == '+') {
		parser.pos++
	}
	digits := 0
	dots := 0
	for parser.pos < len(parser.input) {
		character := parser.input[parser.pos]
		if character >= '0' && character <= '9' {
			digits++
			parser.pos++
			continue
		}
		if character == '.' && dots == 0 {
			dots++
			parser.pos++
			continue
		}
		break
	}
	if digits == 0 {
		parser.pos = start
		return 0, false
	}
	// Exponent forms are rare in icons but legal.
	if parser.pos < len(parser.input) && (parser.input[parser.pos] == 'e' || parser.input[parser.pos] == 'E') {
		save := parser.pos
		parser.pos++
		if parser.pos < len(parser.input) && (parser.input[parser.pos] == '-' || parser.input[parser.pos] == '+') {
			parser.pos++
		}
		exponentDigits := 0
		for parser.pos < len(parser.input) && parser.input[parser.pos] >= '0' && parser.input[parser.pos] <= '9' {
			exponentDigits++
			parser.pos++
		}
		if exponentDigits == 0 {
			parser.pos = save
		}
	}
	value, err := strconv.ParseFloat(parser.input[start:parser.pos], 64)
	if err != nil {
		parser.pos = start
		return 0, false
	}
	return value, true
}

func (parser *pathParser) coords() (float64, float64, bool) {
	x, ok := parser.number()
	if !ok {
		return 0, 0, false
	}
	y, ok := parser.number()
	if !ok {
		return 0, 0, false
	}
	return x, y, true
}

func (parser *pathParser) four() (float64, float64, float64, float64, bool) {
	x1, ok := parser.number()
	if !ok {
		return 0, 0, 0, 0, false
	}
	y1, ok := parser.number()
	if !ok {
		return 0, 0, 0, 0, false
	}
	x2, ok := parser.number()
	if !ok {
		return 0, 0, 0, 0, false
	}
	y2, ok := parser.number()
	if !ok {
		return 0, 0, 0, 0, false
	}
	return x1, y1, x2, y2, true
}

func (parser *pathParser) six() (float64, float64, float64, float64, float64, float64, bool) {
	x1, y1, x2, y2, ok := parser.four()
	if !ok {
		return 0, 0, 0, 0, 0, 0, false
	}
	x3, ok := parser.number()
	if !ok {
		return 0, 0, 0, 0, 0, 0, false
	}
	y3, ok := parser.number()
	if !ok {
		return 0, 0, 0, 0, 0, 0, false
	}
	return x1, y1, x2, y2, x3, y3, true
}

// flag parses an arc flag: exactly one character, 0 or 1. Flags may be written
// with no separator after them ("0 110 3.846" is large-arc 1, sweep 1, x 0, y
// 3.846), so they cannot be read as numbers.
func (parser *pathParser) flag() (float64, bool) {
	parser.skipSpace()
	if parser.pos >= len(parser.input) {
		return 0, false
	}
	switch parser.input[parser.pos] {
	case '0':
		parser.pos++
		return 0, true
	case '1':
		parser.pos++
		return 1, true
	}
	return 0, false
}

// seven parses the seven arc arguments: two radii, the rotation, the two flags
// and the endpoint.
func (parser *pathParser) seven() ([7]float64, bool) {
	var args [7]float64
	for index := 0; index < 3; index++ {
		value, ok := parser.number()
		if !ok {
			return args, false
		}
		args[index] = value
	}
	for index := 3; index < 5; index++ {
		value, ok := parser.flag()
		if !ok {
			return args, false
		}
		args[index] = value
	}
	x, y, ok := parser.coords()
	if !ok {
		return args, false
	}
	args[5], args[6] = x, y
	return args, true
}

func (parser *pathParser) moreNumbers() bool {
	parser.skipSpace()
	if parser.pos >= len(parser.input) {
		return false
	}
	next := parser.input[parser.pos]
	if next == '-' || next == '+' || next == '.' || (next >= '0' && next <= '9') {
		return true
	}
	return false
}

func (state iconState) emitLine(node *svgNode, at transform, emit func(ui.PathCommand)) {
	x1, _ := strconv.ParseFloat(node.Attr("x1"), 64)
	y1, _ := strconv.ParseFloat(node.Attr("y1"), 64)
	x2, _ := strconv.ParseFloat(node.Attr("x2"), 64)
	y2, _ := strconv.ParseFloat(node.Attr("y2"), 64)
	x1t, y1t := state.point(at, x1, y1)
	x2t, y2t := state.point(at, x2, y2)
	emit(ui.MoveTo(x1t, y1t))
	emit(ui.LineTo(x2t, y2t))
}

func (state iconState) emitPoly(node *svgNode, at transform, closed bool, emit func(ui.PathCommand)) {
	fields := strings.Fields(node.Attr("points"))
	if len(fields) < 2 {
		return
	}
	var x, y float64
	for index, field := range fields {
		value, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return
		}
		if index%2 == 0 {
			x = value
			continue
		}
		y = value
		xt, yt := state.point(at, x, y)
		if index == 1 {
			emit(ui.MoveTo(xt, yt))
		} else {
			emit(ui.LineTo(xt, yt))
		}
	}
	if closed {
		emit(ui.ClosePath())
	}
}

func (state iconState) emitRect(node *svgNode, at transform, emit func(ui.PathCommand)) {
	x, _ := strconv.ParseFloat(node.Attr("x"), 64)
	y, _ := strconv.ParseFloat(node.Attr("y"), 64)
	width, _ := strconv.ParseFloat(node.Attr("width"), 64)
	height, _ := strconv.ParseFloat(node.Attr("height"), 64)
	rx, _ := strconv.ParseFloat(node.Attr("rx"), 64)
	ry, _ := strconv.ParseFloat(node.Attr("ry"), 64)
	if width <= 0 || height <= 0 {
		return
	}
	if rx <= 0 && ry > 0 {
		rx = ry
	}
	if ry <= 0 && rx > 0 {
		ry = rx
	}
	if rx > width/2 {
		rx = width / 2
	}
	if ry > height/2 {
		ry = height / 2
	}

	corners := [4][2]float64{
		{x + width - rx, y},
		{x + width, y + height - ry},
		{x + rx, y + height},
		{x, y + ry},
	}
	startX, startY := state.point(at, corners[0][0], corners[0][1])
	emit(ui.MoveTo(startX, startY))
	if rx > 0 {
		// Each arc is centered rx/ry inside its corner and sweeps a quarter turn
		// clockwise (y points down), leaving the point the previous edge ended on.
		state.arcCorner(at, x+width-rx, y+ry, rx, ry, -math.Pi/2, emit)
		px, py := state.point(at, corners[1][0], corners[1][1])
		emit(ui.LineTo(px, py))
		state.arcCorner(at, x+width-rx, y+height-ry, rx, ry, 0, emit)
		px, py = state.point(at, corners[2][0], corners[2][1])
		emit(ui.LineTo(px, py))
		state.arcCorner(at, x+rx, y+height-ry, rx, ry, math.Pi/2, emit)
		px, py = state.point(at, corners[3][0], corners[3][1])
		emit(ui.LineTo(px, py))
		state.arcCorner(at, x+rx, y+ry, rx, ry, math.Pi, emit)
	} else {
		for _, corner := range corners[1:] {
			px, py := state.point(at, corner[0], corner[1])
			emit(ui.LineTo(px, py))
		}
	}
	emit(ui.ClosePath())
}

// arcCorner emits one quarter-circle corner as a cubic approximation
// centered at (cx, cy) starting at the given angle.
func (state iconState) arcCorner(at transform, cx, cy, rx, ry, startAngle float64, emit func(ui.PathCommand)) {
	const kappa = 0.5522847498307936
	endAngle := startAngle + math.Pi/2
	c1x := cx + rx*math.Cos(startAngle)
	c1y := cy + ry*math.Sin(startAngle)
	c2x := cx + rx*math.Cos(endAngle) + kappa*rx*math.Sin(endAngle)
	c2y := cy + ry*math.Sin(endAngle) - kappa*ry*math.Cos(endAngle)
	toX := cx + rx*math.Cos(endAngle)
	toY := cy + ry*math.Sin(endAngle)
	// Control 1 tangent: leaving the start along the arc.
	t1x := c1x - kappa*rx*math.Sin(startAngle)
	t1y := c1y + kappa*ry*math.Cos(startAngle)
	a, b := state.point(at, t1x, t1y)
	c, d := state.point(at, c2x, c2y)
	e, f := state.point(at, toX, toY)
	emit(ui.CubicTo(a, b, c, d, e, f))
}

func (state iconState) emitCircle(node *svgNode, at transform, emit func(ui.PathCommand)) {
	cx, _ := strconv.ParseFloat(node.Attr("cx"), 64)
	cy, _ := strconv.ParseFloat(node.Attr("cy"), 64)
	radius, _ := strconv.ParseFloat(node.Attr("r"), 64)
	if radius <= 0 {
		return
	}
	state.emitEllipsePath(cx, cy, radius, radius, at, emit)
}

// emitEllipse draws an <ellipse>: the same four-quadrant approximation as a
// circle with independent radii. Lucide's cylinder and a few other icons use one
// for their top ring, so dropping it silently would leave the icon half-drawn.
func (state iconState) emitEllipse(node *svgNode, at transform, emit func(ui.PathCommand)) {
	cx, _ := strconv.ParseFloat(node.Attr("cx"), 64)
	cy, _ := strconv.ParseFloat(node.Attr("cy"), 64)
	rx, _ := strconv.ParseFloat(node.Attr("rx"), 64)
	ry, _ := strconv.ParseFloat(node.Attr("ry"), 64)
	if rx <= 0 || ry <= 0 {
		return
	}
	state.emitEllipsePath(cx, cy, rx, ry, at, emit)
}

func (state iconState) emitEllipsePath(cx, cy, radius, ry float64, at transform, emit func(ui.PathCommand)) {
	// radius is the horizontal radius; ry the vertical one.
	if ry <= 0 {
		ry = radius
	}
	if radius <= 0 {
		return
	}
	offsetX := radius * 0.5522847498307936
	offsetY := ry * 0.5522847498307936
	startX, startY := state.point(at, cx+radius, cy)
	emit(ui.MoveTo(startX, startY))
	quarter := func(c1x, c1y, c2x, c2y, x, y float64) {
		c1, d1 := state.point(at, c1x, c1y)
		c2, d2 := state.point(at, c2x, c2y)
		end1, end2 := state.point(at, x, y)
		emit(ui.CubicTo(c1, d1, c2, d2, end1, end2))
	}
	quarter(cx+radius, cy+offsetY, cx+offsetX, cy+ry, cx, cy+ry)
	quarter(cx-offsetX, cy+ry, cx-radius, cy+offsetY, cx-radius, cy)
	quarter(cx-radius, cy-offsetY, cx-offsetX, cy-ry, cx, cy-ry)
	quarter(cx+offsetX, cy-ry, cx+radius, cy-offsetY, cx+radius, cy)
	emit(ui.ClosePath())
}

func formatFloat(value float32) string {
	text := strconv.FormatFloat(float64(value), 'g', 6, 32)
	if !strings.ContainsAny(text, ".eE") {
		text += ".0"
	}
	return text
}

// Generate writes one Go source file declaring every named icon.
func Generate(packageName string, icons map[string]ui.Icon, license string) ([]byte, error) {
	if packageName == "" {
		packageName = "icons"
	}
	var builder strings.Builder
	builder.WriteString("// Code generated by viva icons. DO NOT EDIT.\n")
	if license != "" {
		for _, line := range strings.Split(strings.TrimSpace(license), "\n") {
			builder.WriteString("// ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\npackage ")
	builder.WriteString(packageName)
	builder.WriteString("\n\nimport \"github.com/vivaui/viva/ui\"\n\n")
	names := make([]string, 0, len(icons))
	for name := range icons {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		icon := icons[name]
		fmt.Fprintf(&builder, "// %s is a compiled vector icon.\nvar %s = ui.Icon{\n", name, exportName(name))
		fmt.Fprintf(&builder, "\tViewBox: %s,\n", formatFloat(icon.ViewBox))
		if icon.ViewHeight > 0 {
			fmt.Fprintf(&builder, "\tViewHeight: %s,\n", formatFloat(icon.ViewHeight))
		}
		fmt.Fprintf(&builder, "\tStroke: %s,\n", formatFloat(icon.Stroke))
		fmt.Fprintf(&builder, "\tCap: ui.%s,\n", capName(icon.Cap))
		fmt.Fprintf(&builder, "\tJoin: ui.%s,\n", joinName(icon.Join))
		if len(icon.Commands) > 0 {
			builder.WriteString("\tCommands: []ui.PathCommand{\n")
			for _, command := range icon.Commands {
				builder.WriteString("\t\t")
				builder.WriteString(commandSource(command))
				builder.WriteString(",\n")
			}
			builder.WriteString("\t},\n")
		}
		if len(icon.Layers) > 0 {
			builder.WriteString("\tLayers: []ui.IconLayer{\n")
			for _, layer := range icon.Layers {
				fmt.Fprintf(&builder, "\t\t{Fill: ui.Color{Red: %s, Green: %s, Blue: %s, Alpha: %s}, ",
					formatFloat(layer.Fill.Red), formatFloat(layer.Fill.Green), formatFloat(layer.Fill.Blue), formatFloat(layer.Fill.Alpha))
				if layer.FillRule == ui.PathFillEvenOdd {
					builder.WriteString("FillRule: ui.PathFillEvenOdd, ")
				}
				if gradient := layer.Gradient; gradient != nil {
					fmt.Fprintf(&builder, "Gradient: &ui.LinearGradient{Start: ui.Point{X: %s, Y: %s}, End: ui.Point{X: %s, Y: %s}, ",
						formatFloat(gradient.Start.X), formatFloat(gradient.Start.Y), formatFloat(gradient.End.X), formatFloat(gradient.End.Y))
					fmt.Fprintf(&builder, "From: ui.Color{Red: %s, Green: %s, Blue: %s, Alpha: %s}, To: ui.Color{Red: %s, Green: %s, Blue: %s, Alpha: %s}}, ",
						formatFloat(gradient.From.Red), formatFloat(gradient.From.Green), formatFloat(gradient.From.Blue), formatFloat(gradient.From.Alpha),
						formatFloat(gradient.To.Red), formatFloat(gradient.To.Green), formatFloat(gradient.To.Blue), formatFloat(gradient.To.Alpha))
				}
				builder.WriteString("Commands: []ui.PathCommand{\n")
				for _, command := range layer.Commands {
					builder.WriteString("\t\t\t")
					builder.WriteString(commandSource(command))
					builder.WriteString(",\n")
				}
				builder.WriteString("\t\t}},\n")
			}
			builder.WriteString("\t},\n")
		}
		builder.WriteString("}\n\n")
	}
	// Generated source must be gofmt-clean so regenerating never churns a diff.
	formatted, err := format.Source([]byte(builder.String()))
	if err != nil {
		return nil, fmt.Errorf("iconcompile: format generated source: %w", err)
	}
	return formatted, nil
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func exportName(name string) string {
	parts := strings.Split(name, "-")
	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		builder.WriteString(part[1:])
	}
	return builder.String()
}

func capName(cap ui.StrokeCap) string {
	switch cap {
	case ui.CapRound:
		return "CapRound"
	case ui.CapSquare:
		return "CapSquare"
	default:
		return "CapButt"
	}
}

func joinName(join ui.StrokeJoin) string {
	switch join {
	case ui.JoinRound:
		return "JoinRound"
	case ui.JoinMiter:
		return "JoinMiter"
	default:
		return "JoinBevel"
	}
}

func commandSource(command ui.PathCommand) string {
	switch command.Kind {
	case ui.PathCommandMove:
		return fmt.Sprintf("ui.MoveTo(%s, %s)", formatFloat(command.Point.X), formatFloat(command.Point.Y))
	case ui.PathCommandLine:
		return fmt.Sprintf("ui.LineTo(%s, %s)", formatFloat(command.Point.X), formatFloat(command.Point.Y))
	case ui.PathCommandQuad:
		return fmt.Sprintf("ui.QuadTo(%s, %s, %s, %s)",
			formatFloat(command.Control1.X), formatFloat(command.Control1.Y),
			formatFloat(command.Point.X), formatFloat(command.Point.Y))
	case ui.PathCommandCubic:
		return fmt.Sprintf("ui.CubicTo(%s, %s, %s, %s, %s, %s)",
			formatFloat(command.Control1.X), formatFloat(command.Control1.Y),
			formatFloat(command.Control2.X), formatFloat(command.Control2.Y),
			formatFloat(command.Point.X), formatFloat(command.Point.Y))
	default:
		return "ui.ClosePath()"
	}
}
