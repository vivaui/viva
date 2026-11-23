#include "../core_text.m"
#include "../core_render.m"
#include "../native.m"
#include "../main_menu_darwin.m"
#include <assert.h>
#include <math.h>

// The callbacks native.m reports to Go; this test listens to none of them.
void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {}
#include "window_test_callbacks.h"

static BOOL near(CGFloat a, CGFloat b) { return fabs(a - b) < 0.5; }

// A titled window that is never ordered front, so nothing appears on screen.
static NSWindow *titledWindow(CGFloat width, CGFloat height) {
    NSWindowStyleMask style = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                              NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable;
    return [[NSWindow alloc] initWithContentRect:NSMakeRect(200, 200, width, height)
                                       styleMask:style
                                         backing:NSBackingStoreBuffered
                                           defer:NO];
}

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];

        NSWindow *window = titledWindow(320, 240);
        assert(window != nil);
        [window setFrame:NSMakeRect(200, 200, 320, 240) display:NO];
        // The delegate's size path runs only while the window is live; the
        // rest of the delegate is not involved in this test.
        VivaAppDelegate *delegate = [VivaAppDelegate alloc];
        [delegate setValue:window forKey:@"_window"];
        [delegate setValue:@YES forKey:@"_running"];

        NSRect before = [window contentRectForFrameRect:window.frame];
        [delegate setVivaContentSize:NSMakeSize(420, 300)];
        NSRect after = [window contentRectForFrameRect:window.frame];
        assert(near(after.size.width, 420) && near(after.size.height, 300));
        // The top-left origin is preserved: same x, same top edge.
        assert(near(NSMinX(after), NSMinX(before)));
        assert(near(NSMaxY(after), NSMaxY(before)));

        // The minimum content size wins over a smaller request.
        window.contentMinSize = NSMakeSize(260, 200);
        [delegate setVivaContentSize:NSMakeSize(100, 80)];
        NSRect clamped = [window contentRectForFrameRect:window.frame];
        assert(clamped.size.width >= 260 - 0.5 && clamped.size.height >= 200 - 0.5);

        // A request larger than the screen is clamped to its visible area.
        NSScreen *screen = window.screen ?: NSScreen.mainScreen ?: NSScreen.screens.firstObject;
        assert(screen != nil);
        [delegate setVivaContentSize:NSMakeSize(100000, 100000)];
        NSRect huge = [window contentRectForFrameRect:window.frame];
        assert(huge.size.width <= screen.visibleFrame.size.width + 0.5);
        assert(huge.size.height <= screen.visibleFrame.size.height + 0.5);

        printf("window size: content %.0fx%.0f, min clamp %.0fx%.0f, screen clamp %.0fx%.0f\n",
               after.size.width, after.size.height, clamped.size.width, clamped.size.height,
               huge.size.width, huge.size.height);
        [delegate release];
        [window release];
    }
    return 0;
}
