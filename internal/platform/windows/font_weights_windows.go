package windows

import (
	"slices"
	"syscall"
	"unsafe"
)

// Method slots of IDWriteFontList and IDWriteFont (checked against dwrite.h).
const (
	writeFontListGetFontCount = 4
	writeFontListGetFont      = 5
	writeFontGetWeight        = 4
)

// FontWeights lists the weights family is installed with, ascending: the weights a
// text format of the family may ask for, as DWRITE_FONT_WEIGHT values. A family the
// collection does not have lists none.
func FontWeights(family string) []int {
	services := &textServices{}
	if err := services.initializeWrite(); err != nil {
		return nil
	}
	defer services.close()
	collection, unlock, ok := openCollection(services)
	if !ok {
		return nil
	}
	defer unlock()
	name, err := syscall.UTF16PtrFromString(family)
	if err != nil {
		return nil
	}
	var index, exists uint32
	if err := hresult(comCall(collection, writeCollectionFindFamilyName, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&index)), uintptr(unsafe.Pointer(&exists)))); err != nil || exists == 0 {
		return nil
	}
	var list uintptr
	if err := hresult(comCall(collection, writeCollectionGetFontFamily, uintptr(index), uintptr(unsafe.Pointer(&list)))); err != nil || list == 0 {
		return nil
	}
	defer release(&list)
	// GetFontCount returns the count itself, as GetFontFamilyCount does.
	count := uint32(comCall(list, writeFontListGetFontCount))
	weights := make([]int, 0, count)
	for face := uint32(0); face < count; face++ {
		var font uintptr
		if err := hresult(comCall(list, writeFontListGetFont, uintptr(face), uintptr(unsafe.Pointer(&font)))); err != nil || font == 0 {
			continue
		}
		// GetWeight returns the DWRITE_FONT_WEIGHT itself.
		weight := int(uint32(comCall(font, writeFontGetWeight)))
		release(&font)
		if !slices.Contains(weights, weight) {
			weights = append(weights, weight)
		}
	}
	slices.Sort(weights)
	return weights
}
