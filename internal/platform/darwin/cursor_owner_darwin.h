#ifndef VIVA_CURSOR_OWNER_DARWIN_H
#define VIVA_CURSOR_OWNER_DARWIN_H

#import <AppKit/AppKit.h>

// 原生子视图通过此标记接管光标，不把具体网页类反向耦合到画布或系统光标实现。
@protocol VivaNativeCursorOwner <NSObject>
- (BOOL)vivaManagesCursor;
@end

// 只取鼠标下最前面的应用窗口，不让后方或已经关闭的宿主改变前台窗口的光标。
static inline NSWindow *viva_window_at_pointer(void) {
    NSInteger number = [NSWindow windowNumberAtPoint:NSEvent.mouseLocation belowWindowWithWindowNumber:0];
    return [NSApp windowWithWindowNumber:number];
}

// point 使用 root 的逻辑坐标。先实际命中测试，再向祖先查找所有者：
// 隐藏页面、原生遮罩切口及圆角外侧会落回画布，上方浮层也不会被网页矩形误判。
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
