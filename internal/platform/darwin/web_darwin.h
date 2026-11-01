#ifndef VIVA_INTERNAL_PLATFORM_DARWIN_WEB_H
#define VIVA_INTERNAL_PLATFORM_DARWIN_WEB_H

#include <stddef.h>
#include <stdint.h>

// viva_application_window_view is the content view of a window opened by
// viva_application_open_window, or NULL.
void *viva_application_window_view(void *token);

// viva_web_open puts a page over the view and returns it, or NULL with error (a malloc'd message)
// set. The page is owned by the view's hierarchy until viva_web_close.
void *viva_web_open(void *view, uint32_t id, uintptr_t handle, const char *url, const char *html,
                    const char *base, const char *script, const char *scheme, char **error);
void viva_web_load(void *web, const char *url);
void viva_web_load_html(void *web, const char *html, const char *base);
void viva_web_reload(void *web);
void viva_web_back(void *web);
void viva_web_forward(void *web);
void viva_web_eval(void *web, const char *script);
void viva_web_focus(void *web);
void viva_web_close(void *web);

// viva_web_place puts the page at a place in the view's top-left logical pixels. The first
// visible_count of the count rectangles (x, y, width, height each) are cut out of its picture, where
// Viva's drawing shows through; all of them refuse the pointer, so Viva gets it.
void viva_web_place(void *web, int shown, double x, double y, double width, double height,
                    const double *cuts, size_t visible_count, size_t count);

// viva_web_ready_later calls vivaWebReady(handle) from the main loop once it is idle.
void viva_web_ready_later(uintptr_t handle);

#endif
