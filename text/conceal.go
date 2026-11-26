package text

import (
	"strings"
	"unicode/utf8"
)

// Bullet is the character a secure field paints in place of a rune of its value.
const Bullet = "•"

// A rune of a value takes a bullet's bytes in its bullets, and a line break,
// which stays, its own one.
const (
	bulletBytes = len(Bullet)
	breakBytes  = 1
)

// Bullets is what a secure field shows in place of value: Bullet for each rune
// and every line break as it was, so the bullets take the rows the text takes
// and a field lays its secret out as it lays its text out. It allocates the
// result, so a field that paints every frame keeps it, as a layout keeps rows.
func Bullets(value string) string {
	if value == "" {
		return ""
	}
	size := len(value)
	for _, r := range value {
		if !lineBreak(r) {
			size += bulletBytes - utf8.RuneLen(r)
		}
	}
	var b strings.Builder
	b.Grow(size)
	for _, r := range value {
		if lineBreak(r) {
			b.WriteRune(r)
		} else {
			b.WriteString(Bullet)
		}
	}
	return b.String()
}

// BulletOffset is the offset into Bullets(value) of offset, an offset into
// value: at the rune boundary offset is at, or the one before it.
func BulletOffset(value string, offset int) int {
	offset = clampOffset(value, offset)
	// A rune's bullet is three bytes where the rune took as many as it did, and
	// a line break's one where it took one.
	breaks := strings.Count(value[:offset], "\n") + strings.Count(value[:offset], "\r")
	return bulletBytes*utf8.RuneCountInString(value[:offset]) - (bulletBytes-breakBytes)*breaks
}

// ValueOffset is the offset into value of offset, an offset into
// Bullets(value): where the rune whose bullet holds that offset starts.
func ValueOffset(value string, offset int) int {
	at := 0
	for offset > 0 && at < len(value) {
		r, size := utf8.DecodeRuneInString(value[at:])
		width := breakBytes
		if !lineBreak(r) {
			width = bulletBytes
		}
		if offset < width {
			break
		}
		offset -= width
		at += size
	}
	return at
}

// lineBreak reports whether r ends a row of a field, and so stays in its bullets.
func lineBreak(r rune) bool { return r == '\n' || r == '\r' }

// clampOffset is offset moved back to the rune boundary at or before it.
func clampOffset(value string, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= len(value) {
		return len(value)
	}
	for offset > 0 && value[offset]&0xc0 == 0x80 {
		offset--
	}
	return offset
}
