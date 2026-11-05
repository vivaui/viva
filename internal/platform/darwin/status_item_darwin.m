#import <AppKit/AppKit.h>

#include <string.h>

#include "native.h"
#include "status_item_darwin.h"

/* The side of a menu bar icon in points. The bar is 22 points tall on every
 * Mac; AppKit draws a status image at its own size, so the icon is described
 * at the size the bar shows it and its pixels are as dense as the application
 * supplied them. */
#define VIVA_STATUS_ICON_POINTS 18.0

/* A click within this long after a panel dismissed itself is the press that
 * dismissed it: the press makes the panel lose focus, then the click arrives. */
#define VIVA_PANEL_DISMISS_WINDOW 0.35

extern double viva_panel_dismissed_at;
extern void viva_install_status_hooks(void (*remove)(void *item), void (*clear)(void));

extern void vivaStatusItemClicked(
    uintptr_t callbackHandle,
    int button,
    float x,
    float y,
    float width,
    float height,
    int panelDismissed
);
extern void vivaStatusItemUpdateMenu(uintptr_t callbackHandle);
extern void vivaStatusItemMenuChosen(uintptr_t callbackHandle, int id);

enum {
    VIVA_STATUS_BUTTON_PRIMARY = 1,
    VIVA_STATUS_BUTTON_SECONDARY = 2,
};

/* viva_icon_image wraps RGBA pixels as an image drawn at points x points. */
static NSImage *viva_icon_image(viva_status_icon icon, CGFloat points) {
    if (icon.size < 1 || icon.rgba == NULL) {
        return nil;
    }
    NSBitmapImageRep *representation = [[[NSBitmapImageRep alloc]
        initWithBitmapDataPlanes:NULL
                      pixelsWide:icon.size
                      pixelsHigh:icon.size
                   bitsPerSample:8
                 samplesPerPixel:4
                        hasAlpha:YES
                        isPlanar:NO
                  colorSpaceName:NSDeviceRGBColorSpace
                    bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
                     bytesPerRow:(NSInteger)icon.size * 4
                    bitsPerPixel:32] autorelease];
    if (representation == nil) {
        return nil;
    }
    memcpy(representation.bitmapData, icon.rgba, (size_t)icon.size * (size_t)icon.size * 4);
    representation.size = NSMakeSize(points, points);
    NSImage *image = [[[NSImage alloc] initWithSize:representation.size] autorelease];
    [image addRepresentation:representation];
    return image;
}

static NSImage *viva_status_image(viva_status_icon icon) {
    NSImage *image = viva_icon_image(icon, VIVA_STATUS_ICON_POINTS);
    image.template = icon.template_image != 0;
    return image;
}

@interface VivaStatusItem : NSObject <NSMenuDelegate> {
    NSStatusItem *_item;
    uintptr_t _callbackHandle;
    NSMenu *_menu;
}
- (instancetype)initWithTooltip:(NSString *)tooltip icon:(viva_status_icon)icon callbackHandle:(uintptr_t)callbackHandle hasMenu:(BOOL)hasMenu;
- (BOOL)setIcon:(viva_status_icon)icon;
- (void)setTooltip:(NSString *)tooltip;
- (void)remove;
- (void)setMenuEntries:(const viva_menu_entry *)entries count:(int)count;
- (void)menuChosen:(NSMenuItem *)sender;
@end

@implementation VivaStatusItem

- (instancetype)initWithTooltip:(NSString *)tooltip icon:(viva_status_icon)icon callbackHandle:(uintptr_t)callbackHandle hasMenu:(BOOL)hasMenu {
    self = [super init];
    if (self == nil) {
        return nil;
    }
    NSImage *image = viva_status_image(icon);
    if (image == nil) {
        [self release];
        return nil;
    }
    _callbackHandle = callbackHandle;
    _item = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength] retain];
    _item.button.image = image;
    _item.button.toolTip = tooltip;
    if (hasMenu) {
        _menu = [[NSMenu alloc] initWithTitle:@""];
        _menu.autoenablesItems = NO;
        _menu.delegate = self;
        _item.menu = _menu;
    } else {
        _item.button.target = self;
        _item.button.action = @selector(clicked:);
        [_item.button sendActionOn:NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown];
    }
    return self;
}

- (void)dealloc {
    [self remove];
    [super dealloc];
}

- (BOOL)setIcon:(viva_status_icon)icon {
    NSImage *image = viva_status_image(icon);
    if (image == nil || _item == nil) {
        return NO;
    }
    _item.button.image = image;
    return YES;
}

- (void)setTooltip:(NSString *)tooltip {
    _item.button.toolTip = tooltip;
}

- (void)remove {
    if (_item != nil) {
        _item.menu = nil;
        if (_menu != nil) {
            _menu.delegate = nil;
            [_menu release];
            _menu = nil;
        }
        [[NSStatusBar systemStatusBar] removeStatusItem:_item];
        [_item release];
        _item = nil;
    }
    _callbackHandle = 0;
}

