//go:build !ios

#ifndef VIVA_PROXY_DARWIN_H
#define VIVA_PROXY_DARWIN_H

#include <CoreFoundation/CoreFoundation.h>

// viva_system_proxy asks CFNetwork which proxy the system's settings select for
// target (an absolute URL). Status: 0 direct (no proxy), 1 proxy in *value
// (free with viva_system_free), 2 PAC URL in *value, 3 PAC JavaScript in *value,
// -1 failure with *error_message.
int viva_system_proxy(const char *target, char **value, char **error_message);

// Select the first entry: a failed first choice must not silently become DIRECT.
int viva_proxy_select(CFArrayRef proxies, char **value, char **error_message);

typedef struct viva_proxy_request viva_proxy_request;
viva_proxy_request *viva_proxy_request_create(void);
void viva_proxy_request_cancel(viva_proxy_request *request);
void viva_proxy_request_release(viva_proxy_request *request);
// Evaluate a PAC URL or inline script on this thread's private run-loop mode.
// The source is invalidated on every exit, including cancellation and timeout.
int viva_proxy_execute_pac(viva_proxy_request *request, const char *target,
                           const char *configuration, int from_url, double timeout,
                           char **value, char **error_message);

#endif
