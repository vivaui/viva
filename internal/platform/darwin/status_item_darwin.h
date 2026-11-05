#ifndef VIVA_STATUS_ITEM_DARWIN_H
#define VIVA_STATUS_ITEM_DARWIN_H

#include <stddef.h>
#include <stdint.h>

typedef struct {
    int size;
    const unsigned char *rgba;
    int template_image;
} viva_status_icon;

/* One entry of a menu in the order the application numbered it: the entry,
 * then its submenu's entries. A NULL title is a separator. */
typedef struct {
    const char *title;
    int id;
    int disabled;
    int checked;
    int children;
} viva_menu_entry;

/* Sets the Dock icon of a program that is not in an .app bundle. Returns 0 when
 * it did not. Main thread only. */
int viva_application_set_icon(viva_status_icon icon);
void *viva_status_item_create(const char *tooltip, viva_status_icon icon, uintptr_t callback_handle, int has_menu);
int viva_status_item_set_icon(void *item, viva_status_icon icon);
void viva_status_item_set_tooltip(void *item, const char *tooltip);
void viva_status_item_set_menu_entries(void *item, const viva_menu_entry *entries, int count);

#endif
