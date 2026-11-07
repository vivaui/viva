//go:build darwin || windows || linux

package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// tolerance is the layout drift, in logical pixels, that is reported.
const tolerance = 0.5

// side is one platform's outputs.
type side struct {
	dir      string
	platform Platform
	facts    map[string]*Facts // by scene.theme
	metrics  *MetricsFile
}

func loadSide(dir string) (*side, error) {
	s := &side{dir: dir, facts: map[string]*Facts{}}
	if err := readJSON(filepath.Join(dir, "platform.json"), &s.platform); err != nil {
		return nil, err
	}
	s.metrics = &MetricsFile{}
	if err := readJSON(filepath.Join(dir, "metrics.json"), s.metrics); err != nil {
		return nil, err
	}
	names, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		base := filepath.Base(name)
		if base == "platform.json" || base == "metrics.json" {
			continue
		}
		facts := &Facts{}
		if err := readJSON(name, facts); err != nil {
			return nil, err
		}
		s.facts[factsKey(facts)] = facts
	}
	return s, nil
}

func themeName(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

func factsKey(f *Facts) string { return f.Scene + "." + themeName(f.Dark) }

func compare(args []string) error {
	flags := flag.NewFlagSet("compare", flag.ExitOnError)
	out := flags.String("out", "", "directory for the report and the diff PNGs")
	pixels := flags.Bool("pixels", true, "compare the PNGs outside text")
	flags.Parse(args)
	if flags.NArg() != 2 {
		return fmt.Errorf("compare needs two result directories")
	}
	a, err := loadSide(flags.Arg(0))
	if err != nil {
		return err
	}
	b, err := loadSide(flags.Arg(1))
	if err != nil {
		return err
	}
	var report io.Writer = os.Stdout
	if *out != "" {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			return err
		}
		file, err := os.Create(filepath.Join(*out, "report.txt"))
		if err != nil {
			return err
		}
		defer file.Close()
		report = io.MultiWriter(os.Stdout, file)
	}
	fmt.Fprintf(report, "visual parity: %s (%s) vs %s (%s)\n", a.platform.Name, a.dir, b.platform.Name, b.dir)
	compareMetrics(report, a, b)
	compareLayout(report, a, b)
	compareInk(report, a, b)
	if *pixels {
		return comparePixels(report, a, b, *out)
	}
	return nil
}

// ---------------------------------------------------------------- pairing

