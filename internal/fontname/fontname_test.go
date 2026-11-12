package fontname

import (
	"encoding/binary"
	"testing"
)

func TestReadTrueTypeFamilyWeightAndItalic(t *testing.T) {
	data := sfntWithName("Inter", 600, true)
	font, ok := Read(data)
	if !ok {
		t.Fatal("Read rejected a valid sfnt")
	}
	if font.Family != "Inter" {
		t.Fatalf("Family = %q, want Inter", font.Family)
	}
	if font.Weight != 600 {
		t.Fatalf("Weight = %d, want 600", font.Weight)
	}
	if !font.Italic {
		t.Fatal("Italic = false, want true")
	}
}

func TestReadPrefersTypographicFamily(t *testing.T) {
	data := sfntWithNames(map[uint16]string{1: "Legacy", 16: "Typographic"}, 400, false)
	font, ok := Read(data)
	if !ok || font.Family != "Typographic" {
		t.Fatalf("Family = %q ok=%v, want Typographic", font.Family, ok)
	}
}

func TestReadRejectsOtherData(t *testing.T) {
	if _, ok := Read([]byte("not a font")); ok {
		t.Fatal("Read accepted data that is not an sfnt")
	}
}

func sfntWithName(family string, weight int, italic bool) []byte {
	return sfntWithNames(map[uint16]string{1: family}, weight, italic)
}

func sfntWithNames(names map[uint16]string, weight int, italic bool) []byte {
	nameTable := buildNameTable(names)
	os2Table := make([]byte, 96)
	binary.BigEndian.PutUint16(os2Table[4:6], uint16(weight))
	if italic {
		binary.BigEndian.PutUint16(os2Table[62:64], 1)
	}
	tables := []struct {
		tag  string
		data []byte
	}{{"name", nameTable}, {"OS/2", os2Table}}
	headerSize := 12 + len(tables)*16
	total := headerSize
	for _, table := range tables {
		total += len(table.data)
	}
	data := make([]byte, total)
	binary.BigEndian.PutUint32(data[0:4], 0x00010000)
	binary.BigEndian.PutUint16(data[4:6], uint16(len(tables)))
	offset := headerSize
	for index, table := range tables {
		record := data[12+index*16:]
		copy(record[0:4], table.tag)
		binary.BigEndian.PutUint32(record[8:12], uint32(offset))
		binary.BigEndian.PutUint32(record[12:16], uint32(len(table.data)))
		copy(data[offset:], table.data)
		offset += len(table.data)
	}
	return data
}

func buildNameTable(names map[uint16]string) []byte {
	count := len(names)
	storage := 6 + count*12
	records := make([]byte, 0, count*12)
	var stringsData []byte
	for nameID := range names {
		value := names[nameID]
		encoded := make([]byte, len(value)*2)
		for index, runeValue := range []rune(value) {
			binary.BigEndian.PutUint16(encoded[index*2:], uint16(runeValue))
		}
		record := make([]byte, 12)
		binary.BigEndian.PutUint16(record[0:2], 3)
		binary.BigEndian.PutUint16(record[2:4], 1)
		binary.BigEndian.PutUint16(record[4:6], 0x0409)
		binary.BigEndian.PutUint16(record[6:8], nameID)
		binary.BigEndian.PutUint16(record[8:10], uint16(len(encoded)))
		binary.BigEndian.PutUint16(record[10:12], uint16(len(stringsData)))
		records = append(records, record...)
		stringsData = append(stringsData, encoded...)
	}
	table := make([]byte, 6+len(records)+len(stringsData))
	binary.BigEndian.PutUint16(table[0:2], 0)
	binary.BigEndian.PutUint16(table[2:4], uint16(count))
	binary.BigEndian.PutUint16(table[4:6], uint16(storage))
	copy(table[6:], records)
	copy(table[6+len(records):], stringsData)
	return table
}
