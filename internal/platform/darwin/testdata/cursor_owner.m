#import <AppKit/AppKit.h>
#include <stdio.h>
#include "cursor_owner_darwin.h"

@interface CursorTestRoot : NSView
@end
@implementation CursorTestRoot
- (BOOL)isFlipped { return YES; }
@end

@interface CursorTestPage : CursorTestRoot <VivaNativeCursorOwner> {
@public
    NSRect cut;
}
@end
@implementation CursorTestPage
- (BOOL)vivaManagesCursor { return YES; }
- (NSView *)hitTest:(NSPoint)point {
    NSPoint local = [self convertPoint:point fromView:self.superview];
    if (NSPointInRect(local, cut)) { return nil; }
    return [super hitTest:point];
}
@end

static int check(BOOL actual, BOOL expected, const char *scenario) {
    if (actual == expected) { return 0; }
    fprintf(stderr, "cursor owner mismatch: %s\n", scenario);
    return 1;
}

// 只构造 AppKit 视图树，不创建窗口、WebView、GPU 或屏幕截图。
int main(void) {
    @autoreleasepool {
        int failures = 0;
        NSView *parent = [[CursorTestRoot alloc] initWithFrame:NSMakeRect(0, 0, 240, 200)];
        NSView *root = [[CursorTestRoot alloc] initWithFrame:NSMakeRect(7, 11, 200, 160)];
        [parent addSubview:root];
        NSPoint pagePoint = NSMakePoint(40, 40);
        failures += check(viva_view_has_native_cursor(root, pagePoint), NO, "canvas without native page");
        CursorTestPage *page = [[CursorTestPage alloc] initWithFrame:NSMakeRect(20, 20, 100, 80)];
        [root addSubview:page];
        NSView *content = [[NSView alloc] initWithFrame:NSMakeRect(10, 10, 50, 40)];
        [page addSubview:content];
        failures += check(viva_view_has_native_cursor(root, pagePoint), YES, "nested native content with offset root");
        failures += check(viva_view_has_native_cursor(root, NSMakePoint(5, 5)), NO, "canvas beside page");
        failures += check(viva_view_has_native_cursor(root, NSMakePoint(-1, 40)), NO, "outside host");
        failures += check(viva_view_has_native_cursor(nil, pagePoint), NO, "released host");
        page.hidden = YES;
        failures += check(viva_view_has_native_cursor(root, pagePoint), NO, "hidden native page");
        page.hidden = NO;
        NSView *menu = [[NSView alloc] initWithFrame:NSMakeRect(35, 35, 25, 25)];
        [root addSubview:menu positioned:NSWindowAbove relativeTo:nil];
        failures += check(viva_view_has_native_cursor(root, pagePoint), NO, "native canvas popup above page");
        menu.hidden = YES;
        failures += check(viva_view_has_native_cursor(root, pagePoint), YES, "popup dismissed");
        page->cut = NSMakeRect(0, 0, 30, 30);
        failures += check(viva_view_has_native_cursor(root, pagePoint), NO, "masked page corner or canvas menu cut");
        failures += check(viva_view_has_native_cursor(root, NSMakePoint(80, 60)), YES, "page beside mask cut");
        [page removeFromSuperview];
        failures += check(viva_view_has_native_cursor(root, pagePoint), NO, "page removed after close");
        [menu release]; [content release]; [page release]; [root release]; [parent release];
        return failures != 0;
    }
}
