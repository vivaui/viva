package text

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
)

// The bullets of a value stand one for one for its runes: no character of the
// value but its line breaks is among them, the line breaks are where the value's
// are, and an offset into the value finds the start of its bullet, and back again,
// at every rune boundary.
func TestBulletsStandOneForOneForRunes(t *testing.T) {
	pieces := []string{"a", "é", "中", "😀", "👩‍💻", "\xff", "\n", "\r\n", "secret", "\t"}
	random := rand.New(rand.NewPCG(3, 4))
	for range 500 {
		var b strings.Builder
		for range random.IntN(14) {
			b.WriteString(pieces[random.IntN(len(pieces))])
		}
		value := b.String()
		bullets := Bullets(value)
		if utf8.RuneCountInString(bullets) != utf8.RuneCountInString(value) {
			t.Fatalf("Bullets(%q) = %q: not one a rune", value, bullets)
		}
		for _, r := range bullets {
			if r != '•' && r != '\n' && r != '\r' {
				t.Fatalf("Bullets(%q) = %q carries %q of the value", value, bullets, r)
			}
		}
		if strings.Count(bullets, "\n") != strings.Count(value, "\n") || strings.Count(bullets, "\r") != strings.Count(value, "\r") {
			t.Fatalf("Bullets(%q) = %q: the line breaks moved", value, bullets)
		}
		for at := range value {
			// at runs over the rune boundaries only.
			to := BulletOffset(value, at)
			if ValueOffset(value, to) != at {
				t.Fatalf("offset %d of %q went to %d of %q and back to %d", at, value, to, bullets, ValueOffset(value, to))
			}
			if to >= len(bullets) || to > 0 && bullets[to]&0xc0 == 0x80 {
				t.Fatalf("offset %d of %q lands inside a bullet of %q: %d", at, value, bullets, to)
			}
		}
		for at := 1; at < len(value); at++ {
			// An offset inside a rune is the boundary before it.
			if want := BulletOffset(value, at-1); value[at]&0xc0 == 0x80 && BulletOffset(value, at) != want {
				t.Fatalf("offset %d inside a rune of %q is not %d", at, value, want)
			}
		}
		// Past either end an offset is the last boundary.
		if BulletOffset(value, -5) != 0 || BulletOffset(value, len(value)+7) != len(bullets) || ValueOffset(value, -5) != 0 || ValueOffset(value, len(bullets)+3) != len(value) {
			t.Fatalf("an offset past the end of %q is not its last boundary", value)
		}
	}
	// An offset inside a bullet is the start of the rune it stands for, and past
	// the last it is the end of the value.
	value := "é\nx"
	if bullets := Bullets(value); bullets != "•\n•" {
		t.Fatalf("Bullets(%q) = %q", value, bullets)
	}
	for bullet, want := range map[int]int{0: 0, 1: 0, 2: 0, 3: 2, 4: 3, 5: 3, 6: 3, 7: 4} {
		if got := ValueOffset(value, bullet); got != want {
			t.Fatalf("ValueOffset(%q, %d) = %d, want %d", value, bullet, got, want)
		}
	}
	for at, want := range map[int]int{0: 0, 2: 3, 3: 4, 4: 7} {
		if got := BulletOffset(value, at); got != want {
			t.Fatalf("BulletOffset(%q, %d) = %d, want %d", value, at, got, want)
		}
	}
	if Bullets("") != "" {
		t.Fatal("the bullets of an empty value are not empty")
	}
}
