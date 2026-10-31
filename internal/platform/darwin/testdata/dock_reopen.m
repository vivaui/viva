#include "../native.m"
#include <assert.h>

void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {}
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
int vivaFileDropEvent(uintptr_t handle, void *view, int phase, double x, double y, const char *paths, size_t length) { return 0; }
void vivaScrollEvent(uintptr_t handle, void *view, double deltaX, double deltaY, double x, double y, int modifiers, int precise) {}
void vivaCloseRequested(uintptr_t handle, void *view) {}
void vivaWindowClosed(uintptr_t handle) {}
void vivaFrameChanged(uintptr_t handle, void *view, double x, double y, double width, double height) {}
void vivaWindowMinimized(uintptr_t handle, void *view, int minimized) {}
uint16_t vivaRectangleLayer(uintptr_t handle, size_t index) { return 0; }
int vivaPressAt(uintptr_t handle, double x, double y) { return 0; }
void vivaTextInput(uintptr_t handle, void *view, int kind, const unsigned char *utf8,
                  size_t length, int64_t start, int64_t selectedLength,
                  int64_t replacementStart, int64_t replacementLength, int command) {}

// Exercise the native delegate without showing windows or activating the test.
@interface DockTestApplication : NSApplication
@end
@implementation DockTestApplication
- (void)activateIgnoringOtherApps:(BOOL)flag {}
@end

@interface DockTestWindow : NSWindow {
    BOOL _testVisible;
    BOOL _testMiniaturized;
}
@end
@implementation DockTestWindow
- (BOOL)isVisible { return _testVisible; }
- (BOOL)isMiniaturized { return _testMiniaturized; }
- (void)miniaturize:(id)sender { _testVisible = NO; _testMiniaturized = YES; }
- (void)deminiaturize:(id)sender { _testVisible = YES; _testMiniaturized = NO; }
- (void)makeKeyAndOrderFront:(id)sender { _testVisible = YES; }
- (void)orderOut:(id)sender { _testVisible = NO; }
@end

static int reopenRequests;
static BOOL hasReopenHandler = YES;
static int applicationReopenRequests;
static BOOL hasApplicationReopenHandler;

int vivaApplicationReopenRequested(void) {
    if (!hasApplicationReopenHandler) {
        return 0;
    }
    applicationReopenRequests++;
    return 1;
}

int vivaWindowReopenRequested(uintptr_t handle, void *view) {
    assert(handle == 1);
    reopenRequests++;
    return hasReopenHandler;
}

static VivaAppDelegate *windowDelegate(int mode, uintptr_t handle) {
    DockTestWindow *window = [[DockTestWindow alloc]
        initWithContentRect:NSMakeRect(0, 0, 320, 200)
                  styleMask:NSWindowStyleMaskTitled
                    backing:NSBackingStoreBuffered
                      defer:NO];
    VivaAppDelegate *delegate = [[VivaAppDelegate alloc] init];
    [delegate setValue:window forKey:@"_window"];
    [delegate setValue:@YES forKey:@"_running"];
    [delegate setValue:@(mode) forKey:@"_windowMode"];
    [delegate setValue:@(handle) forKey:@"_callbackHandle"];
    [window release];
    return delegate;
}

int main(void) {
    @autoreleasepool {
        [DockTestApplication sharedApplication];
        NSWindow *frameless = [[VivaFramelessWindow alloc]
            initWithContentRect:NSMakeRect(0, 0, 320, 200)
                      styleMask:NSWindowStyleMaskBorderless
                        backing:NSBackingStoreBuffered
                          defer:NO];
        assert(frameless.canBecomeKeyWindow && frameless.canBecomeMainWindow);
        [frameless release];
        VivaAppDelegate *mainWindow = windowDelegate(VIVA_WINDOW_TITLED, 1);
        VivaAppDelegate *panel = windowDelegate(VIVA_WINDOW_PANEL, 2);
        VivaAppDelegate *visible = windowDelegate(VIVA_WINDOW_TITLED, 3);
        [visible showWindow];
        viva_application_windows = [[NSMutableArray alloc]
            initWithObjects:mainWindow, panel, visible, nil];
        VivaApplicationDelegate *application = [[VivaApplicationDelegate alloc] init];

        // The app chooses its workspace even when settings is already visible.
        // Do not also restore hidden auxiliary windows after a handled request.
        hasApplicationReopenHandler = YES;
        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:YES];
        assert(applicationReopenRequests == 1 && reopenRequests == 0);
        assert([mainWindow isHidden] && [panel isHidden] && ![visible isHidden]);
        [mainWindow showWindow];
        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:YES];
        assert(applicationReopenRequests == 2 && reopenRequests == 0);
        [mainWindow hideWindow];
        [visible hideWindow];
        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:NO];
        assert(applicationReopenRequests == 3 && reopenRequests == 0);
        assert([mainWindow isHidden] && [visible isHidden]);
        hasApplicationReopenHandler = NO;
        [visible showWindow];

        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:YES];
        assert(reopenRequests == 1);
        // The owner queues Window.Show; the delegate must not also show it.
        assert([mainWindow isHidden]);
        assert([panel isHidden]);
        assert(![visible isHidden]);
        [mainWindow showWindow];
        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:NO];
        assert(reopenRequests == 1);
        [mainWindow hideWindow];

        // Single-window runners have no owning app.Window and need the fallback.
        hasReopenHandler = NO;
        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:NO];
        assert(reopenRequests == 2);
        assert(![mainWindow isHidden]);

        [mainWindow hideWindow];
        [mainWindow setValue:@0 forKey:@"_callbackHandle"];
        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:NO];
        assert(reopenRequests == 2);
        assert(![mainWindow isHidden]);

        [mainWindow hideWindow];
        [mainWindow setValue:@NO forKey:@"_running"];
        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:NO];
        assert(reopenRequests == 2);
        assert([mainWindow isHidden]);

        // Miniaturized windows must be reopened just like hidden windows.
        hasReopenHandler = YES;
        VivaAppDelegate *minWindow = windowDelegate(VIVA_WINDOW_TITLED, 1);
        DockTestWindow *rawWin = (DockTestWindow *)[minWindow valueForKey:@"_window"];
        [rawWin miniaturize:nil];
        assert([minWindow isMiniaturized]);
        assert(![minWindow isHidden]);
        [viva_application_windows addObject:minWindow];
        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:YES];
        assert(reopenRequests == 3);
        [viva_application_windows removeObject:minWindow];
        [minWindow release];

        // Miniaturized windows without an owner handler use native fallback deminiaturization.
        hasReopenHandler = NO;
        VivaAppDelegate *fallbackMinWindow = windowDelegate(VIVA_WINDOW_TITLED, 1);
        DockTestWindow *fallbackRawWin = (DockTestWindow *)[fallbackMinWindow valueForKey:@"_window"];
        [fallbackRawWin miniaturize:nil];
        assert([fallbackMinWindow isMiniaturized]);
        [viva_application_windows addObject:fallbackMinWindow];
        [application applicationShouldHandleReopen:NSApp hasVisibleWindows:YES];
        assert(reopenRequests == 4);
        assert(![fallbackMinWindow isMiniaturized]);
        [viva_application_windows removeObject:fallbackMinWindow];
        [fallbackMinWindow release];

        [viva_application_windows release];
        viva_application_windows = nil;
        [application release];
        [mainWindow release];
        [panel release];
        [visible release];
        printf("Dock reopen: application routing, visible/panel filtering and native fallback ok\n");
    }
    return 0;
}
