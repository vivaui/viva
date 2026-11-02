/* The menu bar every Viva application has: the standard application and Window
 * items carry AppKit's own selectors and shortcuts, so Command+H hides, Command+M
 * minimizes, and Command+W is left to the application. */

#import <Cocoa/Cocoa.h>

#include "../main_menu_darwin.h"

static int failures;

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

static void expectShortcut(NSMenu *menu, NSString *key, NSEventModifierFlags modifiers, SEL action, NSString *what) {
    NSMenuItem *item = find(menu, key, modifiers);
    if (item == nil || item.action != action || item.target != nil) {
        fprintf(stderr, "%s: missing or wired to the wrong action\n", [what UTF8String]);
        failures++;
    }
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
        if (find(menu, @"w", command) != nil) {
            fprintf(stderr, "Command+W is a menu key equivalent, so the application never sees it\n");
            failures++;
        }
        if (NSApp.windowsMenu == nil) {
            fprintf(stderr, "the Window menu is not registered with AppKit\n");
            failures++;
        }
        NSMenuItem *hide = menu.itemArray[0].submenu.itemArray[0];
        if (![hide.title isEqualToString:@"Hide Demo"]) {
            fprintf(stderr, "Hide title = %s\n", [hide.title UTF8String]);
            failures++;
        }
    }
    return failures == 0 ? 0 : 1;
}
