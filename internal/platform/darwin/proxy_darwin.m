//go:build !ios

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
    if (settings == NULL) {
        CFRelease(url);
        viva_proxy_set_error(error_message, "the system proxy settings are unavailable");
        return -1;
    }
    CFArrayRef proxies = CFNetworkCopyProxiesForURL(url, settings);
    CFRelease(url);
    CFRelease(settings);
    if (proxies == NULL) {
        viva_proxy_set_error(error_message, "the system returned no proxy decision");
        return -1;
    }
    int status = viva_proxy_select(proxies, value, error_message);
    CFRelease(proxies);
    return status;
}

int viva_proxy_select(CFArrayRef proxies, char **value, char **error_message) {
    if (proxies == NULL || CFArrayGetCount(proxies) == 0) {
        viva_proxy_set_error(error_message, "the policy returned no proxy decision");
        return -1;
    }
    CFDictionaryRef proxy = (CFDictionaryRef)CFArrayGetValueAtIndex(proxies, 0);
    CFStringRef type = (CFStringRef)CFDictionaryGetValue(proxy, kCFProxyTypeKey);
    if (type == NULL) {
        viva_proxy_set_error(error_message, "the selected proxy has no type");
        return -1;
    }
    if (CFEqual(type, kCFProxyTypeNone)) {
        return 0;
    }
    if (CFEqual(type, kCFProxyTypeAutoConfigurationURL) ||
        CFEqual(type, kCFProxyTypeAutoConfigurationJavaScript)) {
        int from_url = CFEqual(type, kCFProxyTypeAutoConfigurationURL);
        CFStringRef configuration = NULL;
        if (from_url) {
            CFURLRef pac_url = (CFURLRef)CFDictionaryGetValue(proxy, kCFProxyAutoConfigurationURLKey);
            if (pac_url != NULL) {
                configuration = CFURLGetString(pac_url);
            }
        } else {
            configuration = (CFStringRef)CFDictionaryGetValue(proxy, kCFProxyAutoConfigurationJavaScriptKey);
        }
        *value = viva_proxy_copy_utf8(configuration);
        if (*value == NULL) {
            viva_proxy_set_error(error_message, "the PAC configuration is missing");
            return -1;
        }
        return from_url ? 2 : 3;
    }
    CFStringRef host = (CFStringRef)CFDictionaryGetValue(proxy, kCFProxyHostNameKey);
    CFNumberRef port = (CFNumberRef)CFDictionaryGetValue(proxy, kCFProxyPortNumberKey);
    char *host_text = viva_proxy_copy_utf8(host);
    int port_value = 0;
    if (port != NULL) {
        CFNumberGetValue(port, kCFNumberIntType, &port_value);
    }
    if (host_text == NULL) {
        viva_proxy_set_error(error_message, "the selected proxy has no host");
        return -1;
    }
    const char *scheme = CFEqual(type, kCFProxyTypeSOCKS) ? "socks5" : "http";
    char buffer[1024];
    if (port_value > 0) {
        if (strchr(host_text, ':') != NULL && host_text[0] != '[') {
            snprintf(buffer, sizeof buffer, "%s://[%s]:%d", scheme, host_text, port_value);
        } else {
            snprintf(buffer, sizeof buffer, "%s://%s:%d", scheme, host_text, port_value);
        }
    } else {
        snprintf(buffer, sizeof buffer, "%s://%s", scheme, host_text);
    }
    *value = strdup(buffer);
    free(host_text);
    if (*value == NULL) {
        viva_proxy_set_error(error_message, "the proxy URL could not be allocated");
        return -1;
    }
    return 1;
}
