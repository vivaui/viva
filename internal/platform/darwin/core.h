// The text and rendering core of the native backend, shared by the window, the
// offscreen renderer and the native editor: system fonts, shaped lines, label
// textures, pipelines and the encoding of a scene. Functions that were private
// to the file they came from are declared here so the files that were split
// from it can reach them.
#ifndef VIVA_INTERNAL_PLATFORM_DARWIN_CORE_H
#define VIVA_INTERNAL_PLATFORM_DARWIN_CORE_H

#include <TargetConditionals.h>
#if TARGET_OS_OSX
#import <Cocoa/Cocoa.h>
#else
#import <UIKit/UIKit.h>
#endif
#include <dlfcn.h>
#import <CoreText/CoreText.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>

#include <mach/mach_time.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#if TARGET_OS_OSX
typedef NSFont VivaFont;
typedef NSFontDescriptor VivaFontDescriptor;
typedef NSColor VivaColor;
#define VivaFontFamilyAttribute NSFontFamilyAttribute
#define VivaFontTraitsAttribute NSFontTraitsAttribute
#define VivaFontWeightTrait NSFontWeightTrait
#define VivaFontCascadeListAttribute NSFontCascadeListAttribute
#define VivaFontBoldTrait NSFontBoldTrait
#else
typedef UIFont VivaFont;
typedef UIFontDescriptor VivaFontDescriptor;
typedef UIColor VivaColor;
#define VivaFontFamilyAttribute UIFontDescriptorFamilyAttribute
#define VivaFontTraitsAttribute UIFontDescriptorTraitsAttribute
#define VivaFontWeightTrait UIFontWeightTrait
#define VivaFontCascadeListAttribute UIFontDescriptorCascadeListAttribute
#define VivaFontBoldTrait UIFontDescriptorTraitBold
#endif

#include "native.h"
#include "label_geometry.h"

typedef struct {
    viva_text_line *shaped;
    NSString *grid;
    CGFloat cell;
} viva_editor_line;

typedef struct {
    BOOL onGrid;
    BOOL includesHan;
    int privateUse;
} viva_mono_family_traits;

@interface VivaTextLayer : NSObject {
    NSString *_value;
    CGRect _frame;
    CGRect _clip;     // where the label may paint, when _clipped
    BOOL _clipped;
    CGFloat _fontSize;
    CGFloat _fontWeight;
    BOOL _italic;
    CGFloat _red;
    CGFloat _green;
    CGFloat _blue;
    CGFloat _alpha;
    CGFloat _lastScale;
    size_t _textureWidth;
    size_t _textureHeight;
    CGFloat _originX;  // where the text starts within the frame, logical pixels
    CGFloat _originY;
    uint64_t _hash;
    uint32_t _fontGeneration; // the interface font generation the picture was made in
    BOOL _mono;
    int _monoFace;
    CGFloat _cellWidth;
    CGFloat _baseline;
    NSData *_utf8;       // the text as bytes, to slice into runs; only for run labels
    viva_text_run *_runs; // owned copy; NULL for an ordinary label
    size_t _runCount;
    NSData *_bitmap;
    uint32_t _bitmapWidth;
    uint32_t _bitmapHeight;
    uint32_t _bitmapStride;
    uint64_t _bitmapID;
    uint32_t _bitmapVersion;
    float _imageRadius;
    id<MTLTexture> _texture;
    NSMutableArray<id<MTLTexture>> *_additionalTextures;
    // A text label wider than a texture holds is rasterized in part: its textures hold the
    // device pixels _windowLeft to _windowRight of it, on the _windowPixelScale grid and
    // counted from the frame's left edge. Unset, the texture holds the whole label.
    BOOL _windowed;
    size_t _windowLeft;
    size_t _windowRight;
    CGFloat _windowPixelScale;
    BOOL _hasQuad;
    // The shape the label's picture is drawn into this frame, and the colour
    // multiplied into each of its corners (viva_text_label.quad, .tint). An
    // ordinary label holds the corners of its box and opaque white.
    float _quad[8];
    float _tint[16];
}
- (instancetype)initWithConfig:(viva_text_label)config;
- (BOOL)matchesConfig:(viva_text_label)config;
- (uint64_t)contentHash;
- (BOOL)contentMatchesSizeAgnostic:(viva_text_label)config;
- (void)updateFrame:(CGRect)frame;
- (void)updateClipFromConfig:(viva_text_label)config;
- (void)updateQuadFromConfig:(viva_text_label)config;
- (const float *)quad;
- (const float *)tint;
- (float)imageRadius;
- (CGRect)frame;
- (BOOL)clipped;
- (CGRect)clip;
- (BOOL)forceTextureRebuildWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale viewport:(CGSize)viewport;
- (void)markContentRebuild;
- (BOOL)rebuildWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale viewport:(CGSize)viewport;
- (BOOL)refreshWindowWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale viewport:(CGSize)viewport;
- (void)drawLineInContext:(CGContextRef)context pixelScale:(CGFloat)pixelScale;
- (void)drawRunsInContext:(CGContextRef)context pixelScale:(CGFloat)pixelScale;
- (id<MTLTexture>)textureAtIndex:(NSUInteger)index;
- (NSUInteger)textureCount;
- (CGRect)quadAtIndex:(NSUInteger)index;
- (CGRect)textureFrameAtIndex:(NSUInteger)index;
- (CGFloat)rasterWidthScale:(CGFloat)scale;
@end

