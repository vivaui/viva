//go:build !ios

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <CoreText/CoreText.h>
#include "watermark_darwin.h"

static const size_t watermarkMaxTileSize = 8192;
static const size_t watermarkMaxTilePixels = 16u << 20;

@interface VivaWatermarkView : NSView {
    viva_watermark _config;
    NSString *_text, *_family;
    CALayer *_clipLayer;
    CALayer *_patternLayer;
    CGFloat _tileScale;
}
- (void)configure:(viva_watermark)config;
@end

static void watermarkDrawTile(void *info, CGContextRef context) {
    CGImageRef image = info;
    // Pattern coordinates are device pixels; the pattern matrix restores logical units.
    CGContextDrawImage(context, CGRectMake(0, 0, CGImageGetWidth(image), CGImageGetHeight(image)), image);
}

// Core Animation presents the bitmap contents upright in the flipped host.
// Only convert device pixels to logical units here; reflecting Y again would
// mirror the glyphs as well as the baseline.
static CGAffineTransform watermarkPatternMatrix(CGFloat scale) {
    return CGAffineTransformMakeScale(1 / scale, 1 / scale);
}

static void watermarkReleaseTile(void *info) {
    CGImageRelease(info);
}

// Extended linear RGB requires floating-point components. The retained tile uses
// 8-bit components, so use bounded linear sRGB to match the canvas color values.
static CGContextRef watermarkCreateTileContext(size_t width, size_t height) {
    CGColorSpaceRef space = CGColorSpaceCreateWithName(kCGColorSpaceLinearSRGB);
    CGContextRef context = CGBitmapContextCreate(NULL, width, height, 8, 0, space, (CGBitmapInfo)kCGImageAlphaPremultipliedLast);
    CGColorSpaceRelease(space);
    return context;
}

// These helpers share production font metrics, pattern ownership and coverage
// geometry with the native contract test, which does not rasterize text.
static CTLineRef watermarkCreateLine(NSString *text, NSString *family, CGFloat size, CGFloat weight, CGColorRef color) {
    NSFont *font = [NSFont systemFontOfSize:size weight:weight];
    if (family.length > 0) {
        NSFontDescriptor *descriptor = [NSFontDescriptor fontDescriptorWithFontAttributes:@{
            NSFontFamilyAttribute: family,
            NSFontTraitsAttribute: @{NSFontWeightTrait: @(weight)},
        }];
        font = [NSFont fontWithDescriptor:descriptor size:size] ?: font;
    }
    NSAttributedString *string = [[NSAttributedString alloc] initWithString:text attributes:@{
        (id)kCTFontAttributeName: font,
        (id)kCTForegroundColorAttributeName: (id)color,
    }];
    CTLineRef line = CTLineCreateWithAttributedString((CFAttributedStringRef)string);
    [string release];
    return line;
}

static CGPoint watermarkTextOrigin(CTLineRef line, CGFloat width, CGFloat height) {
    CGFloat ascent = 0, descent = 0;
    double advance = CTLineGetTypographicBounds(line, &ascent, &descent, NULL);
    return CGPointMake((width - advance) / 2, (height - ascent - descent) / 2 + descent);
}

static CGPatternRef watermarkCreatePattern(CGImageRef image, CGFloat scale) {
    CGPatternCallbacks callbacks = {0, watermarkDrawTile, watermarkReleaseTile};
    size_t width = CGImageGetWidth(image), height = CGImageGetHeight(image);
    return CGPatternCreate(image, CGRectMake(0, 0, width, height), watermarkPatternMatrix(scale), width, height,
                           kCGPatternTilingConstantSpacing, true, &callbacks);
}

static CGRect watermarkPatternBounds(CGFloat width, CGFloat height, CGAffineTransform rotation) {
    if (width <= 0 || height <= 0) {
        return CGRectZero;
    }
    return CGRectIntegral(CGRectApplyAffineTransform(CGRectMake(0, 0, width, height), CGAffineTransformInvert(rotation)));
}

@implementation VivaWatermarkView
- (instancetype)initWithFrame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (self != nil) {
        self.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
        self.wantsLayer = YES;
        self.layer.geometryFlipped = YES;
        _clipLayer = [CALayer layer];
        _clipLayer.masksToBounds = YES;
        _patternLayer = [CALayer layer];
        [_clipLayer addSublayer:_patternLayer];
        [self.layer addSublayer:_clipLayer];
    }
    return self;
}

- (BOOL)isFlipped { return YES; }
- (NSView *)hitTest:(NSPoint)point { return nil; }
- (BOOL)acceptsFirstResponder { return NO; }

- (void)dealloc {
    [_text release];
    [_family release];
    [super dealloc];
}

- (void)configure:(viva_watermark)config {
    NSString *text = [NSString stringWithUTF8String:config.text != NULL ? config.text : ""];
    NSString *family = [NSString stringWithUTF8String:config.font_family ?: ""];
    BOOL rebuild = ![_text isEqualToString:text] || ![_family isEqualToString:family] ||
        _config.red != config.red || _config.green != config.green ||
        _config.blue != config.blue || _config.alpha != config.alpha ||
        _config.font_size != config.font_size || _config.font_weight != config.font_weight ||
        _config.tile_width != config.tile_width || _config.tile_height != config.tile_height;
    [_text release];
    _text = [text copy];
    _config = config;
    _config.text = NULL;
    _config.font_family = NULL;
    [_family release];
    _family = [family copy];
    if (rebuild) {
        _tileScale = 0;
    }
    [self updatePattern];
    [self setNeedsLayout:YES];
    [self layoutSubtreeIfNeeded];
}

