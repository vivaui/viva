#ifndef VIVA_SAVE_PANEL_DARWIN_H
#define VIVA_SAVE_PANEL_DARWIN_H

#import <AppKit/AppKit.h>

/* Sets the allowed types, then the suggested name, on a save panel. Main thread only. */
void viva_configure_save_panel(NSSavePanel *panel, NSString *name, NSArray<NSString *> *allowed);

#endif
