#include "../core_text.m"
#include "../core_render.m"
#include "../watermark_darwin.m"
#include "../native.m"
#include "../main_menu_darwin.m"
#include <assert.h>

void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {}
#include "window_test_callbacks.h"

// CI can exercise the AppKit transition without opening a desktop window.
// The same fixture can be run with --visible to observe a key window locally.
@interface DecorationWindow : VivaWorkspaceWindow
@property BOOL reportsMapped;
@end

@implementation DecorationWindow
- (BOOL)isVisible { return self.reportsMapped || [super isVisible]; }
@end

static NSRect contentRect(NSWindow *window) {
    return [window contentRectForFrameRect:window.frame];
}

static void pumpEvents(void) {
    NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny
        untilDate:[NSDate dateWithTimeIntervalSinceNow:0.04] inMode:NSDefaultRunLoopMode dequeue:YES];
    if (event != nil) [NSApp sendEvent:event];
    [NSApp updateWindows];
}

static void verify(NSWindow *window, NSInteger number, NSView *content, NSResponder *responder,
                   NSRect geometry, NSWindowStyleMask style, NSInteger level, BOOL key) {
    assert(window.windowNumber == number);
    assert(window.contentView == content && window.firstResponder == responder);
    assert(NSEqualRects(contentRect(window), geometry));
    assert(window.styleMask == style && window.level == level);
    assert(window.isVisible && window.isKeyWindow == key);
}

static void check(BOOL custom, BOOL visible, NSRect area, CGFloat width, CGFloat height) {
    NSWindowStyleMask style = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
        NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable;
    if (custom) style |= NSWindowStyleMaskFullSizeContentView;
    NSRect geometry = NSMakeRect(NSMidX(area) - width / 2, NSMidY(area) - height / 2, width, height);
    DecorationWindow *window = [[DecorationWindow alloc]
        initWithContentRect:geometry
                  styleMask:style backing:NSBackingStoreBuffered defer:NO];
    window.releasedWhenClosed = NO;
    NSInteger level = custom ? NSFloatingWindowLevel : NSNormalWindowLevel;
    window.level = level;
    window.titlebarAppearsTransparent = custom;
    window.titleVisibility = custom ? NSWindowTitleHidden : NSWindowTitleVisible;
    NSView *content = window.contentView;
    NSTextView *editor = [[NSTextView alloc] initWithFrame:NSMakeRect(8, 8, 180, 100)];
    editor.string = @"alpha bravo";
    [editor setSelectedRange:NSMakeRange(6, 5)];
    [content addSubview:editor];
    assert([window makeFirstResponder:editor]);

    VivaAppDelegate *delegate = [VivaAppDelegate alloc];
    [delegate setValue:window forKey:@"_window"];
    [delegate setValue:@YES forKey:@"_running"];
    [delegate setValue:@(custom ? VIVA_WINDOW_CUSTOM_TITLEBAR : VIVA_WINDOW_TITLED) forKey:@"_windowMode"];
    assert([delegate changeDecoration:NO] == 0);
    assert(window.styleMask == style);

    if (visible) {
        [window makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:2.0];
        do {
            pumpEvents();
        } while (!window.isKeyWindow && [deadline timeIntervalSinceNow] > 0);
        assert(window.isKeyWindow);
    } else {
        window.reportsMapped = YES;
    }
    NSInteger number = window.windowNumber;
    geometry = contentRect(window);
    // The operation may legitimately refuse if AppKit constrains the new frame
    // to the display. Keep the success fixture inside the available screen.
    assert(NSContainsRect(area, window.frame));
    [delegate setValue:@YES forKey:@"_fullscreenTransition"];
    assert([delegate changeDecoration:NO] == -1);
    verify(window, number, content, editor, geometry, style, level, visible);
    [delegate setValue:@NO forKey:@"_fullscreenTransition"];
    int result = [delegate changeDecoration:NO];
    if (result != 1) {
        fprintf(stderr, "could not remove chrome: result=%d custom=%d frame=%s client=%s screen=%s\n",
            result, custom, NSStringFromRect(window.frame).UTF8String,
            NSStringFromRect(geometry).UTF8String, NSStringFromRect(area).UTF8String);
    }
    assert(result == 1);
    if (visible) pumpEvents();
    verify(window, number, content, editor, geometry,
           style & ~(NSWindowStyleMaskTitled | NSWindowStyleMaskFullSizeContentView), level, visible);
    assert(NSEqualRanges(editor.selectedRange, NSMakeRange(6, 5)));
    assert([delegate changeDecoration:YES] == 1);
    if (visible) pumpEvents();
    verify(window, number, content, editor, geometry, style, level, visible);
    assert(window.titlebarAppearsTransparent == custom);
    assert(window.titleVisibility == (custom ? NSWindowTitleHidden : NSWindowTitleVisible));
    if (visible) {
        NSEvent *event = [NSEvent keyEventWithType:NSEventTypeKeyDown location:NSZeroPoint
            modifierFlags:0 timestamp:NSProcessInfo.processInfo.systemUptime windowNumber:number
            context:nil characters:@"x" charactersIgnoringModifiers:@"x" isARepeat:NO keyCode:7];
        [window sendEvent:event];
    } else {
        [editor insertText:@"x" replacementRange:NSMakeRange(NSNotFound, 0)];
    }
    assert([editor.string isEqualToString:@"alpha x"]);
    printf("PASS %.0fx%.0f custom=%d visible=%d\n", width, height, custom, visible);
    [window close];
    [editor release];
    [delegate release];
    [window release];
}

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        BOOL visible = argc == 2 && strcmp(argv[1], "--visible") == 0;
        [NSApp setActivationPolicy:visible ? NSApplicationActivationPolicyRegular : NSApplicationActivationPolicyProhibited];
        if (visible) [NSApp finishLaunching];
        NSScreen *screen = NSScreen.mainScreen ?: NSScreen.screens.firstObject;
        assert(screen != nil);
        NSRect area = screen.visibleFrame;
        // Leave space for the titlebar even on smaller virtual displays.
        CGFloat width = MIN(NSWidth(area) - 64, 1280);
        CGFloat height = MIN(NSHeight(area) - 64, 800);
        check(NO, visible, area, MIN(width, 640), MIN(height, 400));
        check(YES, visible, area, MIN(width, 640), MIN(height, 400));
        check(NO, visible, area, width, height);
        check(YES, visible, area, width, height);
    }
    return 0;
}
