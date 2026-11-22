//go:build android

package android

/*
#include <android/native_window_jni.h>
#include <jni.h>
#include <stdlib.h>
#include "jni_android.h"
*/
import "C"

import (
	"os"
	"sync"
	"unsafe"
	_ "unsafe" // for go:linkname
)

// The host's calls, made on the UI thread by dev.viva.host.Native. They queue what
// happened for the program's own thread, and return.

//go:linkname applicationMain main.main
func applicationMain()

var started sync.Once

//export Java_dev_viva_host_Native_start
func Java_dev_viva_host_Native_start(env *C.JNIEnv, _ C.jclass, filesDir, cacheDir C.jstring) {
	started.Do(func() {
		if dir := goString(env, filesDir); dir != "" {
			// What a program looks for under the home directory is the app's own.
			os.Setenv("HOME", dir)
		}
		if dir := goString(env, cacheDir); dir != "" {
			os.Setenv("TMPDIR", dir)
		}
		go func() {
			applicationMain()
			// The program's main returned: its window is gone, and so is the activity.
			C.viva_host_finish()
		}()
	})
}

func goString(env *C.JNIEnv, value C.jstring) string {
	chars := C.viva_string_chars(env, value)
	if chars == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(chars))
	return C.GoString(chars)
}

//export Java_dev_viva_host_Native_surfaceChanged
func Java_dev_viva_host_Native_surfaceChanged(env *C.JNIEnv, _ C.jclass, surface C.jobject, width, height C.jint, density C.jfloat) {
	native := C.ANativeWindow_fromSurface(env, surface)
	if native == nil {
		return
	}
	send(event{kind: eventSurface, surface: native, width: int32(width), height: int32(height), density: float32(density)})
}

//export Java_dev_viva_host_Native_surfaceDestroyed
func Java_dev_viva_host_Native_surfaceDestroyed(env *C.JNIEnv, _ C.jclass) {
	// The host takes the surface back when this returns, so the program must have let
	// go of it by then.
	ack := make(chan struct{})
	send(event{kind: eventSurfaceLost, ack: ack})
	<-ack
}

//export Java_dev_viva_host_Native_touch
func Java_dev_viva_host_Native_touch(env *C.JNIEnv, _ C.jclass, action, pointer C.jint, x, y C.jfloat, tool, buttons, modifiers C.jint, uptime C.jlong) {
	ev := event{kind: eventTouch, action: int32(action), pointer: int32(pointer), x: float32(x), y: float32(y),
		tool: int32(tool), buttons: int32(buttons), modifiers: int32(modifiers), time: int64(uptime)}
	if ev.action == actionMove {
		sendMove(ev)
		return
	}
	send(ev)
}

//export Java_dev_viva_host_Native_visible
func Java_dev_viva_host_Native_visible(env *C.JNIEnv, _ C.jclass, visible C.jboolean) {
	send(event{kind: eventVisible, visible: visible != 0})
}

//export Java_dev_viva_host_Native_lifecycle
func Java_dev_viva_host_Native_lifecycle(env *C.JNIEnv, _ C.jclass, kind C.jint) {
	send(event{kind: eventLifecycle, action: int32(kind)})
}

//export Java_dev_viva_host_Native_lowMemory
func Java_dev_viva_host_Native_lowMemory(env *C.JNIEnv, _ C.jclass) {
	send(event{kind: eventLowMemory})
}

//export Java_dev_viva_host_Native_insets
func Java_dev_viva_host_Native_insets(env *C.JNIEnv, _ C.jclass, left, top, right, bottom C.jfloat) {
	send(event{kind: eventInsets, x: float32(left), y: float32(top), dx: float32(right), dy: float32(bottom)})
}

//export Java_dev_viva_host_Native_closeRequested
func Java_dev_viva_host_Native_closeRequested(env *C.JNIEnv, _ C.jclass) {
	send(event{kind: eventClose})
}

// edit queues an input method's edit and returns when the program has applied it: the
// input method asks what is around the cursor as soon as it has typed.
func queueEdit(operation textOperation, text string, first, last int32, codePoints bool) {
	ack := make(chan struct{})
	send(event{kind: eventText, ack: ack, edit: edit{operation: operation, text: text, first: first, last: last, codePoints: codePoints}})
	<-ack
}