- (void)populateMenu:(NSMenu *)menu from:(const viva_menu_entry *)entries cursor:(int *)cursor count:(int)count {
    [menu removeAllItems];
    menu.autoenablesItems = NO;
    for (int index = 0; index < count; index++) {
        const viva_menu_entry *entry = &entries[(*cursor)++];
        if (entry->title == NULL) {
            [menu addItem:[NSMenuItem separatorItem]];
            continue;
        }
        NSString *title = [NSString stringWithUTF8String:entry->title];
        NSMenuItem *item = [[[NSMenuItem alloc] initWithTitle:title ?: @"" action:nil keyEquivalent:@""] autorelease];
        item.tag = entry->id;
        item.enabled = !entry->disabled;
        item.state = entry->checked ? NSControlStateValueOn : NSControlStateValueOff;
        if (entry->children > 0) {
            NSMenu *submenu = [[[NSMenu alloc] initWithTitle:title ?: @""] autorelease];
            [self populateMenu:submenu from:entries cursor:cursor count:entry->children];
            item.submenu = submenu;
        } else {
            item.target = self;
            item.action = @selector(menuChosen:);
        }
        [menu addItem:item];
    }
}

- (void)setMenuEntries:(const viva_menu_entry *)entries count:(int)count {
    if (_menu == nil) {
        return;
    }
    if (entries == NULL || count <= 0) {
        [_menu removeAllItems];
        return;
    }
    int cursor = 0;
    [self populateMenu:_menu from:entries cursor:&cursor count:count];
}

- (void)menuNeedsUpdate:(NSMenu *)menu {
    if (menu == _menu && _callbackHandle != 0) {
        vivaStatusItemUpdateMenu(_callbackHandle);
    }
}

- (void)menuChosen:(NSMenuItem *)sender {
    if (_callbackHandle != 0) {
        vivaStatusItemMenuChosen(_callbackHandle, (int)sender.tag);
    }
}

- (void)clicked:(id)sender {
    (void)sender;
    if (_callbackHandle == 0 || _item == nil) {
        return;
    }
    NSEvent *event = NSApp.currentEvent;
    int button = VIVA_STATUS_BUTTON_PRIMARY;
    if (event.type == NSEventTypeRightMouseDown || event.type == NSEventTypeRightMouseUp ||
        (event.modifierFlags & NSEventModifierFlagControl) != 0) {
        button = VIVA_STATUS_BUTTON_SECONDARY;
    }
    NSRect frame = _item.button.window.frame;
    int dismissed = NSProcessInfo.processInfo.systemUptime - viva_panel_dismissed_at < VIVA_PANEL_DISMISS_WINDOW;
    vivaStatusItemClicked(_callbackHandle, button, frame.origin.x, frame.origin.y, frame.size.width, frame.size.height, dismissed);
}

@end

static NSMutableArray<VivaStatusItem *> *viva_status_items;
static void viva_status_item_remove_now(void *item);
static void viva_status_items_remove_all(void);

int viva_application_set_icon(viva_status_icon icon) {
    @autoreleasepool {
        /* A bundle's own icon is the application's; this is for a program run
         * from a build directory, which the Dock would show as a plain
         * executable. */
        if ([[[NSBundle mainBundle] bundlePath] hasSuffix:@".app"]) {
            return 0;
        }
        /* The Dock draws an icon at up to 128 points, on a 2x screen from twice
         * the pixels, so the image is declared at half its pixel size. */
        NSImage *image = viva_icon_image(icon, icon.size / 2.0);
        if (image == nil) {
            return 0;
        }
        [NSApp setApplicationIconImage:image];
        return 1;
    }
}

void *viva_status_item_create(const char *tooltip, viva_status_icon icon, uintptr_t callback_handle, int has_menu) {
    @autoreleasepool {
        NSString *text = tooltip != NULL ? [NSString stringWithUTF8String:tooltip] : @"";
        VivaStatusItem *item = [[VivaStatusItem alloc] initWithTooltip:text icon:icon callbackHandle:callback_handle hasMenu:has_menu != 0];
        if (item == nil) {
            return NULL;
        }
        if (viva_status_items == nil) {
            viva_status_items = [[NSMutableArray alloc] init];
            viva_install_status_hooks(viva_status_item_remove_now, viva_status_items_remove_all);
        }
        [viva_status_items addObject:item];
        [item release];
        return item;
    }
}

int viva_status_item_set_icon(void *item, viva_status_icon icon) {
    @autoreleasepool {
        return [(VivaStatusItem *)item setIcon:icon] ? 1 : 0;
    }
}

void viva_status_item_set_menu_entries(void *item, const viva_menu_entry *entries, int count) {
    @autoreleasepool {
        [(VivaStatusItem *)item setMenuEntries:entries count:count];
    }
}

void viva_status_item_set_tooltip(void *item, const char *tooltip) {
    @autoreleasepool {
        [(VivaStatusItem *)item setTooltip:tooltip != NULL ? [NSString stringWithUTF8String:tooltip] : @""];
    }
}

/* Takes the item off the bar and forgets it. Called on the event-loop thread. */
static void viva_status_item_remove_now(void *item) {
    @autoreleasepool {
        VivaStatusItem *found = nil;
        for (VivaStatusItem *candidate in viva_status_items) {
            if ((void *)candidate == item) {
                found = candidate;
                break;
            }
        }
        if (found != nil) {
            [found remove];
            [viva_status_items removeObject:found];
        }
    }
}

static void viva_status_items_remove_all(void) {
    @autoreleasepool {
        for (VivaStatusItem *item in viva_status_items) {
            [item remove];
        }
        [viva_status_items removeAllObjects];
    }
}
