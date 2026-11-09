//go:build windows

package windows

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/fontfile"
)

// frPrivate is FR_PRIVATE: the font is for this process only and is gone with it.
const frPrivate = 0x10

var (
	addFontResourceEx    = gdi.NewProc("AddFontResourceExW")
	removeFontResourceEx = gdi.NewProc("RemoveFontResourceExW")
	fonts                fontfile.Registry
)

// Method slots of IDWriteFactory, IDWriteFactory3, IDWriteFontFile and IDWriteFontSetBuilder
// (checked against dwrite.h and dwrite_3.h, and against the runtime on Windows 11). Where a method
// is overloaded the declaration order does not decide the slot: IDWriteFactory3 puts
// CreateFontFaceReference(IDWriteFontFile*, ...) before the overload that takes a path, while
// IDWriteFontSetBuilder puts AddFontFaceReference with properties before the one without.
const (
	writeFactoryCreateFontFileReference = 7
	writeFactory3CreateFaceReference    = 33 // from an IDWriteFontFile and a face index (34 takes a path)
	writeFactory3GetSystemFontSet       = 35
	writeFactory3CreateFontSetBuilder   = 36
	writeFactory3CollectionFromFontSet  = 37
	writeFontFileAnalyze                = 5
	writeSetBuilderAddFace              = 4 // AddFontFaceReference without properties (3 takes properties)
	writeSetBuilderAddFontSet           = 5
	writeSetBuilderCreateFontSet        = 6
)

var writeFactory3IID = guid{0x9a1b41c3, 0xd3bb, 0x466a, [8]byte{0x87, 0xfc, 0xfe, 0x67, 0x55, 0x6a, 0x3b, 0x65}}

// customFonts is the font collection DirectWrite draws from when the application registered
// fonts: the system's fonts and those. It is the process's, since a text format takes it at
// creation. The lock keeps a collection from being released while a format is being made from it.
var customFonts struct {
	sync.RWMutex
	collection uintptr
	files      []string
	revision   uint32 // changes with the collection, so that what was measured in the old one is measured again
}

var _ gfx.FontRegistrar = (*textEngine)(nil)

// RegisterFont implements gfx.FontRegistrar. DirectWrite does not see a font GDI holds for the
// process, so the font goes to both: GDI, which lists it among the families and draws what GDI
// draws, and a DirectWrite font collection that adds it to the system's, which every text format
// of the engine is made from.
func (engine *textEngine) RegisterFont(data []byte) (string, func(), error) {
	family, unregister, err := fonts.Register(data, func(path string) error {
		name, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		if err := rebuildCustomFonts(append(slices.Clone(customFonts.files), path)); err != nil {
			return err
		}
		addFontResourceEx.Call(uintptr(unsafe.Pointer(name)), frPrivate, 0)
		return nil
	}, func(path string) {
		if name, err := syscall.UTF16PtrFromString(path); err == nil {
			removeFontResourceEx.Call(uintptr(unsafe.Pointer(name)), frPrivate, 0)
		}
		rebuildCustomFonts(slices.DeleteFunc(slices.Clone(customFonts.files), func(file string) bool { return file == path }))
	})
	if err != nil {
		return "", nil, err
	}
	engine.fontsChanged()
	return family, func() {
		unregister()
		engine.fontsChanged()
	}, nil
}

// fontsChanged starts shaping over, so text laid out before the font set changed is shaped again.
func (engine *textEngine) fontsChanged() {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.cache.Clear()
	engine.generation.Add(1)
}

// rebuildCustomFonts makes the process's collection of the system's fonts and those in files,
// and drops the collection when no file is left.
func rebuildCustomFonts(files []string) error {
	var collection uintptr
	if len(files) > 0 {
		var err error
		if collection, err = newFontCollection(files); err != nil {
			return err
		}
	}
	customFonts.Lock()
	old := customFonts.collection
	customFonts.collection, customFonts.files = collection, files
	customFonts.revision++
	customFonts.Unlock()
	release(&old)
	return nil
}

// newFontCollection is an IDWriteFontCollection of the system's font set plus every face of files.
func newFontCollection(files []string) (collection uintptr, err error) {
	var factory, factory3, systemSet, builder, fontSet uintptr
	defer func() {
		release(&fontSet)
		release(&builder)
		release(&systemSet)
		release(&factory3)
		release(&factory)
	}()
	status, _, _ := writeFactory.Call(0, uintptr(unsafe.Pointer(&writeIID)), uintptr(unsafe.Pointer(&factory)))
	if err := hresult(status); err != nil {
		return 0, err
	}
	if comCall(factory, 0, uintptr(unsafe.Pointer(&writeFactory3IID)), uintptr(unsafe.Pointer(&factory3))) != 0 || factory3 == 0 {
		return 0, errors.New("windows: DirectWrite cannot add fonts at run time before Windows 10 version 1607")
	}
	if err := hresult(comCall(factory3, writeFactory3GetSystemFontSet, uintptr(unsafe.Pointer(&systemSet)))); err != nil {
		return 0, err
	}
	if err := hresult(comCall(factory3, writeFactory3CreateFontSetBuilder, uintptr(unsafe.Pointer(&builder)))); err != nil {
		return 0, err
	}
	if err := hresult(comCall(builder, writeSetBuilderAddFontSet, systemSet)); err != nil {
		return 0, err
	}
	for _, path := range files {
		if err := addFontFaces(factory, factory3, builder, path); err != nil {
			return 0, err
		}
	}
	if err := hresult(comCall(builder, writeSetBuilderCreateFontSet, uintptr(unsafe.Pointer(&fontSet)))); err != nil {
		return 0, err
	}
	if err := hresult(comCall(factory3, writeFactory3CollectionFromFontSet, fontSet, uintptr(unsafe.Pointer(&collection)))); err != nil {
		return 0, err
	}
	return collection, nil
}

// addFontFaces adds every face of the font file at path to the font set builder.
func addFontFaces(factory, factory3, builder uintptr, path string) error {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var file uintptr
	if err := hresult(comCall(factory, writeFactoryCreateFontFileReference, uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&file)))); err != nil {
		return err
	}
	defer release(&file)
	var supported int32
	var fileType, faceType, faces uint32
	if err := hresult(comCall(file, writeFontFileAnalyze, uintptr(unsafe.Pointer(&supported)), uintptr(unsafe.Pointer(&fileType)), uintptr(unsafe.Pointer(&faceType)), uintptr(unsafe.Pointer(&faces)))); err != nil {
		return err
	}
	if supported == 0 || faces == 0 {
		return fmt.Errorf("windows: DirectWrite does not support the font file")
	}
	for index := uint32(0); index < faces; index++ {
		var reference uintptr
		if err := hresult(comCall(factory3, writeFactory3CreateFaceReference, file, uintptr(index), 0, uintptr(unsafe.Pointer(&reference)))); err != nil {
			return err
		}
		status := comCall(builder, writeSetBuilderAddFace, reference)
		release(&reference)
		if err := hresult(status); err != nil {
			return err
		}
	}
	return nil
}
