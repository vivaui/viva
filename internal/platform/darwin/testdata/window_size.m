#include "../native.m"
#include "../main_menu_darwin.m"
#include <assert.h>
#include <math.h>

// The callbacks native.m reports to Go; this test listens to none of them.
void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {}
void vivaViewResized(uintptr_t handle, void *view, double width, double height) {}
void vivaRedrawRequested(uintptr_t handle, void *view) {}
void vivaKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) {}
void vivaRawKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int flags, unsigned int code, const unsigned char *utf8, size_t utf8Length, const unsigned char *typed, size_t typedLength) {}
void vivaWindowActivated(uintptr_t handle) {}
void vivaFocusLost(uintptr_t handle, void *view) {}
int vivaTerminalCells(uint32_t character) { return 1; }
int vivaAccessibilityAction(uintptr_t handle, void *view, uint64_t node, int action, double number, const unsigned char *utf8, size_t utf8Length, int start, int end) { return 0; }
viva_text_line *vivaShapedLine(const unsigned char *utf8, size_t length, double size, double weight) { return viva_text_line_create(utf8, length, size, weight); }
viva_text_line *vivaShapedLineStyle(const unsigned char *utf8, size_t length, double size, double weight, int italic) { return viva_text_line_create_style(utf8, length, size, weight, italic); }
int vivaShortcutEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) { return 0; }
void vivaOpenURL(const char *url) {}
void vivaOpenFile(const char *path) {}
int vivaApplicationQuitRequested(void) { return 0; }
int vivaApplicationReopenRequested(void) { return 0; }
int vivaFileDropEvent(uintptr_t handle, void *view, int phase, double x, double y, const char *paths, size_t length) { return 0; }
void vivaScrollEvent(uintptr_t handle, void *view, double deltaX, double deltaY, double x, double y, int modifiers, int precise) {}
void vivaCloseRequested(uintptr_t handle, void *view) {}
void vivaWindowClosed(uintptr_t handle) {}
void vivaFrameChanged(uintptr_t handle, void *view, double x, double y, double width, double height) {}
void vivaWindowMinimized(uintptr_t handle, void *view, int minimized) {}
void vivaWindowOccluded(uintptr_t handle, void *view, int visible) {}
void vivaWindowFullscreen(uintptr_t handle, void *view, int fullscreen) {}
int vivaWindowReopenRequested(uintptr_t handle, void *view) { return 0; }
int vivaPressAt(uintptr_t handle, double x, double y) { return 0; }
void vivaTextInput(uintptr_t handle, void *view, int kind, const unsigned char *utf8,
                  size_t length, int64_t start, int64_t selectedLength,
                  int64_t replacementStart, int64_t replacementLength, int command) {}
unsigned char *vivaCopyText(uintptr_t handle, void *view, size_t *length) { *length = 0; return NULL; }

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
