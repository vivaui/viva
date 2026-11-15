#ifndef VIVA_PROXY_DARWIN_H
#define VIVA_PROXY_DARWIN_H

// viva_system_proxy asks CFNetwork which proxy the system's settings select for
// target (an absolute URL). Status: 0 direct (no proxy), 1 proxy in *value
// (free with viva_system_free), 2 the policy is a PAC script, which this
// function does not evaluate, -1 failure with *error_message.
int viva_system_proxy(const char *target, char **value, char **error_message);

#endif
