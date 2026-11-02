package linux

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	xcbCopyFromParent            = 0
	xcbWindowClassInput          = 1
	xcbCWBackPixel               = 1 << 1
	xcbCWEventMask               = 1 << 11
	xcbCWCursor                  = 1 << 14
	xcbEventKeyPress             = 1 << 0
	xcbEventKeyRelease           = 1 << 1
	xcbEventButtonPress          = 1 << 2
	xcbEventButtonRelease        = 1 << 3
	xcbEventEnterWindow          = 1 << 4
	xcbEventLeaveWindow          = 1 << 5
	xcbEventPointerMotion        = 1 << 6
	xcbEventExposure             = 1 << 15
	xcbEventFocusChange          = 1 << 21
	xcbEventStructure            = 1 << 17
	xcbEventSubstructureNotify   = 1 << 19
	xcbEventSubstructureRedirect = 1 << 20
	xcbPropertyReplace           = 0
	xcbAtomNone                  = 0
	xcbAtomAny                   = 0
	xcbAtomAtom                  = 4
	xcbAtomString                = 31
	xcbAtomWMName                = 39
	xcbAtomWMClass               = 67
	xcbKeyPress                  = 2
	xcbKeyRelease                = 3
	xcbButtonPress               = 4
	xcbButtonRelease             = 5
	xcbMotionNotify              = 6
	xcbEnterNotify               = 7
	xcbLeaveNotify               = 8
	xcbFocusIn                   = 9
	xcbFocusOut                  = 10
	xcbExpose                    = 12
	xcbDestroyNotify             = 17
	xcbConfigureNotify           = 22
	xcbSelectionClear            = 29
	xcbSelectionRequest          = 30
	xcbSelectionNotify           = 31
	xcbClientMessage             = 33
	xcbMappingNotify             = 34
	xcbPollInput                 = 1
)

const (
	xcbButtonLeft        = 1
	xcbButtonMiddle      = 2
	xcbButtonRight       = 3
	xcbButtonScrollUp    = 4
	xcbButtonScrollDown  = 5
	xcbButtonScrollLeft  = 6
	xcbButtonScrollRight = 7
	// The side buttons of a mouse: the evdev driver numbers them after the wheel.
	xcbButtonBack    = 8
	xcbButtonForward = 9
)

const (
	xcbMappingModifier = 0
	xcbMappingKeyboard = 1
)

type xcbScreen struct {
	Root, DefaultColormap, WhitePixel, BlackPixel, CurrentInputMasks uint32
	WidthPixels, HeightPixels, WidthMillimeters, HeightMillimeters   uint16
	MinInstalledMaps, MaxInstalledMaps                               uint16
	RootVisual                                                       uint32
	BackingStores, SaveUnders, RootDepth, AllowedDepthsLength        uint8
}

type xcbScreenIterator struct {
	Data       *xcbScreen
	Remaining  int32
	ByteOffset int32
}

type xcbSetup struct {
	Status, Padding                                   uint8
	ProtocolMajor, ProtocolMinor, Length              uint16
	ReleaseNumber, ResourceIDBase, ResourceIDMask     uint32
	MotionBufferSize                                  uint32
	VendorLength, MaximumRequestLength                uint16
	RootsLength, PixmapFormatsLength                  uint8
	ImageByteOrder, BitmapFormatBitOrder              uint8
	BitmapFormatScanlineUnit, BitmapFormatScanlinePad uint8
	MinimumKeycode, MaximumKeycode                    uint8
	Pad                                               [4]byte
}

type xcbVoidCookie struct{ Sequence uint32 }

// xcbRectangle is xcb_rectangle_t.
type xcbRectangle struct {
	X, Y          int16
	Width, Height uint16
}
type xcbAtomCookie struct{ Sequence uint32 }
type xcbSelectionOwnerCookie struct{ Sequence uint32 }
type xcbPropertyCookie struct{ Sequence uint32 }
type xcbTranslateCookie struct{ Sequence uint32 }