//export Java_dev_viva_host_Native_commitText
func Java_dev_viva_host_Native_commitText(env *C.JNIEnv, _ C.jclass, text C.jstring, newCursor C.jint) {
	queueEdit(opCommit, goString(env, text), int32(newCursor), 0, false)
}

//export Java_dev_viva_host_Native_setComposingText
func Java_dev_viva_host_Native_setComposingText(env *C.JNIEnv, _ C.jclass, text C.jstring, newCursor C.jint) {
	queueEdit(opSetComposing, goString(env, text), int32(newCursor), 0, false)
}

//export Java_dev_viva_host_Native_finishComposing
func Java_dev_viva_host_Native_finishComposing(env *C.JNIEnv, _ C.jclass) {
	queueEdit(opFinish, "", 0, 0, false)
}

//export Java_dev_viva_host_Native_deleteSurrounding
func Java_dev_viva_host_Native_deleteSurrounding(env *C.JNIEnv, _ C.jclass, before, after C.jint, codePoints C.jboolean) {
	queueEdit(opDeleteSurrounding, "", int32(before), int32(after), codePoints != 0)
}

//export Java_dev_viva_host_Native_setSelection
func Java_dev_viva_host_Native_setSelection(env *C.JNIEnv, _ C.jclass, start, end C.jint) {
	queueEdit(opSelect, "", int32(start), int32(end), false)
}

//export Java_dev_viva_host_Native_setComposingRegion
func Java_dev_viva_host_Native_setComposingRegion(env *C.JNIEnv, _ C.jclass, start, end C.jint) {
	queueEdit(opMarkRange, "", int32(start), int32(end), false)
}

//export Java_dev_viva_host_Native_editorAction
func Java_dev_viva_host_Native_editorAction(env *C.JNIEnv, _ C.jclass, action C.jint) {
	send(event{kind: eventText, edit: edit{operation: opAction, editorAction: int32(action)}})
}

//export Java_dev_viva_host_Native_key
func Java_dev_viva_host_Native_key(env *C.JNIEnv, _ C.jclass, keyCode, meta, unicode C.jint, repeat C.jboolean) {
	send(event{kind: eventKey, keyCode: int32(keyCode), modifiers: int32(meta), unicode: int32(unicode), repeat: repeat != 0})
}

//export Java_dev_viva_host_Native_hover
func Java_dev_viva_host_Native_hover(env *C.JNIEnv, _ C.jclass, x, y C.jfloat, meta C.jint, uptime C.jlong) {
	sendMove(event{kind: eventHover, x: float32(x), y: float32(y), modifiers: int32(meta), time: int64(uptime)})
}

//export Java_dev_viva_host_Native_hoverExit
func Java_dev_viva_host_Native_hoverExit(env *C.JNIEnv, _ C.jclass) {
	send(event{kind: eventHoverExit})
}

//export Java_dev_viva_host_Native_scroll
func Java_dev_viva_host_Native_scroll(env *C.JNIEnv, _ C.jclass, x, y, dx, dy C.jfloat, meta C.jint) {
	send(event{kind: eventScroll, x: float32(x), y: float32(y), dx: float32(dx), dy: float32(dy), modifiers: int32(meta)})
}

//export Java_dev_viva_host_Native_appearanceChanged
func Java_dev_viva_host_Native_appearanceChanged(env *C.JNIEnv, _ C.jclass) {
	send(event{kind: eventAppearance})
}

//export Java_dev_viva_host_Native_accessibility
func Java_dev_viva_host_Native_accessibility(env *C.JNIEnv, _ C.jclass, on C.jboolean) {
	send(event{kind: eventAccessibility, visible: on != 0})
}

//export Java_dev_viva_host_Native_accessible
func Java_dev_viva_host_Native_accessible(env *C.JNIEnv, _ C.jclass, node C.jint, action C.jint, number C.jfloat, text C.jstring, start, end C.jint) {
	send(event{kind: eventAccessible, index: int32(node), action: int32(action), dx: float32(number), text: goString(env, text), start: int32(start), end: int32(end)})
}
