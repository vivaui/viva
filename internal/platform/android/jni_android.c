#include "jni_android.h"

#include <android/log.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>

static JavaVM *vm;
static jclass hostClass, textClass;
static jmethodID hostFinish, hostSetTitle, hostEditor, hostKeyboard, hostSelection;
static jmethodID hostTree, hostClipboardText, hostSetClipboardText, hostOpenURL, hostLanguages, hostDarkTheme, hostAnimationsOff;
static jmethodID textWidth, textMetrics, textRects, textSetFamily, textBegin, textDraw, textEnd;

/* A JNI_OnLoad finds the host's classes while the loader that holds them is the caller's. */
JNIEXPORT jint JNICALL JNI_OnLoad(JavaVM *jvm, void *reserved) {
	JNIEnv *env;
	vm = jvm;
	if ((*jvm)->GetEnv(jvm, (void **)&env, JNI_VERSION_1_6) != JNI_OK) {
		return JNI_ERR;
	}
	jclass host = (*env)->FindClass(env, "dev/viva/host/Host");
	jclass text = (*env)->FindClass(env, "dev/viva/host/Text");
	if (host == NULL || text == NULL) {
		return JNI_ERR;
	}
	hostClass = (*env)->NewGlobalRef(env, host);
	textClass = (*env)->NewGlobalRef(env, text);
	hostFinish = (*env)->GetStaticMethodID(env, hostClass, "finish", "()V");
	hostSetTitle = (*env)->GetStaticMethodID(env, hostClass, "setTitle", "(Ljava/lang/String;)V");
	hostEditor = (*env)->GetStaticMethodID(env, hostClass, "editor", "(Ljava/lang/String;IIII)V");
	hostKeyboard = (*env)->GetStaticMethodID(env, hostClass, "keyboard", "(ZZZ)V");
	hostSelection = (*env)->GetStaticMethodID(env, hostClass, "selection", "(ZFFFFFFFFF)V");
	hostTree = (*env)->GetStaticMethodID(env, hostClass, "tree", "([F[I[F[C[I)V");
	hostClipboardText = (*env)->GetStaticMethodID(env, hostClass, "clipboardText", "()Ljava/lang/String;");
	hostSetClipboardText = (*env)->GetStaticMethodID(env, hostClass, "setClipboardText", "(Ljava/lang/String;)Z");
	hostOpenURL = (*env)->GetStaticMethodID(env, hostClass, "openURL", "(Ljava/lang/String;)Z");
	hostLanguages = (*env)->GetStaticMethodID(env, hostClass, "languages", "()Ljava/lang/String;");
	hostDarkTheme = (*env)->GetStaticMethodID(env, hostClass, "darkTheme", "()I");
	hostAnimationsOff = (*env)->GetStaticMethodID(env, hostClass, "animationsOff", "()Z");
	textWidth = (*env)->GetStaticMethodID(env, textClass, "width", "(Ljava/lang/String;FIZZ)F");
	textMetrics = (*env)->GetStaticMethodID(env, textClass, "metrics", "(FIZZ[F)V");
	textRects = (*env)->GetStaticMethodID(env, textClass, "rects", "(Ljava/lang/String;FIZZZ[I[F[F)V");
	textSetFamily = (*env)->GetStaticMethodID(env, textClass, "setFamily", "(Ljava/lang/String;)V");
	textBegin = (*env)->GetStaticMethodID(env, textClass, "begin", "(II)V");
	textDraw = (*env)->GetStaticMethodID(env, textClass, "draw", "(Ljava/lang/String;FFFIZZIZZ)V");
	textEnd = (*env)->GetStaticMethodID(env, textClass, "end", "(Ljava/nio/ByteBuffer;)V");
	return JNI_VERSION_1_6;
}

/* A thread that attached itself must detach before it exits, or the runtime aborts:
 * a key whose destructor does it as the thread ends. */
static pthread_key_t detachKey;
static pthread_once_t detachOnce = PTHREAD_ONCE_INIT;

static void detach(void *unused) {
	if (vm != NULL) {
		(*vm)->DetachCurrentThread(vm);
	}
}

static void makeDetachKey(void) { pthread_key_create(&detachKey, detach); }

static JNIEnv *env(void) {
	JNIEnv *e = NULL;
	if (vm == NULL) {
		return NULL;
	}
	if ((*vm)->GetEnv(vm, (void **)&e, JNI_VERSION_1_6) == JNI_EDETACHED) {
		if ((*vm)->AttachCurrentThread(vm, &e, NULL) != JNI_OK) {
			return NULL;
		}
		pthread_once(&detachOnce, makeDetachKey);
		pthread_setspecific(detachKey, e);
	}
	return e;
}

int viva_attach_thread(void) { return env() != NULL; }

