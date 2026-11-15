package darwin

/*
#include <objc/runtime.h>
#include <objc/message.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdint.h>
#include <limits.h>
#include <stdlib.h>
#include <string.h>
#include "native.h"

typedef struct { unsigned long start, length; } tentative_range;
static int tentative_activations, tentative_deactivations, tentative_discards;
static int tentative_discard_unmarks;
static id tentative_context_view;
static BOOL tentative_true(id object, SEL selector) { return YES; }
static void tentative_noop(id object, SEL selector) {}
static void tentative_activate(id object, SEL selector) { tentative_activations++; }
static void tentative_deactivate(id object, SEL selector) { tentative_deactivations++; }
static void tentative_discard(id object, SEL selector) {
    tentative_discards++;
    if (tentative_discard_unmarks) {
        ((void (*)(id, SEL))objc_msgSend)(tentative_context_view, sel_registerName("unmarkText"));
    }
}
static id tentative_context(id object, SEL selector) {
    tentative_context_view = object;
    static id context;
    if (!context) {
        Class kind = objc_allocateClassPair(objc_getClass("NSObject"), "VivaTentativeBridgeContext", 0);
        class_addMethod(kind, sel_registerName("activate"), (IMP)tentative_activate, "v@:");
        class_addMethod(kind, sel_registerName("deactivate"), (IMP)tentative_deactivate, "v@:");
        class_addMethod(kind, sel_registerName("discardMarkedText"), (IMP)tentative_discard, "v@:");
        class_addMethod(kind, sel_registerName("invalidateCharacterCoordinates"), (IMP)tentative_noop, "v@:");
        objc_registerClassPair(kind);
        context = ((id (*)(id, SEL))objc_msgSend)((id)kind, sel_registerName("alloc"));
        context = ((id (*)(id, SEL))objc_msgSend)(context, sel_registerName("init"));
    }
    return context;
}
static void tentative_write(id object, const char *name, const void *value, size_t size) {
    Ivar field = class_getInstanceVariable(object_getClass(object), name);
    if (field == NULL) { abort(); }
    memcpy((char *)object + ivar_getOffset(field), value, size);
}
static void *tentative_create(uintptr_t handle) {
    Class kind = objc_getClass("VivaTentativeBridgeView");
    if (!kind) {
        kind = objc_allocateClassPair(objc_getClass("VivaMetalView"), "VivaTentativeBridgeView", 0);
        class_addMethod(kind, sel_registerName("rebuildTextTexture"), (IMP)tentative_true, "B@:");
        class_addMethod(kind, sel_registerName("inputContext"), (IMP)tentative_context, "@@:");
        class_addMethod(kind, sel_registerName("restartCaretBlink"), (IMP)tentative_noop, "v@:");
        class_addMethod(kind, sel_registerName("stopCaretBlink"), (IMP)tentative_noop, "v@:");
        class_addMethod(kind, sel_registerName("syncSecureEventInput"), (IMP)tentative_noop, "v@:");
        class_addMethod(kind, sel_registerName("render"), (IMP)tentative_noop, "v@:");
        objc_registerClassPair(kind);
    }
    id object = ((id (*)(id, SEL))objc_msgSend)((id)kind, sel_registerName("alloc"));
    object = ((id (*)(id, SEL))objc_msgSend)(object, sel_registerName("init"));
    BOOL dynamic = YES;
    tentative_write(object, "_dynamicTextInput", &dynamic, sizeof(dynamic));
    tentative_write(object, "_callbackHandle", &handle, sizeof(handle));
    return object;
}
static CFStringRef tentative_string(const char *value) {
    return CFStringCreateWithCString(NULL, value, kCFStringEncodingUTF8);
}
static void tentative_mark(void *view, const char *value, unsigned long start, unsigned long length, unsigned long replacementStart, unsigned long replacementLength, int hasReplacement) {
    CFStringRef string = tentative_string(value);
    tentative_range selection = {start, length};
    tentative_range replacement = {hasReplacement ? replacementStart : LONG_MAX, replacementLength};
    ((void (*)(id, SEL, id, tentative_range, tentative_range))objc_msgSend)((id)view,
        sel_registerName("setMarkedText:selectedRange:replacementRange:"), (id)string, selection, replacement);
    CFRelease(string);
}
static void tentative_insert(void *view, const char *value, unsigned long start, unsigned long length, int hasReplacement) {
    CFStringRef string = tentative_string(value);
    tentative_range replacement = {hasReplacement ? start : LONG_MAX, length};
    ((void (*)(id, SEL, id, tentative_range))objc_msgSend)((id)view,
        sel_registerName("insertText:replacementRange:"), (id)string, replacement);
    CFRelease(string);
}
static void tentative_unmark(void *view) {
    ((void (*)(id, SEL))objc_msgSend)((id)view, sel_registerName("unmarkText"));
}
static void tentative_cancel(void *view) {
    ((void (*)(id, SEL, SEL))objc_msgSend)((id)view, sel_registerName("doCommandBySelector:"), sel_registerName("cancelOperation:"));
}
static void tentative_select(void *view, unsigned long start, unsigned long length) {
    tentative_range selection = {start, length};
    ((void (*)(id, SEL, tentative_range))objc_msgSend)((id)view, sel_registerName("selectTextRange:"), selection);
}
static void tentative_command(void *view) {
    ((void (*)(id, SEL, SEL))objc_msgSend)((id)view, sel_registerName("doCommandBySelector:"), sel_registerName("moveWordLeft:"));
}
static void tentative_resign(void *view) {
    ((BOOL (*)(id, SEL))objc_msgSend)((id)view, sel_registerName("resignFirstResponder"));
}
static void tentative_detach(void *view) {
    ((void (*)(id, SEL))objc_msgSend)((id)view, sel_registerName("detachCallbacks"));
}
static void tentative_unmark_on_discard(int enabled) { tentative_discard_unmarks = enabled; }
static const char *tentative_text(void *pointer) {
    id object = (id)pointer;
    id value = object_getIvar(object, class_getInstanceVariable(object_getClass(object), "_text"));
    return ((const char *(*)(id, SEL))objc_msgSend)(value, sel_registerName("UTF8String"));
}
static int tentative_marked(void *pointer) {
    id object = (id)pointer;
    BOOL marked;
    Ivar field = class_getInstanceVariable(object_getClass(object), "_hasMarkedText");
    memcpy(&marked, (char *)object + ivar_getOffset(field), sizeof(marked));
    return marked != NO;
}
static void tentative_destroy(void *view) {
    uintptr_t handle = 0;
    tentative_write((id)view, "_callbackHandle", &handle, sizeof(handle));
    ((void (*)(id, SEL))objc_msgSend)((id)view, sel_registerName("release"));
}
static void tentative_reset_counts(void) { tentative_activations = tentative_deactivations = tentative_discards = 0; }
static int tentative_activate_count(void) { return tentative_activations; }
static int tentative_deactivate_count(void) { return tentative_deactivations; }
static int tentative_discard_count(void) { return tentative_discards; }
static size_t tentative_view_size(void) { return class_getInstanceSize(objc_getClass("VivaMetalView")); }
static void tentative_benchmark_insert(void *view) {
    id pool = ((id (*)(id, SEL))objc_msgSend)((id)objc_getClass("NSAutoreleasePool"), sel_registerName("alloc"));
    pool = ((id (*)(id, SEL))objc_msgSend)(pool, sel_registerName("init"));
    tentative_range replacement = {LONG_MAX, 0};
    ((void (*)(id, SEL, id, tentative_range))objc_msgSend)((id)view,
        sel_registerName("insertText:replacementRange:"), (id)CFSTR("x"), replacement);
    ((void (*)(id, SEL))objc_msgSend)(pool, sel_registerName("drain"));
}
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

func tentativeBridgeView(state *runState) (unsafe.Pointer, func()) {
	handle := cgo.NewHandle(state)
	view := C.tentative_create(C.uintptr_t(handle))
	return view, func() {
		C.tentative_destroy(view)
		handle.Delete()
	}
}

func tentativeBridgeMark(view unsafe.Pointer, value string, selectedStart, selectedLength, replacementStart, replacementLength int, hasReplacement bool) {
	string := C.CString(value)
	defer C.free(unsafe.Pointer(string))
	C.tentative_mark(view, string, C.ulong(selectedStart), C.ulong(selectedLength), C.ulong(replacementStart), C.ulong(replacementLength), C.int(boolInt(hasReplacement)))
}

func tentativeBridgeInsert(view unsafe.Pointer, value string, start, length int, hasReplacement bool) {
	string := C.CString(value)
	defer C.free(unsafe.Pointer(string))
	C.tentative_insert(view, string, C.ulong(start), C.ulong(length), C.int(boolInt(hasReplacement)))
}

func tentativeBridgeUnmark(view unsafe.Pointer) { C.tentative_unmark(view) }
func tentativeBridgeCancel(view unsafe.Pointer) { C.tentative_cancel(view) }
func tentativeBridgeSelect(view unsafe.Pointer, start, length int) {
	C.tentative_select(view, C.ulong(start), C.ulong(length))
}
func tentativeBridgeCommand(view unsafe.Pointer) { C.tentative_command(view) }
func tentativeBridgeResign(view unsafe.Pointer)  { C.tentative_resign(view) }
func tentativeBridgeDetach(view unsafe.Pointer)  { C.tentative_detach(view) }
func tentativeBridgeUnmarkOnDiscard(enabled bool) {
	C.tentative_unmark_on_discard(C.int(boolInt(enabled)))
}
func tentativeBridgeText(view unsafe.Pointer) string { return C.GoString(C.tentative_text(view)) }
func tentativeBridgeMarked(view unsafe.Pointer) bool { return C.tentative_marked(view) != 0 }
func resetTentativeBridgeCounts()                    { C.tentative_reset_counts() }
func tentativeBridgeCounts() (int, int, int) {
	return int(C.tentative_activate_count()), int(C.tentative_deactivate_count()), int(C.tentative_discard_count())
}

func tentativeBridgeSizes() (int, int) {
	return int(C.sizeof_viva_text_input_config), int(C.tentative_view_size())
}

func tentativeBridgeBenchmarkInsert(view unsafe.Pointer) { C.tentative_benchmark_insert(view) }
