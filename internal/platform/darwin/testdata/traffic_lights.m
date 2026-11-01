#include "../native.m"
#include <assert.h>

// The callbacks native.m reports to Go; this test listens to none of them.
void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {}
void vivaViewResized(uintptr_t handle, void *view, double width, double height) {}
void vivaRedrawRequested(uintptr_t handle, void *view) {}
void vivaKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) {}
void vivaRawKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int flags, unsigned int code, const unsigned char *utf8, size_t utf8Length, const unsigned char *typed, size_t typedLength) {}
void vivaFocusLost(uintptr_t handle, void *view) {}
int vivaTerminalCells(uint32_t character) { return 1; }
int vivaAccessibilityAction(uintptr_t handle, void *view, uint64_t node, int action, double number, const unsigned char *utf8, size_t utf8Length, int start, int end) { return 0; }
viva_text_line *vivaShapedLine(const unsigned char *utf8, size_t length, double size, double weight) { return viva_text_line_create(utf8, length, size, weight); }
int vivaShortcutEvent(uintptr_t handle, void *view, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) { return 0; }
void vivaOpenURL(const char *url) {}
int vivaApplicationQuitRequested(void) { return 0; }
int vivaFileDropEvent(uintptr_t handle, void *view, int phase, double x, double y, const char *paths, size_t length) { return 0; }
void vivaScrollEvent(uintptr_t handle, void *view, double deltaX, double deltaY, double x, double y, int modifiers, int precise) {}
void vivaCloseRequested(uintptr_t handle, void *view) {}
void vivaWindowClosed(uintptr_t handle) {}
void vivaFrameChanged(uintptr_t handle, void *view, double x, double y, double width, double height) {}
void vivaWindowMinimized(uintptr_t handle, void *view, int minimized) {}
int vivaWindowReopenRequested(uintptr_t handle, void *view) { return 0; }
uint16_t vivaRectangleLayer(uintptr_t handle, size_t index) { return 0; }
int vivaPressAt(uintptr_t handle, double x, double y) { return 0; }
void vivaTextInput(uintptr_t handle, void *view, int kind, const unsigned char *utf8,
                  size_t length, int64_t start, int64_t selectedLength,
                  int64_t replacementStart, int64_t replacementLength, int command) {}

// A custom-titlebar window that is never ordered front: nothing appears on screen.
static NSWindow *customTitlebarWindow(CGFloat width, CGFloat height) {
    NSWindowStyleMask style = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                              NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable |
                              NSWindowStyleMaskFullSizeContentView;
    NSWindow *window = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, width, height)
                                                   styleMask:style
                                                     backing:NSBackingStoreBuffered
                                                       defer:NO];
    window.titlebarAppearsTransparent = YES;
    window.titleVisibility = NSWindowTitleHidden;
    return window;
}

// Lets the run loop go round once: AppKit reports some titlebar moves as frame
// changes a turn late, and the window is drawn only after they are handled.
static void settle(void) {
    [[NSRunLoop currentRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.05]];
}

static void assertCenter(NSWindow *window, CGFloat want, const char *when) {
    [window layoutIfNeeded]; // what the next display pass would do
    settle();
    CGFloat got = viva_traffic_light_center(window);
    if (fabs(got - want) > 0.01) {
        fprintf(stderr, "traffic lights centered %.2f points below the top %s, want %.2f\n", got, when, want);
        abort();
    }
}

// Every button is where the others are, at the same height.
static void assertAligned(NSWindow *window, const char *when) {
    settle();
    CGFloat first = 0;
    for (int index = 0; index < 3; index++) {
        NSButton *button = [window standardWindowButton:viva_traffic_buttons[index]];
        CGFloat y = [button convertRect:button.bounds toView:nil].origin.y;
        if (index == 0) {
            first = y;
        } else if (fabs(y - first) > 0.01) {
            fprintf(stderr, "traffic light %d is at y=%.2f, the close button at %.2f %s\n", index, y, first, when);
            abort();
        }
    }
}

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];

        // A strip no taller than the system's leaves the traffic lights alone.
        NSWindow *window = customTitlebarWindow(800, 600);
        CGFloat system = viva_traffic_light_center(window);
        assert(system > 0);
        viva_place_traffic_lights(window, VIVA_DEFAULT_TITLEBAR_HEIGHT);
        assertCenter(window, system, "in a strip of the system's height");
        viva_place_traffic_lights(window, 20);
        assertCenter(window, system, "in a strip shorter than the system's");

        // A taller strip centers them in it, and placing again changes nothing.
        viva_place_traffic_lights(window, 36);
        assertCenter(window, 18, "in a 36 point strip");
        assertAligned(window, "in a 36 point strip");
        assert(fabs(NSHeight(viva_titlebar_container(window).frame) - 36) < 0.01);
        NSRect container = viva_titlebar_container(window).frame;
        viva_place_traffic_lights(window, 36);
        assert(NSEqualRects(container, viva_titlebar_container(window).frame));
        viva_place_traffic_lights(window, 44);
        assertCenter(window, 22, "in a 44 point strip");

        // AppKit puts the buttons back whenever it lays the titlebar out, a
        // resize being one; the delegate puts them back in the same pass.
        window = customTitlebarWindow(800, 600);
        VivaAppDelegate *delegate = [VivaAppDelegate alloc];
        [delegate setValue:window forKey:@"_window"];
        [delegate setValue:[NSNumber numberWithDouble:36] forKey:@"_titlebarHeight"];
        [delegate setValue:[NSNumber numberWithInt:VIVA_WINDOW_CUSTOM_TITLEBAR] forKey:@"_windowMode"];
        window.delegate = delegate;
        [delegate performSelector:@selector(keepTrafficLightsCentered)];
        assertCenter(window, 18, "once the delegate keeps them centered");
        [window setFrame:NSMakeRect(0, 85, 1000, 700) display:NO];
        assertCenter(window, 18, "after the window grew");
        assertAligned(window, "after the window grew");
        [window setFrame:NSMakeRect(0, 85, 500, 400) display:NO];
        assertCenter(window, 18, "after the window shrank");
        window.titlebarAppearsTransparent = NO;
        window.titlebarAppearsTransparent = YES;
        assertCenter(window, 18, "after the titlebar was restyled");
        [delegate release];

        // A window with no traffic lights to move is left alone.
        viva_place_traffic_lights(nil, 36);
        printf("traffic lights: ok (system center %.1f, strip 36 -> 18, strip 44 -> 22)\n", system);
    }
    return 0;
}