// xcbTranslateReply is the answer to a coordinate translation.
type xcbTranslateReply struct {
	ResponseType uint8
	SameScreen   uint8
	Sequence     uint16
	Length       uint32
	Child        uint32
	DstX, DstY   int16
}
type xcbKeyboardMappingCookie struct{ Sequence uint32 }
type xcbAtomNameReply struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Length       uint32
	NameLength   uint16
	Pad          [22]byte
}
type xcbGenericError struct {
	ResponseType uint8
	ErrorCode    uint8
	Sequence     uint16
	ResourceID   uint32
	MinorCode    uint16
	MajorCode    uint8
	Padding      uint8
}
type xcbAtomReply struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Length       uint32
	Atom         uint32
}
type xcbSelectionOwnerReply struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Length       uint32
	Owner        uint32
}
type xcbPropertyReply struct {
	ResponseType uint8
	Format       uint8
	Sequence     uint16
	Length       uint32
	Type         uint32
	BytesAfter   uint32
	ValueLength  uint32
	Pad          [12]byte
}
type xcbGenericEvent struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Data         [7]uint32
}
type xcbInputEvent struct {
	ResponseType uint8
	Detail       uint8
	Sequence     uint16
	Time         uint32
	Root         uint32
	Event        uint32
	Child        uint32
	RootX        int16
	RootY        int16
	EventX       int16
	EventY       int16
	State        uint16
	SameScreen   uint8
	Padding      uint8
}
type xcbExposeEvent struct {
	ResponseType  uint8
	Padding       uint8
	Sequence      uint16
	Window        uint32
	X, Y          uint16
	Width, Height uint16
	Count         uint16
	Pad           [14]byte
}
type xcbFocusEvent struct {
	ResponseType uint8
	Detail       uint8
	Sequence     uint16
	Event        uint32
	Mode         uint8
	Pad          [23]byte
}
type xcbMappingEvent struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Request      uint8
	FirstKeycode uint8
	Count        uint8
	Pad          [25]byte
}
type xcbKeyboardMappingReply struct {
	ResponseType      uint8
	KeysymsPerKeycode uint8
	Sequence          uint16
	Length            uint32
	Pad               [24]byte
}
type xcbModifierMappingCookie struct{ Sequence uint32 }
type xcbModifierMappingReply struct {
	ResponseType        uint8
	KeycodesPerModifier uint8
	Sequence            uint16
	Length              uint32
	Pad                 [24]byte
}
type xcbConfigureEvent struct {
	ResponseType             uint8
	Padding                  uint8
	Sequence                 uint16
	Event, Window, Above     uint32
	X, Y                     int16
	Width, Height, Border    uint16
	OverrideRedirect, PadEnd uint8
}
type xcbClientEvent struct {
	ResponseType uint8
	Format       uint8
	Sequence     uint16
	Window, Type uint32
	Data         [20]byte
}
type xcbSelectionRequestEvent struct {
	ResponseType                      uint8
	Padding                           uint8
	Sequence                          uint16
	Time, Owner, Requestor, Selection uint32
	Target, Property                  uint32
}
type xcbSelectionNotifyEvent struct {
	ResponseType               uint8
	Padding                    uint8
	Sequence                   uint16
	Time, Requestor, Selection uint32
	Target, Property           uint32
}
type xcbSelectionNotifyWire struct {
	xcbSelectionNotifyEvent
	Padding [8]byte
}
type xcbSelectionClearEvent struct {
	ResponseType           uint8
	Padding                uint8
	Sequence               uint16
	Time, Owner, Selection uint32
}
type pollFD struct {
	FD      int32
	Events  int16
	REvents int16
}

