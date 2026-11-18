#import "save_panel_darwin.h"

void viva_configure_save_panel(NSSavePanel *panel, NSString *name, NSArray<NSString *> *allowed) {
    if (allowed.count > 0) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        panel.allowedFileTypes = allowed;
#pragma clang diagnostic pop
        // With other types disallowed, AppKit rewrites a name whose extension is not
        // in allowed (export.log -> export.log.txt). Keep such a name intact, as Linux
        // and Windows do; an extensionless name still takes the first allowed type.
        panel.allowsOtherFileTypes = name.pathExtension.length > 0;
        panel.extensionHidden = NO;
    }
    if (name.length > 0) {
        panel.nameFieldStringValue = name;
    }
}