// VivaRenderer draws the scene of one surface into a CAMetalLayer: the rectangle
// instances, the label layers and the native editor's texture, in the order the
// composite plan gives. The surface owns its layer and its events; the renderer
// owns what is drawn into it, so the AppKit view, the UIKit view and the
// offscreen target share one implementation of it.
@interface VivaRenderer : NSObject {
    CAMetalLayer *_layer;
    id<MTLCommandQueue> _commandQueue;
    id<MTLRenderPipelineState> _rectanglePipeline;
    id<MTLRenderPipelineState> _textPipeline;
    id<MTLBuffer> _rectangleBuffer;
    NSUInteger _rectangleCount;
    // Capacity is the slots the buffers hold; count is the instances the scene
    // owns. An append past the capacity grows it by doubling; rebuilds
    // re-tighten it to the exact count.
    NSUInteger _rectangleCapacity;
    viva_clip *_rectangleClips;
    NSMutableArray<VivaTextLayer *> *_textLabels;
    uint16_t *_textLabelLayers;
    uint32_t *_textLabelShapesBefore;
    NSUInteger _textLabelCount;
    viva_composite_command *_compositeCommands;
    NSUInteger _compositeCommandCount;
    MTLClearColor _clearColor;
}
@property(nonatomic, readonly) CAMetalLayer *layer;
@property(nonatomic, readonly) NSMutableArray<VivaTextLayer *> *textLabels;
@property(nonatomic, readonly) NSUInteger rectangleCount;
@property(nonatomic) MTLClearColor clearColor;
@property(nonatomic, readonly) id<MTLRenderPipelineState> textPipeline;

// initWithLayer takes the Metal objects the scene was first built on, and the
// scene itself; viewport is the layer's size in points, which labels are laid out
// in. It returns nil when a label or the composite plan cannot be made.
- (instancetype)initWithLayer:(CAMetalLayer *)layer
                 commandQueue:(id<MTLCommandQueue>)commandQueue
            rectanglePipeline:(id<MTLRenderPipelineState>)rectanglePipeline
              rectangleBuffer:(id<MTLBuffer>)rectangleBuffer
               rectangleCount:(NSUInteger)rectangleCount
               rectangleClips:(const viva_clip *)rectangleClips
                 textPipeline:(id<MTLRenderPipelineState>)textPipeline
                   textLabels:(const viva_text_label *)textLabels
               textLabelCount:(NSUInteger)textLabelCount
                   clearColor:(MTLClearColor)clearColor
                     viewport:(CGSize)viewport;
- (BOOL)rebuildTextLabelsWithViewport:(CGSize)viewport;
// forceRebuildTextLabelsWithViewport draws every label's texture again, as the
// surface's size and scale call for after a live resize.
- (void)forceRebuildTextLabelsWithViewport:(CGSize)viewport;
- (int)updateRectangles:(const viva_rectangle *)rectangles
                  clips:(const viva_clip *)clips
             totalCount:(NSUInteger)totalCount
                  first:(NSUInteger)first
                  count:(NSUInteger)count
                rebuild:(BOOL)rebuild;