// pairByKey pairs two sequences by their longest common subsequence of keys, so
// an inserted or missing item does not shift every pair after it.
func pairByKey(left, right []string) (pairs [][2]int, leftOnly, rightOnly []int) {
	n, m := len(left), len(right)
	table := make([][]int32, n+1)
	for i := range table {
		table[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if left[i] == right[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case left[i] == right[j]:
			pairs = append(pairs, [2]int{i, j})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			leftOnly = append(leftOnly, i)
			i++
		default:
			rightOnly = append(rightOnly, j)
			j++
		}
	}
	for ; i < n; i++ {
		leftOnly = append(leftOnly, i)
	}
	for ; j < m; j++ {
		rightOnly = append(rightOnly, j)
	}
	return
}

func q(v float32, step float32) int { return int(math.Round(float64(v / step))) }

func hueKey(h Hue) string {
	return fmt.Sprintf("%d,%d,%d,%d", q(h[0], 0.004), q(h[1], 0.004), q(h[2], 0.004), q(h[3], 0.004))
}

func boxDelta(a, b Box) (dx, dy, dw, dh float32) {
	return b.X - a.X, b.Y - a.Y, b.W - a.W, b.H - a.H
}

func abs32(v float32) float32 { return float32(math.Abs(float64(v))) }

// ------------------------------------------------------------ text metrics

type styleStats struct {
	ratios []float64
}

func (s *styleStats) add(r float64) { s.ratios = append(s.ratios, r) }

func (s *styleStats) summary() (mean, lo, hi float64) {
	if len(s.ratios) == 0 {
		return 0, 0, 0
	}
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, r := range s.ratios {
		mean += r
		lo, hi = math.Min(lo, r), math.Max(hi, r)
	}
	return mean / float64(len(s.ratios)), lo, hi
}

type lineKey struct {
	sample string
	size   float32
	weight float32
}

func compareMetrics(w io.Writer, a, b *side) {
	fmt.Fprintf(w, "\n== Text measurement (metrics.json): %s -> %s ==\n", a.platform.Name, b.platform.Name)
	index := func(m *MetricsFile) map[lineKey]Measured {
		r := map[lineKey]Measured{}
		for _, l := range m.Lines {
			r[lineKey{l.Sample, l.Size, l.Weight}] = l.M
		}
		return r
	}
	ma, mb := index(a.metrics), index(b.metrics)
	bySample := map[string]*styleStats{}
	byWeight := map[float32]*styleStats{}
	var order []string
	for _, s := range samples {
		order = append(order, s.name)
	}
	for k, x := range ma {
		y, ok := mb[k]
		if !ok || x.Width == 0 || y.Width == 0 {
			continue
		}
		r := float64(y.Width / x.Width)
		if bySample[k.sample] == nil {
			bySample[k.sample] = &styleStats{}
		}
		bySample[k.sample].add(r)
		if k.sample == "latin-sentence" {
			if byWeight[k.weight] == nil {
				byWeight[k.weight] = &styleStats{}
			}
			byWeight[k.weight].add(r)
		}
	}
	fmt.Fprintln(w, "width ratio (B/A) of the same string, over sizes 11..24 and the four weights:")
	fmt.Fprintf(w, "  %-16s %8s %8s %8s\n", "sample", "mean", "min", "max")
	for _, name := range order {
		if s := bySample[name]; s != nil {
			mean, lo, hi := s.summary()
			fmt.Fprintf(w, "  %-16s %8.3f %8.3f %8.3f\n", name, mean, lo, hi)
		}
	}
	fmt.Fprintln(w, "latin sentence, by weight:")
	for _, wt := range weights {
		if s := byWeight[wt.weight]; s != nil {
			mean, lo, hi := s.summary()
			fmt.Fprintf(w, "  %-16s %8.3f %8.3f %8.3f\n", wt.name, mean, lo, hi)
		}
	}
	fmt.Fprintln(w, "font line box of the latin word, by size (the fonts' own ascent / descent / height; Viva does not use them to place the baseline, see the glyph placement below):")
	fmt.Fprintf(w, "  %5s | %-22s | %-22s | %s\n", "size", a.platform.Name, b.platform.Name, "height ratio")
	for _, size := range []float32{11, 12, 13, 14, 16, 18, 20, 24} {
		x, okx := ma[lineKey{"latin-word", size, 0}]
		y, oky := mb[lineKey{"latin-word", size, 0}]
		if !okx || !oky {
			continue
		}
		fmt.Fprintf(w, "  %5g | %6.2f %6.2f %6.2f | %6.2f %6.2f %6.2f | %.3f\n", size, x.Ascent, x.Descent, x.Height, y.Ascent, y.Descent, y.Height, y.Height/x.Height)
	}
	fmt.Fprintln(w, "fixed-pitch cell (width x height):")
	cells := func(m *MetricsFile) map[[2]float32]MonoCell {
		r := map[[2]float32]MonoCell{}
		for _, c := range m.Mono {
			r[[2]float32{float32(c.Face), c.Size}] = c
		}
		return r
	}
	ca, cb := cells(a.metrics), cells(b.metrics)
	for _, face := range []float32{0, 1} {
		for _, size := range []float32{11, 12, 13, 14, 16, 20} {
			x, okx := ca[[2]float32{face, size}]
			y, oky := cb[[2]float32{face, size}]
			if okx && oky {
				fmt.Fprintf(w, "  face %d size %-3g %6.2fx%-5.1f vs %6.2fx%-5.1f  (width ratio %.3f)\n", int(face), size, x.Width, x.Height, y.Width, y.Height, y.Width/x.Width)
			}
		}
	}
	for _, n := range []*side{a, b} {
		if len(n.platform.Notes) > 0 {
			keys := make([]string, 0, len(n.platform.Notes))
			for k := range n.platform.Notes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			fmt.Fprintf(w, "fonts resolved on %s:\n", n.platform.Name)
			for _, k := range keys {
				fmt.Fprintf(w, "  %-22s %s\n", k, strings.TrimSpace(n.platform.Notes[k]))
			}
		}
	}
}

// ------------------------------------------------------------------ layout

type drift struct {
	scene, item string
	axes        string
	amount      float32
}

type bucket struct {
	count int
	max   float32
	items []drift
}

func (b *bucket) add(d drift) {
	b.count++
	b.max = max(b.max, d.amount)
	if len(b.items) < 4 {
		b.items = append(b.items, d)
	}
}

func axesOf(dx, dy, dw, dh float32) (axes string, amount float32) {
	for _, v := range []struct {
		name  string
		value float32
	}{{"x", dx}, {"y", dy}, {"w", dw}, {"h", dh}} {
		if abs32(v.value) > tolerance {
			axes += v.name
			amount = max(amount, abs32(v.value))
		}
	}
	return
}

// kindResult is the pairing of one kind of fact in one scene.
var kinds = []string{"text", "rect", "hit", "triangle"}

type kindResult struct {
	pairs               [][2]int
	leftOnly, rightOnly []int
	drifting            map[int]string // left index -> axes
}

func compareLayout(w io.Writer, a, b *side) {
	fmt.Fprintf(w, "\n== Layout facts (tolerance %.1f px) ==\n", tolerance)
	var keys []string
	for k := range a.facts {
		if _, ok := b.facts[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		fmt.Fprintln(w, "no scene in common")
		return
	}
	causes := map[string]*bucket{}
	add := func(cause string, d drift) {
		if causes[cause] == nil {
			causes[cause] = &bucket{}
		}
		causes[cause].add(d)
	}
	type rowSummary struct {
		key                   string
		items, drifting, only int
		max                   float32
	}
	var rows []rowSummary
	for _, key := range keys {
		fa, fb := a.facts[key], b.facts[key]
		row := rowSummary{key: key}
		results := compareKinds(fa, fb)
		for _, kind := range kinds {
			res := results[kind]
			row.items += len(res.pairs)
			row.only += len(res.leftOnly) + len(res.rightOnly)
			for _, i := range res.leftOnly {
				add("structure: "+kind+" only on "+a.platform.Name, drift{scene: key, item: describe(fa, kind, i)})
			}
			for _, j := range res.rightOnly {
				add("structure: "+kind+" only on "+b.platform.Name, drift{scene: key, item: describe(fb, kind, j)})
			}
			for _, pair := range res.pairs {
				axes, ok := res.drifting[pair[0]]
				if !ok {
					continue
				}
				dx, dy, dw, dh := itemDelta(fa, fb, kind, pair)
				_, amount := axesOf(dx, dy, dw, dh)
				row.drifting++
				row.max = max(row.max, amount)
				add(fmt.Sprintf("%s drifts in %s", kind, axes), drift{scene: key, item: describe(fa, kind, pair[0]), axes: axes, amount: amount})
			}
		}
		rows = append(rows, row)
	}
	fmt.Fprintf(w, "%-34s %8s %9s %8s %8s\n", "scene.theme", "paired", "drifting", "unpaired", "max px")
	for _, r := range rows {
		fmt.Fprintf(w, "%-34s %8d %9d %8d %8.2f\n", r.key, r.items, r.drifting, r.only, r.max)
	}
	fmt.Fprintln(w, "\ndrift by cause (count, largest, examples):")
	var names []string
	for k := range causes {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return causes[names[i]].count > causes[names[j]].count })
	for _, name := range names {
		c := causes[name]
		fmt.Fprintf(w, "  %5d  max %7.2f  %s\n", c.count, c.max, name)
		for _, d := range c.items {
			fmt.Fprintf(w, "           e.g. %s: %s (%.2f px)\n", d.scene, d.item, d.amount)
		}
	}
	if len(causes) == 0 {
		fmt.Fprintln(w, "  none: every text, rectangle, icon and hit region is where it is on the other platform")
	}
}

func weightName(weight float32) string {
	for _, w := range weights {
		if w.weight == weight {
			return w.name
		}
	}
	return fmt.Sprintf("%g", weight)
}

func describe(f *Facts, kind string, i int) string {
	switch kind {
	case "text":
		return fmt.Sprintf("text %q", f.Texts[i].Value)
	case "rect":
		r := f.Rectangles[i]
		return fmt.Sprintf("rect #%d %.1f,%.1f %.1fx%.1f r=%.1f", i, r.Bounds.X, r.Bounds.Y, r.Bounds.W, r.Bounds.H, r.Radius)
	case "hit":
		return "hit " + f.Hits[i].ID
	default:
		return fmt.Sprintf("triangle #%d", i)
	}
}

func itemDelta(fa, fb *Facts, kind string, pair [2]int) (dx, dy, dw, dh float32) {
	i, j := pair[0], pair[1]
	switch kind {
	case "text":
		return boxDelta(fa.Texts[i].Bounds, fb.Texts[j].Bounds)
	case "rect":
		return boxDelta(fa.Rectangles[i].Bounds, fb.Rectangles[j].Bounds)
	case "hit":
		return boxDelta(fa.Hits[i].Bounds, fb.Hits[j].Bounds)
	default:
		var m float32
		for p := 0; p < 3; p++ {
			for c := 0; c < 2; c++ {
				m = max(m, abs32(fb.Triangles[j].Points[p][c]-fa.Triangles[i].Points[p][c]))
			}
		}
		return m, 0, 0, 0
	}
}

func compareKinds(fa, fb *Facts) map[string]*kindResult {
	results := map[string]*kindResult{}
	finish := func(kind string, ka, kb []string) *kindResult {
		res := &kindResult{drifting: map[int]string{}}
		res.pairs, res.leftOnly, res.rightOnly = pairByKey(ka, kb)
		results[kind] = res
		return res
	}
	var ka, kb []string
	for _, t := range fa.Texts {
		ka = append(ka, fmt.Sprintf("%s|%g|%g|%v", t.Value, t.Size, t.Weight, t.Mono))
	}
	for _, t := range fb.Texts {
		kb = append(kb, fmt.Sprintf("%s|%g|%g|%v", t.Value, t.Size, t.Weight, t.Mono))
	}
	finish("text", ka, kb)
	ka, kb = nil, nil
	for _, r := range fa.Rectangles {
		ka = append(ka, fmt.Sprintf("%s|%d|%d", hueKey(r.Color), q(r.Radius, 0.5), r.Layer))
	}
	for _, r := range fb.Rectangles {
		kb = append(kb, fmt.Sprintf("%s|%d|%d", hueKey(r.Color), q(r.Radius, 0.5), r.Layer))
	}
	finish("rect", ka, kb)
	ka, kb = nil, nil
	for _, h := range fa.Hits {
		ka = append(ka, h.ID+"|"+h.Label)
	}
	for _, h := range fb.Hits {
		kb = append(kb, h.ID+"|"+h.Label)
	}
	finish("hit", ka, kb)
	ka, kb = nil, nil
	for _, t := range fa.Triangles {
		ka = append(ka, fmt.Sprintf("%s|%d", hueKey(t.Color), t.Layer))
	}
	for _, t := range fb.Triangles {
		kb = append(kb, fmt.Sprintf("%s|%d", hueKey(t.Color), t.Layer))
	}
	finish("triangle", ka, kb)
	for kind, res := range results {
		for _, pair := range res.pairs {
			dx, dy, dw, dh := itemDelta(fa, fb, kind, pair)
			if axes, _ := axesOf(dx, dy, dw, dh); axes != "" {
				res.drifting[pair[0]] = axes
			}
		}
	}
	return results
}
