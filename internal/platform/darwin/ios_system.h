//go:build ios

#ifndef VIVA_IOS_SYSTEM_H
#define VIVA_IOS_SYSTEM_H

/* What the system gives an application, answered on the main thread whichever
 * thread asks. Strings are UTF-8, malloc'ed and freed by the caller. */

/* The text on the pasteboard, NULL when there is none. */
char *viva_ios_clipboard_text(void);
void viva_ios_set_clipboard_text(const char *text);

/* Opens a link in the application that takes it; nonzero when one was found. */
int viva_ios_open_url(const char *url);

/* The user's languages, most preferred first, joined by newlines. */
char *viva_ios_preferred_languages(void);

/* Nonzero when the user asked for less motion. */
int viva_ios_reduce_motion(void);

/* 1 for the dark look, 0 for the light one, -1 when unknown. */
int viva_ios_dark_theme(void);

#endif
