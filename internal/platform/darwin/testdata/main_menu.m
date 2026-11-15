/* The menu bar every Viva application has: the standard application and Window
 * items carry AppKit's own selectors and shortcuts, so Command+H hides, Command+M
 * minimizes, and Command+W is left to the application. The application's own
 * menus sit between them, with their key equivalents, enablement, check state
 * and actions. */

#import <Cocoa/Cocoa.h>

#include "../main_menu_darwin.h"
#include "../menu_bar_darwin.h"

static int failures;
static int32_t last_action;

/* The Go side normally exports this; the harness records what a choice runs. */
void vivaMenuAction(int32_t action) {
    last_action = action;
}

static NSMenuItem *find(NSMenu *menu, NSString *key, NSEventModifierFlags modifiers) {
    for (NSMenuItem *top in menu.itemArray) {
        for (NSMenuItem *item in top.submenu.itemArray) {
            if ([item.keyEquivalent isEqualToString:key] && item.keyEquivalentModifierMask == modifiers) {
                return item;
            }
        }
    }
    return nil;
}

static NSMenuItem *findTitle(NSMenu *menu, NSString *title) {
    for (NSMenuItem *top in menu.itemArray) {
        for (NSMenuItem *item in top.submenu.itemArray) {
            if ([item.title isEqualToString:title]) {
                return item;
            }
        }
    }
    return nil;
}

static void expectShortcut(NSMenu *menu, NSString *key, NSEventModifierFlags modifiers, SEL action, NSString *what) {
    NSMenuItem *item = find(menu, key, modifiers);
    if (item == nil || item.action != action || item.target != nil) {
        fprintf(stderr, "%s: missing or wired to the wrong action\n", [what UTF8String]);
        failures++;
    }
}

static void build_application_menus(void) {
    viva_menu_bar_begin();
    void *file = viva_menu_create("File");
    viva_menu_add_item(file, "New Window", "n", NSEventModifierFlagCommand, 1, 0, 7);
    viva_menu_add_item(file, "Disabled Entry", "", 0, 0, 0, 8);
    viva_menu_add_separator(file);
    void *recent = viva_menu_create("Open Recent");
    viva_menu_add_item(recent, "alpha", "", 0, 1, 0, 9);
    viva_menu_add_submenu(file, "Open Recent", recent);
    viva_menu_bar_add(file);
    void *view = viva_menu_create("View");
    viva_menu_add_item(view, "Show Sidebar", "", 0, 1, 1, 10);
    viva_menu_bar_add(view);
    viva_menu_bar_install();
}

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        viva_install_main_menu(@"Demo");
        NSMenu *menu = NSApp.mainMenu;
        NSEventModifierFlags command = NSEventModifierFlagCommand;
        expectShortcut(menu, @"h", command, @selector(hide:), @"Command+H hides the application");
        expectShortcut(menu, @"h", command | NSEventModifierFlagOption,
                       @selector(hideOtherApplications:), @"Option+Command+H hides others");
        expectShortcut(menu, @"q", command, @selector(terminate:), @"Command+Q quits");
        expectShortcut(menu, @"m", command, @selector(performMiniaturize:), @"Command+M minimizes");
        expectShortcut(menu, @"f", command | NSEventModifierFlagControl,
                       @selector(toggleFullScreen:), @"Control+Command+F toggles fullscreen");
        if (find(menu, @"w", command) != nil) {
            fprintf(stderr, "Command+W is a menu key equivalent, so the application never sees it\n");
            failures++;
        }
        NSMenuItem *hide = menu.itemArray[0].submenu.itemArray[0];
        if (![hide.title isEqualToString:@"Hide Demo"]) {
            fprintf(stderr, "Hide title = %s\n", [hide.title UTF8String]);
            failures++;
        }

        build_application_menus();
        menu = NSApp.mainMenu;
        if (menu.itemArray.count != 4) {
            fprintf(stderr, "menu bar has %lu top-level menus, want 4\n", (unsigned long)menu.itemArray.count);
            failures++;
        }
        if (![[menu.itemArray[1] submenu].title isEqualToString:@"File"] ||
            ![[menu.itemArray[2] submenu].title isEqualToString:@"View"]) {
            fprintf(stderr, "the application menus are not between the application and Window menus\n");
            failures++;
        }
        NSMenuItem *newWindow = findTitle(menu, @"New Window");
        if (newWindow == nil || ![newWindow.keyEquivalent isEqualToString:@"n"] ||
            newWindow.keyEquivalentModifierMask != command || newWindow.tag != 7 || !newWindow.enabled) {
            fprintf(stderr, "the application item did not carry its shortcut, tag and enablement\n");
            failures++;
        }
        NSMenuItem *disabled = findTitle(menu, @"Disabled Entry");
        if (disabled == nil || disabled.enabled) {
            fprintf(stderr, "a disabled entry was enabled\n");
            failures++;
        }
        NSMenuItem *checked = findTitle(menu, @"Show Sidebar");
        if (checked == nil || checked.state != NSControlStateValueOn) {
            fprintf(stderr, "a checked entry was not checked\n");
            failures++;
        }
        NSMenuItem *recent = findTitle(menu, @"Open Recent");
        if (recent == nil || recent.submenu == nil || recent.submenu.itemArray.count != 1) {
            fprintf(stderr, "a submenu was not installed\n");
            failures++;
        }
        last_action = -1;
        [newWindow.target performSelector:newWindow.action withObject:newWindow];
        if (last_action != 7) {
            fprintf(stderr, "choosing the item ran action %d, want 7\n", last_action);
            failures++;
        }
        if (NSApp.windowsMenu == nil) {
            fprintf(stderr, "the Window menu is not registered with AppKit\n");
            failures++;
        }
    }
    return failures == 0 ? 0 : 1;
}
