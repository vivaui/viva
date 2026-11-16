#ifndef VIVA_SLEEP_DARWIN_H
#define VIVA_SLEEP_DARWIN_H

#include <stdint.h>

// Asks IOKit to keep the machine out of idle sleep until the assertion is
// released. The assertion id is written to *assertion on success; on failure
// *error_message holds a description to free with viva_system_free.
int viva_system_keep_awake(const char *reason, uint32_t *assertion, char **error_message);
void viva_system_release_awake(uint32_t assertion);

#endif
