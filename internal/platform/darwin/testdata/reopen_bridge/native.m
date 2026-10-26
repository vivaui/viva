#import <Cocoa/Cocoa.h>
#import <Metal/Metal.h>

extern int reopenBridgeStep(int visible);

int reopen_bridge_available(void) {
    @autoreleasepool {
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        BOOL available = device != nil;
        [device release];
        return available;
    }
}

void reopen_bridge_start(void) {
    [NSTimer scheduledTimerWithTimeInterval:0.01 repeats:YES block:^(NSTimer *timer) {
        NSWindow *window = nil;
        for (NSWindow *candidate in NSApp.windows) {
            if ([candidate.title isEqualToString:@"Viva reopen bridge regression"]) {
                window = candidate;
                break;
            }
        }
        switch (reopenBridgeStep(window.isVisible)) {
            case 1:
                [[window standardWindowButton:NSWindowCloseButton] performClick:nil];
                break;
            case 2:
                [NSApp.delegate applicationShouldHandleReopen:NSApp hasVisibleWindows:NO];
                break;
            case 3:
                [timer invalidate];
                break;
        }
    }];
}
