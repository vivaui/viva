#include "main_menu_darwin.h"

static NSMenuItem *viva_menu_item(NSMenu *menu, NSString *title, SEL action, NSString *key,
                                  NSEventModifierFlags modifiers) {
    NSMenuItem *item = [[[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key] autorelease];
    item.keyEquivalentModifierMask = modifiers;
    [menu addItem:item];
    return item;
}

void viva_install_main_menu(NSString *applicationName) {
    NSMenu *menuBar = [[[NSMenu alloc] init] autorelease];

    NSMenuItem *applicationItem = [[[NSMenuItem alloc] init] autorelease];
    [menuBar addItem:applicationItem];
    NSMenu *applicationMenu = [[[NSMenu alloc] init] autorelease];
    viva_menu_item(applicationMenu, [@"Hide " stringByAppendingString:applicationName],
                   @selector(hide:), @"h", NSEventModifierFlagCommand);
    viva_menu_item(applicationMenu, @"Hide Others", @selector(hideOtherApplications:), @"h",
                   NSEventModifierFlagCommand | NSEventModifierFlagOption);
    viva_menu_item(applicationMenu, @"Show All", @selector(unhideAllApplications:), @"", 0);
    [applicationMenu addItem:[NSMenuItem separatorItem]];
    viva_menu_item(applicationMenu, [@"Quit " stringByAppendingString:applicationName],
                   @selector(terminate:), @"q", NSEventModifierFlagCommand);
    applicationItem.submenu = applicationMenu;

    NSMenuItem *windowItem = [[[NSMenuItem alloc] init] autorelease];
    [menuBar addItem:windowItem];
    NSMenu *windowMenu = [[[NSMenu alloc] initWithTitle:@"Window"] autorelease];
    viva_menu_item(windowMenu, @"Minimize", @selector(performMiniaturize:), @"m", NSEventModifierFlagCommand);
    viva_menu_item(windowMenu, @"Zoom", @selector(performZoom:), @"", 0);
    viva_menu_item(windowMenu, @"Toggle Full Screen", @selector(toggleFullScreen:), @"f",
                   NSEventModifierFlagControl | NSEventModifierFlagCommand);
    [windowMenu addItem:[NSMenuItem separatorItem]];
    viva_menu_item(windowMenu, @"Bring All to Front", @selector(arrangeInFront:), @"", 0);
    windowItem.submenu = windowMenu;

    NSApp.mainMenu = menuBar;
    /* AppKit lists the open windows in this menu and keeps the checkmark. */
    NSApp.windowsMenu = windowMenu;
}
