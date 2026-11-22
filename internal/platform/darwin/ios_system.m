//go:build ios

// What the system gives an iPhone application: the pasteboard, links, the user's
// languages and the look they chose.

#import <UIKit/UIKit.h>
#include <stdlib.h>
#include <string.h>

#include "ios_system.h"

// onMain runs work on the main thread, where UIKit may be used, and waits for it.
static void viva_on_main(dispatch_block_t work) {
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
}

static char *viva_copy(NSString *value) {
    const char *utf8 = value.UTF8String;
    return utf8 == NULL ? NULL : strdup(utf8);
}

char *viva_ios_clipboard_text(void) {
    __block char *result = NULL;
    viva_on_main(^{
        // hasStrings does not read the contents, so asking is free of the paste prompt.
        if (UIPasteboard.generalPasteboard.hasStrings) {
            result = viva_copy(UIPasteboard.generalPasteboard.string);
        }
    });
    return result;
}

void viva_ios_set_clipboard_text(const char *text) {
    NSString *value = [NSString stringWithUTF8String:text];
    viva_on_main(^{
        UIPasteboard.generalPasteboard.string = value;
    });
}

int viva_ios_open_url(const char *text) {
    NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:text]];
    if (url == nil) {
        return 0;
    }
    __block int opened = 0;
    viva_on_main(^{
        if ([UIApplication.sharedApplication canOpenURL:url] || [url.scheme hasPrefix:@"http"]) {
            [UIApplication.sharedApplication openURL:url options:@{} completionHandler:nil];
            opened = 1;
        }
    });
    return opened;
}

char *viva_ios_preferred_languages(void) {
    return viva_copy([[NSLocale preferredLanguages] componentsJoinedByString:@"\n"]);
}

int viva_ios_reduce_motion(void) {
    __block int reduced = 0;
    viva_on_main(^{
        reduced = UIAccessibilityIsReduceMotionEnabled() ? 1 : 0;
    });
    return reduced;
}

int viva_ios_dark_theme(void) {
    __block int dark = -1;
    viva_on_main(^{
        switch (UIScreen.mainScreen.traitCollection.userInterfaceStyle) {
        case UIUserInterfaceStyleDark:
            dark = 1;
            break;
        case UIUserInterfaceStyleLight:
            dark = 0;
            break;
        default:
            break;
        }
    });
    return dark;
}
