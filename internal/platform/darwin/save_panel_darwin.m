#import "save_panel_darwin.h"

void viva_configure_save_panel(NSSavePanel *panel, NSString *name, NSArray<NSString *> *allowed) {
    if (allowed.count > 0) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        panel.allowedFileTypes = allowed;
#pragma clang diagnostic pop
        panel.allowsOtherFileTypes = NO;
        panel.extensionHidden = NO;
    }
    if (name.length > 0) {
        panel.nameFieldStringValue = name;
    }
}
