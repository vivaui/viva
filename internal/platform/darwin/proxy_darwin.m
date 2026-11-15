#import <Foundation/Foundation.h>
#import <CFNetwork/CFNetwork.h>

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "proxy_darwin.h"

static char *viva_proxy_copy_utf8(CFStringRef value) {
    if (value == NULL) {
        return NULL;
    }
    CFIndex length = CFStringGetMaximumSizeForEncoding(CFStringGetLength(value), kCFStringEncodingUTF8) + 1;
    char *buffer = calloc((size_t)length, 1);
    if (buffer == NULL) {
        return NULL;
    }
    if (!CFStringGetCString(value, buffer, length, kCFStringEncodingUTF8)) {
        free(buffer);
        return NULL;
    }
    return buffer;
}

static void viva_proxy_set_error(char **error_message, const char *message) {
    if (error_message != NULL) {
        *error_message = strdup(message);
    }
}

int viva_system_proxy(const char *target, char **value, char **error_message) {
    if (target == NULL || value == NULL) {
        viva_proxy_set_error(error_message, "the target is missing");
        return -1;
    }
    *value = NULL;
    CFURLRef url = CFURLCreateWithBytes(kCFAllocatorDefault, (const UInt8 *)target, (CFIndex)strlen(target),
                                        kCFStringEncodingUTF8, NULL);
    if (url == NULL) {
        viva_proxy_set_error(error_message, "the target is not a UTF-8 URL");
        return -1;
    }
    CFDictionaryRef settings = CFNetworkCopySystemProxySettings();
    CFArrayRef proxies = CFNetworkCopyProxiesForURL(url, settings);
    CFRelease(url);
    if (settings != NULL) {
        CFRelease(settings);
    }
    if (proxies == NULL || CFArrayGetCount(proxies) == 0) {
        if (proxies != NULL) {
            CFRelease(proxies);
        }
        return 0;
    }
    CFDictionaryRef proxy = (CFDictionaryRef)CFArrayGetValueAtIndex(proxies, 0);
    CFStringRef type = (CFStringRef)CFDictionaryGetValue(proxy, kCFProxyTypeKey);
    int status = 0;
    if (type != NULL && CFStringCompare(type, kCFProxyTypeNone, 0) != kCFCompareEqualTo) {
        if (CFStringCompare(type, kCFProxyTypeAutoConfigurationURL, 0) == 0 ||
            CFStringCompare(type, kCFProxyTypeAutoConfigurationJavaScript, 0) == 0) {
            status = 2;
        } else {
            CFStringRef host = (CFStringRef)CFDictionaryGetValue(proxy, kCFProxyHostNameKey);
            CFNumberRef port = (CFNumberRef)CFDictionaryGetValue(proxy, kCFProxyPortNumberKey);
            char *host_text = viva_proxy_copy_utf8(host);
            int port_value = 0;
            if (port != NULL) {
                CFNumberGetValue(port, kCFNumberIntType, &port_value);
            }
            if (host_text == NULL) {
                viva_proxy_set_error(error_message, "the selected proxy has no host");
                status = -1;
            } else {
                const char *scheme = CFStringCompare(type, kCFProxyTypeSOCKS, 0) == 0 ? "socks5" : "http";
                char buffer[1024];
                if (port_value > 0) {
                    snprintf(buffer, sizeof buffer, "%s://%s:%d", scheme, host_text, port_value);
                } else {
                    snprintf(buffer, sizeof buffer, "%s://%s", scheme, host_text);
                }
                *value = strdup(buffer);
                free(host_text);
                status = *value == NULL ? -1 : 1;
                if (status < 0) {
                    viva_proxy_set_error(error_message, "the proxy URL could not be allocated");
                }
            }
        }
    }
    CFRelease(proxies);
    return status;
}
