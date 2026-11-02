#ifndef VIVA_MAIN_MENU_DARWIN_H
#define VIVA_MAIN_MENU_DARWIN_H

#import <Cocoa/Cocoa.h>

/* Installs the menu bar every Viva application has: the application menu with
 * Hide, Hide Others, Show All and Quit, and a Window menu with Minimize, Zoom
 * and Bring All to Front. AppKit supplies the behaviour of each item and its
 * shortcut (Command+H, Option+Command+H, Command+Q, Command+M).
 *
 * There is no Close item. Command+W is not a window command here: an
 * application decides what it closes, and a menu key equivalent would be
 * offered the key before the application's own handler. Main thread only. */
void viva_install_main_menu(NSString *applicationName);

#endif
