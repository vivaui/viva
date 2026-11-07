// Package fontname reads the family, weight and style an sfnt font names.
// It reads only the tables needed for registration; shaping belongs to the
// platform engines.
package fontname

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// Font is what a font file says about itself.
type Font struct {
	// Family is the typographic family (name ID 16) when the font has one,
	// otherwise the legacy family (name ID 1).
	Family string
	// Weight is the OS/2 usWeightClass, or 400 when the font has no OS/2 table.
	Weight int
	// Italic reports the OS/2 fsSelection italic bit.
	Italic bool
}

// Read reads the first font of data. It accepts TrueType, OpenType and a
// collection; ok is false when data is not an sfnt or has no readable family.
func Read(data []byte) (Font, bool) {
	offset, ok := firstFontOffset(data)
	if !ok {
		return Font{}, false
	}
	tables, ok := tableRecords(data, offset)
	if !ok {
		return Font{}, false
	}
	name, ok := tables["name"]
	if !ok {
		return Font{}, false
	}
	family, ok := readFamily(data, name.offset, name.length)
	if !ok || family == "" {
		return Font{}, false
	}
	font := Font{Family: family, Weight: 400}
	if os2, ok := tables["OS/2"]; ok {
		if weight, ok := readWeight(data, os2.offset, os2.length); ok {
			font.Weight = weight
		}
		if italic, ok := readItalic(data, os2.offset, os2.length); ok {
			font.Italic = italic
		}
	}
	return font, true
}

type tableRecord struct {
	offset uint32
	length uint32
}

type sfntHeader struct {
	version uint32
	count   uint16
}

func firstFontOffset(data []byte) (uint32, bool) {
	if len(data) < 12 {
		return 0, false
	}
	if string(data[:4]) == "ttcf" {
		count := binary.BigEndian.Uint32(data[8:12])
		if count == 0 || uint64(12)+uint64(count)*4 > uint64(len(data)) {
			return 0, false
		}
		offset := binary.BigEndian.Uint32(data[12:16])
		if uint64(offset)+12 > uint64(len(data)) {
			return 0, false
		}
		return offset, true
	}
	return 0, true
}

func tableRecords(data []byte, offset uint32) (map[string]tableRecord, bool) {
	if uint64(offset)+12 > uint64(len(data)) {
		return nil, false
	}
	header := data[offset:]
	version := binary.BigEndian.Uint32(header[:4])
	switch version {
	case 0x00010000, 0x4f54544f, 0x74727565, 0x74797031: // TrueType, OTTO, 'true', 'typ1'
	default:
		return nil, false
	}
	count := binary.BigEndian.Uint16(header[4:6])
	if uint64(12)+uint64(count)*16 > uint64(len(header)) {
		return nil, false
	}
	tables := make(map[string]tableRecord, count)
	for index := 0; index < int(count); index++ {
		record := header[12+index*16:]
		tag := string(record[:4])
		tables[tag] = tableRecord{
			offset: binary.BigEndian.Uint32(record[8:12]),
			length: binary.BigEndian.Uint32(record[12:16]),
		}
	}
	return tables, true
}

func readFamily(data []byte, offset, length uint32) (string, bool) {
	table, ok := tableSlice(data, offset, length)
	if !ok || len(table) < 6 {
		return "", false
	}
	count := int(binary.BigEndian.Uint16(table[2:4]))
	storage := int(binary.BigEndian.Uint16(table[4:6]))
	if count < 0 || 6+count*12 > len(table) || storage > len(table) {
		return "", false
	}
	best := ""
	bestRank := -1
	for index := 0; index < count; index++ {
		record := table[6+index*12:]
		platform := binary.BigEndian.Uint16(record[0:2])
		encoding := binary.BigEndian.Uint16(record[2:4])
		nameID := binary.BigEndian.Uint16(record[6:8])
		length := int(binary.BigEndian.Uint16(record[8:10]))
		start := storage + int(binary.BigEndian.Uint16(record[10:12]))
		if nameID != 1 && nameID != 16 {
			continue
		}
		if start < 0 || start+length > len(table) {
			continue
		}
		value, ok := decodeName(platform, encoding, table[start:start+length])
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		rank := 0
		if nameID == 16 {
			rank += 4
		}
		if platform == 3 {
			rank += 2
		}
		if encoding == 1 || encoding == 10 {
			rank++
		}
		if rank > bestRank {
			best, bestRank = strings.TrimSpace(value), rank
		}
	}
	return best, best != ""
}

func decodeName(platform, encoding uint16, value []byte) (string, bool) {
	if platform == 0 || (platform == 3 && (encoding == 1 || encoding == 10)) {
		if len(value)%2 != 0 {
			return "", false
		}
		units := make([]uint16, len(value)/2)
		for index := range units {
			units[index] = binary.BigEndian.Uint16(value[index*2:])
		}
		return string(utf16.Decode(units)), true
	}
	if platform == 1 && encoding == 0 {
		// MacRoman's ASCII range is identical to Latin-1; family names are
		// typically ASCII. Bytes above it are kept as Latin-1 rather than
		// guessing a table.
		runes := make([]rune, len(value))
		for index, byteValue := range value {
			runes[index] = rune(byteValue)
		}
		return string(runes), true
	}
	return "", false
}

func readWeight(data []byte, offset, length uint32) (int, bool) {
	table, ok := tableSlice(data, offset, length)
	if !ok || len(table) < 6 {
		return 0, false
	}
	weight := int(binary.BigEndian.Uint16(table[4:6]))
	if weight < 1 || weight > 1000 {
		return 0, false
	}
	return weight, true
}

func readItalic(data []byte, offset, length uint32) (bool, bool) {
	table, ok := tableSlice(data, offset, length)
	if !ok || len(table) < 64 {
		return false, false
	}
	return binary.BigEndian.Uint16(table[62:64])&0x01 != 0, true
}

func tableSlice(data []byte, offset, length uint32) ([]byte, bool) {
	if uint64(offset)+uint64(length) > uint64(len(data)) {
		return nil, false
	}
	return data[offset : offset+length], true
}
