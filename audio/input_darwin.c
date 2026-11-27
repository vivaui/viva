//go:build darwin && !ios

#include <AudioToolbox/AudioToolbox.h>
#include <CoreAudio/CoreAudio.h>
#include <pthread.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct viva_audio_input viva_audio_input;

// viva_audio_input captures from the default input HAL AudioUnit into a ring
// buffer. CoreAudio's input callback runs on its own real-time thread and only
// copies bytes under the mutex; Go reads from the ring without touching CoreAudio.
struct viva_audio_input {
	pthread_mutex_t mutex;
	pthread_cond_t cond;
	uint8_t *data;
	size_t capacity;
	size_t frame;
	size_t read;
	size_t used;
	AudioUnit unit;
	AudioConverterRef converter;
	AudioStreamBasicDescription client_fmt;
	AudioStreamBasicDescription target_fmt;
	uint8_t *capture_buf;
	size_t capture_cap;
	uint8_t *convert_buf;
	size_t convert_cap;
	const uint8_t *conv_ptr;
	UInt32 conv_bytes;
	int started;
	int closed;
};

static size_t viva_in_min(size_t a, size_t b) { return a < b ? a : b; }

static void viva_in_ring_push(struct viva_audio_input *a, const uint8_t *src, size_t len) {
	if (len == 0) {
		return;
	}
	pthread_mutex_lock(&a->mutex);
	if (a->closed) {
		pthread_mutex_unlock(&a->mutex);
		return;
	}
	if (len >= a->capacity) {
		src += len - a->capacity;
		len = a->capacity;
		a->read = 0;
		a->used = 0;
	} else if (a->used + len > a->capacity) {
		size_t drop = a->used + len - a->capacity;
		a->read = (a->read + drop) % a->capacity;
		a->used -= drop;
	}
	size_t write = (a->read + a->used) % a->capacity;
	size_t first = viva_in_min(len, a->capacity - write);
	memcpy(a->data + write, src, first);
	if (len > first) {
		memcpy(a->data, src + first, len - first);
	}
	a->used += len;
	pthread_cond_signal(&a->cond);
	pthread_mutex_unlock(&a->mutex);
}

static OSStatus viva_in_conv_supply(AudioConverterRef converter, UInt32 *packets,
                                    AudioBufferList *data,
                                    AudioStreamPacketDescription **desc,
                                    void *ref) {
	(void)converter;
	(void)desc;
	struct viva_audio_input *a = ref;
	if (a->conv_bytes == 0) {
		*packets = 0;
		data->mBuffers[0].mDataByteSize = 0;
		data->mBuffers[0].mData = NULL;
		return noErr;
	}
	UInt32 avail = a->conv_bytes / a->client_fmt.mBytesPerPacket;
	UInt32 take = *packets < avail ? *packets : avail;
	UInt32 take_bytes = take * a->client_fmt.mBytesPerPacket;
	data->mNumberBuffers = 1;
	data->mBuffers[0].mNumberChannels = a->client_fmt.mChannelsPerFrame;
	data->mBuffers[0].mData = (void *)a->conv_ptr;
	data->mBuffers[0].mDataByteSize = take_bytes;
	a->conv_ptr += take_bytes;
	a->conv_bytes -= take_bytes;
	*packets = take;
	return noErr;
}

static OSStatus viva_in_callback(void *ref, AudioUnitRenderActionFlags *flags,
                                 const AudioTimeStamp *timestamp, UInt32 bus,
                                 UInt32 frames, AudioBufferList *io_data) {
	(void)io_data;
	struct viva_audio_input *a = ref;
	size_t need = (size_t)frames * a->client_fmt.mBytesPerFrame;
	if (need == 0 || need > a->capture_cap) {
		return noErr;
	}

	AudioBufferList abl;
	abl.mNumberBuffers = 1;
	abl.mBuffers[0].mNumberChannels = a->client_fmt.mChannelsPerFrame;
	abl.mBuffers[0].mDataByteSize = (UInt32)need;
	abl.mBuffers[0].mData = a->capture_buf;

	OSStatus err = AudioUnitRender(a->unit, flags, timestamp, bus, frames, &abl);
	if (err != noErr) {
		return err;
	}

	if (a->converter == NULL) {
		viva_in_ring_push(a, a->capture_buf, abl.mBuffers[0].mDataByteSize);
		return noErr;
	}

	a->conv_ptr = a->capture_buf;
	a->conv_bytes = abl.mBuffers[0].mDataByteSize;
	while (a->conv_bytes > 0) {
		UInt32 out_packets = (UInt32)(a->convert_cap / a->target_fmt.mBytesPerPacket);
		AudioBufferList out_abl;
		out_abl.mNumberBuffers = 1;
		out_abl.mBuffers[0].mNumberChannels = a->target_fmt.mChannelsPerFrame;
		out_abl.mBuffers[0].mDataByteSize = out_packets * a->target_fmt.mBytesPerPacket;
		out_abl.mBuffers[0].mData = a->convert_buf;

		err = AudioConverterFillComplexBuffer(a->converter, viva_in_conv_supply, a,
		                                      &out_packets, &out_abl, NULL);
		if (out_packets > 0 && out_abl.mBuffers[0].mDataByteSize > 0) {
			viva_in_ring_push(a, a->convert_buf, out_abl.mBuffers[0].mDataByteSize);
		}
		if (err != noErr || out_packets == 0) {
			break;
		}
	}
	return noErr;
}

