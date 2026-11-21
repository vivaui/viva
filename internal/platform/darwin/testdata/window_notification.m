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
        while (!owner.isKeyWindow && deadline.timeIntervalSinceNow > 0) {
            [[NSRunLoop currentRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.01]];
        }
        assert(NSApp.keyWindow == owner);

        viva_window_config config = {.title = "Viva notification", .width = 180, .height = 80};
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        id<MTLCommandQueue> queue = [device newCommandQueue];
        VivaAppDelegate *popup = [[VivaAppDelegate alloc] initWithConfig:config device:device
            commandQueue:queue rectanglePipeline:nil rectangleBuffer:nil rectangleCount:0 rectangleClips:NULL
            textPipeline:nil textLabels:NULL textLabelCount:0 accessibility:NULL accessibilityCount:0
            textInput:(viva_text_input_config){0} dynamicTextInputEnabled:NO pointerEventsEnabled:YES
            resizeEventsEnabled:NO keyEventsEnabled:NO scrollEventsEnabled:NO windowMode:VIVA_WINDOW_NOTIFICATION
            closeInterceptEnabled:NO callbackHandle:1];
        [popup applicationDidFinishLaunching:nil];
        NSWindow *window = [popup valueForKey:@"window"];
        assert(window.isVisible && NSApp.keyWindow == owner);
        assert(!window.canBecomeKeyWindow && !window.canBecomeMainWindow && !window.ignoresMouseEvents);
        assert(window.level > NSNormalWindowLevel);

        for (NSNumber *type in @[@(NSEventTypeLeftMouseDown), @(NSEventTypeLeftMouseUp)]) {
            NSEvent *event = [NSEvent mouseEventWithType:(NSEventType)type.unsignedIntegerValue
                location:NSMakePoint(30, 20) modifierFlags:0 timestamp:NSProcessInfo.processInfo.systemUptime
                windowNumber:window.windowNumber context:nil eventNumber:1 clickCount:1 pressure:1];
            assert([window.contentView acceptsFirstMouse:event]);
            [window sendEvent:event];
        }
        assert(downs == 1 && ups == 1 && NSApp.keyWindow == owner);
        [popup hideWindow];
        assert(!window.isVisible);
        [popup showWindow];
        assert(window.isVisible && NSApp.keyWindow == owner);
        [popup hideWindow];
        [owner orderOut:nil];
        [popup release];
        [queue release];
        [device release];
        [owner release];
        puts("AppKit notification received the click and preserved the owner's keyboard focus on open and show");
    }
    return 0;
}
