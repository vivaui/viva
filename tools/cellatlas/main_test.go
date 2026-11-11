package main

import "testing"

func TestParseAtlas(t *testing.T) {
	a, ok := parseAtlas("/atlases/U+1FB00...U+1FBFF-12x24+3.png")
	if !ok {
		t.Fatal("a reference atlas's name was not read")
	}
	if a.first != 0x1FB00 || a.cell() != "12x24+3" {
		t.Fatalf("read %s starting at U+%04X", a.cell(), a.first)
	}
	if x, y := a.stripe(); x != 18 || y != 36 {
		t.Fatalf("a sprite is %dx%d, want 18x36", x, y)
	}
	for _, name := range []string{"atlas.png", "U+1FB00-12x24+3.png", "U+1FB00...U+1FBFE-12x24+3.png"} {
		if _, ok := parseAtlas(name); ok {
			t.Fatalf("%q was read as a reference atlas", name)
		}
	}
}

func TestCodePoint(t *testing.T) {
	for _, value := range []string{"U+1FB98", "1fb98"} {
		if r, err := codePoint(value); err != nil || r != 0x1FB98 {
			t.Fatalf("%q is U+%04X, %v", value, r, err)
		}
	}
	if _, err := codePoint("hachure"); err == nil {
		t.Fatal("a word was read as a code point")
	}
}