viva_audio_input *viva_audio_input_open(double rate, uint32_t channels, int32_t *status) {
	*status = 0;
	if (rate < 8000 || rate > 192000 || channels < 1 || channels > 2) {
		*status = kAudioUnitErr_InvalidParameter;
		return NULL;
	}

	AudioObjectPropertyAddress dev_addr = {
		.mSelector = kAudioHardwarePropertyDefaultInputDevice,
		.mScope = kAudioObjectPropertyScopeGlobal,
		.mElement = kAudioObjectPropertyElementMain,
	};
	AudioDeviceID dev_id = kAudioObjectUnknown;
	UInt32 dev_size = sizeof(dev_id);
	OSStatus err = AudioObjectGetPropertyData(kAudioObjectSystemObject, &dev_addr, 0, NULL, &dev_size, &dev_id);
	if (err != noErr || dev_id == kAudioObjectUnknown) {
		*status = -1;
		return NULL;
	}

	AudioComponentDescription description = {
		.componentType = kAudioUnitType_Output,
		.componentSubType = kAudioUnitSubType_HALOutput,
		.componentManufacturer = kAudioUnitManufacturer_Apple,
	};
	AudioComponent component = AudioComponentFindNext(NULL, &description);
	if (component == NULL) {
		*status = -1;
		return NULL;
	}

	struct viva_audio_input *a = calloc(1, sizeof(*a));
	if (a == NULL) {
		*status = kAudioUnitErr_NoConnection;
		return NULL;
	}
	a->frame = (size_t)channels * 2;
	a->capacity = (size_t)(rate / 4.0) * a->frame;
	if (a->capacity < 4096) {
		a->capacity = 4096;
	}
	a->capacity -= a->capacity % a->frame;
	a->data = malloc(a->capacity);
	a->capture_cap = 65536;
	a->capture_buf = malloc(a->capture_cap);
	a->convert_cap = 65536;
	a->convert_buf = malloc(a->convert_cap);
	if (a->data == NULL || a->capture_buf == NULL || a->convert_buf == NULL) {
		free(a->data);
		free(a->capture_buf);
		free(a->convert_buf);
		free(a);
		*status = kAudioUnitErr_NoConnection;
		return NULL;
	}
	if (pthread_mutex_init(&a->mutex, NULL) != 0 ||
	    pthread_cond_init(&a->cond, NULL) != 0) {
		free(a->data);
		free(a->capture_buf);
		free(a->convert_buf);
		free(a);
		*status = kAudioUnitErr_NoConnection;
		return NULL;
	}

	err = AudioComponentInstanceNew(component, &a->unit);
	if (err != noErr) {
		goto fail;
	}

	UInt32 enable_in = 1;
	err = AudioUnitSetProperty(a->unit, kAudioOutputUnitProperty_EnableIO,
	                           kAudioUnitScope_Input, 1, &enable_in, sizeof(enable_in));
	if (err != noErr) {
		goto fail;
	}

	UInt32 disable_out = 0;
	err = AudioUnitSetProperty(a->unit, kAudioOutputUnitProperty_EnableIO,
	                           kAudioUnitScope_Output, 0, &disable_out, sizeof(disable_out));
	if (err != noErr) {
		goto fail;
	}

	err = AudioUnitSetProperty(a->unit, kAudioOutputUnitProperty_CurrentDevice,
	                           kAudioUnitScope_Global, 0, &dev_id, sizeof(dev_id));
	if (err != noErr) {
		goto fail;
	}

	AudioStreamBasicDescription hw_fmt;
	UInt32 hw_size = sizeof(hw_fmt);
	err = AudioUnitGetProperty(a->unit, kAudioUnitProperty_StreamFormat,
	                           kAudioUnitScope_Input, 1, &hw_fmt, &hw_size);
	if (err != noErr) {
		goto fail;
	}

	UInt32 hw_channels = hw_fmt.mChannelsPerFrame;
	if (hw_channels == 0) {
		hw_channels = channels;
	}
	if (hw_channels > 2) {
		hw_channels = 2;
	}
	double hw_rate = hw_fmt.mSampleRate > 0 ? hw_fmt.mSampleRate : rate;

	a->client_fmt = (AudioStreamBasicDescription){
		.mSampleRate = hw_rate,
		.mFormatID = kAudioFormatLinearPCM,
		.mFormatFlags = kLinearPCMFormatFlagIsSignedInteger | kLinearPCMFormatFlagIsPacked,
		.mBytesPerPacket = hw_channels * 2,
		.mFramesPerPacket = 1,
		.mBytesPerFrame = hw_channels * 2,
		.mChannelsPerFrame = hw_channels,
		.mBitsPerChannel = 16,
	};
	a->target_fmt = (AudioStreamBasicDescription){
		.mSampleRate = rate,
		.mFormatID = kAudioFormatLinearPCM,
		.mFormatFlags = kLinearPCMFormatFlagIsSignedInteger | kLinearPCMFormatFlagIsPacked,
		.mBytesPerPacket = (UInt32)a->frame,
		.mFramesPerPacket = 1,
		.mBytesPerFrame = (UInt32)a->frame,
		.mChannelsPerFrame = channels,
		.mBitsPerChannel = 16,
	};

	err = AudioUnitSetProperty(a->unit, kAudioUnitProperty_StreamFormat,
	                           kAudioUnitScope_Output, 1, &a->client_fmt, sizeof(a->client_fmt));
	if (err != noErr) {
		goto fail;
	}

	if (a->client_fmt.mSampleRate != a->target_fmt.mSampleRate ||
	    a->client_fmt.mChannelsPerFrame != a->target_fmt.mChannelsPerFrame) {
		err = AudioConverterNew(&a->client_fmt, &a->target_fmt, &a->converter);
		if (err != noErr) {
			goto fail;
		}
	}

	AURenderCallbackStruct callback = {
		.inputProc = viva_in_callback,
		.inputProcRefCon = a,
	};
	err = AudioUnitSetProperty(a->unit, kAudioOutputUnitProperty_SetInputCallback,
	                           kAudioUnitScope_Global, 0, &callback, sizeof(callback));
	if (err != noErr) {
		goto fail;
	}

	err = AudioUnitInitialize(a->unit);
	if (err != noErr) {
		goto fail;
	}
	err = AudioOutputUnitStart(a->unit);
	if (err != noErr) {
		AudioUnitUninitialize(a->unit);
		goto fail;
	}
	a->started = 1;
	return a;

fail:
	*status = err;
	if (a->converter != NULL) {
		AudioConverterDispose(a->converter);
	}
	if (a->unit != NULL) {
		AudioComponentInstanceDispose(a->unit);
	}
	pthread_mutex_destroy(&a->mutex);
	pthread_cond_destroy(&a->cond);
	free(a->data);
	free(a->capture_buf);
	free(a->convert_buf);
	free(a);
	return NULL;
}

