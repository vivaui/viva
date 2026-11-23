//go:build !ios

#ifndef VIVA_SECRET_DARWIN_H
#define VIVA_SECRET_DARWIN_H

#include <stddef.h>

/* Each call returns an OSStatus: 0 for success. viva_secret_load allocates *data
 * with malloc; the caller frees it. */
int viva_secret_store(const char *service, const char *account, const unsigned char *data, size_t length);
int viva_secret_load(const char *service, const char *account, unsigned char **data, size_t *length);
int viva_secret_delete(const char *service, const char *account);

#endif
