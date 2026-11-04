/* The Dock tile's progress bar, the way viva_application_set_progress draws it:
 * a fraction puts a view over the tile that draws the icon again with a bar
 * along its foot, a second fraction reuses that view, and a negative or
 * undefined fraction leaves the view drawing the icon alone. That last part is
 * the one worth guarding: taking the view away would hand the tile back to the
 * system, which draws the icon of a program in an .app bundle but the plain
 * executable of one run out of a build directory, so the icon would change the
 * moment a transfer ended. Drawing the view once into an offscreen bitmap is
 * what proves it paints at all. */

#import <Cocoa/Cocoa.h>

#include "../dock_progress_darwin.h"

static int failures;

static NSView *tileView(void) { return [[NSApp dockTile] contentView]; }

static double viewFraction(NSView *view) {
    SEL selector = NSSelectorFromString(@"fraction");
    if (view == nil || ![view respondsToSelector:selector]) {
        return -2;
    }
    double (*read)(id, SEL) = (double (*)(id, SEL))[view methodForSelector:selector];
    return read(view, selector);
}

/* expectTile asks that the tile be drawn by our view with the given fraction,
 * and hands back the view so a later call can ask that it be the same one. */
static NSView *expectTile(double fraction, const char *what) {
    NSView *view = tileView();
    if (view == nil) {
        fprintf(stderr, "%s: no content view on the tile\n", what);
        failures++;
        return nil;
    }
    if (![[view className] isEqualToString:@"VivaDockProgressView"]) {
        fprintf(stderr, "%s: content view is %s\n", what, [[view className] UTF8String]);
        failures++;
        return view;
    }
    if (viewFraction(view) != fraction) {
        fprintf(stderr, "%s: fraction = %f, want %f\n", what, viewFraction(view), fraction);
        failures++;
    }
    return view;
}

/* draw paints the view into a bitmap and asks that it left more than one
 * colour there: a bar that draws nothing, or a view that draws nothing at all,
 * fails here. */
static void expectDraws(const char *what) {
    NSView *view = tileView();
    if (view == nil) {
        fprintf(stderr, "%s: no content view on the tile\n", what);
        failures++;
        return;
    }
    NSBitmapImageRep *bitmap = [[NSBitmapImageRep alloc] initWithBitmapDataPlanes:NULL
                                                                       pixelsWide:128
                                                                       pixelsHigh:128
                                                                    bitsPerSample:8
                                                                  samplesPerPixel:4
                                                                         hasAlpha:YES
                                                                         isPlanar:NO
                                                                   colorSpaceName:NSCalibratedRGBColorSpace
                                                                      bytesPerRow:0
                                                                     bitsPerPixel:0];
    NSGraphicsContext *context = [NSGraphicsContext graphicsContextWithBitmapImageRep:bitmap];
    [NSGraphicsContext saveGraphicsState];
    [NSGraphicsContext setCurrentContext:context];
    [view drawRect:[view bounds]];
    [NSGraphicsContext restoreGraphicsState];
    NSInteger different = 0;
    NSColor *first = [bitmap colorAtX:0 y:0];
    for (NSInteger y = 0; y < 128 && different < 2; y += 4) {
        for (NSInteger x = 0; x < 128 && different < 2; x += 4) {
            NSColor *colour = [bitmap colorAtX:x y:y];
            if (colour != nil && first != nil && ![colour isEqual:first]) {
                different++;
            }
        }
    }
    if (different < 1) {
        fprintf(stderr, "%s: the view left the bitmap a single colour\n", what);
        failures++;
    }
    [bitmap release];
}

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        if ([NSApp dockTile] == nil) {
            fprintf(stderr, "the application has no Dock tile\n");
            return 1;
        }
        // A program that never reports progress keeps the tile the system gave
        // it: no view is installed until there is a bar to draw.
        if (tileView() != nil) {
            fprintf(stderr, "before any progress: the tile already has a content view\n");
            failures++;
        }

        viva_application_set_progress(0.5);
        NSView *first = expectTile(0.5, "half");
        expectDraws("half");

        // A second fraction reuses the view rather than replacing it: replacing
        // it every chunk of a transfer would make the tile flicker.
        viva_application_set_progress(0.25);
        if (expectTile(0.25, "a quarter") != first) {
            fprintf(stderr, "a second fraction made a new view\n");
            failures++;
        }
        viva_application_set_progress(1.0);
        expectTile(1.0, "all of it");

        // Clearing keeps the view and asks it for the icon alone.
        viva_application_set_progress(-1.0);
        if (expectTile(-1.0, "negative leaves the icon") != first) {
            fprintf(stderr, "clearing replaced the view\n");
            failures++;
        }
        expectDraws("the icon alone");

        viva_application_set_progress(0.5);
        if (expectTile(0.5, "set again") != first) {
            fprintf(stderr, "setting again after a clear made a new view\n");
            failures++;
        }
        viva_application_set_progress(NAN);
        expectTile(-1.0, "NaN leaves the icon");
        expectDraws("the icon alone after NaN");
    }
    return failures == 0 ? 0 : 1;
}
