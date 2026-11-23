//go:build !ios

#import <CFNetwork/CFNetwork.h>

#include <stdlib.h>
#include <string.h>
#include <time.h>

#include "proxy_darwin.h"

struct viva_proxy_request {
    int references;
    int cancelled;
    int done;
    CFArrayRef proxies;
    CFErrorRef error;
};

viva_proxy_request *viva_proxy_request_create(void) {
    viva_proxy_request *request = calloc(1, sizeof *request);
    if (request != NULL) {
        request->references = 1;
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
        free(request);
    }
}

static void viva_proxy_context_release(void *info) {
    viva_proxy_request_release(info);
}

void viva_proxy_request_cancel(viva_proxy_request *request) {
    __atomic_store_n(&request->cancelled, 1, __ATOMIC_RELEASE);
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
    CFRunLoopAddSource(loop, source, mode);
    double deadline = viva_proxy_monotonic_seconds() + timeout;
    while (!request->done && !__atomic_load_n(&request->cancelled, __ATOMIC_ACQUIRE)) {
        double remaining = deadline - viva_proxy_monotonic_seconds();
        if (remaining <= 0) {
            break;
        }
        // A private mode does not dispatch the application's UI. Short slices
        // let request cancellation interrupt a stalled PAC download or script.
        SInt32 result = CFRunLoopRunInMode(mode, remaining < 0.05 ? remaining : 0.05, true);
        if (result == kCFRunLoopRunFinished || result == kCFRunLoopRunStopped) {
            break;
        }
    }
    // Apple requires invalidating this source to terminate unfinished work.
    // The source retains the heap context until CFNetwork is finished with it.
    CFRunLoopSourceInvalidate(source);
    CFRelease(source);
    if (__atomic_load_n(&request->cancelled, __ATOMIC_ACQUIRE)) {
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