char *viva_string_chars(JNIEnv *e, jstring value) {
	if (value == NULL) {
		return NULL;
	}
	const char *chars = (*e)->GetStringUTFChars(e, value, NULL);
	if (chars == NULL) {
		return NULL;
	}
	char *copy = strdup(chars);
	(*e)->ReleaseStringUTFChars(e, value, chars);
	return copy;
}

/* clear drops a pending Java exception: a call into the host must not leave one behind. */
static int failed(JNIEnv *e) {
	if ((*e)->ExceptionCheck(e)) {
		(*e)->ExceptionClear(e);
		return 1;
	}
	return 0;
}

float viva_text_width(const uint16_t *value, int length, float size, int weight, int italic, int mono) {
	JNIEnv *e = env();
	if (e == NULL) {
		return -1;
	}
	jstring s = (*e)->NewString(e, (const jchar *)value, length);
	jfloat width = (*e)->CallStaticFloatMethod(e, textClass, textWidth, s, size, weight, italic != 0, mono != 0);
	(*e)->DeleteLocalRef(e, s);
	return failed(e) ? -1 : width;
}

void viva_text_metrics(float size, int weight, int italic, int mono, float *out) {
	JNIEnv *e = env();
	out[0] = out[1] = out[2] = 0;
	if (e == NULL) {
		return;
	}
	jfloatArray array = (*e)->NewFloatArray(e, 3);
	(*e)->CallStaticVoidMethod(e, textClass, textMetrics, size, weight, italic != 0, mono != 0, array);
	if (!failed(e)) {
		(*e)->GetFloatArrayRegion(e, array, 0, 3, out);
	}
	(*e)->DeleteLocalRef(e, array);
}

int viva_text_rects(const uint16_t *value, int length, float size, int weight, int italic, int mono, int rtl,
                    const int32_t *bounds, int boundCount, float *lefts, float *rights) {
	JNIEnv *e = env();
	if (e == NULL) {
		return 0;
	}
	int count = boundCount - 1;
	jstring s = (*e)->NewString(e, (const jchar *)value, length);
	jintArray b = (*e)->NewIntArray(e, boundCount);
	jfloatArray l = (*e)->NewFloatArray(e, count);
	jfloatArray r = (*e)->NewFloatArray(e, count);
	(*e)->SetIntArrayRegion(e, b, 0, boundCount, (const jint *)bounds);
	(*e)->CallStaticVoidMethod(e, textClass, textRects, s, size, weight, italic != 0, mono != 0, rtl != 0, b, l, r);
	int ok = !failed(e);
	if (ok) {
		(*e)->GetFloatArrayRegion(e, l, 0, count, lefts);
		(*e)->GetFloatArrayRegion(e, r, 0, count, rights);
	}
	(*e)->DeleteLocalRef(e, s);
	(*e)->DeleteLocalRef(e, b);
	(*e)->DeleteLocalRef(e, l);
	(*e)->DeleteLocalRef(e, r);
	return ok;
}

void viva_text_set_family(const uint16_t *name, int length) {
	JNIEnv *e = env();
	if (e == NULL) {
		return;
	}
	jstring s = length > 0 ? (*e)->NewString(e, (const jchar *)name, length) : NULL;
	(*e)->CallStaticVoidMethod(e, textClass, textSetFamily, s);
	failed(e);
	if (s != NULL) {
		(*e)->DeleteLocalRef(e, s);
	}
}

int viva_text_begin(int width, int height) {
	JNIEnv *e = env();
	if (e == NULL) {
		return 0;
	}
	(*e)->CallStaticVoidMethod(e, textClass, textBegin, width, height);
	return !failed(e);
}

void viva_text_draw(const uint16_t *value, int length, float x, float baseline, float size, int weight, int italic,
                    int mono, int argb, int underline, int rtl) {
	JNIEnv *e = env();
	if (e == NULL) {
		return;
	}
	jstring s = (*e)->NewString(e, (const jchar *)value, length);
	(*e)->CallStaticVoidMethod(e, textClass, textDraw, s, x, baseline, size, weight, italic != 0, mono != 0, argb,
	                           underline != 0, rtl != 0);
	failed(e);
	(*e)->DeleteLocalRef(e, s);
}

int viva_text_end(void *out, int64_t capacity) {
	JNIEnv *e = env();
	if (e == NULL) {
		return 0;
	}
	jobject buffer = (*e)->NewDirectByteBuffer(e, out, capacity);
	if (buffer == NULL) {
		failed(e);
		return 0;
	}
	(*e)->CallStaticVoidMethod(e, textClass, textEnd, buffer);
	(*e)->DeleteLocalRef(e, buffer);
	return !failed(e);
}

void viva_host_finish(void) {
	JNIEnv *e = env();
	if (e != NULL) {
		(*e)->CallStaticVoidMethod(e, hostClass, hostFinish);
		failed(e);
	}
}

