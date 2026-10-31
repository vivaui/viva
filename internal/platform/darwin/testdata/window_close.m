#include "../native.m"
#include <assert.h>

static VivaMetalView *closingView;
static NSEvent *exitEvent;
static int pointerCalls;
static int closedCalls;
static BOOL released;

void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {
    assert(!released || handle != 1);
    assert(kind == VIVA_POINTER_EXIT);
    pointerCalls++;
}
void vivaViewResized(uintptr_t handle, void *view, double width, double height) {}
void vivaRedrawRequested(uintptr_t handle, void *view) {}
void vivaKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) {}
void vivaRawKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int flags, unsigned int code, const unsigned char *utf8, size_t utf8Length, const unsigned char *typed, size_t typedLength) {}
void vivaFocusLost(uintptr_t handle, void *view) {}
int vivaAccessibilityAction(uintptr_t handle, void *view, uint64_t node, int action, double number, const unsigned char *utf8, size_t utf8Length, int start, int end) { return 0; }
viva_text_line *vivaShapedLine(const unsigned char *utf8, size_t length, double size, double weight) { return viva_text_line_create(utf8, length, size, weight); }
int vivaShortcutEvent(uintptr_t handle, void *view, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) { return 0; }
void vivaOpenURL(const char *url) {}
int vivaApplicationQuitRequested(void) { return 0; }
int vivaWindowReopenRequested(uintptr_t handle, void *view) { return 0; }
int vivaFileDropEvent(uintptr_t handle, void *view, int phase, double x, double y, const char *paths, size_t length) { return 0; }
void vivaScrollEvent(uintptr_t handle, void *view, double deltaX, double deltaY, double x, double y, int modifiers, int precise) {}
void vivaCloseRequested(uintptr_t handle, void *view) {}
void vivaFrameChanged(uintptr_t handle, void *view, double x, double y, double width, double height) {}
void vivaWindowMinimized(uintptr_t handle, void *view, int minimized) {}
uint16_t vivaRectangleLayer(uintptr_t handle, size_t index) { return 0; }
int vivaPressAt(uintptr_t handle, double x, double y) { return 0; }
void vivaTextInput(uintptr_t handle, void *view, int kind, const unsigned char *utf8,
                  size_t length, int64_t start, int64_t selectedLength,
                  int64_t replacementStart, int64_t replacementLength, int command) {}

void vivaWindowClosed(uintptr_t handle) {
    assert(handle == 1);
    closedCalls++;
    released = YES;
    // AppKit may reenter input delivery as the Go window releases its handle.
    [closingView mouseExited:exitEvent];
}

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        exitEvent = [NSEvent enterExitEventWithType:NSEventTypeMouseExited
            location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0
            context:nil eventNumber:0 trackingNumber:0 userData:NULL];
        NSWindow *window = [[NSWindow alloc]
            initWithContentRect:NSMakeRect(0, 0, 320, 200)
                      styleMask:NSWindowStyleMaskTitled
                        backing:NSBackingStoreBuffered
                          defer:NO];
        window.releasedWhenClosed = NO;
        closingView = [[VivaMetalView alloc] initWithFrame:NSMakeRect(0, 0, 320, 200)];
        [closingView setValue:@1 forKey:@"_callbackHandle"];
        [closingView setValue:@YES forKey:@"_pointerEventsEnabled"];
        [window setContentView:closingView];
        VivaAppDelegate *delegate = [[VivaAppDelegate alloc] init];
        [delegate setValue:window forKey:@"_window"];
        [delegate setValue:@1 forKey:@"_callbackHandle"];
        [delegate setValue:@YES forKey:@"_running"];
        [window release];

        NSNotification *closing = [NSNotification notificationWithName:NSWindowWillCloseNotification object:window];
        [closingView mouseExited:exitEvent];
        assert(pointerCalls == 1);
        [delegate windowWillClose:closing];
        assert(closedCalls == 1);
        [closingView mouseExited:exitEvent];
        assert(pointerCalls == 1);
        [delegate windowWillClose:closing];
        assert(closedCalls == 1);

        // Detaching this window must not silence another window's input.
        VivaMetalView *liveView = [[VivaMetalView alloc] initWithFrame:NSMakeRect(0, 0, 320, 200)];
        [liveView setValue:@2 forKey:@"_callbackHandle"];
        [liveView setValue:@YES forKey:@"_pointerEventsEnabled"];
        [liveView mouseExited:exitEvent];
        assert(pointerCalls == 2);
        [liveView release];
        [delegate release];
        [closingView release];
        printf("window close: late view callbacks detached; other window input remains live\n");
    }
    return 0;
}
