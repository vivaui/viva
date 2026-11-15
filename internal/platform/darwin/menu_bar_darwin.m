#import <Cocoa/Cocoa.h>

#include "menu_bar_darwin.h"

#include <stdlib.h>

extern void vivaMenuAction(int32_t action);

@interface VivaMenuTarget : NSObject
- (void)vivaMenuSelected:(id)sender;
@end

@implementation VivaMenuTarget
- (void)vivaMenuSelected:(id)sender {
    vivaMenuAction((int32_t)[sender tag]);
}
@end

static VivaMenuTarget *viva_menu_target;

static NSString *viva_menu_string(const char *value) {
    return value != NULL ? [NSString stringWithUTF8String:value] : @"";
}

void *viva_menu_create(const char *title) {
    NSMenu *menu = [[NSMenu alloc] initWithTitle:viva_menu_string(title)];
    menu.autoenablesItems = NO;
    return menu;
}

void viva_menu_add_item(void *handle, const char *title, const char *key, uint32_t modifiers,
                        int enabled, int checked, int32_t action) {
    NSMenu *menu = (NSMenu *)handle;
    if (viva_menu_target == nil) {
        viva_menu_target = [[VivaMenuTarget alloc] init];
    }
    NSString *equivalent = key != NULL ? viva_menu_string(key) : @"";
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:viva_menu_string(title)
                                                  action:@selector(vivaMenuSelected:)
                                           keyEquivalent:equivalent];
    item.keyEquivalentModifierMask = (NSEventModifierFlags)modifiers;
    item.target = viva_menu_target;
    item.tag = action;
    item.enabled = enabled != 0;
    item.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
    [menu addItem:item];
    [item release];
}

void viva_menu_add_separator(void *handle) {
    [(NSMenu *)handle addItem:[NSMenuItem separatorItem]];
}

void viva_menu_add_submenu(void *handle, const char *title, void *submenu) {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:viva_menu_string(title) action:nil keyEquivalent:@""];
    item.submenu = (NSMenu *)submenu;
    [(NSMenu *)handle addItem:item];
    [item release];
    [(NSMenu *)submenu release]; /* the item's submenu retains it */
}
