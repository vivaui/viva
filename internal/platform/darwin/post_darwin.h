#ifndef VIVA_POST_DARWIN_H
#define VIVA_POST_DARWIN_H

/* Wakes the main queue and drains the functions the Go side posted to it. */
void viva_application_post(void);

#endif
