//go:build darwin && !ios

#include <AudioToolbox/AudioToolbox.h>
#include <pthread.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct viva_audio viva_audio;

// viva_audio is a ring buffer feeding the default output AudioUnit. The unit's
// render callback runs on CoreAudio's own thread and takes the lock only to
// move bytes; Go never runs on that thread.
struct viva_audio {
	pthread_mutex_t mutex;
	pthread_cond_t cond;
	uint8_t *data;
	size_t capacity; // bytes
	size_t frame;    // bytes per frame
	size_t read;     // byte offset of the oldest frame
	size_t used;     // bytes waiting to play
	AudioUnit unit;
	int started;
	int closed;
};

static size_t viva_min(size_t a, size_t b) { return a < b ? a : b; }

static OSStatus viva_audio_render(void *ref, AudioUnitRenderActionFlags *flags,
                                  const AudioTimeStamp *timestamp, UInt32 bus,
                                  UInt32 frames, AudioBufferList *buffers) {
	(void)flags;
	(void)timestamp;
	(void)bus;
	struct viva_audio *a = ref;
	size_t need = (size_t)frames * a->frame;

	pthread_mutex_lock(&a->mutex);
	// The input scope carries one interleaved buffer. Anything else is a
	// format CoreAudio did not accept, and silence is the safe answer.
	if (buffers->mNumberBuffers != 1) {
		for (UInt32 i = 0; i < buffers->mNumberBuffers; i++) {
			memset(buffers->mBuffers[i].mData, 0, buffers->mBuffers[i].mDataByteSize);
		}
		pthread_mutex_unlock(&a->mutex);
		return noErr;
	}
	AudioBuffer *buffer = &buffers->mBuffers[0];
	size_t wanted = buffer->mDataByteSize;
	size_t take = viva_min(wanted, viva_min(need, a->used));
	uint8_t *dst = buffer->mData;
	size_t first = viva_min(take, a->capacity - a->read);
	memcpy(dst, a->data + a->read, first);
	if (take > first) {
		memcpy(dst + first, a->data, take - first);
	}
	if (take < wanted) {
		memset(dst + take, 0, wanted - take);
	}
	a->read = (a->read + take) % a->capacity;
	a->used -= take;
	pthread_cond_signal(&a->cond);
	pthread_mutex_unlock(&a->mutex);
	return noErr;
}

viva_audio *viva_audio_open(double rate, uint32_t channels, int32_t *status) {
	*status = 0;
	if (rate < 8000 || rate > 192000 || channels < 1 || channels > 2) {
		*status = kAudioUnitErr_InvalidParameter;
		return NULL;
	}

	AudioComponentDescription description = {
		.componentType = kAudioUnitType_Output,
		.componentSubType = kAudioUnitSubType_DefaultOutput,
		.componentManufacturer = kAudioUnitManufacturer_Apple,
	};
	AudioComponent component = AudioComponentFindNext(NULL, &description);
	if (component == NULL) {
		*status = -1;
		return NULL;
	}

	struct viva_audio *a = calloc(1, sizeof(*a));
	if (a == NULL) {
		*status = kAudioUnitErr_NoConnection;
		return NULL;
	}
	a->frame = (size_t)channels * 2;
	a->capacity = (size_t)(rate / 5.0) * a->frame;
	if (a->capacity < 4096) {
		a->capacity = 4096;
	}
	a->capacity -= a->capacity % a->frame;
	a->data = malloc(a->capacity);
	if (a->data == NULL) {
		free(a);
		*status = kAudioUnitErr_NoConnection;
		return NULL;
	}
	if (pthread_mutex_init(&a->mutex, NULL) != 0 ||
	    pthread_cond_init(&a->cond, NULL) != 0) {
		free(a->data);
		free(a);
		*status = kAudioUnitErr_NoConnection;
		return NULL;
	}

	OSStatus err = AudioComponentInstanceNew(component, &a->unit);
	if (err != noErr) {
		goto fail;
	}

	AudioStreamBasicDescription format = {
		.mSampleRate = rate,
		.mFormatID = kAudioFormatLinearPCM,
		.mFormatFlags = kLinearPCMFormatFlagIsSignedInteger |
		                kLinearPCMFormatFlagIsPacked,
		.mBytesPerPacket = (UInt32)a->frame,
		.mFramesPerPacket = 1,
		.mBytesPerFrame = (UInt32)a->frame,
		.mChannelsPerFrame = channels,
		.mBitsPerChannel = 16,
	};
	err = AudioUnitSetProperty(a->unit, kAudioUnitProperty_StreamFormat,
	                           kAudioUnitScope_Input, 0, &format,
	                           sizeof(format));
	if (err != noErr) {
		goto fail;
	}

	AURenderCallbackStruct callback = {
		.inputProc = viva_audio_render,
		.inputProcRefCon = a,
	};
	err = AudioUnitSetProperty(a->unit, kAudioUnitProperty_SetRenderCallback,
	                           kAudioUnitScope_Input, 0, &callback,
	                           sizeof(callback));
	if (err != noErr) {
		goto fail;
	}

	err = AudioUnitInitialize(a->unit);
	if (err != noErr) {
		goto fail;
	}
	return a;

fail:
	*status = err;
	AudioComponentInstanceDispose(a->unit);
	pthread_mutex_destroy(&a->mutex);
	pthread_cond_destroy(&a->cond);
	free(a->data);
	free(a);
	return NULL;
}

int64_t viva_audio_write(viva_audio *a, const void *pcm, uintptr_t len) {
	if (len == 0) {
		return 0;
	}
	const uint8_t *src = pcm;
	size_t left = (size_t)len;

	pthread_mutex_lock(&a->mutex);
	if (a->closed) {
		pthread_mutex_unlock(&a->mutex);
		return 0;
	}
	int start = !a->started && left > 0;
	a->started = 1;
	pthread_mutex_unlock(&a->mutex);

	if (start) {
		OSStatus err = AudioOutputUnitStart(a->unit);
		if (err != noErr) {
			pthread_mutex_lock(&a->mutex);
			a->started = 0;
			pthread_mutex_unlock(&a->mutex);
			return err;
		}
	}

	pthread_mutex_lock(&a->mutex);
	while (left > 0) {
		while (a->used == a->capacity && !a->closed) {
			pthread_cond_wait(&a->cond, &a->mutex);
		}
		if (a->closed) {
			pthread_mutex_unlock(&a->mutex);
			return 0;
		}
		size_t write = (a->read + a->used) % a->capacity;
		size_t room = a->capacity - a->used;
		size_t first = viva_min(viva_min(left, room), a->capacity - write);
		memcpy(a->data + write, src, first);
		if (left > first && first < room) {
			size_t second = viva_min(left - first, room - first);
			memcpy(a->data, src + first, second);
			first += second;
		}
		a->used += first;
		src += first;
		left -= first;
	}
	pthread_mutex_unlock(&a->mutex);
	return (int64_t)len;
}

void viva_audio_close(viva_audio *a) {
	if (a == NULL) {
		return;
	}
	pthread_mutex_lock(&a->mutex);
	a->closed = 1;
	pthread_cond_broadcast(&a->cond);
	int started = a->started;
	a->started = 0;
	pthread_mutex_unlock(&a->mutex);

	if (started) {
		AudioOutputUnitStop(a->unit);
	}
	AudioUnitUninitialize(a->unit);
	AudioComponentInstanceDispose(a->unit);
	pthread_mutex_destroy(&a->mutex);
	pthread_cond_destroy(&a->cond);
	free(a->data);
	free(a);
}
