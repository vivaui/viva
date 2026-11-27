package gfx

import "github.com/vivaui/viva/internal/bidi"

// BidiRun is a logical range of a line, in UTF-8 byte offsets into the source
// paragraph. An odd Level lays the run out right to left.
type BidiRun struct {
	Start, End int
	Level      uint8
}

// BidiParagraph resolves a paragraph before a caller breaks it into lines.
// Its zero value is ready to use. It is not safe for concurrent use.
type BidiParagraph struct {
	resolver bidi.Resolver
	resolved bidi.Paragraph
	runs     []bidi.Run
}

// Resolve sets the source text and base direction for LineRuns.
func (p *BidiParagraph) Resolve(value string, direction TextDirection) {
	base := bidi.Auto
	switch direction {
	case DirectionLTR:
		base = bidi.LTR
	case DirectionRTL:
		base = bidi.RTL
	}
	p.resolved = p.resolver.Resolve(value, base)
}

// LineRuns appends the visually ordered runs of the range [start, end) to dst.
// The range is in source bytes; a boundary inside a rune snaps to the next rune.
func (p *BidiParagraph) LineRuns(start, end int, dst []BidiRun) []BidiRun {
	p.runs = p.resolver.LineRuns(p.resolved, start, end, p.runs)
	for _, run := range p.runs {
		dst = append(dst, BidiRun(run))
	}
	return dst
}
