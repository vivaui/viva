#include "dock_progress_darwin.h"

#import <Cocoa/Cocoa.h>

/* The Dock draws a tile from its content view, and a tile without one is the
 * application's icon. A bar therefore replaces that icon with a view that draws
 * it again and puts the bar over its foot: the icon image is asked of the
 * application each time the view draws, so the tile keeps following the picture
 * the program was given (a bundle's, or WindowIcon's).
 *
 * The bar is a pill inset into the tile, outlined so the part still to fill
 * reads on a dark icon as well as a light one, with the system's accent colour
 * over as much of it as the fraction says. Drawing rather than an
 * NSProgressIndicator keeps the bar at any tile size without a subview to lay
 * out, and the Dock re-asks the view only when it is told the tile changed.
 *
 * The tile's badge label is drawn by the Dock over whatever the content view
 * draws, so a development build keeps its mark while a transfer runs. */
@interface VivaDockProgressView : NSView
@property (nonatomic) double fraction;
@end

@implementation VivaDockProgressView

- (BOOL)isOpaque { return NO; }

- (void)drawRect:(NSRect)dirtyRect {
    NSRect bounds = [self bounds];
    NSImage *icon = [NSApp applicationIconImage];
    if (icon != nil) {
        [icon drawInRect:bounds
                fromRect:NSZeroRect
               operation:NSCompositingOperationSourceOver
                fraction:1.0
          respectFlipped:YES
                   hints:nil];
    }
    // A tile the program is not working on is the icon and nothing else: the
    // view stays on the tile once a bar has been asked for (see below), so this
    // is what the Dock draws between transfers.
    if (self.fraction < 0) {
        return;
    }
    // The bar is a pill inset into the tile rather than a strip across its
    // foot: a margin wide enough to leave the icon's rounded corners showing,
    // and a step above the bottom edge so the icon's own outline still reads
    // as a rounded square instead of one the bar had sliced flat.
    CGFloat margin = NSWidth(bounds) * 0.12;
    CGFloat height = MAX(NSWidth(bounds) * 0.10, 4.0);
    NSRect bar = NSMakeRect(margin, NSWidth(bounds) * 0.14, NSWidth(bounds) - margin * 2, height);
    if (NSWidth(bar) <= 0) {
        return;
    }
    NSBezierPath *track = [NSBezierPath bezierPathWithRoundedRect:bar
                                                          xRadius:height / 2
                                                          yRadius:height / 2];
    // The track is dark so the bar reads on a light icon, and outlined so the
    // part of it still to fill reads on a dark one: without the outline the
    // track disappears into an icon that is nearly black and the bar looks like
    // a stub of colour that ends nowhere.
    [[NSColor colorWithCalibratedWhite:0.0 alpha:0.55] setFill];
    [track fill];
    CGFloat line = MAX(NSWidth(bounds) * 0.012, 1.0);
    [track setLineWidth:line];
    [[NSColor colorWithCalibratedWhite:1.0 alpha:0.35] setStroke];
    [track stroke];
    if (self.fraction <= 0) {
        return;
    }
    // The fill is clipped to the track so its ends stay round however little
    // of it is drawn.
    [NSGraphicsContext saveGraphicsState];
    [track addClip];
    NSRect fill = NSMakeRect(NSMinX(bar), NSMinY(bar), NSWidth(bar) * self.fraction, height);
    if (@available(macOS 10.14, *)) {
        [[NSColor controlAccentColor] setFill];
    } else {
        [[NSColor systemBlueColor] setFill];
    }
    NSRectFillUsingOperation(fill, NSCompositingOperationSourceOver);
    [NSGraphicsContext restoreGraphicsState];
}

@end

void viva_application_set_progress(double fraction) {
    @autoreleasepool {
        NSDockTile *tile = [NSApp dockTile];
        if (tile == nil) {
            return;
        }
        // A negative fraction, and one that is not a number, both mean there is
        // no bar to draw.
        BOOL running = fraction >= 0.0 && fraction <= 1.0;
        VivaDockProgressView *view = (VivaDockProgressView *)[tile contentView];
        if (![view isKindOfClass:[VivaDockProgressView class]]) {
            // Nothing to take away, and nothing to install for a program that
            // never reports progress: its tile stays the one the system draws.
            if (!running) {
                return;
            }
            NSSize size = [tile size];
            if (size.width <= 0 || size.height <= 0) {
                size = NSMakeSize(128, 128);
            }
            view = [[VivaDockProgressView alloc] initWithFrame:NSMakeRect(0, 0, size.width, size.height)];
            [view setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
            [tile setContentView:view];
            [view release];
        }
        // The view stays on the tile once it is there, drawing the icon alone
        // when there is no bar. Taking it away instead would hand the tile back
        // to the system, which draws the icon of an application in an .app
        // bundle but the plain executable of one run from a build directory:
        // the icon would change the moment a transfer ended.
        [view setFraction:running ? fraction : -1.0];
        [tile display];
    }
}
