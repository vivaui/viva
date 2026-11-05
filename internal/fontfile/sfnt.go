// Package fontfile holds what an application's embedded fonts need before a platform can take
// them: the family a font file names, and a file for the platforms whose font engines only open
// one.
package fontfile

import (
	"encoding/binary"
	"errors"
	"unicode/utf16"
)

// Family returns the family a TrueType or OpenType font (or the first font of a collection) names:
// its typographic family (name ID 16) when it has one, else its family (name ID 1). That is the
// name the platforms group a font's weights under and an application passes to a font lookup.
//
// Only sfnt data is read; a WOFF file is not one and is refused.
func Family(data []byte) (string, error) {
	base := 0
	if len(data) >= 12 && string(data[:4]) == "ttcf" {
		if len(data) < 16 || binary.BigEndian.Uint32(data[8:]) == 0 {
			return "", errors.New("fontfile: an empty font collection")
		}
		base = int(binary.BigEndian.Uint32(data[12:]))
	}
	if base < 0 || base+12 > len(data) {
		return "", errors.New("fontfile: not a TrueType or OpenType font")
	}
	switch tag := string(data[base : base+4]); tag {
	case "\x00\x01\x00\x00", "OTTO", "true", "typ1":
	default:
		return "", errors.New("fontfile: not a TrueType or OpenType font")
	}
	tables := int(binary.BigEndian.Uint16(data[base+4:]))
	record := base + 12
	for i := 0; i < tables; i++ {
		entry := record + 16*i
		if entry+16 > len(data) {
			break
		}
		if string(data[entry:entry+4]) != "name" {
			continue
		}
		offset, length := int(binary.BigEndian.Uint32(data[entry+8:])), int(binary.BigEndian.Uint32(data[entry+12:]))
		if offset < 0 || length < 0 || offset > len(data) || length > len(data)-offset {
			return "", errors.New("fontfile: the name table lies outside the file")
		}
		return nameFamily(data[offset : offset+length])
	}
	return "", errors.New("fontfile: the font has no name table")
}

// nameFamily picks the family from a name table, preferring Windows Unicode and then Macintosh
// Roman records, and the typographic family over the legacy one.
func nameFamily(table []byte) (string, error) {
	if len(table) < 6 {
		return "", errors.New("fontfile: a truncated name table")
	}
	count, storage := int(binary.BigEndian.Uint16(table[2:])), int(binary.BigEndian.Uint16(table[4:]))
	best, bestRank := "", 0
	for i := 0; i < count; i++ {
		record := 6 + 12*i
		if record+12 > len(table) {
			break
		}
		platform, encoding := binary.BigEndian.Uint16(table[record:]), binary.BigEndian.Uint16(table[record+2:])
		id := binary.BigEndian.Uint16(table[record+6:])
		length, offset := int(binary.BigEndian.Uint16(table[record+8:])), int(binary.BigEndian.Uint16(table[record+10:]))
		if id != 1 && id != 16 {
			continue
		}
		start := storage + offset
		if start < 0 || start+length > len(table) {
			continue
		}
		raw := table[start : start+length]
		var text string
		switch {
		case platform == 3 && (encoding == 1 || encoding == 10), platform == 0:
			units := make([]uint16, len(raw)/2)
			for j := range units {
				units[j] = binary.BigEndian.Uint16(raw[2*j:])
			}
			text = string(utf16.Decode(units))
		case platform == 1 && encoding == 0:
			text = string(raw) // Mac Roman; family names are ASCII in practice
		default:
			continue
		}
		rank := 1
		if id == 16 {
			rank = 4
		}
		if platform != 1 {
			rank += 2
		}
		if text != "" && rank > bestRank {
			best, bestRank = text, rank
		}
	}
	if best == "" {
		return "", errors.New("fontfile: the font names no family")
	}
	return best, nil
}