type xcbAPI struct {
	library uintptr
	libc    uintptr

	connect                   func(*byte, *int32) uintptr
	disconnect                func(uintptr)
	connectionHasError        func(uintptr) int32
	getSetup                  func(uintptr) *xcbSetup
	rootsIterator             func(*xcbSetup) xcbScreenIterator
	screenNext                func(*xcbScreenIterator)
	generateID                func(uintptr) uint32
	createWindow              func(uintptr, uint8, uint32, uint32, int16, int16, uint16, uint16, uint16, uint16, uint32, uint32, *uint32) xcbVoidCookie
	changeWindow              func(uintptr, uint32, uint32, *uint32) xcbVoidCookie
	configureWindow           func(uintptr, uint32, uint16, *uint32) xcbVoidCookie
	getAtomName               func(uintptr, uint32) xcbVoidCookie
	getAtomNameReply          func(uintptr, xcbVoidCookie, *unsafe.Pointer) *xcbAtomNameReply
	atomNameValue             func(*xcbAtomNameReply) *byte
	atomNameLength            func(*xcbAtomNameReply) int32
	changeProperty            func(uintptr, uint8, uint32, uint32, uint32, uint8, uint32, unsafe.Pointer) xcbVoidCookie
	getProperty               func(uintptr, uint8, uint32, uint32, uint32, uint32, uint32) xcbPropertyCookie
	getPropertyReply          func(uintptr, xcbPropertyCookie, *unsafe.Pointer) *xcbPropertyReply
	getPropertyValue          func(*xcbPropertyReply) unsafe.Pointer
	getPropertyLength         func(*xcbPropertyReply) int32
	getKeyboardMapping        func(uintptr, uint8, uint8) xcbKeyboardMappingCookie
	keyboardMappingReply      func(uintptr, xcbKeyboardMappingCookie, *unsafe.Pointer) *xcbKeyboardMappingReply
	keyboardMappingKeysyms    func(*xcbKeyboardMappingReply) *uint32
	getModifierMapping        func(uintptr) xcbModifierMappingCookie
	modifierMappingReply      func(uintptr, xcbModifierMappingCookie, *unsafe.Pointer) *xcbModifierMappingReply
	modifierMappingKeycodes   func(*xcbModifierMappingReply) *uint8
	mapWindow                 func(uintptr, uint32) xcbVoidCookie
	unmapWindow               func(uintptr, uint32) xcbVoidCookie
	destroyWindow             func(uintptr, uint32) xcbVoidCookie
	openFont                  func(uintptr, uint32, uint16, *byte) xcbVoidCookie
	closeFont                 func(uintptr, uint32) xcbVoidCookie
	createPixmap              func(uintptr, uint8, uint32, uint32, uint16, uint16) xcbVoidCookie
	freePixmap                func(uintptr, uint32) xcbVoidCookie
	createGC                  func(uintptr, uint32, uint32, uint32, *uint32) xcbVoidCookie
	polyFillRectangle         func(uintptr, uint32, uint32, uint32, *xcbRectangle) xcbVoidCookie
	freeGC                    func(uintptr, uint32) xcbVoidCookie
	putImage                  func(uintptr, uint8, uint32, uint32, uint16, uint16, int16, int16, uint8, uint8, uint32, *byte) xcbVoidCookie
	createCursor              func(uintptr, uint32, uint32, uint32, uint16, uint16, uint16, uint16, uint16, uint16, uint16, uint16) xcbVoidCookie
	createGlyphCursor         func(uintptr, uint32, uint32, uint32, uint16, uint16, uint16, uint16, uint16, uint16, uint16, uint16) xcbVoidCookie
	freeCursor                func(uintptr, uint32) xcbVoidCookie
	bell                      func(uintptr, int8) xcbVoidCookie
	setSelectionOwner         func(uintptr, uint32, uint32, uint32) xcbVoidCookie
	getSelectionOwner         func(uintptr, uint32) xcbSelectionOwnerCookie
	selectionOwnerReply       func(uintptr, xcbSelectionOwnerCookie, *unsafe.Pointer) *xcbSelectionOwnerReply
	convertSelection          func(uintptr, uint32, uint32, uint32, uint32, uint32) xcbVoidCookie
	translateCoordinates      func(uintptr, uint32, uint32, int16, int16) xcbTranslateCookie
	translateCoordinatesReply func(uintptr, xcbTranslateCookie, *unsafe.Pointer) *xcbTranslateReply
	sendEvent                 func(uintptr, uint8, uint32, uint32, *byte) xcbVoidCookie
	ungrabPointer             func(uintptr, uint32) xcbVoidCookie
	requestCheck              func(uintptr, xcbVoidCookie) *xcbGenericError
	flush                     func(uintptr) int32
	waitForEvent              func(uintptr) unsafe.Pointer
	pollForEvent              func(uintptr) unsafe.Pointer
	fileDescriptor            func(uintptr) int32
	internAtom                func(uintptr, uint8, uint16, *byte) xcbAtomCookie
	internAtomReply           func(uintptr, xcbAtomCookie, *unsafe.Pointer) *xcbAtomReply
	poll                      func(*pollFD, uintptr, int32) int32
	errnoLocation             func() *int32
	free                      func(unsafe.Pointer)
}

