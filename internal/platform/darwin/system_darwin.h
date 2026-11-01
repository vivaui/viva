#ifndef VIVA_SYSTEM_DARWIN_H
#define VIVA_SYSTEM_DARWIN_H

#include <stddef.h>
#include <stdint.h>

typedef struct {
    char **items;
    size_t count;
} viva_system_string_array;

int viva_system_clipboard_text(char **text, char **error_message);
int viva_system_set_clipboard_text(const char *text, char **error_message);
// Clipboard images. A NULL pasteboard name is the general pasteboard (main
// thread); a name selects a private one, which the tests use.
int viva_system_clipboard_has_image(const char *pasteboard);
// Status: 0 PNG returned in *data (free with viva_system_free), 1 no image,
// 2 too large, -1 failure with *error_message.
int viva_system_clipboard_image_png(const char *pasteboard, size_t max_png, size_t max_raw, uint8_t **data, size_t *length, char **error_message);
// Puts an image (PNG bytes) on a named pasteboard, as TIFF when as_tiff is set.
int viva_system_pasteboard_put_image(const char *pasteboard, const uint8_t *png, size_t length, int as_tiff);
// Shows a modal alert. It must run on the main thread; elsewhere it returns -1.
int viva_system_show_fatal_error(const char *title, const char *message);
int viva_system_set_cursor(uint8_t shape, char **error_message);
int viva_system_open_files(
    const char *title,
    const char *const *extensions,
    size_t extension_count,
    int allow_multiple,
    int choose_directories,
    viva_system_string_array *paths,
    char **error_message
);
int viva_system_save_file(
    const char *title,
    const char *suggested_name,
    const char *default_extension,
    const char *const *extensions,
    size_t extension_count,
    char **path,
    char **error_message
);
int viva_system_open_url(const char *target, char **error_message);
// Whether any application is registered for the URL scheme (no trailing colon):
// 1 yes, 0 no, -1 failure with *error_message.
int viva_system_scheme_handler(const char *scheme, char **error_message);
int viva_system_reduce_motion(void);
char *viva_system_preferred_languages(void);
int viva_system_dark_appearance(void);
int viva_system_effective_dark(void);
void viva_system_set_appearance(int mode);
void viva_system_watch_appearance(void);
int viva_system_play_sound(uint8_t sound, char **error_message);
int viva_system_send_notification(
    const char *title,
    const char *body,
    char **error_message
);

void viva_system_free(void *memory);
void viva_system_free_string_array(viva_system_string_array values);

#endif
