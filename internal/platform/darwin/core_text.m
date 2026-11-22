#import "core.h"
#include "label_geometry.h"

NSRange viva_clamp_range(NSRange range, NSUInteger length) {
    if (range.location == NSNotFound) {
        return NSMakeRange(length, 0);
    }
    NSUInteger start = MIN(range.location, length);
    NSUInteger remaining = length - start;
    return NSMakeRange(start, MIN(range.length, remaining));
}

// The few places the platforms' font and color classes differ. Everything else here
// is CoreText and CoreGraphics, which are the same on macOS and iOS.
#if TARGET_OS_OSX
static NSArray<NSString *> *viva_font_family_names(void) {
    return [[NSFontManager sharedFontManager] availableFontFamilies];
}

// viva_font_in_family is the face of an installed family nearest regular (or bold), or nil.
static VivaFont *viva_font_in_family(NSString *family, CGFloat size, BOOL bold) {
    return [[NSFontManager sharedFontManager] fontWithFamily:family
                                                      traits:(bold ? NSBoldFontMask : 0)
                                                      weight:5
                                                        size:size];
}

static VivaFont *viva_font_user_fixed_pitch(CGFloat size) {
    return [NSFont userFixedPitchFontOfSize:size];
}

BOOL viva_color_srgb(VivaColor *color, CGFloat rgba[4]) {
    NSColor *converted = [color colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
    rgba[0] = converted.redComponent;
    rgba[1] = converted.greenComponent;
    rgba[2] = converted.blueComponent;
    rgba[3] = converted.alphaComponent;
    return converted != nil;
}
#else
static NSArray<NSString *> *viva_font_family_names(void) {
    return [UIFont familyNames];
}

static VivaFont *viva_font_in_family(NSString *family, CGFloat size, BOOL bold) {
    // UIFont answers a family it does not have with the system font, so ask first.
    if (![[UIFont familyNames] containsObject:family]) {
        return nil;
    }
    UIFontDescriptor *descriptor = [UIFontDescriptor fontDescriptorWithFontAttributes:@{UIFontDescriptorFamilyAttribute: family}];
    if (bold) {
        UIFontDescriptor *heavier = [descriptor fontDescriptorWithSymbolicTraits:UIFontDescriptorTraitBold];
        if (heavier != nil) {
            descriptor = heavier;
        }
    }
    return [UIFont fontWithDescriptor:descriptor size:size];
}

static VivaFont *viva_font_user_fixed_pitch(CGFloat size) {
    VivaFont *menlo = [UIFont fontWithName:@"Menlo-Regular" size:size];
    return menlo != nil ? menlo : [UIFont monospacedSystemFontOfSize:size weight:UIFontWeightRegular];
}

BOOL viva_color_srgb(VivaColor *color, CGFloat rgba[4]) {
    CGFloat red = 0, green = 0, blue = 0, alpha = 0;
    BOOL resolved = [color getRed:&red green:&green blue:&blue alpha:&alpha];
    rgba[0] = red;
    rgba[1] = green;
    rgba[2] = blue;
    rgba[3] = alpha;
    return resolved;
}
#endif

// A line of proportional text is shaped once by CoreText, in the system font with its font
// fallback, and everything that needs a position asks that line: the width
// layout uses, the glyphs a label draws, the caret, the selection and the character a click
// lands on. The engine in Go (text_engine.go) caches the lines; vivaShapedLine is how the
// drawing here reaches the same cache.
extern viva_text_line *vivaShapedLine(const unsigned char *utf8, size_t length, double size, double weight);
extern viva_text_line *vivaShapedLineStyle(const unsigned char *utf8, size_t length, double size, double weight, int italic);

// viva_interface_family is the family viva_text_set_family last named, "" for the
// system font. A plain C buffer, not an NSString: nothing here manages memory for
// the interface font, and the name is read under the shaping the engine serializes.
static char viva_interface_family[256] = "";

// viva_system_font is the proportional font that measurement, labels and the editor all
// use (a retained CTFont). It is an VivaFont, which is a CTFont already. With a family
// named, its face of that family at the weight stands in for the system font; a family
// the system does not have falls back to the system font, as AppKit does for a name.
CTFontRef viva_system_font(CGFloat size, CGFloat weight) {
    if (viva_interface_family[0] != '\0') {
        NSString *family = [NSString stringWithUTF8String:viva_interface_family];
        if (family != nil) {
            // The weight is the system font's own continuous scale (NSFontWeight), so a
            // family's regular, medium, semibold and bold faces are told apart as they are
            // in the system font; the nearest face the family has stands in for one it
            // lacks. NSFontManager's weight is a 0-15 table that cannot say medium.
            VivaFontDescriptor *descriptor = [VivaFontDescriptor fontDescriptorWithFontAttributes:@{
                VivaFontFamilyAttribute: family,
                VivaFontTraitsAttribute: @{VivaFontWeightTrait: @(weight)},
            }];
            VivaFont *named = [VivaFont fontWithDescriptor:descriptor size:size];
            if (named != nil) {
                return (CTFontRef)CFRetain(named);
            }
        }
    }
    VivaFont *font = [VivaFont systemFontOfSize:size weight:weight];
    return font != nil ? (CTFontRef)CFRetain(font) : NULL;
}

CTFontRef viva_system_font_style(CGFloat size, CGFloat weight, BOOL italic) {
    CTFontRef base = viva_system_font(size, weight);
    if (!italic || base == NULL) {
        return base;
    }
    // The family's own italic face when it has one. When it has none the base font is
    // kept and the glyphs are slanted as they are drawn (viva_draw_text_line), the same
    // slant the fallback fonts of a line get.
    CTFontRef face = CTFontCreateCopyWithSymbolicTraits(base, size, NULL, kCTFontItalicTrait, kCTFontItalicTrait);
    if (face == NULL) {
        return base;
    }
    CFRelease(base);
    return face;
}

void viva_text_set_family(const char *family) {
    if (family == NULL) {
        viva_interface_family[0] = '\0';
        return;
    }
    strncpy(viva_interface_family, family, sizeof(viva_interface_family) - 1);
    viva_interface_family[sizeof(viva_interface_family) - 1] = '\0';
}

viva_text_line *viva_text_line_create_style(const unsigned char *utf8, size_t length, double font_size, double font_weight, int italic) {
    @autoreleasepool {
        static const unsigned char empty[1] = {0};
        CFStringRef string = CFStringCreateWithBytes(
            kCFAllocatorDefault, utf8 != NULL ? utf8 : empty, utf8 != NULL ? (CFIndex)length : 0, kCFStringEncodingUTF8, false);
        if (string == NULL) {
            return NULL;
        }
        CTFontRef font = viva_system_font_style(font_size, font_weight, italic != 0);
        if (font == NULL) {
            CFRelease(string);
            return NULL;
        }
        // The color comes from the context when the line is drawn, so one line serves every color.
        const void *keys[] = {kCTFontAttributeName, kCTForegroundColorFromContextAttributeName};
        const void *values[] = {font, kCFBooleanTrue};
        CFDictionaryRef attributes = CFDictionaryCreate(
            kCFAllocatorDefault, keys, values, 2, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
        CFAttributedStringRef attributed = CFAttributedStringCreate(kCFAllocatorDefault, string, attributes);
        CTLineRef line = CTLineCreateWithAttributedString(attributed);
        CFRelease(attributed);
        CFRelease(attributes);
        viva_text_line *shaped = line != NULL ? calloc(1, sizeof(viva_text_line)) : NULL;
        if (shaped == NULL) {
            if (line != NULL) {
                CFRelease(line);
            }
            CFRelease(font);
            CFRelease(string);
            return NULL;
        }
        CGFloat ascent = 0, descent = 0, leading = 0;
        shaped->width = CTLineGetTypographicBounds(line, &ascent, &descent, &leading);
        if (ascent + descent <= 0) {
            // A line with no glyphs reports zero for both, which would give an empty
            // field's caret zero height: the font's own metrics stand in.
            ascent = CTFontGetAscent(font);
            descent = CTFontGetDescent(font);
        }
        shaped->ascent = ascent;
        shaped->descent = descent;
        shaped->line = (void *)line;
        shaped->string = (void *)string;
        shaped->references = 1;
        shaped->italic = italic != 0;
        CFRelease(font);
        return shaped;
    }
}

viva_text_line *viva_text_line_create(const unsigned char *utf8, size_t length, double font_size, double font_weight) {
    return viva_text_line_create_style(utf8, length, font_size, font_weight, 0);
}

void viva_text_line_retain(viva_text_line *line) {
    if (line != NULL) {
        __atomic_fetch_add(&line->references, 1, __ATOMIC_RELAXED);
    }
}

void viva_text_line_release(viva_text_line *line) {
    if (line != NULL && __atomic_fetch_sub(&line->references, 1, __ATOMIC_ACQ_REL) == 1) {
        CFRelease((CTLineRef)line->line);
        CFRelease((CFStringRef)line->string);
        free(line);
    }
}

// viva_cluster_start moves offset back to the start of the character cluster it falls in.
CFIndex viva_cluster_start(CFStringRef string, size_t offset) {
    CFIndex length = CFStringGetLength(string);
    if (offset >= (size_t)length) {
        return length;
    }
    return CFStringGetRangeOfComposedCharactersAtIndex(string, (CFIndex)offset).location;
}

double viva_text_line_caret(const viva_text_line *line, size_t offset) {
    CGFloat secondary = 0;
    return CTLineGetOffsetForStringIndex((CTLineRef)line->line, viva_cluster_start((CFStringRef)line->string, offset), &secondary);
}

size_t viva_text_line_index_at(const viva_text_line *line, double x) {
    CFStringRef string = (CFStringRef)line->string;
    CTLineRef ct = (CTLineRef)line->line;
    CFIndex length = CFStringGetLength(string);
    if (length == 0) {
        return 0;
    }
    CFIndex index = CTLineGetStringIndexForPosition(ct, CGPointMake(x, 0));
    if (index == kCFNotFound) {
        return x <= 0 ? 0 : (size_t)length;
    }
    index = MIN(index, length);
    if (index < length) {
        CFRange cluster = CFStringGetRangeOfComposedCharactersAtIndex(string, index);
        if (cluster.location != index) {
            CGFloat secondary = 0;
            CGFloat start = CTLineGetOffsetForStringIndex(ct, cluster.location, &secondary);
            CGFloat end = CTLineGetOffsetForStringIndex(ct, cluster.location + cluster.length, &secondary);
            index = fabs(x - start) <= fabs(x - end) ? cluster.location : cluster.location + cluster.length;
        }
    }
    return (size_t)index;
}

viva_text_hit viva_text_line_hit_at(const viva_text_line *line, double x) {
    size_t index = viva_text_line_index_at(line, x);
    CGFloat alternate = 0;
    CGFloat primary = CTLineGetOffsetForStringIndex((CTLineRef)line->line, (CFIndex)index, &alternate);
    return (viva_text_hit){index, fabs(x - alternate) < fabs(x - primary) ? alternate : primary};
}

CGFloat viva_fitted_mono_origin(CTLineRef line, CFIndex length) {
    // CoreText puts trailing RTL whitespace before the line's origin, but
    // includes its advance in typographic width. Fit both into the grid span.
    CGFloat alternate = 0;
    CGFloat end = CTLineGetOffsetForStringIndex(line, length, &alternate);
    return MIN(0, MIN(end, alternate));
}

int viva_fitted_mono_carets(const unsigned char *bytes, size_t length, double size, double weight,
                          int face, double span, viva_fitted_mono_caret *carets, size_t count) {
    @autoreleasepool {
        NSString *shown = [[NSString alloc] initWithBytes:bytes length:length encoding:NSUTF8StringEncoding];
        if (shown == nil) {
            return 0;
        }
        CFIndex units = (CFIndex)shown.length;
        CTLineRef line = viva_mono_fallback_line(shown, viva_mono_font(size, weight, face), face, NULL);
        [shown release];
        if (line == NULL) {
            return 0;
        }
        CGFloat width = CTLineGetTypographicBounds(line, NULL, NULL, NULL);
        if (width > 0 && span > 0) {
            CGFloat scale = span / width;
            CGFloat origin = viva_fitted_mono_origin(line, units);
            for (size_t i = 0; i < count; ++i) {
                CGFloat leading = 0;
                CGFloat trailing = CTLineGetOffsetForStringIndex(line, (CFIndex)carets[i].index, &leading);
                carets[i].leading = (leading - origin) * scale;
                carets[i].trailing = (trailing - origin) * scale;
            }
        }
        CFRelease(line);
        return width > 0 && span > 0;
    }
}

// The most extents viva_text_line_selection tells apart before it folds the rest into the
// last: a selection crossing a change of direction more often than this is not text.
#define VIVA_SELECTION_PIECES 32

size_t viva_text_line_selection(const viva_text_line *line, size_t start, size_t end, double *spans, size_t capacity) {
    CFStringRef string = (CFStringRef)line->string;
    CTLineRef ct = (CTLineRef)line->line;
    CFIndex from = viva_cluster_start(string, start);
    CFIndex to = viva_cluster_start(string, end);
    if (from > to) {
        CFIndex swap = from;
        from = to;
        to = swap;
    }
    if (capacity == 0) {
        return 0;
    }
    double left[VIVA_SELECTION_PIECES], right[VIVA_SELECTION_PIECES];
    size_t count = 0;
    CGFloat secondary = 0;
    // A cluster stands between its leading edge (the secondary offset of its first unit, where
    // a boundary between two directions has two) and the trailing edge of its last, which is
    // the primary offset of the unit after it.
    for (CFIndex at = from; at < to;) {
        CFRange cluster = CFStringGetRangeOfComposedCharactersAtIndex(string, at);
        CFIndex next = MIN(cluster.location + cluster.length, to);
        CGFloat leading = 0;
        CTLineGetOffsetForStringIndex(ct, at, &leading);
        CGFloat trailing = CTLineGetOffsetForStringIndex(ct, next, &secondary);
        double lo = MIN(leading, trailing), hi = MAX(leading, trailing);
        if (count > 0 && lo <= right[count - 1] + 0.01 && hi >= left[count - 1] - 0.01) {
            left[count - 1] = MIN(left[count - 1], lo);
            right[count - 1] = MAX(right[count - 1], hi);
        } else if (count < VIVA_SELECTION_PIECES) {
            left[count] = lo;
            right[count++] = hi;
        } else {
            left[count - 1] = MIN(left[count - 1], lo);
            right[count - 1] = MAX(right[count - 1], hi);
        }
        at = next;
    }
    if (count == 0) {
        // An empty selection is the caret.
        left[0] = right[0] = CTLineGetOffsetForStringIndex(ct, from, &secondary);
        count = 1;
    }
    // Pieces of the selection that the reordering put side by side are one extent.
    for (size_t i = 1; i < count; i++) {
        for (size_t j = i; j > 0 && left[j] < left[j - 1]; j--) {
            double l = left[j], r = right[j];
            left[j] = left[j - 1];
            right[j] = right[j - 1];
            left[j - 1] = l;
            right[j - 1] = r;
        }
    }
    size_t written = 0;
    for (size_t i = 0; i < count; i++) {
        if (written > 0 && left[i] <= spans[2 * (written - 1)] + spans[2 * (written - 1) + 1] + 0.01) {
            double merged = MAX(spans[2 * (written - 1)] + spans[2 * (written - 1) + 1], right[i]);
            spans[2 * (written - 1) + 1] = merged - spans[2 * (written - 1)];
        } else if (written < capacity) {
            spans[2 * written] = left[i];
            spans[2 * written + 1] = right[i] - left[i];
            written++;
        }
    }
    return written;
}

// viva_shared_text_line is the engine's line for shown in the system font, retained for the
// caller to release with viva_text_line_release.
viva_text_line *viva_shared_text_line(NSString *shown, CGFloat fontSize, CGFloat weight) {
    NSData *bytes = [shown dataUsingEncoding:NSUTF8StringEncoding];
    return vivaShapedLine(bytes != nil ? (const unsigned char *)bytes.bytes : NULL, bytes != nil ? bytes.length : 0, fontSize, weight);
}

viva_text_line *viva_shared_text_line_style(NSString *shown, CGFloat fontSize, CGFloat weight, BOOL italic) {
    NSData *bytes = [shown dataUsingEncoding:NSUTF8StringEncoding];
    return vivaShapedLineStyle(bytes != nil ? (const unsigned char *)bytes.bytes : NULL, bytes != nil ? bytes.length : 0, fontSize, weight, italic ? 1 : 0);
}

// viva_draw_text_line draws the shaped line in one call, its origin at x on baseline, in
// color (the line carries none, so the same line serves every color).
// VIVA_SYNTHETIC_ITALIC_SLANT is the shear of a glyph drawn in italics from a font that has
// no italic face: about eleven degrees, as the tilt of the system font's own italic.
#define VIVA_SYNTHETIC_ITALIC_SLANT 0.2

// viva_run_needs_slant says whether a run of an italic line must be slanted: its font has no
// italic face, as a CJK fallback font has none. A font of color glyphs (emoji) is left upright.
BOOL viva_run_needs_slant(CTRunRef run) {
    CFDictionaryRef attributes = CTRunGetAttributes(run);
    CTFontRef font = attributes != NULL ? (CTFontRef)CFDictionaryGetValue(attributes, kCTFontAttributeName) : NULL;
    if (font == NULL) {
        return NO;
    }
    CTFontSymbolicTraits traits = CTFontGetSymbolicTraits(font);
    return (traits & (kCTFontItalicTrait | kCTFontColorGlyphsTrait)) == 0;
}

void viva_draw_text_line(CGContextRef context, const viva_text_line *line, CGColorRef color, CGFloat x, CGFloat baseline) {
    CGContextSaveGState(context);
    CGContextSetFillColorWithColor(context, color);
    if (!line->italic) {
        CGContextSetTextPosition(context, x, baseline);
        CTLineDraw((CTLineRef)line->line, context);
    } else {
        // Run by run: the runs whose font has no italic face are drawn under a shear about
        // the baseline. The glyph positions of a run are relative to the line's origin, so
        // the shear leaves them where the line put them and only tilts the glyphs.
        CFArrayRef runs = CTLineGetGlyphRuns((CTLineRef)line->line);
        for (CFIndex index = 0, count = CFArrayGetCount(runs); index < count; index++) {
            CTRunRef run = (CTRunRef)CFArrayGetValueAtIndex(runs, index);
            CGFloat slant = viva_run_needs_slant(run) ? VIVA_SYNTHETIC_ITALIC_SLANT : 0;
            CGContextSetTextMatrix(context, CGAffineTransformMake(1, 0, slant, 1, x, baseline));
            CTRunDraw(run, context, CFRangeMake(0, 0));
        }
        // The text matrix is not part of the saved state, and other labels set only a position.
        CGContextSetTextMatrix(context, CGAffineTransformIdentity);
    }
    CGContextRestoreGState(context);
}

// The geometry of the text a native editor shows: the shaped line of proportional text, or
// a terminal's preedit, which is laid on the terminal's cells and not on the advances of
// its font: the text, and the width of one cell.

void viva_editor_line_free(viva_editor_line line) {
    viva_text_line_release(line.shaped);
    [line.grid release];
}

// viva_editor_line_made says whether the line could be made.
BOOL viva_editor_line_made(viva_editor_line line) {
    return line.shaped != NULL || line.grid != nil;
}

// viva_terminal_step reads the character of shown at UTF-16 index `at`: how many units it
// takes, and how many cells (see vivaTerminalCells; a combining mark has none).
NSUInteger viva_terminal_step(NSString *shown, NSUInteger at, int *cells) {
    unichar first = [shown characterAtIndex:at];
    uint32_t point = first;
    NSUInteger length = 1;
    if (CFStringIsSurrogateHighCharacter(first) && at + 1 < shown.length) {
        unichar second = [shown characterAtIndex:at + 1];
        if (CFStringIsSurrogateLowCharacter(second)) {
            point = CFStringGetLongCharacterForSurrogatePair(first, second);
            length = 2;
        }
    }
    *cells = vivaTerminalCells(point);
    return length;
}

// viva_terminal_column is the cell column of the boundary before the UTF-16 unit `index`
// of shown, counted from the first cell of the text.
NSUInteger viva_terminal_column(NSString *shown, NSUInteger index) {
    NSUInteger column = 0;
    for (NSUInteger at = 0; at < index && at < shown.length;) {
        int cells = 0;
        NSUInteger length = viva_terminal_step(shown, at, &cells);
        if (at + length > index) {
            break;
        }
        column += (NSUInteger)cells;
        at += length;
    }
    return column;
}

// viva_text_offset is where the editor draws the boundary before the UTF-16 unit `index`
// of shown, from the left of the text.
CGFloat viva_text_offset(viva_editor_line line, NSUInteger index, BOOL terminal) {
    if (terminal) {
        return (CGFloat)viva_terminal_column(line.grid, index) * line.cell;
    }
    return viva_text_line_caret(line.shaped, index);
}

// viva_text_spans is the extents of range in the editor, left to right.
size_t viva_text_spans(viva_editor_line line, NSRange range, BOOL terminal, double *spans, size_t capacity) {
    if (terminal) {
        CGFloat start = viva_text_offset(line, range.location, terminal);
        CGFloat end = viva_text_offset(line, NSMaxRange(range), terminal);
        spans[0] = MIN(start, end);
        spans[1] = fabs(end - start);
        return 1;
    }
    return viva_text_line_selection(line.shaped, range.location, NSMaxRange(range), spans, capacity);
}

// viva_text_index_at is the boundary of shown nearest to x, the inverse of viva_text_offset.
NSUInteger viva_text_index_at(viva_editor_line line, NSString *shown, CGFloat x, BOOL terminal) {
    if (!terminal) {
        return viva_text_line_index_at(line.shaped, x);
    }
    // The boundary between characters whose cell edge is nearest x; a boundary inside a
    // character with its combining marks is none, and of two on one edge the later wins.
    NSUInteger position = 0;
    CGFloat nearest = fabs(x);
    NSUInteger column = 0;
    for (NSUInteger at = 0; at < shown.length;) {
        int cells = 0;
        at += viva_terminal_step(shown, at, &cells);
        column += (NSUInteger)cells;
        if (at < shown.length && [shown rangeOfComposedCharacterSequenceAtIndex:at].location != at) {
            continue;
        }
        CGFloat distance = fabs(x - (CGFloat)column * line.cell);
        if (distance <= nearest) {
            nearest = distance;
            position = at;
        }
    }
    return position;
}

// viva_mono_font is the system fixed-pitch font, the one every fixed-pitch label
// and the cell metrics are built from.
//
// A face from 2 up stands for an installed family and the families that draw what it
// lacks; viva_register_mono_family teaches the numbers, in registration order.
#define VIVA_MONO_FAMILY_SLOTS 4098 /* the 2 built-in faces and gfx.MaxMonoFaces */
static NSString *viva_mono_families[VIVA_MONO_FAMILY_SLOTS];
static NSArray<NSString *> *viva_mono_fallbacks[VIVA_MONO_FAMILY_SLOTS];
static BOOL viva_mono_classic[VIVA_MONO_FAMILY_SLOTS];
NSObject *viva_mono_family_lock(void) {
    static NSObject *lock;
    static dispatch_once_t once;
    dispatch_once(&once, ^{ lock = [[NSObject alloc] init]; });
    return lock;
}

void viva_register_mono_family(int face, const char *utf8, int classic, const char *fallbacks) {
    if (face < 2 || face >= VIVA_MONO_FAMILY_SLOTS) {
        return;
    }
    @synchronized(viva_mono_family_lock()) {
        [viva_mono_families[face] release];
        [viva_mono_fallbacks[face] release];
        viva_mono_families[face] = (utf8 != NULL && utf8[0] != '\0') ? [[NSString alloc] initWithUTF8String:utf8] : nil;
        viva_mono_classic[face] = classic != 0;
        // The fallbacks arrive as one name per line.
        NSString *joined = fallbacks != NULL ? [[NSString alloc] initWithUTF8String:fallbacks] : nil;
        viva_mono_fallbacks[face] = (joined != nil && joined.length > 0) ? [[joined componentsSeparatedByString:@"\n"] retain] : nil;
        [joined release];
    }
}

// viva_mono_face_traits copies what a face stands for: its primary family (nil for a
// built-in one), whether the built-in primary is the classic face, and its fallback
// families in order.
void viva_mono_face_traits(int face, NSString **family, BOOL *classic, NSArray<NSString *> **fallbacks) {
    *family = nil;
    *classic = face == 1;
    *fallbacks = nil;
    if (face < 2 || face >= VIVA_MONO_FAMILY_SLOTS) {
        return;
    }
    @synchronized(viva_mono_family_lock()) {
        *family = [[viva_mono_families[face] retain] autorelease];
        *classic = viva_mono_classic[face];
        *fallbacks = [[viva_mono_fallbacks[face] retain] autorelease];
    }
}

// viva_mono_font_on_a_grid reports whether the font advances every printable ASCII
// character by the same width: the one property a terminal cell grid needs.
//
// The font's own fixed-pitch bit is not enough to ask. Families built for coding
// (Nerd Font patches, ligature builds) often leave it clear while drawing on a grid
// all the same, and the bit alone kept them out of the list. The advances are
// compared with a percent of the cell to spare, because a grid font still rounds a
// glyph by a hair (Roboto Mono for Powerline gives '-' 7.207 where its cell is
// 7.201) while a proportional font's differ by a fraction of the cell.
BOOL viva_mono_font_on_a_grid(VivaFont *font) {
    if ((CTFontGetSymbolicTraits((CTFontRef)font) & kCTFontTraitMonoSpace) != 0) {
        return YES;
    }
    enum { first = 0x20, last = 0x7E, count = last - first + 1 };
    UniChar characters[count];
    CGGlyph glyphs[count];
    CGSize advances[count];
    for (CFIndex index = 0; index < count; index++) {
        characters[index] = (UniChar)(first + index);
    }
    CTFontRef ctFont = (CTFontRef)font;
    if (!CTFontGetGlyphsForCharacters(ctFont, characters, glyphs, count)) {
        return NO; // it cannot draw one of them, so it is no terminal face
    }
    CTFontGetAdvancesForGlyphs(ctFont, kCTFontOrientationHorizontal, glyphs, advances, count);
    CGFloat cell = advances[0].width;
    if (cell <= 0.0) {
        return NO;
    }
    for (CFIndex index = 1; index < count; index++) {
        if (fabs(advances[index].width - cell) > cell * 0.01) {
            return NO;
        }
    }
    return YES;
}

char *viva_list_font_families(void) {
    @autoreleasepool {
        NSMutableArray<NSString *> *names = [NSMutableArray array];
        for (NSString *family in viva_font_family_names()) {
            if ([family hasPrefix:@"."]) {
                continue;
            }
            [names addObject:family];
        }
        [names sortUsingSelector:@selector(localizedCaseInsensitiveCompare:)];
        return strdup([[names componentsJoinedByString:@"\n"] UTF8String]);
    }
}

char *viva_list_mono_families(void) {
    @autoreleasepool {
        NSMutableArray<NSString *> *names = [NSMutableArray array];
        for (NSString *family in viva_font_family_names()) {
            if ([family hasPrefix:@"."]) {
                continue;
            }
            VivaFont *font = viva_font_in_family(family, 12, NO);
            if (font != nil && viva_mono_font_on_a_grid(font)) {
                [names addObject:family];
            }
        }
        [names sortUsingSelector:@selector(localizedCaseInsensitiveCompare:)];
        return strdup([[names componentsJoinedByString:@"\n"] UTF8String]);
    }
}

// viva_mono_base_font is the primary font of a face, without its fallbacks.
VivaFont *viva_mono_base_font(CGFloat size, CGFloat weight, NSString *family, BOOL classic) {
    if (family != nil) {
        VivaFont *named = viva_font_in_family(family, size, weight >= 0.34);
        if (named != nil) {
            return named;
        }
    }
    if (classic) {
        VivaFont *classicFont = viva_font_user_fixed_pitch(size);
        if (classicFont != nil) {
            return classicFont;
        }
    }
    VivaFont *font = [VivaFont monospacedSystemFontOfSize:size weight:weight];
    return font != nil ? font : viva_font_user_fixed_pitch(size);
}

// viva_mono_font_primary is the primary font of a face alone, the one the cell is measured in.
VivaFont *viva_mono_font_primary(CGFloat size, CGFloat weight, int face) {
    NSString *family = nil;
    BOOL classic = NO;
    NSArray<NSString *> *fallbacks = nil;
    viva_mono_face_traits(face, &family, &classic, &fallbacks);
    return viva_mono_base_font(size, weight, family, classic);
}

// viva_mono_font is the font of a face at a size and weight: its primary font, with the
// families that draw what the primary lacks as the font's cascade list, so CoreText
// tries them in order, before its own choice, for every character the primary has no
// glyph for. A family that is not installed is skipped. The cell is the primary's: the
// fallbacks never take part in the metrics, and every glyph is placed on the grid.
VivaFont *viva_mono_font(CGFloat size, CGFloat weight, int face) {
    NSString *family = nil;
    BOOL classic = NO;
    NSArray<NSString *> *fallbacks = nil;
    viva_mono_face_traits(face, &family, &classic, &fallbacks);
    VivaFont *base = viva_mono_base_font(size, weight, family, classic);
    if (base == nil || fallbacks.count == 0) {
        return base;
    }
    NSMutableArray<VivaFontDescriptor *> *cascade = [NSMutableArray arrayWithCapacity:fallbacks.count];
    for (NSString *name in fallbacks) {
        [cascade addObject:[VivaFontDescriptor fontDescriptorWithFontAttributes:@{VivaFontFamilyAttribute: name}]];
    }
    VivaFontDescriptor *descriptor = [base.fontDescriptor fontDescriptorByAddingAttributes:@{VivaFontCascadeListAttribute: cascade}];
    VivaFont *stacked = [VivaFont fontWithDescriptor:descriptor size:size];
    return stacked != nil ? stacked : base;
}

/* ---- fallback to the installed fonts for glyphs no family of the face has ---
 *
 * CoreText draws .LastResort for a character neither the face's families nor its
 * own fallback have: private-use icon code points (Powerline, Nerd Fonts) above
 * all. It never looks at the fonts the user installed, while the Linux
 * (fontconfig) and the Windows (DirectWrite) text stacks do -- so a terminal
 * loses the icons of the very font its user installed, and a prompt made of
 * Powerline separators reads as boxes.
 *
 * A cluster that CoreText would draw with the last-resort glyph is handed to the
 * installed family that has it, ranked so that a terminal icon font wins over a
 * CJK font that maps the same code point to one of its own characters: fixed
 * pitch first, then one that draws no Han, then the one covering the most
 * private-use code points, then the family name. The choice is remembered per
 * cluster (a terminal draws the same icons again and again) and per family (what
 * a family covers cannot change while the process runs), so a family installed
 * after a character was first drawn is only picked up by a new process.
 */

// VIVA_MONO_FALLBACK_CACHE bounds the clusters remembered; past it a cluster is
// looked up again each time instead of growing the table without end.
#define VIVA_MONO_FALLBACK_CACHE 4096
// VIVA_MONO_CLUSTER_LENGTH bounds a cluster; the longest real ones are emoji
// sequences of a few code points.
#define VIVA_MONO_CLUSTER_LENGTH 64


static NSMutableDictionary<NSString *, NSValue *> *viva_mono_trait_cache;
static NSMutableDictionary<NSString *, id> *viva_mono_cluster_cache;

NSObject *viva_mono_fallback_lock(void) {
    static NSObject *lock;
    static dispatch_once_t once;
    dispatch_once(&once, ^{ lock = [[NSObject alloc] init]; });
    return lock;
}

// viva_mono_family_font resolves an installed family at a size and weight, or nil.
VivaFont *viva_mono_family_font(NSString *family, CGFloat size, CGFloat weight) {
    return viva_font_in_family(family, size, weight >= 0.34);
}

// viva_mono_replacement_font is viva_mono_family_font for a replacement glyph, remembering
// the families asked for: the font manager lookup is most of what a replacement costs, and
// a terminal draws the same icon clusters line after line at the same size. The font is
// returned retained and autoreleased, so it outlives an eviction.
#define VIVA_MONO_REPLACEMENT_CACHE 8

typedef struct {
    NSString *family;
    CGFloat size;
    BOOL bold;
    VivaFont *font;
} viva_mono_replacement;

static viva_mono_replacement viva_mono_replacements[VIVA_MONO_REPLACEMENT_CACHE];
static int viva_mono_replacement_count;
static int viva_mono_replacement_next;

VivaFont *viva_mono_replacement_font(NSString *family, CGFloat size, BOOL bold) {
    @synchronized(viva_mono_fallback_lock()) {
        for (int index = 0; index < viva_mono_replacement_count; index++) {
            viva_mono_replacement *entry = &viva_mono_replacements[index];
            if (entry->size == size && entry->bold == bold && [entry->family isEqualToString:family]) {
                return [[entry->font retain] autorelease];
            }
        }
    }
    VivaFont *font = viva_mono_family_font(family, size, bold ? 1 : 0);
    if (font == nil) {
        return nil;
    }
    @synchronized(viva_mono_fallback_lock()) {
        viva_mono_replacement *entry;
        if (viva_mono_replacement_count < VIVA_MONO_REPLACEMENT_CACHE) {
            entry = &viva_mono_replacements[viva_mono_replacement_count++];
        } else {
            entry = &viva_mono_replacements[viva_mono_replacement_next++ % VIVA_MONO_REPLACEMENT_CACHE];
            [entry->family release];
            [entry->font release];
        }
        entry->family = [family copy];
        entry->size = size;
        entry->bold = bold;
        entry->font = [font retain];
    }
    return [[font retain] autorelease];
}

// viva_mono_font_covers reports whether font draws every code unit of value.
BOOL viva_mono_font_covers(CTFontRef font, NSString *value) {
    NSUInteger length = value.length;
    if (font == NULL || length == 0 || length > VIVA_MONO_CLUSTER_LENGTH) {
        return NO;
    }
    UniChar characters[VIVA_MONO_CLUSTER_LENGTH];
    CGGlyph glyphs[VIVA_MONO_CLUSTER_LENGTH];
    [value getCharacters:characters range:NSMakeRange(0, length)];
    if (!CTFontGetGlyphsForCharacters(font, characters, glyphs, (CFIndex)length)) {
        return NO;
    }
    for (NSUInteger index = 0; index < length; index++) {
        if (glyphs[index] == 0) {
            return NO;
        }
    }
    return YES;
}

// viva_mono_draws_han reports whether a font draws Chinese text at all. A family
// that does is a text face, and the glyph it maps a private-use code point to is
// one of its own characters rather than the icon the line asks for.
BOOL viva_mono_draws_han(CTFontRef font) {
    static const UniChar probes[] = {0x4E2D, 0x5B57, 0x8A9E}; /* 中 字 語 */
    CGGlyph glyphs[3] = {0};
    CTFontGetGlyphsForCharacters(font, probes, glyphs, 3);
    return glyphs[0] != 0 || glyphs[1] != 0 || glyphs[2] != 0;
}

// viva_mono_private_use counts the private-use code points a font covers, from
// the plane's bitmap: a Nerd Font covers thousands, a CJK font a handful.
int viva_mono_private_use(CTFontRef font) {
    CFCharacterSetRef characters = CTFontCopyCharacterSet(font);
    if (characters == NULL) {
        return 0;
    }
    CFDataRef bitmap = CFCharacterSetCreateBitmapRepresentation(kCFAllocatorDefault, characters);
    CFRelease(characters);
    if (bitmap == NULL) {
        return 0;
    }
    const UInt8 *bytes = CFDataGetBytePtr(bitmap);
    CFIndex length = CFDataGetLength(bitmap);
    int count = 0;
    for (int code = 0xE000; code <= 0xF8FF; code++) {
        CFIndex index = code >> 3;
        if (index >= length) {
            break;
        }
        if ((bytes[index] & (1 << (code & 7))) != 0) {
            count++;
        }
    }
    CFRelease(bitmap);
    return count;
}

// viva_mono_family_traits_of is what ranks a family: whether it draws on a grid (the
// same test that decides which families are offered, so a coding family that leaves
// the fixed-pitch bit clear ranks with the ones that set it), whether it draws Han,
// and how much private-use it covers. Read once per family.
viva_mono_family_traits viva_mono_family_traits_of(NSString *family) {
    viva_mono_family_traits traits = {NO, YES, 0};
    @synchronized(viva_mono_fallback_lock()) {
        NSValue *known = [viva_mono_trait_cache objectForKey:family];
        if (known != nil) {
            [known getValue:&traits size:sizeof(traits)];
            return traits;
        }
    }
    VivaFont *font = viva_mono_family_font(family, 12, 0);
    if (font != nil) {
        traits.onGrid = viva_mono_font_on_a_grid(font);
        traits.includesHan = viva_mono_draws_han((CTFontRef)font);
        traits.privateUse = viva_mono_private_use((CTFontRef)font);
    }
    NSValue *value = [NSValue valueWithBytes:&traits objCType:@encode(viva_mono_family_traits)];
    @synchronized(viva_mono_fallback_lock()) {
        if (viva_mono_trait_cache == nil) {
            viva_mono_trait_cache = [[NSMutableDictionary alloc] init];
        }
        [viva_mono_trait_cache setObject:value forKey:family];
    }
    return traits;
}

// viva_mono_family_better reports whether candidate ranks before best for a cluster.
BOOL viva_mono_family_better(viva_mono_family_traits candidate, NSString *family,
                                   viva_mono_family_traits best, NSString *bestFamily) {
    if (bestFamily == nil) {
        return YES;
    }
    if (candidate.onGrid != best.onGrid) {
        return candidate.onGrid;
    }
    if (candidate.includesHan != best.includesHan) {
        return !candidate.includesHan;
    }
    if (candidate.privateUse != best.privateUse) {
        return candidate.privateUse > best.privateUse;
    }
    return [family localizedCaseInsensitiveCompare:bestFamily] == NSOrderedAscending;
}

// viva_mono_find_family is the installed family that draws cluster, or nil. It
// asks the system for the fonts covering the cluster's code points, leaves out
// the ones that do not draw every code point of it, and ranks the rest.
NSString *viva_mono_find_family(NSString *cluster) {
    CFCharacterSetRef characters = CFCharacterSetCreateWithCharactersInString(kCFAllocatorDefault, (CFStringRef)cluster);
    if (characters == NULL) {
        return nil;
    }
    const void *keys[] = {kCTFontCharacterSetAttribute};
    const void *values[] = {characters};
    CFDictionaryRef query = CFDictionaryCreate(kCFAllocatorDefault, keys, values, 1,
                                               &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    CFRelease(characters);
    if (query == NULL) {
        return nil;
    }
    CTFontDescriptorRef descriptor = CTFontDescriptorCreateWithAttributes(query);
    CFRelease(query);
    if (descriptor == NULL) {
        return nil;
    }
    CFArrayRef matches = CTFontDescriptorCreateMatchingFontDescriptors(descriptor, NULL);
    CFRelease(descriptor);
    if (matches == NULL) {
        return nil;
    }
    NSString *best = nil;
    viva_mono_family_traits bestTraits = {NO, YES, 0};
    CFIndex count = CFArrayGetCount(matches);
    for (CFIndex index = 0; index < count; index++) {
        CTFontDescriptorRef match = (CTFontDescriptorRef)CFArrayGetValueAtIndex(matches, index);
        NSString *family = (NSString *)CTFontDescriptorCopyAttribute(match, kCTFontFamilyNameAttribute);
        if (family == nil || family.length == 0) {
            if (family != NULL) {
                CFRelease(family);
            }
            continue;
        }
        VivaFont *font = viva_mono_family_font(family, 12, 0);
        if (font == nil || !viva_mono_font_covers((CTFontRef)font, cluster)) {
            CFRelease(family);
            continue;
        }
        viva_mono_family_traits traits = viva_mono_family_traits_of(family);
        if (viva_mono_family_better(traits, family, bestTraits, best)) {
            best = [family autorelease];
            bestTraits = traits;
        } else {
            CFRelease(family);
        }
    }
    CFRelease(matches);
    return best;
}

// viva_mono_face_fallback is the family the face itself puts behind its primary
// that draws cluster, or nil. CoreText ignores a cascade list for private-use code
// points as well, so the application's own fallbacks -- the ones the settings page
// offers -- have to be tried here, before the ranking.
NSString *viva_mono_face_fallback(NSString *cluster, int face) {
    NSString *family = nil;
    BOOL classic = NO;
    NSArray<NSString *> *fallbacks = nil;
    viva_mono_face_traits(face, &family, &classic, &fallbacks);
    for (NSString *candidate in fallbacks) {
        VivaFont *font = viva_mono_family_font(candidate, 12, 0);
        if (font != nil && viva_mono_font_covers((CTFontRef)font, cluster)) {
            return candidate;
        }
    }
    return nil;
}

// viva_mono_fallback_family is the installed family that draws a cluster CoreText
// cannot, or nil when no installed family has it.
NSString *viva_mono_fallback_family(NSString *cluster) {
    if (cluster.length == 0 || cluster.length > VIVA_MONO_CLUSTER_LENGTH) {
        return nil;
    }
    @synchronized(viva_mono_fallback_lock()) {
        id known = [viva_mono_cluster_cache objectForKey:cluster];
        if (known != nil) {
            return known == [NSNull null] ? nil : known;
        }
    }
    NSString *family = viva_mono_find_family(cluster);
    @synchronized(viva_mono_fallback_lock()) {
        if (viva_mono_cluster_cache == nil) {
            viva_mono_cluster_cache = [[NSMutableDictionary alloc] init];
        }
        if (viva_mono_cluster_cache.count < VIVA_MONO_FALLBACK_CACHE) {
            [viva_mono_cluster_cache setObject:(family != nil ? family : (id)[NSNull null]) forKey:cluster];
        }
    }
    return family;
}

// viva_mono_font_is_last_resort reports whether font is the face CoreText draws a
// character no installed family has with. The face calls itself LastResort, with a
// leading dot in its family name and without it in its PostScript name.
BOOL viva_mono_is_last_resort_name(CFStringRef name) {
    if (name == NULL) {
        return NO;
    }
    return CFStringCompare(name, CFSTR("LastResort"), kCFCompareCaseInsensitive) == kCFCompareEqualTo ||
           CFStringCompare(name, CFSTR(".LastResort"), kCFCompareCaseInsensitive) == kCFCompareEqualTo;
}

BOOL viva_mono_font_is_last_resort(CTFontRef font) {
    if (font == NULL) {
        return NO;
    }
    CFStringRef postscript = CTFontCopyPostScriptName(font);
    CFStringRef family = CTFontCopyFamilyName(font);
    BOOL last = viva_mono_is_last_resort_name(postscript) || viva_mono_is_last_resort_name(family);
    if (postscript != NULL) {
        CFRelease(postscript);
    }
    if (family != NULL) {
        CFRelease(family);
    }
    return last;
}

// viva_mono_fallback_line shapes a fixed-pitch line in base (the face's font at the
// weight, and with the slant, the caller already resolved): for the clusters CoreText
// would draw with its last-resort glyph, the installed family that has them, at base's
// size and boldness. A line with nothing to replace is shaped once, as before; one with
// a replacement is shaped again with the fallback fonts in place, so every glyph sits
// where the grid says. The line is +1: the caller releases it.
CTLineRef viva_mono_fallback_line(NSString *value, VivaFont *base, int face, CGColorRef color) {
    if (base == nil || value.length == 0) {
        return NULL;
    }
    CGFloat size = base.pointSize;
    BOOL bold = (base.fontDescriptor.symbolicTraits & VivaFontBoldTrait) != 0;
    CFIndex length = (CFIndex)value.length;
    CFMutableAttributedStringRef attributed = CFAttributedStringCreateMutable(kCFAllocatorDefault, 0);
    if (attributed == NULL) {
        return NULL;
    }
    CFAttributedStringReplaceString(attributed, CFRangeMake(0, 0), (CFStringRef)value);
    CFAttributedStringSetAttribute(attributed, CFRangeMake(0, length), kCTFontAttributeName, (CTFontRef)base);
    if (color != NULL) {
        CFAttributedStringSetAttribute(attributed, CFRangeMake(0, length), kCTForegroundColorAttributeName, color);
    }
    CTLineRef line = CTLineCreateWithAttributedString(attributed);
    if (line == NULL) {
        CFRelease(attributed);
        return NULL;
    }
    BOOL replaced = NO;
    CFArrayRef runs = CTLineGetGlyphRuns(line);
    CFIndex runCount = runs != NULL ? CFArrayGetCount(runs) : 0;
    for (CFIndex index = 0; index < runCount; index++) {
        CTRunRef run = (CTRunRef)CFArrayGetValueAtIndex(runs, index);
        CFDictionaryRef runAttributes = CTRunGetAttributes(run);
        CTFontRef runFont = runAttributes != NULL ? (CTFontRef)CFDictionaryGetValue(runAttributes, kCTFontAttributeName) : NULL;
        if (!viva_mono_font_is_last_resort(runFont)) {
            continue;
        }
        CFRange runRange = CTRunGetStringRange(run);
        CFIndex end = runRange.location + runRange.length;
        for (CFIndex at = runRange.location; at < end;) {
            NSRange cluster = [value rangeOfComposedCharacterSequenceAtIndex:(NSUInteger)at];
            if (cluster.length == 0) {
                break;
            }
            NSString *text = [value substringWithRange:cluster];
            NSString *family = viva_mono_face_fallback(text, face);
            if (family == nil) {
                family = viva_mono_fallback_family(text);
            }
            VivaFont *fallback = family != nil ? viva_mono_replacement_font(family, size, bold) : nil;
            if (fallback != nil) {
                CFAttributedStringSetAttribute(attributed,
                                               CFRangeMake((CFIndex)cluster.location, (CFIndex)cluster.length),
                                               kCTFontAttributeName, (CTFontRef)fallback);
                replaced = YES;
            }
            at = (CFIndex)NSMaxRange(cluster);
        }
    }
    if (!replaced) {
        CFRelease(attributed);
        return line;
    }
    CFRelease(line);
    line = CTLineCreateWithAttributedString(attributed);
    CFRelease(attributed);
    return line;
}

// viva_text_system_thickening reports whether the user leaves the system's own font
// smoothing on, the weight AppKit's text starts at: AppleFontSmoothing absent or nonzero
// means on, zero (or a value that is not a number, as AppKit reads it) off. A bitmap
// context does not heed the setting, so the engine asks for it here, once, at its start;
// standardUserDefaults reads the argument domain too, which is how tests pin it.
int viva_text_system_thickening(void) {
    @autoreleasepool {
        NSUserDefaults *defaults = [NSUserDefaults standardUserDefaults];
        if ([defaults objectForKey:@"AppleFontSmoothing"] == nil) {
            return 1;
        }
        return [defaults integerForKey:@"AppleFontSmoothing"] != 0;
    }
}

// viva_text_thickening is whether Viva draws its text with a thicker stroke, as AppKit draws
// the system's own text: CoreText's font smoothing, which darkens the stems. Off is the
// font's own outlines, so the glyphs stay the weight the font gives them (measured against
// AppKit text: smoothing draws 16 to 45 percent more ink, the lighter the text the more).
// It starts at the system's own weight (viva_text_system_thickening) and is then the
// weight the application asked for.
static int viva_text_thickening = 0;

int viva_text_set_thickening(int thickening) {
    int previous = viva_text_thickening;
    viva_text_thickening = thickening;
    return previous != thickening;
}

// viva_prepare_text_context sets a bitmap context up to draw text the way AppKit draws it.
void viva_prepare_text_context(CGContextRef context, CGFloat scale) {
    CGContextScaleCTM(context, scale, scale);
    CGContextSetTextMatrix(context, CGAffineTransformIdentity);
    CGContextSetShouldSmoothFonts(context, viva_text_thickening != 0);
}

// viva_draw_terminal_preedit draws a terminal's preedit text the way the terminal draws its
// own line: every character on the cells of the grid, a plain ASCII one at the left of its
// cell and any other centered in the cells it takes (two for a wide one), with its combining
// marks. The native editor and the headless renderer both draw it here, so they can only ever
// look the same, and the text does not move when it is committed to the grid.
void viva_draw_terminal_preedit(
    CGContextRef context, NSString *shown, CGFloat fontSize, int monoFace,
    VivaColor *color, CGFloat cellWidth, CGFloat baseline
) {
    // The face's font, with the installed fallback for what no family of it draws.
    VivaFont *font = viva_mono_font(fontSize, 0, monoFace);
    NSUInteger column = 0;
    for (NSUInteger at = 0; at < shown.length;) {
        int cells = 0;
        NSUInteger end = at + viva_terminal_step(shown, at, &cells);
        for (int marks = 0; end < shown.length;) {
            NSUInteger next = viva_terminal_step(shown, end, &marks);
            if (marks != 0) {
                break;
            }
            end += next;
        }
        NSString *cluster = [shown substringWithRange:NSMakeRange(at, end - at)];
        CTLineRef line = viva_mono_fallback_line(cluster, font, monoFace, color.CGColor);
        if (line != NULL) {
            CGFloat x = (CGFloat)column * cellWidth;
            unichar only = cluster.length == 1 ? [cluster characterAtIndex:0] : 0;
            if (cells > 0 && !(only >= 0x20 && only < 0x7F)) {
                CGFloat width = CTLineGetTypographicBounds(line, NULL, NULL, NULL);
                x += ((CGFloat)cells * cellWidth - width) / 2;
            }
            CGContextSetTextPosition(context, x, baseline);
            CTLineDraw(line, context);
            CFRelease(line);
        }
        column += (NSUInteger)cells;
        at = end;
    }
}

// viva_new_editor_line is the geometry of the text the editor shows: one shaped line
// shared with layout for proportional text, the terminal's cells for a terminal. A line
// that was not made could not be shaped.
viva_editor_line viva_new_editor_line(NSString *shown, CGFloat fontSize, CGFloat fontWeight, BOOL terminal, CGFloat cellWidth) {
    viva_editor_line line = {NULL, nil, 0};
    if (terminal) {
        line.grid = [shown copy];
        line.cell = cellWidth;
    } else {
        line.shaped = viva_shared_text_line(shown, fontSize, fontWeight);
    }
    return line;
}

CGFloat viva_editor_alignment_inset(viva_editor_line line, CGFloat width, int alignment, BOOL terminal) {
    if (terminal || line.shaped == NULL || (alignment != 1 && alignment != 2)) {
        return 0;
    }
    CGFloat free = MAX(0, width - line.shaped->width);
    return alignment == 1 ? free / 2 : free;
}

CGFloat viva_editor_visible_caret(CGFloat offset, CGFloat width, int alignment, BOOL terminal) {
    if (alignment != 2 || terminal) {
        return offset;
    }
    return MIN(offset, MAX(0, width - 2));
}

// viva_baseline is the height of the baseline above the bottom of a label's texture,
// in points. The baseline is the same row every backend uses (platform.LabelBaseline):
// a fixed distance below the middle of the line box, on a whole physical pixel, so
// the font's own ascent and descent do not move the text.
CGFloat viva_baseline(CGFloat height, CGFloat fontSize, CGFloat pixelScale) {
    CGFloat rows = MAX(1.0, ceil(height * pixelScale));
    CGFloat fromTop = floor(rows * 0.5 + VIVA_BASELINE_BELOW_CENTER * fontSize * pixelScale + 1e-6);
    return (rows - fromTop) / pixelScale;
}

// viva_fixed_baseline is the height above a texture's bottom of the row a canvas worked out
// for a fixed-pitch line, counted from the texture's top in points
// (platform.TextLabel.Baseline), on a whole physical pixel.
CGFloat viva_fixed_baseline(CGFloat height, CGFloat fromTop, CGFloat pixelScale) {
    CGFloat rows = MAX(1.0, ceil(height * pixelScale));
    return (rows - floor(fromTop * pixelScale + 0.5)) / pixelScale;
}

int viva_text_line_rasterize(
    const viva_text_line *line,
    double font_size,
    double scale,
    size_t width,
    size_t height,
    double red,
    double green,
    double blue,
    double alpha,
    unsigned char *pixels
) {
    if (line == NULL || pixels == NULL || font_size <= 0 || scale <= 0 ||
        width < 1 || height < 1 || width > 16384 || height > 16384 ||
        width > SIZE_MAX / 4 || height > SIZE_MAX / (width * 4)) {
        return 1;
    }
    @autoreleasepool {
        memset(pixels, 0, width * height * 4);
        CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
        CGContextRef context = CGBitmapContextCreate(
            pixels, width, height, 8, width * 4, colorSpace,
            kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little
        );
        CGColorSpaceRelease(colorSpace);
        if (context == NULL) {
            return 1;
        }
        viva_prepare_text_context(context, scale);
        CGColorRef color = CGColorCreateSRGB(red, green, blue, alpha);
        CGFloat logicalHeight = (CGFloat)height / (CGFloat)scale;
        CGFloat baseline = viva_baseline(logicalHeight, font_size, scale);
        viva_draw_text_line(context, line, color, 0, baseline);
        CGColorRelease(color);
        CGContextRelease(context);
        return 0;
    }
}

// viva_text_input_texture rasterizes the native editor's current appearance:
// the shown text (bullets for a password), the selection or composition, and
// the caret when it is in its visible phase. It returns a retained texture, or
// nil when the frame cannot be rasterized.
id<MTLTexture> viva_text_input_texture(
    id<MTLDevice> device,
    CGFloat scale,
    NSString *shown,
    CGFloat fontSize,
    CGFloat fontWeight,
    VivaColor *color,
    CGSize frameSize,
    NSRange selectedRange,
    BOOL hasMarked,
    NSRange markedRange,
    BOOL caretVisible,
    BOOL terminal,
    BOOL hideNativeOverlays,
    int monoFace,
    CGFloat cellWidth,
    CGFloat cellBaseline,
    int alignment
) {
    size_t pixelWidth = (size_t)MAX(1.0, ceil(frameSize.width * scale));
    size_t pixelHeight = (size_t)MAX(1.0, ceil(frameSize.height * scale));
    if (pixelWidth > 16384 || pixelHeight > 16384 || pixelWidth > SIZE_MAX / 4) {
        return nil;
    }
    size_t rowBytes = pixelWidth * 4;
    if (pixelHeight > SIZE_MAX / rowBytes) {
        return nil;
    }
    void *pixels = calloc(pixelHeight, rowBytes);
    if (pixels == NULL) {
        return nil;
    }

    CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
    CGContextRef context = CGBitmapContextCreate(
        pixels,
        pixelWidth,
        pixelHeight,
        8,
        rowBytes,
        colorSpace,
        kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little
    );
    CGColorSpaceRelease(colorSpace);
    if (context == NULL) {
        free(pixels);
        return nil;
    }

    viva_prepare_text_context(context, scale);
    viva_editor_line line = viva_new_editor_line(shown, fontSize, fontWeight, terminal, cellWidth);
    if (!viva_editor_line_made(line)) {
        CGContextRelease(context);
        free(pixels);
        return nil;
    }
    CGFloat inset = viva_editor_alignment_inset(line, frameSize.width, alignment, terminal);

    CGFloat ascent = 0;
    CGFloat descent = 0;
    CGFloat baseline = 0;
    if (terminal) {
        // Sit on the same row as the terminal's own text, whatever the composed
        // characters' fallback fonts measure. The primary font alone stands in for the
        // ascent and descent of the caret and selection, the fallbacks never taking part.
        CTFontRef gridFont = CTFontCreateWithName((CFStringRef)viva_mono_font_primary(fontSize, 0, monoFace).fontName, fontSize, NULL);
        ascent = CTFontGetAscent(gridFont);
        descent = CTFontGetDescent(gridFont);
        CFRelease(gridFont);
        baseline = cellBaseline > 0
            ? viva_fixed_baseline(frameSize.height, cellBaseline, ceil(scale))
            : viva_baseline(frameSize.height, fontSize, ceil(scale));
    } else {
        ascent = line.shaped->ascent;
        descent = line.shaped->descent;
        // The editor's text stands on the row the unfocused label stands on, so
        // focusing a field does not move it; only the caret and selection use
        // the line's own ascent and descent.
        baseline = viva_baseline(frameSize.height, fontSize, ceil(scale));
    }
    NSUInteger textLength = shown.length;
    NSRange selected = viva_clamp_range(selectedRange, textLength);
    // Fully transparent text means an editor that exists only to receive input
    // (an OTP field paints its own slots): draw none of it, caret and selection
    // included. A translucent color still gets its caret and highlight.
    CGFloat textRGBA[4];
    BOOL invisible = viva_color_srgb(color, textRGBA) && textRGBA[3] <= 0.0;
    double spans[2 * VIVA_SELECTION_PIECES];
    if (selected.length > 0 && !terminal && !hideNativeOverlays && !invisible) {
        size_t count = viva_text_spans(line, selected, terminal, spans, VIVA_SELECTION_PIECES);
        CGContextSetRGBFillColor(context, 0.18, 0.42, 0.90, 0.55);
        for (size_t piece = 0; piece < count; piece++) {
            CGContextFillRect(
                context,
                CGRectMake(inset + spans[2 * piece], baseline - descent, MAX(1.0, spans[2 * piece + 1]), ascent + descent)
            );
        }
    }

    if (terminal) {
        viva_draw_terminal_preedit(context, shown, fontSize, monoFace, color, cellWidth, baseline);
    } else {
        // The text is drawn as the one line the offsets above and below are read from, so
        // selection, caret and glyphs agree.
        viva_draw_text_line(context, line.shaped, color.CGColor, inset, baseline);
    }

    if (invisible) {
        // nothing more to draw
    } else if (hasMarked) {
        NSRange marked = viva_clamp_range(markedRange, textLength);
        size_t count = viva_text_spans(line, marked, terminal, spans, VIVA_SELECTION_PIECES);
        CGContextSetRGBFillColor(context, 0.55, 0.72, 1.0, 1.0);
        for (size_t piece = 0; piece < count; piece++) {
            // A terminal's row is only as tall as its text, so below the descenders is the
            // edge of the row, not two points further down, where nothing would be drawn.
            CGContextFillRect(context, CGRectMake(inset + spans[2 * piece], MAX(baseline - descent - 2.0, 0.0), MAX(1.0, spans[2 * piece + 1]), 1.0));
        }
    } else if (selected.length == 0 && caretVisible && !terminal && !hideNativeOverlays) {
        CGFloat caret = viva_editor_visible_caret(inset + viva_text_offset(line, selected.location, terminal), frameSize.width, alignment, terminal);
        CGFloat caretRGBA[4];
        viva_color_srgb(color, caretRGBA); // a color with no sRGB form gives zeros, as nil did
        CGContextSetRGBFillColor(context, caretRGBA[0], caretRGBA[1], caretRGBA[2], MAX(caretRGBA[3], 0.9));
        CGContextFillRect(context, CGRectMake(caret, baseline - descent, 2.0, ascent + descent));
    }

    MTLTextureDescriptor *descriptor = [MTLTextureDescriptor
        texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                     width:pixelWidth
                                    height:pixelHeight
                                 mipmapped:NO];
    descriptor.storageMode = MTLStorageModeShared;
    descriptor.usage = MTLTextureUsageShaderRead;
    id<MTLTexture> texture = [device newTextureWithDescriptor:descriptor];
    if (texture != nil) {
        [texture replaceRegion:MTLRegionMake2D(0, 0, pixelWidth, pixelHeight)
                   mipmapLevel:0
                     withBytes:pixels
                   bytesPerRow:rowBytes];
    }

    viva_editor_line_free(line);
    CGContextRelease(context);
    free(pixels);
    if (texture == nil) {
        return nil;
    }
    viva_text_texture_rebuild_count++;
    return texture;
}

int viva_measure_text(
    const unsigned char *utf8,
    size_t utf8_length,
    double font_size,
    double font_weight,
    viva_text_metrics *metrics
) {
    if (metrics == NULL || font_size <= 0 || (utf8 == NULL && utf8_length > 0)) {
        return 1;
    }
    @autoreleasepool {
        CFStringRef string = CFStringCreateWithBytes(
            kCFAllocatorDefault,
            utf8,
            utf8_length,
            kCFStringEncodingUTF8,
            false
        );
        if (string == NULL) {
            return 2;
        }

        // The font a label of this weight is drawn in (drawLineInContext).
        VivaFont *nativeFont = [VivaFont systemFontOfSize:font_size weight:font_weight];
        CTFontRef font = (CTFontRef)CFRetain(nativeFont);
        if (font == NULL) {
            CFRelease(string);
            return 3;
        }

        const void *keys[] = {kCTFontAttributeName};
        const void *values[] = {font};
        CFDictionaryRef attributes = CFDictionaryCreate(
            kCFAllocatorDefault,
            keys,
            values,
            1,
            &kCFTypeDictionaryKeyCallBacks,
            &kCFTypeDictionaryValueCallBacks
        );
        CFAttributedStringRef attributed = CFAttributedStringCreate(
            kCFAllocatorDefault,
            string,
            attributes
        );
        CTLineRef line = CTLineCreateWithAttributedString(attributed);

        CGFloat ascent = 0;
        CGFloat descent = 0;
        CGFloat leading = 0;
        double width = CTLineGetTypographicBounds(line, &ascent, &descent, &leading);
        metrics->width = width;
        metrics->height = ascent + descent + leading;
        metrics->ascent = ascent;
        metrics->descent = descent;
        metrics->leading = leading;

        CFRelease(line);
        CFRelease(attributed);
        CFRelease(attributes);
        CFRelease(font);
        CFRelease(string);
        return 0;
    }
}

int viva_measure_mono(double font_size, int mono_face, double *advance, double *ascent, double *descent, double *leading, double *rule, double *heavy) {
    if (advance == NULL || ascent == NULL || descent == NULL || leading == NULL || rule == NULL || heavy == NULL || font_size <= 0) {
        return 1;
    }
    @autoreleasepool {
        // The cell is the primary font's, whatever the face's fallbacks hold.
        VivaFont *native = viva_mono_font_primary(font_size, 0, mono_face);
        if (native == nil) {
            return 2;
        }
        CTFontRef font = CTFontCreateWithName((CFStringRef)native.fontName, font_size, NULL);
        if (font == NULL) {
            return 3;
        }
        UniChar glyphCharacter = 'M';
        CGGlyph glyph = 0;
        CTFontGetGlyphsForCharacters(font, &glyphCharacter, &glyph, 1);
        CGSize advances;
        CTFontGetAdvancesForGlyphs(font, kCTFontOrientationHorizontal, &glyph, &advances, 1);
        *advance = advances.width;
        *ascent = CTFontGetAscent(font);
        *descent = CTFontGetDescent(font);
        *leading = CTFontGetLeading(font);
        // The weight of the face's own box drawing lines: the ink width of a light and a
        // heavy stem, which the face may draw at a different weight than its underline.
        // The lookup answers false when either is missing, and still fills the other, so
        // each glyph is checked on its own.
        *rule = 0;
        *heavy = 0;
        UniChar boxCharacters[2] = {0x2502, 0x2503};  /* │ ┃ */
        CGGlyph boxGlyphs[2] = {0, 0};
        CTFontGetGlyphsForCharacters(font, boxCharacters, boxGlyphs, 2);
        CGRect bounds[2] = {CGRectZero, CGRectZero};
        CTFontGetBoundingRectsForGlyphs(font, kCTFontOrientationHorizontal, boxGlyphs, bounds, 2);
        if (boxGlyphs[0] != 0) {
            *rule = bounds[0].size.width;
        }
        if (boxGlyphs[1] != 0) {
            *heavy = bounds[1].size.width;
        }
        CFRelease(font);
        return *advance > 0 ? 0 : 4;
    }
}