func loadXCB() (*xcbAPI, error) {
	library, err := purego.Dlopen("libxcb.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("linux: load libxcb: %w", err)
	}
	libc, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		purego.Dlclose(library)
		return nil, fmt.Errorf("linux: load libc: %w", err)
	}
	api := &xcbAPI{library: library, libc: libc}
	purego.RegisterLibFunc(&api.connect, library, "xcb_connect")
	purego.RegisterLibFunc(&api.disconnect, library, "xcb_disconnect")
	purego.RegisterLibFunc(&api.connectionHasError, library, "xcb_connection_has_error")
	purego.RegisterLibFunc(&api.getSetup, library, "xcb_get_setup")
	purego.RegisterLibFunc(&api.rootsIterator, library, "xcb_setup_roots_iterator")
	purego.RegisterLibFunc(&api.screenNext, library, "xcb_screen_next")
	purego.RegisterLibFunc(&api.generateID, library, "xcb_generate_id")
	purego.RegisterLibFunc(&api.createWindow, library, "xcb_create_window")
	purego.RegisterLibFunc(&api.changeWindow, library, "xcb_change_window_attributes_checked")
	purego.RegisterLibFunc(&api.configureWindow, library, "xcb_configure_window")
	purego.RegisterLibFunc(&api.getAtomName, library, "xcb_get_atom_name")
	purego.RegisterLibFunc(&api.getAtomNameReply, library, "xcb_get_atom_name_reply")
	purego.RegisterLibFunc(&api.atomNameValue, library, "xcb_get_atom_name_name")
	purego.RegisterLibFunc(&api.atomNameLength, library, "xcb_get_atom_name_name_length")
	purego.RegisterLibFunc(&api.changeProperty, library, "xcb_change_property")
	purego.RegisterLibFunc(&api.getProperty, library, "xcb_get_property")
	purego.RegisterLibFunc(&api.getPropertyReply, library, "xcb_get_property_reply")
	purego.RegisterLibFunc(&api.getPropertyValue, library, "xcb_get_property_value")
	purego.RegisterLibFunc(&api.getPropertyLength, library, "xcb_get_property_value_length")
	purego.RegisterLibFunc(&api.getKeyboardMapping, library, "xcb_get_keyboard_mapping")
	purego.RegisterLibFunc(&api.keyboardMappingReply, library, "xcb_get_keyboard_mapping_reply")
	purego.RegisterLibFunc(&api.keyboardMappingKeysyms, library, "xcb_get_keyboard_mapping_keysyms")
	purego.RegisterLibFunc(&api.getModifierMapping, library, "xcb_get_modifier_mapping")
	purego.RegisterLibFunc(&api.modifierMappingReply, library, "xcb_get_modifier_mapping_reply")
	purego.RegisterLibFunc(&api.modifierMappingKeycodes, library, "xcb_get_modifier_mapping_keycodes")
	purego.RegisterLibFunc(&api.mapWindow, library, "xcb_map_window")
	purego.RegisterLibFunc(&api.unmapWindow, library, "xcb_unmap_window")
	purego.RegisterLibFunc(&api.destroyWindow, library, "xcb_destroy_window")
	purego.RegisterLibFunc(&api.openFont, library, "xcb_open_font_checked")
	purego.RegisterLibFunc(&api.closeFont, library, "xcb_close_font")
	purego.RegisterLibFunc(&api.createPixmap, library, "xcb_create_pixmap_checked")
	purego.RegisterLibFunc(&api.freePixmap, library, "xcb_free_pixmap_checked")
	purego.RegisterLibFunc(&api.createGC, library, "xcb_create_gc_checked")
	purego.RegisterLibFunc(&api.polyFillRectangle, library, "xcb_poly_fill_rectangle_checked")
	purego.RegisterLibFunc(&api.freeGC, library, "xcb_free_gc")
	purego.RegisterLibFunc(&api.putImage, library, "xcb_put_image_checked")
	purego.RegisterLibFunc(&api.createCursor, library, "xcb_create_cursor_checked")
	purego.RegisterLibFunc(&api.createGlyphCursor, library, "xcb_create_glyph_cursor_checked")
	purego.RegisterLibFunc(&api.freeCursor, library, "xcb_free_cursor")
	purego.RegisterLibFunc(&api.bell, library, "xcb_bell")
	purego.RegisterLibFunc(&api.setSelectionOwner, library, "xcb_set_selection_owner")
	purego.RegisterLibFunc(&api.getSelectionOwner, library, "xcb_get_selection_owner")
	purego.RegisterLibFunc(&api.selectionOwnerReply, library, "xcb_get_selection_owner_reply")
	purego.RegisterLibFunc(&api.convertSelection, library, "xcb_convert_selection")
	purego.RegisterLibFunc(&api.translateCoordinates, library, "xcb_translate_coordinates")
	purego.RegisterLibFunc(&api.translateCoordinatesReply, library, "xcb_translate_coordinates_reply")
	purego.RegisterLibFunc(&api.sendEvent, library, "xcb_send_event")
	purego.RegisterLibFunc(&api.ungrabPointer, library, "xcb_ungrab_pointer")
	purego.RegisterLibFunc(&api.requestCheck, library, "xcb_request_check")
	purego.RegisterLibFunc(&api.flush, library, "xcb_flush")
	purego.RegisterLibFunc(&api.waitForEvent, library, "xcb_wait_for_event")
	purego.RegisterLibFunc(&api.pollForEvent, library, "xcb_poll_for_event")
	purego.RegisterLibFunc(&api.fileDescriptor, library, "xcb_get_file_descriptor")
	purego.RegisterLibFunc(&api.internAtom, library, "xcb_intern_atom")
	purego.RegisterLibFunc(&api.internAtomReply, library, "xcb_intern_atom_reply")
	purego.RegisterLibFunc(&api.poll, libc, "poll")
	purego.RegisterLibFunc(&api.errnoLocation, libc, "__errno_location")
	purego.RegisterLibFunc(&api.free, libc, "free")
	return api, nil
}

func (api *xcbAPI) close() {
	purego.Dlclose(api.libc)
	purego.Dlclose(api.library)
}

func (api *xcbAPI) atom(connection uintptr, name string) (uint32, error) {
	value := append([]byte(name), 0)
	cookie := api.internAtom(connection, 0, uint16(len(value)-1), &value[0])
	var nativeError unsafe.Pointer
	reply := api.internAtomReply(connection, cookie, &nativeError)
	if nativeError != nil {
		api.free(nativeError)
		if reply != nil {
			api.free(unsafe.Pointer(reply))
		}
		return 0, fmt.Errorf("linux: resolve X atom %q", name)
	}
	if reply == nil {
		return 0, fmt.Errorf("linux: resolve X atom %q", name)
	}
	defer api.free(unsafe.Pointer(reply))
	return reply.Atom, nil
}
