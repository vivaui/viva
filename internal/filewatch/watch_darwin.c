//go:build darwin

#include <CoreServices/CoreServices.h>
#include <dispatch/dispatch.h>
#include <stdlib.h>
#include "watch_darwin.h"
#include "_cgo_export.h"

struct viva_fsevents {
	FSEventStreamRef stream;
	dispatch_queue_t queue;
};

static void viva_fsevents_callback(ConstFSEventStreamRef stream, void *info, size_t count,
		void *paths, const FSEventStreamEventFlags flags[], const FSEventStreamEventId ids[]) {
	const char **list = (const char **)paths;
	for (size_t i = 0; i < count; i++) {
		vivaFileEvent((uintptr_t)info, (char *)list[i], (int)flags[i]);
	}
}

static void viva_fsevents_noop(void *context) {}

viva_fsevents *viva_fsevents_open(const char *path, uintptr_t handle) {
	CFStringRef root = CFStringCreateWithCString(NULL, path, kCFStringEncodingUTF8);
	if (root == NULL) {
		return NULL;
	}
	CFArrayRef roots = CFArrayCreate(NULL, (const void **)&root, 1, &kCFTypeArrayCallBacks);
	CFRelease(root);
	if (roots == NULL) {
		return NULL;
	}
	FSEventStreamContext context = {0, (void *)handle, NULL, NULL, NULL};
	// A short latency lets FSEvents merge a burst into one delivery; it is a
	// coalescing window, not a poll. File events and the root watch are what make
	// in-place writes and a vanished directory visible.
	FSEventStreamRef stream = FSEventStreamCreate(NULL, viva_fsevents_callback, &context, roots,
		kFSEventStreamEventIdSinceNow, 0.05,
		kFSEventStreamCreateFlagFileEvents | kFSEventStreamCreateFlagNoDefer | kFSEventStreamCreateFlagWatchRoot);
	CFRelease(roots);
	if (stream == NULL) {
		return NULL;
	}
	viva_fsevents *result = malloc(sizeof *result);
	if (result == NULL) {
		FSEventStreamRelease(stream);
		return NULL;
	}
	result->stream = stream;
	result->queue = dispatch_queue_create("viva.filewatch", DISPATCH_QUEUE_SERIAL);
	FSEventStreamSetDispatchQueue(stream, result->queue);
	if (!FSEventStreamStart(stream)) {
		FSEventStreamInvalidate(stream);
		FSEventStreamRelease(stream);
		dispatch_release(result->queue);
		free(result);
		return NULL;
	}
	return result;
}

void viva_fsevents_close(viva_fsevents *result) {
	FSEventStreamStop(result->stream);
	FSEventStreamInvalidate(result->stream);
	// The stream delivers on the queue; a synchronous no-op behind it proves the
	// last callback has returned.
	dispatch_sync_f(result->queue, NULL, viva_fsevents_noop);
	FSEventStreamRelease(result->stream);
	dispatch_release(result->queue);
	free(result);
}
