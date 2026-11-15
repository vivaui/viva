#ifndef VIVA_MENU_BAR_DARWIN_H
#define VIVA_MENU_BAR_DARWIN_H

#include <stddef.h>
#include <stdint.h>

/* Opaque menus: the header stays usable from a cgo preamble, which cannot parse
 * Objective-C headers. viva_menu_create returns a retained menu; the adders
 * take void* handles, and viva_menu_bar_add hands ownership to the pending bar. */
void *viva_menu_create(const char *title);
void viva_menu_add_item(void *menu, const char *title, const char *key, uint32_t modifiers,
                        int enabled, int checked, int32_t action);
void viva_menu_add_separator(void *menu);
void viva_menu_add_submenu(void *menu, const char *title, void *submenu);

/* Collects the application's menus and installs them between the application
 * menu and the Window menu. begin may be called without add; install always
 * rebuilds the bar and keeps the application name last installed. */
void viva_menu_bar_begin(void);
void viva_menu_bar_add(void *menu);
void viva_menu_bar_install(void);

#endif
