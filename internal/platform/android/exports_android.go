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

//export Java_dev_viva_host_Native_lowMemory
func Java_dev_viva_host_Native_lowMemory(env *C.JNIEnv, _ C.jclass) {
	send(event{kind: eventLowMemory})
}

//export Java_dev_viva_host_Native_closeRequested
func Java_dev_viva_host_Native_closeRequested(env *C.JNIEnv, _ C.jclass) {
	send(event{kind: eventClose})
}
