package gallery

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/vivaui/viva/gfx"
)

// ReloadStateVersion is bumped whenever the snapshot layout changes. 5 replaced
// the hand-written pages' state with the open component, appearance and scroll
// position; the examples themselves start fresh after a reload.
const ReloadStateVersion = 5

// SnapshotReloadState keeps what a reload should not lose: which component is
// open (by ID, so a catalog that changed shape cannot point at another one),
// the appearance, and the scroll position.
func (state *State) SnapshotReloadState() []byte {
	component := ""
	if state.page != 0 {
		component = catalog[state.page-1].ID
	}
	data := make([]byte, 0, 16+len(component))
	data = appendString(data, component)
	flags := byte(0)
	if state.lightMode {
		flags |= 1
	}
	data = append(data, flags, byte(state.density))
	data = appendUint32(data, math.Float32bits(state.viewScroll))
	return data
}

// RestoreReloadState applies a snapshot. A component that no longer exists
// restores to the catalog.
func (state *State) RestoreReloadState(data []byte) error {
	reader := reloadStateReader{data: data}
	component := reader.string()
	flags := reader.uint8()
	density := gfx.Density(reader.uint8())
	viewScroll := math.Float32frombits(reader.uint32())
	if err := reader.finish(); err != nil {
		return err
	}
	if flags&^1 != 0 || density > gfx.DensityCompact || !(viewScroll >= 0) || math.IsInf(float64(viewScroll), 0) {
		return fmt.Errorf("gallery: invalid reload state")
	}
	page := 0
	for index := range catalog {
		if catalog[index].ID == component {
			page = index + 1
		}
	}
	state.lightMode = flags&1 != 0
	state.density = density
	state.openPage(page)
	state.viewScroll = viewScroll
	return nil
}

func appendUint32(data []byte, value uint32) []byte {
	var encoded [4]byte
	binary.LittleEndian.PutUint32(encoded[:], value)
	return append(data, encoded[:]...)
}

func appendString(data []byte, value string) []byte {
	data = appendUint32(data, uint32(len(value)))
	return append(data, value...)
}

type reloadStateReader struct {
	data   []byte
	offset int
	err    error
}

func (reader *reloadStateReader) uint8() uint8 {
	if reader.err != nil || reader.offset >= len(reader.data) {
		reader.err = fmt.Errorf("gallery: truncated reload state")
		return 0
	}
	value := reader.data[reader.offset]
	reader.offset++
	return value
}

func (reader *reloadStateReader) uint32() uint32 {
	if reader.err != nil || len(reader.data)-reader.offset < 4 {
		reader.err = fmt.Errorf("gallery: truncated reload state")
		return 0
	}
	value := binary.LittleEndian.Uint32(reader.data[reader.offset : reader.offset+4])
	reader.offset += 4
	return value
}

func (reader *reloadStateReader) string() string {
	length := reader.uint32()
	if reader.err != nil || uint64(length) > uint64(len(reader.data)-reader.offset) {
		reader.err = fmt.Errorf("gallery: truncated reload state")
		return ""
	}
	value := string(reader.data[reader.offset : reader.offset+int(length)])
	reader.offset += int(length)
	return value
}

func (reader *reloadStateReader) finish() error {
	if reader.err != nil {
		return reader.err
	}
	if reader.offset != len(reader.data) {
		return fmt.Errorf("gallery: reload state has trailing data")
	}
	return nil
}