- (int)updateLabels:(const viva_text_label *)labels
              count:(NSUInteger)count
           viewport:(CGSize)viewport;
// renderToDrawable encodes the scene, the editor's texture over it, and presents the
// drawable. The command buffer is returned so the surface can wait for it to be
// scheduled.
- (id<MTLCommandBuffer>)renderToDrawable:(id<CAMetalDrawable>)drawable
                                viewport:(CGSize)viewport
                            inputTexture:(id<MTLTexture>)inputTexture
                              inputFrame:(CGRect)inputFrame
                              inputLayer:(uint16_t)inputLayer;
// recoverWithViewport makes the Metal resources again after the device stopped
// serving them, on the system's default device. The scene must be sent again as a
// rebuild. NO when the device, its queue or the pipelines cannot be made.
- (BOOL)recoverWithViewport:(CGSize)viewport;
@end

extern int vivaTerminalCells(uint32_t character);
int viva_make_scene_resources(
    const viva_window_config *config,
    id<MTLDevice> *device,
    id<MTLCommandQueue> *queue,
    id<MTLRenderPipelineState> *rectangle_pipeline,
    id<MTLRenderPipelineState> *text_pipeline,
    id<MTLBuffer> *rectangle_buffer
);
extern void *viva_shader_library_bytes;
extern size_t viva_shader_library_length;
int viva_remember_shader_library(const unsigned char *bytes, size_t length);

BOOL viva_color_srgb(VivaColor *color, CGFloat rgba[4]);


extern uint64_t viva_gpu_submission_count;
extern uint64_t viva_gpu_draw_call_count;
extern uint64_t viva_rectangle_instance_count;
extern uint64_t viva_gpu_uploaded_bytes;
extern uint64_t viva_gpu_full_rebuild_count;
extern uint64_t viva_gpu_partial_update_count;
extern uint64_t viva_text_texture_rebuild_count;
extern uint64_t viva_label_texture_rebuild_count;
extern uint64_t viva_label_update_count;
extern NSUInteger viva_label_initial_count;


NSRange viva_clamp_range(NSRange range, NSUInteger length);
CTFontRef viva_system_font(CGFloat size, CGFloat weight);
CTFontRef viva_system_font_style(CGFloat size, CGFloat weight, BOOL italic);
CFIndex viva_cluster_start(CFStringRef string, size_t offset);
viva_text_line *viva_shared_text_line(NSString *shown, CGFloat fontSize, CGFloat weight);
viva_text_line *viva_shared_text_line_style(NSString *shown, CGFloat fontSize, CGFloat weight, BOOL italic);
BOOL viva_run_needs_slant(CTRunRef run);
void viva_draw_text_line(CGContextRef context, const viva_text_line *line, CGColorRef color, CGFloat x, CGFloat baseline);
void viva_editor_line_free(viva_editor_line line);
BOOL viva_editor_line_made(viva_editor_line line);
NSUInteger viva_terminal_step(NSString *shown, NSUInteger at, int *cells);
NSUInteger viva_terminal_column(NSString *shown, NSUInteger index);
CGFloat viva_text_offset(viva_editor_line line, NSUInteger index, BOOL terminal);
size_t viva_text_spans(viva_editor_line line, NSRange range, BOOL terminal, double *spans, size_t capacity);
NSUInteger viva_text_index_at(viva_editor_line line, NSString *shown, CGFloat x, BOOL terminal);
NSObject *viva_mono_family_lock(void);
void viva_mono_face_traits(int face, NSString **family, BOOL *classic, NSArray<NSString *> **fallbacks);
BOOL viva_mono_font_on_a_grid(VivaFont *font);
VivaFont *viva_mono_base_font(CGFloat size, CGFloat weight, NSString *family, BOOL classic);
VivaFont *viva_mono_font_primary(CGFloat size, CGFloat weight, int face);
VivaFont *viva_mono_font(CGFloat size, CGFloat weight, int face);
NSObject *viva_mono_fallback_lock(void);
VivaFont *viva_mono_family_font(NSString *family, CGFloat size, CGFloat weight);
VivaFont *viva_mono_replacement_font(NSString *family, CGFloat size, BOOL bold);
BOOL viva_mono_font_covers(CTFontRef font, NSString *value);
BOOL viva_mono_draws_han(CTFontRef font);
int viva_mono_private_use(CTFontRef font);
viva_mono_family_traits viva_mono_family_traits_of(NSString *family);
BOOL viva_mono_family_better(viva_mono_family_traits candidate, NSString *family,
                                   viva_mono_family_traits best, NSString *bestFamily);
