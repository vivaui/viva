//go:build !ios

#include "dock_badge_darwin.h"

#import <Cocoa/Cocoa.h>

void viva_application_set_badge_label(const char *label) {
    @autoreleasepool {
        /* A NULL label, and an empty one, both take the badge away: AppKit
         * draws a badge for an empty string otherwise. */
        NSString *text = (label == NULL || label[0] == '\0') ? nil : [NSString stringWithUTF8String:label];
        [[NSApp dockTile] setBadgeLabel:text];
    }
}
