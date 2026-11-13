#ifndef VIVA_CURSOR_OWNER_DARWIN_H
#define VIVA_CURSOR_OWNER_DARWIN_H

#import <AppKit/AppKit.h>

// Native subviews use this marker to own the cursor without coupling the canvas to WebKit.
@protocol VivaNativeCursorOwner <NSObject>
- (BOOL)vivaManagesCursor;
@end

// Use the frontmost application window under the pointer so obscured hosts cannot override it.
static inline NSWindow *viva_window_at_pointer(void) {
    NSInteger number = [NSWindow windowNumberAtPoint:NSEvent.mouseLocation belowWindowWithWindowNumber:0];
    return [NSApp windowWithWindowNumber:number];
}

// point is in the root view's logical coordinates. Hit test before walking the ancestors:
// hidden pages, mask cuts, rounded corners and overlays must defer to the actual hit view.
static inline BOOL viva_view_has_native_cursor(NSView *root, NSPoint point) {
    if (root == nil || !NSPointInRect(point, root.bounds)) {
        return NO;
    }
    NSPoint parentPoint = [root convertPoint:point toView:root.superview];
    for (NSView *hit = [root hitTest:parentPoint]; hit != nil && hit != root; hit = hit.superview) {
        if ([hit respondsToSelector:@selector(vivaManagesCursor)] &&
            [(id<VivaNativeCursorOwner>)hit vivaManagesCursor]) {
            return YES;
        }
    }
    return NO;
}

#endif
