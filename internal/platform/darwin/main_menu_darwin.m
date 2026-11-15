#include "main_menu_darwin.h"
#include "menu_bar_darwin.h"

static NSString *viva_application_name;
static NSMutableArray<NSMenu *> *viva_pending_menus;

static NSMenuItem *viva_menu_item(NSMenu *menu, NSString *title, SEL action, NSString *key,
                                  NSEventModifierFlags modifiers) {
    NSMenuItem *item = [[[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key] autorelease];
    item.keyEquivalentModifierMask = modifiers;
    [menu addItem:item];
    return item;
}

static NSMenu *viva_application_menu(NSString *applicationName) {
    NSMenu *applicationMenu = [[[NSMenu alloc] init] autorelease];
    viva_menu_item(applicationMenu, [@"Hide " stringByAppendingString:applicationName],
                   @selector(hide:), @"h", NSEventModifierFlagCommand);
    viva_menu_item(applicationMenu, @"Hide Others", @selector(hideOtherApplications:), @"h",
                   NSEventModifierFlagCommand | NSEventModifierFlagOption);
    viva_menu_item(applicationMenu, @"Show All", @selector(unhideAllApplications:), @"", 0);
    [applicationMenu addItem:[NSMenuItem separatorItem]];
    viva_menu_item(applicationMenu, [@"Quit " stringByAppendingString:applicationName],
                   @selector(terminate:), @"q", NSEventModifierFlagCommand);
    return applicationMenu;
}

static NSMenu *viva_window_menu(void) {
    NSMenu *windowMenu = [[[NSMenu alloc] initWithTitle:@"Window"] autorelease];
    viva_menu_item(windowMenu, @"Minimize", @selector(performMiniaturize:), @"m", NSEventModifierFlagCommand);
    viva_menu_item(windowMenu, @"Zoom", @selector(performZoom:), @"", 0);
    viva_menu_item(windowMenu, @"Toggle Full Screen", @selector(toggleFullScreen:), @"f",
                   NSEventModifierFlagCommand | NSEventModifierFlagControl);
    [windowMenu addItem:[NSMenuItem separatorItem]];
    viva_menu_item(windowMenu, @"Bring All to Front", @selector(arrangeInFront:), @"", 0);
    return windowMenu;
}

void viva_menu_bar_begin(void) {
    [viva_pending_menus release];
    viva_pending_menus = [[NSMutableArray alloc] init];
}

void viva_menu_bar_add(void *handle) {
    if (viva_pending_menus == nil) {
        viva_menu_bar_begin();
    }
    if (handle != NULL) {
        [viva_pending_menus addObject:(NSMenu *)handle];
        [(NSMenu *)handle release];
    }
}

void viva_menu_bar_install(void) {
    NSString *applicationName = viva_application_name != nil ? viva_application_name : @"";
    NSMenu *menuBar = [[[NSMenu alloc] init] autorelease];

    NSMenuItem *applicationItem = [[[NSMenuItem alloc] init] autorelease];
    [menuBar addItem:applicationItem];
    NSMenu *applicationMenu = viva_application_menu(applicationName);
    applicationItem.submenu = applicationMenu;

    for (NSMenu *menu in viva_pending_menus) {
        NSMenuItem *item = [[[NSMenuItem alloc] init] autorelease];
        item.submenu = menu;
        [menuBar addItem:item];
    }

    NSMenuItem *windowItem = [[[NSMenuItem alloc] init] autorelease];
    [menuBar addItem:windowItem];
    NSMenu *windowMenu = viva_window_menu();
    windowItem.submenu = windowMenu;

    NSApp.mainMenu = menuBar;
    /* AppKit lists the open windows in this menu and keeps the checkmark. */
    NSApp.windowsMenu = windowMenu;

    [viva_pending_menus release];
    viva_pending_menus = nil;
}

void viva_install_main_menu(NSString *applicationName) {
    viva_application_name = [applicationName retain];
    viva_menu_bar_begin();
    viva_menu_bar_install();
}
