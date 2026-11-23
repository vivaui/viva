#include "../core_text.m"
#include "../core_render.m"
#include "../native.m"
#include "../main_menu_darwin.m"
#include "window_test_callbacks.h"
#include <assert.h>

static unsigned downs, ups;
void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {
    assert(handle == 1);
    if (kind == VIVA_POINTER_DOWN) downs++;
    if (kind == VIVA_POINTER_UP) ups++;
}

static void dispatchEvents(NSTimeInterval seconds) {
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:seconds];
    while (deadline.timeIntervalSinceNow > 0) {
        NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:deadline
                                               inMode:NSDefaultRunLoopMode dequeue:YES];
        if (event != nil) [NSApp sendEvent:event];
    }
}

// This runs on a private CI desktop. The native window and event dispatch are
// real. No accessibility permission or system-wide mouse injection is needed.
int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        [NSApp finishLaunching];
        NSWindow *owner = [[NSWindow alloc] initWithContentRect:NSMakeRect(100, 100, 240, 120)
                                                     styleMask:NSWindowStyleMaskTitled
                                                       backing:NSBackingStoreBuffered defer:NO];
        [NSApp activateIgnoringOtherApps:YES];
        [owner makeKeyAndOrderFront:nil];
        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:3];
        while (NSApp.keyWindow != owner && deadline.timeIntervalSinceNow > 0) {
            dispatchEvents(0.05);
        }
        NSWindow *key = NSApp.keyWindow;
        pid_t foreground = NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier;
        // WindowServer may keep a different application in front of a test
        // executable. In that case verify that application's focus as well.
        assert(key == owner || (foreground > 0 && foreground != NSProcessInfo.processInfo.processIdentifier));

        viva_window_config config = {.title = "Viva notification", .width = 180, .height = 80};
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        id<MTLCommandQueue> queue = [device newCommandQueue];
        VivaAppDelegate *popup = [[VivaAppDelegate alloc] initWithConfig:config device:device
            commandQueue:queue rectanglePipeline:nil rectangleBuffer:nil rectangleCount:0 rectangleClips:NULL
            textPipeline:nil textLabels:NULL textLabelCount:0 accessibility:NULL accessibilityCount:0
            textInput:(viva_text_input_config){0} dynamicTextInputEnabled:NO pointerEventsEnabled:YES
            resizeEventsEnabled:NO keyEventsEnabled:NO scrollEventsEnabled:NO windowMode:VIVA_WINDOW_NOTIFICATION
            closeInterceptEnabled:NO callbackHandle:1];
        [popup applicationDidFinishLaunching:[NSNotification notificationWithName:NSApplicationDidFinishLaunchingNotification object:NSApp]];
        NSWindow *window = [popup valueForKey:@"window"];
        dispatchEvents(0.1);
        assert(window.isVisible && NSApp.keyWindow == key);
        assert(NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier == foreground);
        assert(!window.canBecomeKeyWindow && !window.canBecomeMainWindow && !window.ignoresMouseEvents);
        // The popup is cut like a panel, so its content aligns with the system's rounding.
        assert(window.contentView.layer.cornerRadius == VIVA_PANEL_CORNER_RADIUS);
        assert(window.level > NSNormalWindowLevel);

        for (NSNumber *type in @[@(NSEventTypeLeftMouseDown), @(NSEventTypeLeftMouseUp)]) {
            NSEvent *event = [NSEvent mouseEventWithType:(NSEventType)type.unsignedIntegerValue
                location:NSMakePoint(30, 20) modifierFlags:0 timestamp:NSProcessInfo.processInfo.systemUptime
                windowNumber:window.windowNumber context:nil eventNumber:1 clickCount:1 pressure:1];
            assert([window.contentView acceptsFirstMouse:event]);
            [window sendEvent:event];
        }
        dispatchEvents(0.1);
        assert(downs == 1 && ups == 1 && NSApp.keyWindow == key);
        assert(NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier == foreground);
        [popup hideWindow];
        assert(!window.isVisible);
        [popup showWindow];
        dispatchEvents(0.1);
        assert(window.isVisible && NSApp.keyWindow == key);
        assert(NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier == foreground);
        [popup hideWindow];
        [owner orderOut:nil];
        [popup release];
        [queue release];
        [device release];
        [owner release];
        printf("AppKit notification received the click and preserved the %s focus on open and show\n",
               key == owner ? "owner window's" : "other application's");
    }
    return 0;
}
