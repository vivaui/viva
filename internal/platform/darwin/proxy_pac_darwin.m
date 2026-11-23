//go:build !ios

#import <CFNetwork/CFNetwork.h>

#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "proxy_darwin.h"

// lock guards cancelled, waiting and wake. Cancellation signals the wake source
// and wakes waiting's run loop, so a stalled PAC evaluation returns at once
// instead of on the next poll.
struct viva_proxy_request {
    int references;
    int cancelled;
    int done;
    CFArrayRef proxies;
    CFErrorRef error;
    pthread_mutex_t lock;
    CFRunLoopRef waiting;
    CFRunLoopSourceRef wake;
};

viva_proxy_request *viva_proxy_request_create(void) {
    viva_proxy_request *request = calloc(1, sizeof *request);
    if (request != NULL) {
        request->references = 1;
        pthread_mutex_init(&request->lock, NULL);
    }
    return request;
}

static void *viva_proxy_request_retain(void *info) {
    viva_proxy_request *request = info;
    __atomic_fetch_add(&request->references, 1, __ATOMIC_RELAXED);
    return request;
}

void viva_proxy_request_release(viva_proxy_request *request) {
    if (request != NULL && __atomic_fetch_sub(&request->references, 1, __ATOMIC_ACQ_REL) == 1) {
        if (request->proxies != NULL) {
            CFRelease(request->proxies);
        }
        if (request->error != NULL) {
            CFRelease(request->error);
        }
        pthread_mutex_destroy(&request->lock);
        free(request);
    }
}

static void viva_proxy_context_release(void *info) {
    viva_proxy_request_release(info);
}

// viva_proxy_wake only exists so that signalling the wake source makes the run
// loop return from CFRunLoopRunInMode; there is no work to do in it.
static void viva_proxy_wake(void *info) {
    (void)info;
}

void viva_proxy_request_cancel(viva_proxy_request *request) {
    pthread_mutex_lock(&request->lock);
    request->cancelled = 1;
    if (request->waiting != NULL) {
        CFRunLoopSourceSignal(request->wake);
        CFRunLoopWakeUp(request->waiting);
    }
    pthread_mutex_unlock(&request->lock);
}

static void viva_proxy_pac_result(void *info, CFArrayRef proxies, CFErrorRef error) {
    viva_proxy_request *request = info;
    // CFNetwork invokes this on the run loop that scheduled its source. Only
    // cancellation touches the request from another thread, through its atomic.
    if (!request->done) {
        request->proxies = proxies == NULL ? NULL : (CFArrayRef)CFRetain(proxies);
        request->error = error == NULL ? NULL : (CFErrorRef)CFRetain(error);
        request->done = 1;
    }
}

static int viva_proxy_pac_error(char **error_message, const char *message) {
    *error_message = strdup(message);
    return -1;
}

static double viva_proxy_monotonic_seconds(void) {
    struct timespec now;
    clock_gettime(CLOCK_MONOTONIC, &now);
    return (double)now.tv_sec + (double)now.tv_nsec / 1000000000.0;
}

int viva_proxy_execute_pac(viva_proxy_request *request, const char *target,
                           const char *configuration, int from_url, double timeout,
                           char **value, char **error_message) {
    *value = NULL;
    CFURLRef url = CFURLCreateWithBytes(kCFAllocatorDefault, (const UInt8 *)target,
                                        (CFIndex)strlen(target), kCFStringEncodingUTF8, NULL);
    CFStringRef script = CFStringCreateWithCString(kCFAllocatorDefault, configuration, kCFStringEncodingUTF8);
    if (url == NULL || script == NULL) {
        if (url != NULL) CFRelease(url);
        if (script != NULL) CFRelease(script);
        return viva_proxy_pac_error(error_message, "the PAC configuration or target is not UTF-8");
    }
    CFStreamClientContext context = {0, request, viva_proxy_request_retain, viva_proxy_context_release, NULL};
    CFRunLoopSourceRef source = NULL;
    if (from_url) {
        CFURLRef pac_url = CFURLCreateWithString(kCFAllocatorDefault, script, NULL);
        if (pac_url != NULL) {
            source = CFNetworkExecuteProxyAutoConfigurationURL(pac_url, url, viva_proxy_pac_result, &context);
            CFRelease(pac_url);
        }
    } else {
        source = CFNetworkExecuteProxyAutoConfigurationScript(script, url, viva_proxy_pac_result, &context);
    }
    CFRelease(script);
    CFRelease(url);
    if (source == NULL) {
        return viva_proxy_pac_error(error_message, "the PAC evaluation could not be started");
    }

    CFRunLoopRef loop = CFRunLoopGetCurrent();
    CFStringRef mode = CFSTR("dev.vivaui.proxy.PAC");
    CFRunLoopSourceContext wake_context = {0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, viva_proxy_wake};
    CFRunLoopSourceRef wake = CFRunLoopSourceCreate(kCFAllocatorDefault, 0, &wake_context);
    int scheduled = 0;
    pthread_mutex_lock(&request->lock);
    if (!request->cancelled && wake != NULL) {
        request->waiting = loop;
        request->wake = wake;
        CFRunLoopAddSource(loop, source, mode);
        CFRunLoopAddSource(loop, wake, mode);
        scheduled = 1;
    }
    pthread_mutex_unlock(&request->lock);

    double deadline = viva_proxy_monotonic_seconds() + timeout;
    while (scheduled && !request->done) {
        pthread_mutex_lock(&request->lock);
        int cancelled = request->cancelled;
        pthread_mutex_unlock(&request->lock);
        double remaining = deadline - viva_proxy_monotonic_seconds();
        if (cancelled || remaining <= 0) {
            break;
        }
        // Blocks until the PAC work finishes, the wake source is signalled by
        // cancellation, or the remaining time runs out. A private mode does not
        // dispatch the application's UI.
        SInt32 result = CFRunLoopRunInMode(mode, remaining, true);
        if (result == kCFRunLoopRunFinished || result == kCFRunLoopRunStopped) {
            break;
        }
    }

    // Stop cancellation from touching the wake source before it is released.
    pthread_mutex_lock(&request->lock);
    request->waiting = NULL;
    request->wake = NULL;
    int was_cancelled = request->cancelled;
    pthread_mutex_unlock(&request->lock);
    if (scheduled) {
        CFRunLoopRemoveSource(loop, wake, mode);
    }
    if (wake != NULL) {
        CFRunLoopSourceInvalidate(wake);
        CFRelease(wake);
    }
    // Apple requires invalidating this source to terminate unfinished work.
    // The source retains the heap context until CFNetwork is finished with it.
    CFRunLoopSourceInvalidate(source);
    CFRelease(source);
    if (was_cancelled) {
        return viva_proxy_pac_error(error_message, "the PAC evaluation was canceled");
    }
    if (!request->done) {
        return viva_proxy_pac_error(error_message, "the PAC evaluation timed out");
    }
    if (request->error != NULL) {
        CFStringRef description = CFErrorCopyDescription(request->error);
        char message[1024];
        int copied = description != NULL && CFStringGetCString(description, message, sizeof message, kCFStringEncodingUTF8);
        if (description != NULL) CFRelease(description);
        return viva_proxy_pac_error(error_message, copied ? message : "the PAC evaluation failed");
    }
    int status = viva_proxy_select(request->proxies, value, error_message);
    if (status == 2 || status == 3) {
        free(*value);
        *value = NULL;
        return viva_proxy_pac_error(error_message, "the PAC evaluation returned another PAC configuration");
    }
    return status;
}