- (void)viewDidMoveToWindow {
    [super viewDidMoveToWindow];
    [self updatePattern];
}

- (void)viewDidChangeBackingProperties {
    [super viewDidChangeBackingProperties];
    [self updatePattern];
}

- (void)updatePattern {
    // Wait for the actual backing scale before rasterizing.
    if (self.window == nil) {
        return;
    }
    CGFloat scale = self.window.backingScaleFactor;
    if (scale <= 0) {
        scale = 1;
    }
    if (_tileScale == scale || _text.length == 0) {
        return;
    }
    size_t width = ceil(_config.tile_width * scale);
    size_t height = ceil(_config.tile_height * scale);
    if (width == 0 || height == 0 || width > watermarkMaxTileSize || height > watermarkMaxTileSize || width * height > watermarkMaxTilePixels) {
        return;
    }
    CGContextRef context = watermarkCreateTileContext(width, height);
    if (context == NULL) {
        return;
    }
    CGColorSpaceRef space = CGBitmapContextGetColorSpace(context);
    CGContextScaleCTM(context, scale, scale);
    CGFloat channels[] = {_config.red, _config.green, _config.blue, _config.alpha};
    CGColorRef color = CGColorCreate(space, channels);
    CTLineRef line = watermarkCreateLine(_text, _family, _config.font_size, _config.font_weight, color);
    CGPoint origin = watermarkTextOrigin(line, _config.tile_width, _config.tile_height);
    CGContextSetTextPosition(context, origin.x, origin.y);
    CTLineDraw(line, context);
    CFRelease(line);
    CGColorRelease(color);
    CGImageRef image = CGBitmapContextCreateImage(context);
    CGContextRelease(context);
    if (image == NULL) {
        return;
    }
    CGPatternRef pattern = watermarkCreatePattern(image, scale);
    if (pattern == NULL) {
        CGImageRelease(image);
        return;
    }
    CGColorSpaceRef patternSpace = CGColorSpaceCreatePattern(NULL);
    CGFloat alpha = 1;
    CGColorRef patternColor = CGColorCreateWithPattern(patternSpace, pattern, &alpha);
    [CATransaction begin];
    [CATransaction setDisableActions:YES];
    _patternLayer.backgroundColor = patternColor;
    [CATransaction commit];
    CGColorRelease(patternColor);
    CGColorSpaceRelease(patternSpace);
    CGPatternRelease(pattern);
    _tileScale = scale;
}

- (void)layout {
    [super layout];
    CGFloat width = MAX(0, self.bounds.size.width - _config.left - _config.right);
    CGFloat height = MAX(0, self.bounds.size.height - _config.top - _config.bottom);
    CGAffineTransform rotation = CGAffineTransformMakeRotation(_config.angle * M_PI / 180);
    CGRect bounds = watermarkPatternBounds(width, height, rotation);
    [CATransaction begin];
    [CATransaction setDisableActions:YES];
    _clipLayer.frame = CGRectMake(_config.left, _config.top, width, height);
    // Anchor pattern coordinate (0,0) at the clip origin. Changing size changes
    // only the inverse-rotated coverage rectangle, never the pattern phase.
    _patternLayer.bounds = bounds;
    _patternLayer.anchorPoint = CGPointMake(bounds.size.width > 0 ? -bounds.origin.x / bounds.size.width : 0,
                                           bounds.size.height > 0 ? -bounds.origin.y / bounds.size.height : 0);
    _patternLayer.position = CGPointZero;
    _patternLayer.affineTransform = rotation;
    [CATransaction commit];
}
@end

static VivaWatermarkView *watermarkIn(NSView *host) {
    for (NSView *view in host.subviews) {
        if ([view isKindOfClass:[VivaWatermarkView class]]) {
            return (VivaWatermarkView *)view;
        }
    }
    return nil;
}

// Native view creators call this after changing the host's view tree.
void viva_raise_watermark(void *value) {
    NSView *host = value;
    VivaWatermarkView *view = watermarkIn(host);
    if (view != nil) {
        [host addSubview:view positioned:NSWindowAbove relativeTo:nil];
    }
}

void viva_watermark_apply(void *value, viva_watermark config) {
    NSView *host = value;
    if (host == nil) {
        return;
    }
    VivaWatermarkView *view = watermarkIn(host);
    if (config.text == NULL || config.text[0] == 0 || config.alpha <= 0) {
        [view removeFromSuperview];
        return;
    }
    if (view == nil) {
        view = [[[VivaWatermarkView alloc] initWithFrame:host.bounds] autorelease];
        [host addSubview:view positioned:NSWindowAbove relativeTo:nil];
    }
    [view configure:config];
    viva_raise_watermark(host);
}