NSString *viva_mono_find_family(NSString *cluster);
NSString *viva_mono_face_fallback(NSString *cluster, int face);
NSString *viva_mono_fallback_family(NSString *cluster);
BOOL viva_mono_is_last_resort_name(CFStringRef name);
BOOL viva_mono_font_is_last_resort(CTFontRef font);
CTLineRef viva_mono_fallback_line(NSString *value, VivaFont *base, int face, CGColorRef color);
void viva_prepare_text_context(CGContextRef context, CGFloat scale);
void viva_draw_terminal_preedit(
    CGContextRef context, NSString *shown, CGFloat fontSize, int monoFace,
    VivaColor *color, CGFloat cellWidth, CGFloat baseline
);
viva_editor_line viva_new_editor_line(NSString *shown, CGFloat fontSize, CGFloat fontWeight, BOOL terminal, CGFloat cellWidth);
CGFloat viva_editor_alignment_inset(viva_editor_line line, CGFloat width, int alignment, BOOL terminal);
CGFloat viva_editor_visible_caret(CGFloat offset, CGFloat width, int alignment, BOOL terminal);
CGFloat viva_baseline(CGFloat height, CGFloat fontSize, CGFloat pixelScale);
CGFloat viva_fixed_baseline(CGFloat height, CGFloat fromTop, CGFloat pixelScale);
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
);
id<MTLRenderPipelineState> viva_create_pipeline(
    id<MTLDevice> device,
    id<MTLLibrary> library,
    NSString *vertexName,
    NSString *fragmentName,
    NSError **error
);
uint64_t viva_hash_bytes(uint64_t hash, const void *data, size_t length);
float viva_quantized_origin(float value);
uint64_t viva_label_content_hash(viva_text_label config);
NSData *viva_downsample_bitmap(NSData *source, uint32_t sourceWidth, uint32_t sourceHeight, uint32_t sourceStride, size_t width, size_t height);
void viva_bitmap_texture_extent(CGRect frame, CGFloat scale, size_t sourceWidth, size_t sourceHeight, size_t *width, size_t *height);
BOOL viva_bitmap_extent_stale(size_t have, size_t need);
BOOL viva_label_visible_span(const float *quad, const float *tint, BOOL clipped, CGRect clip, CGSize viewport,
                                    size_t full, size_t *left, size_t *right);
void viva_label_window(size_t full, size_t limit, BOOL visible, size_t left, size_t right,
                              size_t *windowLeft, size_t *windowRight);
int viva_update_label_layers(
    NSArray<VivaTextLayer *> *previous,
    const viva_text_label *labels,
    NSUInteger count,
    id<MTLDevice> device,
    CGFloat scale,
    CGSize viewport,
    NSMutableArray<VivaTextLayer *> **updated
);
void viva_label_box_quad(float *quad, float x, float y, float width, float height);
void viva_label_plain_tint(float *tint);
size_t viva_label_tile_mesh(
    const float *quad,
    const float *tint,
    CGRect frame,
    CGRect tile,
    CGRect texture,
    viva_label_vertex *vertices
);
void viva_encode_input(
    id<MTLRenderCommandEncoder> encoder,
    id<MTLRenderPipelineState> textPipeline,
    id<MTLTexture> inputTexture,
    CGRect inputFrame,
    CGSize viewport
);
void viva_encode_scene(
    id<MTLRenderCommandEncoder> encoder,
    id<MTLRenderPipelineState> rectanglePipeline,
    id<MTLRenderPipelineState> textPipeline,
    id<MTLBuffer> rectangleBuffer,
    const viva_composite_command *commands,
    NSUInteger commandCount,
    NSArray<VivaTextLayer *> *labels,
    id<MTLTexture> inputTexture,
    CGRect inputFrame,
    uint16_t inputLayer,
    CGSize viewport,
    NSUInteger targetWidth,
    NSUInteger targetHeight
);

#endif