int64_t viva_audio_input_read(viva_audio_input *a, void *pcm, uintptr_t len) {
	if (a == NULL || len == 0) {
		return 0;
	}
	pthread_mutex_lock(&a->mutex);
	while (a->used == 0 && !a->closed) {
		pthread_cond_wait(&a->cond, &a->mutex);
	}
	if (a->closed) {
		pthread_mutex_unlock(&a->mutex);
		return 0;
	}
	size_t take = viva_in_min((size_t)len, a->used);
	take -= take % a->frame;
	if (take == 0 && a->used > 0) {
		take = viva_in_min((size_t)len, a->used);
	}
	uint8_t *dst = pcm;
	size_t first = viva_in_min(take, a->capacity - a->read);
	memcpy(dst, a->data + a->read, first);
	if (take > first) {
		memcpy(dst + first, a->data, take - first);
	}
	a->read = (a->read + take) % a->capacity;
	a->used -= take;
	pthread_mutex_unlock(&a->mutex);
	return (int64_t)take;
}

void viva_audio_input_stop(viva_audio_input *a) {
	if (a == NULL) {
		return;
	}
	pthread_mutex_lock(&a->mutex);
	a->closed = 1;
	pthread_cond_broadcast(&a->cond);
	pthread_mutex_unlock(&a->mutex);
}

void viva_audio_input_close(viva_audio_input *a) {
	if (a == NULL) {
		return;
	}
	viva_audio_input_stop(a);
	if (a->started) {
		AudioOutputUnitStop(a->unit);
		a->started = 0;
	}
	AudioUnitUninitialize(a->unit);
	AudioComponentInstanceDispose(a->unit);
	if (a->converter != NULL) {
		AudioConverterDispose(a->converter);
	}
	pthread_mutex_destroy(&a->mutex);
	pthread_cond_destroy(&a->cond);
	free(a->data);
	free(a->capture_buf);
	free(a->convert_buf);
	free(a);
}