void viva_host_set_title(const uint16_t *title, int length) {
	JNIEnv *e = env();
	if (e == NULL) {
		return;
	}
	jstring s = (*e)->NewString(e, (const jchar *)title, length);
	(*e)->CallStaticVoidMethod(e, hostClass, hostSetTitle, s);
	failed(e);
	(*e)->DeleteLocalRef(e, s);
}

void viva_log(int priority, const char *line) { __android_log_write(priority, "viva", line); }

void viva_host_editor(const uint16_t *text, int length, int selectionStart, int selectionEnd, int compositionStart, int compositionEnd) {
	JNIEnv *e = env();
	if (e == NULL) {
		return;
	}
	jstring s = (*e)->NewString(e, (const jchar *)text, length);
	(*e)->CallStaticVoidMethod(e, hostClass, hostEditor, s, selectionStart, selectionEnd, compositionStart, compositionEnd);
	failed(e);
	(*e)->DeleteLocalRef(e, s);
}

void viva_host_keyboard(int editing, int multiline, int secure) {
	JNIEnv *e = env();
	if (e != NULL) {
		(*e)->CallStaticVoidMethod(e, hostClass, hostKeyboard, editing != 0, multiline != 0, secure != 0);
		failed(e);
	}
}


void viva_host_selection(int show, float startX, float startY, float endX, float endY, float lineHeight,
                         float left, float top, float right, float bottom) {
	JNIEnv *e = env();
	if (e != NULL) {
		(*e)->CallStaticVoidMethod(e, hostClass, hostSelection, show != 0, startX, startY, endX, endY, lineHeight,
		                           left, top, right, bottom);
		failed(e);
	}
}

static char *host_string(jmethodID method) {
	JNIEnv *e = env();
	if (e == NULL) {
		return NULL;
	}
	jstring s = (jstring)(*e)->CallStaticObjectMethod(e, hostClass, method);
	if (failed(e) || s == NULL) {
		return NULL;
	}
	char *copy = viva_string_chars(e, s);
	(*e)->DeleteLocalRef(e, s);
	return copy;
}

char *viva_host_clipboard_text(void) { return host_string(hostClipboardText); }

char *viva_host_languages(void) { return host_string(hostLanguages); }

static int host_text_call(jmethodID method, const uint16_t *text, int length) {
	JNIEnv *e = env();
	if (e == NULL) {
		return 0;
	}
	jstring s = (*e)->NewString(e, (const jchar *)text, length);
	jboolean ok = (*e)->CallStaticBooleanMethod(e, hostClass, method, s);
	(*e)->DeleteLocalRef(e, s);
	return !failed(e) && ok;
}

int viva_host_set_clipboard_text(const uint16_t *text, int length) { return host_text_call(hostSetClipboardText, text, length); }

int viva_host_open_url(const uint16_t *url, int length) { return host_text_call(hostOpenURL, url, length); }

int viva_host_dark_theme(void) {
	JNIEnv *e = env();
	if (e == NULL) {
		return -1;
	}
	jint theme = (*e)->CallStaticIntMethod(e, hostClass, hostDarkTheme);
	return failed(e) ? -1 : theme;
}

int viva_host_animations_off(void) {
	JNIEnv *e = env();
	if (e == NULL) {
		return 0;
	}
	jboolean off = (*e)->CallStaticBooleanMethod(e, hostClass, hostAnimationsOff);
	return !failed(e) && off;
}

void viva_host_tree(int count, const float *bounds, const int32_t *ints, const float *ranges, const uint16_t *chars, int charCount, const int32_t *offsets) {
	JNIEnv *e = env();
	if (e == NULL) {
		return;
	}
	jfloatArray b = (*e)->NewFloatArray(e, 4 * count);
	jintArray i = (*e)->NewIntArray(e, 5 * count);
	jfloatArray r = (*e)->NewFloatArray(e, 3 * count);
	jcharArray c = (*e)->NewCharArray(e, charCount);
	jintArray o = (*e)->NewIntArray(e, 4 * count + 1);
	if (count > 0) {
		(*e)->SetFloatArrayRegion(e, b, 0, 4 * count, bounds);
		(*e)->SetIntArrayRegion(e, i, 0, 5 * count, (const jint *)ints);
		(*e)->SetFloatArrayRegion(e, r, 0, 3 * count, ranges);
	}
	if (charCount > 0) {
		(*e)->SetCharArrayRegion(e, c, 0, charCount, (const jchar *)chars);
	}
	(*e)->SetIntArrayRegion(e, o, 0, 4 * count + 1, (const jint *)offsets);
	(*e)->CallStaticVoidMethod(e, hostClass, hostTree, b, i, r, c, o);
	failed(e);
	(*e)->DeleteLocalRef(e, b);
	(*e)->DeleteLocalRef(e, i);
	(*e)->DeleteLocalRef(e, r);
	(*e)->DeleteLocalRef(e, c);
	(*e)->DeleteLocalRef(e, o);
}
