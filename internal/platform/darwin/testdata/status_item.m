#import <Cocoa/Cocoa.h>

#include "../status_item_darwin.h"

double viva_panel_dismissed_at = 0.0;
static void (*status_remove_hook)(void *item);
static void (*status_clear_hook)(void);

void viva_install_status_hooks(void (*remove)(void *item), void (*clear)(void)) {
    status_remove_hook = remove;
    status_clear_hook = clear;
}

static int last_chosen_id = 0;
static int update_menu_called = 0;

void vivaStatusItemClicked(uintptr_t callbackHandle, int button, float x, float y, float width, float height, int panelDismissed) {
    (void)callbackHandle; (void)button; (void)x; (void)y; (void)width; (void)height; (void)panelDismissed;
}

void vivaStatusItemUpdateMenu(uintptr_t callbackHandle) {
    (void)callbackHandle;
    update_menu_called++;
}

void vivaStatusItemMenuChosen(uintptr_t callbackHandle, int id) {
    (void)callbackHandle;
    last_chosen_id = id;
}

static int failures = 0;

static void check(int condition, const char *msg) {
    if (!condition) {
        fprintf(stderr, "FAIL: %s\n", msg);
        failures++;
    }
}

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];

        // 18x18 dummy RGBA icon
        unsigned char rgba[18 * 18 * 4] = {0};
        for (int i = 0; i < 18 * 18 * 4; i += 4) {
            rgba[i] = 255;
            rgba[i + 3] = 255;
        }
        viva_status_icon icon = {18, rgba, 0};

        // 1. Create status item with Menu
        void *item = viva_status_item_create("Test Tooltip", icon, 1234, 1);
        check(item != NULL, "item created");

        NSStatusItem *statusItem = [(id)item valueForKey:@"_item"];
        check(statusItem != nil, "NSStatusItem exists");
        check([statusItem.button.toolTip isEqualToString:@"Test Tooltip"], "tooltip set");

        NSMenu *menu = statusItem.menu;
        check(menu != nil, "NSStatusItem has menu");
        check(menu.delegate == (id<NSMenuDelegate>)item, "NSMenu delegate is item");

        // 2. Trigger menuNeedsUpdate
        [menu.delegate menuNeedsUpdate:menu];
        check(update_menu_called == 1, "menuNeedsUpdate called vivaStatusItemUpdateMenu");

        // 3. Set menu entries from application
        viva_menu_entry entries[] = {
            {"First Item", 101, 0, 0, 0},
            {NULL, 0, 0, 0, 0}, // separator
            {"Submenu Parent", 102, 0, 0, 2},
            {"Child 1", 103, 0, 0, 0},
            {"Child 2", 104, 1, 1, 0}, // disabled, checked
            {"Quit Item", 105, 0, 0, 0},
        };
        viva_status_item_set_menu_entries(item, entries, 4); // 4 top-level items (First, Separator, Submenu, Quit)

        check(menu.numberOfItems == 4, "menu has 4 items (including separator)");
        check([[menu itemAtIndex:0].title isEqualToString:@"First Item"], "item 0 title");
        check([menu itemAtIndex:0].tag == 101, "item 0 tag");
        check([menu itemAtIndex:1].isSeparatorItem, "item 1 is separator");
        check([menu itemAtIndex:2].hasSubmenu, "item 2 has submenu");
        check([menu itemAtIndex:2].submenu.numberOfItems == 2, "submenu has 2 children");
        check([[menu itemAtIndex:2].submenu itemAtIndex:1].state == NSControlStateValueOn, "child 2 is checked");
        check(![[menu itemAtIndex:2].submenu itemAtIndex:1].isEnabled, "child 2 is disabled");
        check([[menu itemAtIndex:3].title isEqualToString:@"Quit Item"], "item 3 is Quit");

        // 4. Simulate choosing Quit item
        NSMenuItem *quitItem = [menu itemAtIndex:3];
        #pragma clang diagnostic push
        #pragma clang diagnostic ignored "-Warc-performSelector-leaks"
        [quitItem.target performSelector:quitItem.action withObject:quitItem];
        #pragma clang diagnostic pop
        check(last_chosen_id == 105, "menuChosen reported Quit Item tag 105");

        // 5. Create status item without Menu (OnClick only)
        void *clickItem = viva_status_item_create("Click Item", icon, 5678, 0);
        check(clickItem != NULL, "clickItem created");
        NSStatusItem *clickStatusItem = [(id)clickItem valueForKey:@"_item"];
        check(clickStatusItem != nil, "clickStatusItem exists");
        check(clickStatusItem.menu == nil, "clickStatusItem has no menu");
        check(clickStatusItem.button.target == (id)clickItem, "button target is clickItem");

        // 6. Clean up
        if (status_clear_hook) {
            status_clear_hook();
        }
    }
    return failures == 0 ? 0 : 1;
}
