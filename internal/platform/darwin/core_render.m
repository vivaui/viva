#import "core.h"
#include "label_geometry.h"

/* The compiled shader library the application passes as it opens its windows,
 * kept after the first one compiled its pipelines: a window that makes its
 * Metal resources again builds a library from these bytes on the device it is
 * given, which need not be the device the first library was compiled for. */
void *viva_shader_library_bytes = NULL;
size_t viva_shader_library_length = 0;

int viva_remember_shader_library(const unsigned char *bytes, size_t length) {
    if (bytes == NULL || length == 0) {
        return 0;
    }
    if (viva_shader_library_bytes != NULL && viva_shader_library_length == length &&
        memcmp(viva_shader_library_bytes, bytes, length) == 0) {
        return 1;
    }
    void *copy = malloc(length);
    if (copy == NULL) {
        return 0;
    }
    memcpy(copy, bytes, length);
    free(viva_shader_library_bytes);
    viva_shader_library_bytes = copy;
    viva_shader_library_length = length;
    return 1;
}

uint16_t viva_rectangle_layer(viva_rectangle rectangle) {
    if (rectangle.reserved0 <= 0) {
        return 0;
    }
    if (rectangle.reserved0 >= UINT16_MAX) {
        return UINT16_MAX;
    }
    return (uint16_t)rectangle.reserved0;
}

static int viva_compare_composite_commands(const void *left, const void *right) {
    const viva_composite_command *a = left;
    const viva_composite_command *b = right;
    if (a->layer != b->layer) {
        return a->layer < b->layer ? -1 : 1;
    }
    if (a->sequence != b->sequence) {
        return a->sequence < b->sequence ? -1 : 1;
    }
    if (a->kind != b->kind) {
        return a->kind < b->kind ? -1 : 1;
    }
    if (a->first != b->first) {
        return a->first < b->first ? -1 : 1;
    }
    return 0;
}

// viva_same_batch reports whether two instances can share one draw: the same
// layer and the same clip. An instance no clip cuts ignores its stale clip fields.
static int viva_same_batch(
    const viva_rectangle *rectangles,
    const viva_clip *clips,
    size_t left,
    size_t right
) {
    if (viva_rectangle_layer(rectangles[left]) != viva_rectangle_layer(rectangles[right])) {
        return 0;
    }
    int left_clipped = clips != NULL && clips[left].clipped != 0;
    int right_clipped = clips != NULL && clips[right].clipped != 0;
    if (left_clipped != right_clipped) {
        return 0;
    }
    if (!left_clipped) {
        return 1;
    }
    return clips[left].x == clips[right].x && clips[left].y == clips[right].y &&
           clips[left].width == clips[right].width &&
           clips[left].height == clips[right].height;
}

/* viva_rectangle_capacity answers the instance slots the renderer's rectangle
 * buffers need to hold count instances, given the slots they already hold. A
 * buffer that has room keeps it, so an append landing in the headroom of an
 * earlier growth costs nothing; a buffer that must grow doubles, so a scene
 * that appends one instance at a time allocates a logarithmic number of times
 * and the buffers stay within twice the instances that exist. */
size_t viva_rectangle_capacity(size_t capacity, size_t count) {
    if (capacity >= count) {
        return capacity;
    }
    size_t next = capacity > 0 ? capacity : 64;
    while (next < count) {
        if (next > SIZE_MAX / 2) {
            return count;
        }
        next *= 2;
    }
    return next;
}

int viva_plan_composite(
    const viva_rectangle *rectangles,
    const viva_clip *clips,
    size_t rectangle_count,
    const uint16_t *text_layers,
    const uint32_t *text_shapes_before,
    size_t text_count,
    viva_composite_command *commands,
    size_t command_capacity,
    size_t *command_count
) {
    if (command_count == NULL ||
        (rectangle_count > 0 && (rectangles == NULL || clips == NULL)) ||
        (text_count > 0 && (text_layers == NULL || text_shapes_before == NULL)) ||
        rectangle_count > UINT32_MAX || text_count > UINT32_MAX) {
        return 1;
    }

    // At most one command per shape and one per label: a shape batch can be cut
    // at every label boundary.
    if (rectangle_count > SIZE_MAX - text_count) {
        return 2;
    }
    size_t required = rectangle_count + text_count;
    if (required > command_capacity || (required > 0 && commands == NULL)) {
        return 2;
    }

    // Shapes and labels arrive in paint order. A label that says k shapes were
    // painted before it belongs after instance k-1 and before instance k, so
    // the plan walks both lists by that boundary, cutting a shape batch when a
    // label falls inside it.
    size_t command_index = 0;
    size_t rectangle_index = 0;
    size_t text_index = 0;
    while (rectangle_index < rectangle_count || text_index < text_count) {
        if (text_index < text_count &&
            (text_shapes_before[text_index] <= rectangle_index || rectangle_index >= rectangle_count)) {
            uint32_t sequence = (uint32_t)command_index;
            commands[command_index++] = (viva_composite_command){
                .layer = text_layers[text_index],
                .kind = VIVA_COMPOSITE_TEXT,
                .first = (uint32_t)text_index,
                .count = 1,
                .sequence = sequence,
            };
            text_index++;
            continue;
        }
        size_t first = rectangle_index;
        size_t end = first + 1;
        while (end < rectangle_count &&
               viva_same_batch(rectangles, clips, first, end) &&
               (text_index >= text_count || end < text_shapes_before[text_index])) {
            end++;
        }
        uint16_t layer = viva_rectangle_layer(rectangles[first]);
        uint32_t sequence = (uint32_t)command_index;
        viva_composite_command command = {
            .layer = layer,
            .kind = VIVA_COMPOSITE_RECTANGLES,
            .first = (uint32_t)first,
            .count = (uint32_t)(end - first),
            .sequence = sequence,
        };
        if (clips[first].clipped != 0) {
            command.clipped = 1;
            command.clip_x = clips[first].x;
            command.clip_y = clips[first].y;
            command.clip_width = clips[first].width;
            command.clip_height = clips[first].height;
        }
        commands[command_index++] = command;
        rectangle_index = end;
    }
    qsort(commands, command_index, sizeof(viva_composite_command),
          viva_compare_composite_commands);
    *command_count = command_index;
    return 0;
}


uint64_t viva_gpu_submission_count;
uint64_t viva_gpu_draw_call_count;
uint64_t viva_rectangle_instance_count;
uint64_t viva_gpu_uploaded_bytes;
uint64_t viva_gpu_full_rebuild_count;
uint64_t viva_gpu_partial_update_count;
uint64_t viva_text_texture_rebuild_count;
uint64_t viva_label_texture_rebuild_count;
uint64_t viva_label_update_count;
NSUInteger viva_label_initial_count;

// viva_fragment_blends_premultiplied says how a fragment function's output must
// be blended. Rectangles output a straight color and blend with SourceAlpha.
// Text returns texels CoreGraphics already multiplied by coverage, so it blends
// with One; SourceAlpha would multiply by alpha twice and thin every glyph edge
// by roughly a fifth.
int viva_fragment_blends_premultiplied(const char *fragmentName) {
    return fragmentName != NULL && strcmp(fragmentName, "viva_text_fragment") == 0;
}

id<MTLRenderPipelineState> viva_create_pipeline(
    id<MTLDevice> device,
    id<MTLLibrary> library,
    NSString *vertexName,
    NSString *fragmentName,
    NSError **error
) {
    BOOL premultipliedSource = viva_fragment_blends_premultiplied(fragmentName.UTF8String) != 0;
    id<MTLFunction> vertex = [library newFunctionWithName:vertexName];
    id<MTLFunction> fragment = [library newFunctionWithName:fragmentName];
    if (vertex == nil || fragment == nil) {
        [fragment release];
        [vertex release];
        return nil;
    }
    MTLRenderPipelineDescriptor *descriptor =
        [[[MTLRenderPipelineDescriptor alloc] init] autorelease];
    descriptor.vertexFunction = vertex;
    descriptor.fragmentFunction = fragment;
    descriptor.colorAttachments[0].pixelFormat = MTLPixelFormatBGRA8Unorm;
    descriptor.colorAttachments[0].blendingEnabled = YES;
    descriptor.colorAttachments[0].rgbBlendOperation = MTLBlendOperationAdd;
    descriptor.colorAttachments[0].alphaBlendOperation = MTLBlendOperationAdd;
    descriptor.colorAttachments[0].sourceRGBBlendFactor =
        premultipliedSource ? MTLBlendFactorOne : MTLBlendFactorSourceAlpha;
    descriptor.colorAttachments[0].destinationRGBBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
    descriptor.colorAttachments[0].sourceAlphaBlendFactor = MTLBlendFactorOne;
    descriptor.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
    id<MTLRenderPipelineState> pipeline = [device
        newRenderPipelineStateWithDescriptor:descriptor
                                      error:error];
    [fragment release];
    [vertex release];
    return pipeline;
}

uint64_t viva_hash_bytes(uint64_t hash, const void *data, size_t length) {
    const unsigned char *bytes = (const unsigned char *)data;
    for (size_t index = 0; index < length; index++) {
        hash = (hash ^ bytes[index]) * 1099511628211ULL;
    }
    return hash;
}

// viva_plan_label_reuse decides which old label texture each new label can take
// over. source[i] is the index of the old label new label i reuses, or -1 when
// it needs a texture of its own. A label first looks at its own index (the
// common case: nothing moved), then at any unused old label with the same
// hash anywhere (a list or terminal that scrolled). Each old label is used at
// most once. Equal hashes only propose a match; the caller confirms it.
int viva_plan_label_reuse(
    const uint64_t *old_hashes,
    size_t old_count,
    const uint64_t *new_hashes,
    size_t new_count,
    int64_t *source
) {
    if ((new_count > 0 && source == NULL) || (old_count > 0 && old_hashes == NULL) || (new_count > 0 && new_hashes == NULL)) {
        return 1;
    }
    size_t table_size = 16;
    while (table_size < old_count * 2) {
        table_size <<= 1;
    }
    int64_t *heads = malloc(table_size * sizeof(int64_t));
    int64_t *next = malloc(MAX(old_count, 1) * sizeof(int64_t));
    unsigned char *taken = calloc(MAX(old_count, 1), 1);
    if (heads == NULL || next == NULL || taken == NULL) {
        free(heads);
        free(next);
        free(taken);
        return 2;
    }
    for (size_t slot = 0; slot < table_size; slot++) {
        heads[slot] = -1;
    }
    for (size_t old = 0; old < old_count; old++) {
        size_t slot = (size_t)(old_hashes[old] & (table_size - 1));
        next[old] = heads[slot];
        heads[slot] = (int64_t)old;
    }
    for (size_t index = 0; index < new_count; index++) {
        source[index] = -1;
        if (index < old_count && !taken[index] && old_hashes[index] == new_hashes[index]) {
            source[index] = (int64_t)index;
            taken[index] = 1;
        }
    }
    for (size_t index = 0; index < new_count; index++) {
        if (source[index] >= 0) {
            continue;
        }
        size_t slot = (size_t)(new_hashes[index] & (table_size - 1));
        for (int64_t old = heads[slot]; old >= 0; old = next[old]) {
            if (!taken[old] && old_hashes[old] == new_hashes[index]) {
                source[index] = old;
                taken[old] = 1;
                break;
            }
        }
    }
    free(heads);
    free(next);
    free(taken);
    return 0;
}

// viva_quantized_origin is a label's sub-pixel origin to a sixteenth of a logical pixel,
// finer than Core Graphics places a glyph (a fifth of a device pixel at best), so labels
// that differ by less share a texture.
float viva_quantized_origin(float value) {
    return roundf(value * 16.0f) / 16.0f;
}

// viva_label_content_hash identifies what a label looks like: its text, runs,
// font and color. Position and layer are not part of it. Labels with equal
// hashes are candidates for sharing one texture; matchesConfig confirms.
uint64_t viva_label_content_hash(viva_text_label config) {
    uint64_t hash = 14695981039346656037ULL;
    hash = viva_hash_bytes(hash, config.utf8, config.utf8_length);
    // Bitmap textures retain their source dimensions. Resizing their draw quad
    // must find the same texture when Go omits an unchanged pixel payload.
    BOOL bitmap = (config.flags & VIVA_LABEL_BITMAP) != 0;
    float values[] = {bitmap ? 0 : config.width, bitmap ? 0 : config.height, config.font_size, config.font_weight,
                      config.red, config.green, config.blue, config.alpha, config.cell_width,
                      viva_quantized_origin(config.origin_x), viva_quantized_origin(config.origin_y), config.baseline};
    hash = viva_hash_bytes(hash, values, sizeof(values));
    uint32_t contentFlags = config.flags & ~VIVA_LABEL_CLIPPED;
    hash = viva_hash_bytes(hash, &contentFlags, sizeof(contentFlags));
    hash = viva_hash_bytes(hash, &config.mono_face, sizeof(config.mono_face));
    hash = viva_hash_bytes(hash, &config.font_generation, sizeof(config.font_generation));
    if (config.runs != NULL) {
        hash = viva_hash_bytes(hash, config.runs, config.run_count * sizeof(viva_text_run));
    }
    uint64_t bitmapIdentity[] = {
        config.bitmap_id,
        config.bitmap_version,
        config.bitmap_width,
        config.bitmap_height,
        config.bitmap_stride,
    };
    hash = viva_hash_bytes(hash, bitmapIdentity, sizeof(bitmapIdentity));
    if (config.bitmap != NULL && config.bitmap_id == 0) {
        hash = viva_hash_bytes(hash, config.bitmap, config.bitmap_length);
    }
    return hash;
}


NSData *viva_downsample_bitmap(NSData *source, uint32_t sourceWidth, uint32_t sourceHeight, uint32_t sourceStride, size_t width, size_t height) {
    if (source == nil || width == 0 || height == 0 || width > sourceWidth || height > sourceHeight || sourceStride < sourceWidth * 4) {
        return nil;
    }
    if (sourceHeight - 1 > SIZE_MAX / sourceStride || source.length < (size_t)(sourceHeight - 1) * sourceStride + (size_t)sourceWidth * 4 || width > SIZE_MAX / 4 || height > SIZE_MAX / (width * 4)) {
        return nil;
    }
    size_t rowBytes = width * 4;
    NSMutableData *pixels = [[NSMutableData alloc] initWithLength:height * rowBytes];
    if (pixels == nil) {
        return nil;
    }
    CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
    CGDataProviderRef provider = CGDataProviderCreateWithData(NULL, source.bytes, source.length, NULL);
    CGImageRef image = provider == NULL ? NULL : CGImageCreate(
        sourceWidth, sourceHeight, 8, 32, sourceStride, colorSpace,
        kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little,
        provider, NULL, true, kCGRenderingIntentDefault);
    CGContextRef context = image == NULL ? NULL : CGBitmapContextCreate(
        pixels.mutableBytes, width, height, 8, rowBytes, colorSpace,
        kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little);
    if (context != NULL) {
        CGContextSetInterpolationQuality(context, kCGInterpolationHigh);
        CGContextDrawImage(context, CGRectMake(0, 0, width, height), image);
    }
    if (context != NULL) {
        CGContextRelease(context);
    }
    if (image != NULL) {
        CGImageRelease(image);
    }
    if (provider != NULL) {
        CGDataProviderRelease(provider);
    }
    CGColorSpaceRelease(colorSpace);
    if (context == NULL) {
        [pixels release];
        return nil;
    }
    return pixels;
}

// viva_bitmap_texture_extent is the size of the texture an image label is drawn from: the
// stored image, averaged down to the device pixels the label covers when it is drawn smaller
// than it is stored. internal/platform.BitmapTextureExtent computes the same size.
void viva_bitmap_texture_extent(CGRect frame, CGFloat scale, size_t sourceWidth, size_t sourceHeight, size_t *width, size_t *height) {
    CGFloat pixelScale = MAX(scale, 1.0);
    size_t shownWidth = (size_t)MAX(1.0, floor((frame.origin.x + frame.size.width) * pixelScale + 0.5) - floor(frame.origin.x * pixelScale + 0.5));
    size_t shownHeight = (size_t)MAX(1.0, floor((frame.origin.y + frame.size.height) * pixelScale + 0.5) - floor(frame.origin.y * pixelScale + 0.5));
    *width = sourceWidth;
    *height = sourceHeight;
    if (shownWidth <= sourceWidth && shownHeight <= sourceHeight && (shownWidth < sourceWidth || shownHeight < sourceHeight)) {
        *width = shownWidth;
        *height = shownHeight;
    }
}

// viva_bitmap_extent_stale reports whether a texture of one dimension no longer suits an image
// that needs another: smaller than needed would blur it, and more than twice as large would
// alias it. Between those the texture stays, so an animated size does not rebuild every frame.
// internal/platform.BitmapTextureStale applies the same rule.
BOOL viva_bitmap_extent_stale(size_t have, size_t need) {
    return need > have || have >= 2 * need;
}

// VIVA_LABEL_TEXTURE_LIMIT is the widest and tallest texture a label is rasterized into, in
// pixels: the 2D texture limit of every Metal device Viva runs on.
#define VIVA_LABEL_TEXTURE_LIMIT 16384
#define VIVA_LABEL_TILE_WIDTH (VIVA_LABEL_TEXTURE_LIMIT - 2)

// viva_label_visible_span clips the destination triangles to the clip and viewport,
// then maps their visible UV range to source pixels, at most full.
// NO when none of the label can be seen.
BOOL viva_label_visible_span(const float *quad, const float *tint, BOOL clipped, CGRect clip, CGSize viewport,
                                    size_t full, size_t *left, size_t *right) {
    CGRect visible = CGRectMake(0, 0, viewport.width, viewport.height);
    if (clipped) {
        visible = CGRectIntersection(visible, clip);
    }
    if (CGRectIsEmpty(visible)) {
        return NO;
    }
    float sourceLeft, sourceRight;
    if (!viva_label_source_span(quad, tint, CGRectGetMinX(visible), CGRectGetMinY(visible), CGRectGetMaxX(visible), CGRectGetMaxY(visible), &sourceLeft, &sourceRight)) {
        return NO;
    }
    CGFloat from = floor((double)sourceLeft * full);
    CGFloat to = ceil((double)sourceRight * full);
    *left = (size_t)MIN(MAX(from, 0), (CGFloat)full);
    *right = (size_t)MIN(MAX(to, 0), (CGFloat)full);
    return *right > *left;
}

// viva_label_window chooses the device pixels of a label full pixels wide that a texture of
// at most limit pixels holds. The visible span left to right is kept with as much margin on
// both sides as the span itself, so a scroll of up to a viewport keeps the texture; at an
// end of the label the margin goes to the other side. A span wider than limit is split
// across textures, without a margin. A label none of which is visible keeps its start.
void viva_label_window(size_t full, size_t limit, BOOL visible, size_t left, size_t right,
                              size_t *windowLeft, size_t *windowRight) {
    size_t width = MIN(full, limit);
    if (!visible) {
        *windowLeft = 0;
        *windowRight = width;
        return;
    }
    size_t span = right - left;
    if (span >= width) {
        *windowLeft = left;
        *windowRight = right;
        return;
    }
    size_t margin = MIN(span, (width - span) / 2);
    width = span + 2 * margin;
    size_t low = left > margin ? left - margin : 0;
    if (low + width > full) {
        low = full - width;
    }
    *windowLeft = low;
    *windowRight = low + width;
}

@implementation VivaTextLayer

- (instancetype)initWithConfig:(viva_text_label)config {
    self = [super init];
    if (self != nil) {
        // Bitmap labels carry no UTF-8 buffer. Give them an explicit empty
        // string so image-only labels can still own a Metal texture.
        if (config.utf8_length == 0) {
            _value = [@"" retain];
        } else if (config.utf8 == NULL) {
            [self release];
            return nil;
        } else {
            _value = [[NSString alloc] initWithBytes:config.utf8
                                              length:config.utf8_length
                                            encoding:NSUTF8StringEncoding];
        }
        if (_value == nil) {
            [self release];
            return nil;
        }
        _frame = CGRectMake(config.x, config.y, config.width, config.height);
        [self updateClipFromConfig:config];
        [self updateQuadFromConfig:config];
        _fontSize = config.font_size;
        _fontWeight = config.font_weight;
        _italic = (config.flags & VIVA_LABEL_ITALIC) != 0;
        _red = config.red;
        _green = config.green;
        _blue = config.blue;
        _alpha = config.alpha;
        _lastScale = 0;
        _originX = viva_quantized_origin(config.origin_x);
        _originY = viva_quantized_origin(config.origin_y);
        _hash = viva_label_content_hash(config);
        _fontGeneration = config.font_generation;
        _mono = (config.flags & VIVA_LABEL_MONO) != 0;
        _monoFace = config.mono_face;
        _cellWidth = config.cell_width;
        _baseline = config.baseline;
        if (config.run_count > 0 && config.runs != NULL) {
            _runs = malloc(config.run_count * sizeof(viva_text_run));
            if (_runs == NULL) {
                [self release];
                return nil;
            }
            memcpy(_runs, config.runs, config.run_count * sizeof(viva_text_run));
            _runCount = config.run_count;
            _utf8 = [[NSData alloc] initWithBytes:config.utf8 length:config.utf8_length];
        }
        if ((config.flags & VIVA_LABEL_BITMAP) != 0) {
            // An image the Go side did not send has nothing to draw from: fail the update
            // rather than keep a layer that renders blank.
            if (config.bitmap == NULL) {
                [self release];
                return nil;
            }
            _bitmap = [[NSData alloc] initWithBytes:config.bitmap length:config.bitmap_length];
            _bitmapWidth = config.bitmap_width;
            _bitmapHeight = config.bitmap_height;
            _bitmapStride = config.bitmap_stride;
            _bitmapID = config.bitmap_id;
            _bitmapVersion = config.bitmap_version;
        }
    }
    return self;
}

- (void)dealloc {
    [_texture release];
    [_additionalTextures release];
    [_value release];
    [_utf8 release];
    [_bitmap release];
    free(_runs);
    [super dealloc];
}

- (uint64_t)contentHash {
    return _hash;
}

- (BOOL)matchesConfig:(viva_text_label)config {
    if (![self contentMatchesSizeAgnostic:config]) {
        return NO;
    }
    if (_bitmap != nil) {
        return YES;
    }
    return _frame.size.width == config.width && _frame.size.height == config.height;
}

- (BOOL)contentMatchesSizeAgnostic:(viva_text_label)config {
    // Content identity: font, color, and text. Position never matters (the
    // quad supplies it each draw); size matters only for texel alignment,
    // which live resize may temporarily relax.
    if (_fontSize != config.font_size || _fontWeight != config.font_weight || _fontGeneration != config.font_generation) {
        return NO;
    }
    if (_italic != ((config.flags & VIVA_LABEL_ITALIC) != 0)) {
        return NO;
    }
    if (_originX != viva_quantized_origin(config.origin_x) || _originY != viva_quantized_origin(config.origin_y)) {
        return NO;
    }
    if (_red != config.red || _green != config.green ||
        _blue != config.blue || _alpha != config.alpha) {
        return NO;
    }
    if (_mono != ((config.flags & VIVA_LABEL_MONO) != 0) ||
        _monoFace != config.mono_face || _cellWidth != config.cell_width || _baseline != config.baseline ||
        _runCount != (config.runs == NULL ? 0 : config.run_count) ||
        (_runCount > 0 && memcmp(_runs, config.runs, _runCount * sizeof(viva_text_run)) != 0)) {
        return NO;
    }
    BOOL bitmap = (config.flags & VIVA_LABEL_BITMAP) != 0;
    if ((_bitmap != nil) != bitmap) {
        return NO;
    }
    if (bitmap) {
        if (_bitmapWidth != config.bitmap_width || _bitmapHeight != config.bitmap_height ||
            _bitmapStride != config.bitmap_stride || _bitmapID != config.bitmap_id ||
            _bitmapVersion != config.bitmap_version || _bitmap.length != config.bitmap_length) {
            return NO;
        }
        if (config.bitmap_upload != 0 && (config.bitmap == NULL || memcmp(_bitmap.bytes, config.bitmap, config.bitmap_length) != 0)) {
            return NO;
        }
    }
    size_t valueLength = [_value lengthOfBytesUsingEncoding:NSUTF8StringEncoding];
    if (config.utf8_length != valueLength) {
        return NO;
    }
    if (config.utf8_length == 0) {
        return YES;
    }
    if (config.utf8 == NULL) {
        return NO;
    }
    return memcmp(config.utf8, [_value UTF8String], config.utf8_length) == 0;
}

- (void)updateClipFromConfig:(viva_text_label)config {
    _clipped = (config.flags & VIVA_LABEL_CLIPPED) != 0;
    _clip = _clipped ? CGRectMake(config.clip_x, config.clip_y, config.clip_width, config.clip_height) : CGRectZero;
}

- (void)updateQuadFromConfig:(viva_text_label)config {
    // The quad is where the label lands, not what it looks like: it changes
    // every frame of an animation while the texture stays the same one.
    BOOL hasQuad = NO;
    for (int index = 0; index < 8; index++) {
        _quad[index] = config.quad[index];
        hasQuad = hasQuad || config.quad[index] != 0;
    }
    for (int index = 0; index < 16; index++) {
        _tint[index] = config.tint[index];
    }
    _hasQuad = hasQuad;
    _imageRadius = hasQuad ? 0 : config.image_radius;
    if (hasQuad) {
        return;
    }
    // An ordinary label keeps its box as the quad and opaque white as its tint,
    // so the renderer has no case to special-case per frame.
    _quad[0] = config.x;
    _quad[1] = config.y;
    _quad[2] = config.x + config.width;
    _quad[3] = config.y;
    _quad[4] = config.x;
    _quad[5] = config.y + config.height;
    _quad[6] = config.x + config.width;
    _quad[7] = config.y + config.height;
    for (int index = 0; index < 16; index++) {
        _tint[index] = 1;
    }
    if (config.image_fade > 0) {
        // Fade is draw-time: it scales the premultiplied tint, so the picture's
        // texture is untouched.
        float alpha = 1 - config.image_fade;
        for (int index = 0; index < 16; index++) {
            _tint[index] *= alpha;
        }
    }
}

- (const float *)quad {
    return _quad;
}

- (const float *)tint {
    return _tint;
}

- (float)imageRadius {
    return _imageRadius;
}

- (BOOL)clipped {
    return _clipped;
}

- (CGRect)clip {
    return _clip;
}

- (void)updateFrame:(CGRect)frame {
    _frame = frame;
}

- (BOOL)forceTextureRebuildWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale viewport:(CGSize)viewport {
    _lastScale = 0;
    return [self rebuildWithDevice:device scale:scale viewport:viewport];
}

- (void)markContentRebuild {
    viva_label_texture_rebuild_count++;
}

// refreshWindowWithDevice rasterizes a label held in part again when the part of it that
// can be seen in viewport is no longer all in its texture: it moved, or its clip or the
// viewport did. Any other label is left as it is.
- (BOOL)refreshWindowWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale viewport:(CGSize)viewport {
    CGFloat widthScale = [self rasterWidthScale:scale];
    if (_texture == nil || _lastScale != scale ||
        viva_bitmap_extent_stale((size_t)ceil(_frame.size.width * _windowPixelScale), (size_t)ceil(_frame.size.width * widthScale))) {
        _lastScale = 0;
        BOOL rebuilt = [self rebuildWithDevice:device scale:scale viewport:viewport];
        if (rebuilt) [self markContentRebuild];
        return rebuilt;
    }
    if (!_windowed) {
        return YES;
    }
    size_t full = (size_t)MAX(1.0, ceil(_frame.size.width * _windowPixelScale));
    size_t left = 0, right = 0;
    if (!viva_label_visible_span(_quad, _tint, _clipped, _clip, viewport, full, &left, &right) ||
        (left >= _windowLeft && right <= _windowRight)) {
        return YES;
    }
    _lastScale = 0;
    if (![self rebuildWithDevice:device scale:scale viewport:viewport]) {
        return NO;
    }
    [self markContentRebuild];
    return YES;
}

- (CGFloat)rasterWidthScale:(CGFloat)scale {
    CGFloat pixelScale = ceil(scale);
    if (!_hasQuad || _frame.size.width <= 0) {
        return pixelScale;
    }
    CGFloat top = hypot((double)_quad[2] - _quad[0], (double)_quad[3] - _quad[1]);
    CGFloat bottom = hypot((double)_quad[6] - _quad[4], (double)_quad[7] - _quad[5]);
    return MIN(pixelScale, MAX(1, ceil(MAX(top, bottom) * pixelScale)) / _frame.size.width);
}

- (BOOL)rebuildWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale viewport:(CGSize)viewport {
    size_t wantWidth = 0, wantHeight = 0;
    if (_bitmap != nil) {
        viva_bitmap_texture_extent(_frame, scale, _bitmapWidth, _bitmapHeight, &wantWidth, &wantHeight);
    }
    if (_texture != nil && (_bitmap != nil
            ? !viva_bitmap_extent_stale(_textureWidth, wantWidth) && !viva_bitmap_extent_stale(_textureHeight, wantHeight)
            : _lastScale == scale)) {
        return YES;
    }
    if (_bitmap != nil) {
        if (_bitmapWidth == 0 || _bitmapHeight == 0 ||
            _bitmapStride < (size_t)_bitmapWidth * 4 || _bitmapHeight - 1 > SIZE_MAX / _bitmapStride ||
            _bitmap.length < (size_t)(_bitmapHeight - 1) * _bitmapStride + (size_t)_bitmapWidth * 4) {
            return NO;
        }
        size_t uploadWidth = _bitmapWidth, uploadHeight = _bitmapHeight;
        NSData *uploadData = _bitmap;
        uint32_t uploadStride = _bitmapStride;
        if (wantWidth != _bitmapWidth || wantHeight != _bitmapHeight) {
            NSData *reduced = viva_downsample_bitmap(_bitmap, _bitmapWidth, _bitmapHeight, _bitmapStride, wantWidth, wantHeight);
            if (reduced == nil) {
                return NO;
            }
            uploadData = reduced;
            uploadWidth = wantWidth;
            uploadHeight = wantHeight;
            uploadStride = (uint32_t)(wantWidth * 4);
        }
        MTLTextureDescriptor *descriptor = [MTLTextureDescriptor
            texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                         width:uploadWidth
                                        height:uploadHeight
                                     mipmapped:NO];
        descriptor.storageMode = MTLStorageModeShared;
        descriptor.usage = MTLTextureUsageShaderRead;
        id<MTLTexture> texture = [device newTextureWithDescriptor:descriptor];
        if (texture != nil) {
            [texture replaceRegion:MTLRegionMake2D(0, 0, uploadWidth, uploadHeight)
                       mipmapLevel:0
                         withBytes:uploadData.bytes
                       bytesPerRow:uploadStride];
        }
        if (uploadData != _bitmap) {
            [uploadData release];
        }
        if (texture == nil) {
            return NO;
        }
        [_texture release];
        _texture = texture;
        _textureWidth = uploadWidth;
        _textureHeight = uploadHeight;
        _lastScale = scale;
        return YES;
    }
    // Rasterize on the integer physical-pixel grid: a fractional frame origin
    // must not bleed anti-aliasing across texel boundaries.
    CGFloat pixelScale = ceil(scale);
    CGFloat widthScale = [self rasterWidthScale:scale];
    size_t fullWidth = (size_t)MAX(1.0, ceil(_frame.size.width * widthScale));
    size_t pixelHeight = (size_t)MAX(1.0, ceil(_frame.size.height * pixelScale));
    if (pixelHeight > VIVA_LABEL_TEXTURE_LIMIT) {
        return NO;
    }
    // A line wider than a texture holds (a long line laid out at its full width and
    // scrolled) is rasterized only around the part of it that can be seen.
    BOOL windowed = fullWidth > VIVA_LABEL_TEXTURE_LIMIT;
    size_t windowLeft = 0, windowRight = fullWidth;
    if (windowed) {
        size_t left = 0, right = 0;
        BOOL visible = viva_label_visible_span(_quad, _tint, _clipped, _clip, viewport, fullWidth, &left, &right);
        viva_label_window(fullWidth, VIVA_LABEL_TEXTURE_LIMIT, visible, left, right, &windowLeft, &windowRight);
    }
    id<MTLTexture> texture = nil;
    NSMutableArray<id<MTLTexture>> *additionalTextures = nil;
    for (size_t tileLeft = windowLeft; tileLeft < windowRight;) {
        size_t tileRight = tileLeft + MIN(windowRight - tileLeft, VIVA_LABEL_TILE_WIDTH);
        size_t textureLeft = tileLeft > 0 ? tileLeft - 1 : 0;
        size_t textureRight = MIN(tileRight + 1, fullWidth);
        size_t pixelWidth = textureRight - textureLeft;
        size_t rowBytes = pixelWidth * 4;
        if (pixelHeight > SIZE_MAX / rowBytes) {
            [texture release];
            [additionalTextures release];
            return NO;
        }
        void *pixels = calloc(pixelHeight, rowBytes);
        if (pixels == NULL) {
            [texture release];
            [additionalTextures release];
            return NO;
        }
        CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
        CGContextRef context = CGBitmapContextCreate(
            pixels, pixelWidth, pixelHeight, 8, rowBytes, colorSpace,
            kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little
        );
        CGColorSpaceRelease(colorSpace);
        if (context == NULL) {
            free(pixels);
            [texture release];
            [additionalTextures release];
            return NO;
        }
        viva_prepare_text_context(context, pixelScale);
        CGContextScaleCTM(context, widthScale / pixelScale, 1);
        if (textureLeft > 0) {
            CGContextTranslateCTM(context, -(CGFloat)textureLeft / widthScale, 0);
        }
        if (_runCount > 0) {
            [self drawRunsInContext:context pixelScale:pixelScale];
        } else {
            [self drawLineInContext:context pixelScale:pixelScale];
        }
        MTLTextureDescriptor *descriptor = [MTLTextureDescriptor
            texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                         width:pixelWidth
                                        height:pixelHeight
                                     mipmapped:NO];
        descriptor.storageMode = MTLStorageModeShared;
        descriptor.usage = MTLTextureUsageShaderRead;
        id<MTLTexture> nextTexture = [device newTextureWithDescriptor:descriptor];
        if (nextTexture != nil) {
            [nextTexture replaceRegion:MTLRegionMake2D(0, 0, pixelWidth, pixelHeight)
                           mipmapLevel:0 withBytes:pixels bytesPerRow:rowBytes];
        }
        CGContextRelease(context);
        free(pixels);
        if (nextTexture == nil) {
            [texture release];
            [additionalTextures release];
            return NO;
        }
        if (texture == nil) {
            texture = nextTexture;
        } else {
            if (additionalTextures == nil) {
                additionalTextures = [[NSMutableArray alloc] init];
                if (additionalTextures == nil) {
                    [nextTexture release];
                    [texture release];
                    return NO;
                }
            }
            [additionalTextures addObject:nextTexture];
            [nextTexture release];
        }
        tileLeft = tileRight;
    }
    [_texture release];
    _texture = texture;
    [_additionalTextures release];
    _additionalTextures = additionalTextures;
    _lastScale = scale;
    _windowed = windowed;
    _windowLeft = windowLeft;
    _windowRight = windowRight;
    _windowPixelScale = widthScale;
    return YES;
}

// drawLineInContext draws an ordinary label: one line in one font and color,
// vertically centered.
- (void)drawLineInContext:(CGContextRef)context pixelScale:(CGFloat)pixelScale {
    CGColorRef foreground = CGColorCreateSRGB(_red, _green, _blue, _alpha);
    CGFloat baseline = _baseline > 0
        ? viva_fixed_baseline(_frame.size.height, _baseline, pixelScale)
        : viva_baseline(_frame.size.height, _fontSize, pixelScale);
    // Text laid out at a fraction of a pixel below the frame: Core Graphics puts a baseline
    // on the next device row down (it floors in its y-up space, so AppKit does), and a
    // glyph at a fraction of a pixel across (it quantizes the position, in drawing the line).
    baseline -= ceil(_originY * pixelScale - 1e-4) / pixelScale;
    if (_mono) {
        // A fixed-pitch label is not proportional text: it keeps its own advances.
        // The line carries the face's font (the VivaFont itself: looking it up again by
        // name fails for the monospaced system font at a weight, whose name is private
        // to that instance, and CoreText answers with Times New Roman) and, for the
        // clusters no family of it draws, the installed fallback.
        CTLineRef line = viva_mono_fallback_line(_value, viva_mono_font(_fontSize, _fontWeight, _monoFace), _monoFace, foreground);
        if (line != NULL) {
            CGContextSetTextPosition(context, _originX, baseline);
            CTLineDraw(line, context);
            CFRelease(line);
        }
    } else {
        viva_text_line *shaped = viva_shared_text_line_style(_value, _fontSize, _fontWeight, _italic);
        if (shaped != NULL) {
            viva_draw_text_line(context, shaped, foreground, _originX, baseline);
            viva_text_line_release(shaped);
        }
    }
    CFRelease(foreground);
}

// drawRunsInContext draws a fixed-pitch grid line. Every run is shaped on its
// own and placed at its cell, so colors, wide characters and glyphs that come
// from fallback fonts cannot push later text off its column. All runs share
// one baseline taken from the base font.
- (void)drawRunsInContext:(CGContextRef)context pixelScale:(CGFloat)pixelScale {
    VivaFont *baseFont = viva_mono_font(_fontSize, _fontWeight, _monoFace);
    CGFloat baseline = _baseline > 0
        ? viva_fixed_baseline(_frame.size.height, _baseline, pixelScale)
        : viva_baseline(_frame.size.height, _fontSize, pixelScale);
    CGFloat hairline = 1.0 / pixelScale;
    const unsigned char *bytes = (const unsigned char *)_utf8.bytes;
    size_t length = _utf8.length;
    size_t offset = 0;

    // Fonts differ only by weight and slant; a line rarely has more than a few.
    enum { kFontCache = 4, kMaxGlyphRun = 256 };
    CTFontRef cachedFont[kFontCache] = {NULL};
    float cachedWeight[kFontCache] = {0};
    BOOL cachedItalic[kFontCache] = {NO};
    int cacheCount = 0;

    for (size_t index = 0; index < _runCount; index++) {
        const viva_text_run *run = &_runs[index];
        if (run->bytes > length - offset) {
            break;
        }
        const unsigned char *start = bytes + offset;
        size_t count = run->bytes;
        offset += count;
        BOOL fitted = (run->flags & VIVA_RUN_LIGATURES) != 0;
        BOOL slant = (run->flags & VIVA_RUN_ITALIC) != 0;
        BOOL italic = slant && !fitted;
        CTFontRef font = NULL;
        for (int slot = 0; slot < cacheCount; slot++) {
            if (cachedWeight[slot] == run->weight && cachedItalic[slot] == italic) {
                font = cachedFont[slot];
                break;
            }
        }
        if (font == NULL) {
            VivaFont *native = viva_mono_font(_fontSize, run->weight, _monoFace);
            font = (CTFontRef)CFRetain(native);
            if (italic) {
                CGAffineTransform slant = CGAffineTransformMake(1, 0, 0.2126, 1, 0, 0);
                CTFontRef slanted = CTFontCreateCopyWithAttributes(font, _fontSize, &slant, NULL);
                if (slanted != NULL) {
                    CFRelease(font);
                    font = slanted;
                }
            }
            if (cacheCount < kFontCache) {
                cachedFont[cacheCount] = font;
                cachedWeight[cacheCount] = run->weight;
                cachedItalic[cacheCount] = italic;
                cacheCount++;
            }
        }
        CGColorRef foreground = CGColorCreateSRGB(run->red, run->green, run->blue, run->alpha);
        CGFloat cellLeft = (CGFloat)run->column * _cellWidth;
        CGFloat span = (CGFloat)run->cells * _cellWidth;
        BOOL ascii = YES;
        for (size_t at = 0; ascii && at < count; at++) {
            ascii = start[at] >= 0x20 && start[at] < 0x7F;
        }
        if (fitted && count > 0) {
            // A ligature run is shaped as one line and scaled to its cells, so
            // the font's substitutions survive and the run still ends where
            // the grid says it does.
            NSString *shown = [[NSString alloc] initWithBytes:start
                                                       length:count
                                                     encoding:NSUTF8StringEncoding];
            if (shown != nil) {
                CTLineRef line = viva_mono_fallback_line(shown, (VivaFont *)font, _monoFace, foreground);
                if (line != NULL) {
                    CGFloat width = CTLineGetTypographicBounds(line, NULL, NULL, NULL);
                    if (width > 0 && span > 0) {
                        CGContextSaveGState(context);
                        CGContextTranslateCTM(context, cellLeft, baseline);
                        CGContextScaleCTM(context, span / width, 1);
                        // Oblique fitted text keeps the unmodified font's shaping and caret advances.
                        if (slant) {
                            CGContextConcatCTM(context, CGAffineTransformMake(1, 0, 0.2126, 1, 0, 0));
                        }
                        CGFloat origin = ascii ? 0 : viva_fitted_mono_origin(line, (CFIndex)shown.length);
                        CGContextSetTextPosition(context, -origin, 0);
                        CTLineDraw(line, context);
                        CGContextRestoreGState(context);
                    }
                    CFRelease(line);
                }
                [shown release];
            }
        } else if (ascii) {
            // Plain ASCII needs no shaping in a fixed-pitch font: map the
            // characters to glyphs and put glyph i exactly on cell i. The run
            // is drawn in pieces of kMaxGlyphRun, every glyph still on its
            // own cell.
            UniChar characters[kMaxGlyphRun];
            CGGlyph glyphs[kMaxGlyphRun];
            CGPoint positions[kMaxGlyphRun];
            CGContextSetFillColorWithColor(context, foreground);
            // Glyph positions are relative to the text position.
            CGContextSetTextPosition(context, 0, 0);
            for (size_t first = 0; first < count; first += kMaxGlyphRun) {
                size_t piece = count - first;
                if (piece > kMaxGlyphRun) {
                    piece = kMaxGlyphRun;
                }
                for (size_t at = 0; at < piece; at++) {
                    characters[at] = start[first + at];
                    positions[at] = CGPointMake(cellLeft + (CGFloat)(first + at) * _cellWidth, baseline);
                }
                CTFontGetGlyphsForCharacters(font, characters, glyphs, (CFIndex)piece);
                CTFontDrawGlyphs(font, glyphs, positions, (CFIndex)piece, context);
            }
        } else {
            NSString *shown = [[NSString alloc] initWithBytes:start
                                                       length:count
                                                     encoding:NSUTF8StringEncoding];
            if (shown == nil) {
                CFRelease(foreground);
                continue;
            }
            CTLineRef line = viva_mono_fallback_line(shown, (VivaFont *)font, _monoFace, foreground);
            if (line == NULL) {
                [shown release];
                CFRelease(foreground);
                continue;
            }
            CGFloat x = cellLeft;
            if (run->cells > 0) {
                CFRange cluster = CFStringGetRangeOfComposedCharactersAtIndex((CFStringRef)shown, 0);
                if (cluster.length == (CFIndex)shown.length) {
                    // A lone character takes the middle of its cells, so wide
                    // characters and symbols from other fonts sit evenly.
                    CGFloat width = CTLineGetTypographicBounds(line, NULL, NULL, NULL);
                    x = cellLeft + (span - width) / 2;
                }
            }
            CGContextSetTextPosition(context, x, baseline);
            CTLineDraw(line, context);
            CFRelease(line);
            [shown release];
        }

        if ((run->flags & (VIVA_RUN_UNDERLINE | VIVA_RUN_STRIKE)) != 0 && span > 0) {
            CGContextSetFillColorWithColor(context, foreground);
            CGFloat thickness = MAX(hairline, CTFontGetUnderlineThickness(font));
            thickness = ceil(thickness * pixelScale) / pixelScale;
            if ((run->flags & VIVA_RUN_UNDERLINE) != 0) {
                CGFloat y = baseline + CTFontGetUnderlinePosition(font) - thickness / 2;
                y = floor(y * pixelScale) / pixelScale;
                CGContextFillRect(context, CGRectMake(cellLeft, y, span, thickness));
            }
            if ((run->flags & VIVA_RUN_STRIKE) != 0) {
                CGFloat y = baseline + CTFontGetXHeight(font) * 0.5 - thickness / 2;
                y = floor(y * pixelScale) / pixelScale;
                CGContextFillRect(context, CGRectMake(cellLeft, y, span, thickness));
            }
        }

        CFRelease(foreground);
    }
    for (int slot = 0; slot < cacheCount; slot++) {
        CFRelease(cachedFont[slot]);
    }
}

- (id<MTLTexture>)textureAtIndex:(NSUInteger)index {
    return index == 0 ? _texture : [_additionalTextures objectAtIndex:index - 1];
}

- (NSUInteger)textureCount {
    return 1 + _additionalTextures.count;
}

- (CGRect)frame {
    return _frame;
}

- (CGRect)quadAtIndex:(NSUInteger)index {
    if (_bitmap != nil) {
        return _frame;
    }
    size_t full = (size_t)MAX(1.0, ceil(_frame.size.width * _windowPixelScale));
    size_t tileLeft = _windowLeft + index * VIVA_LABEL_TILE_WIDTH;
    size_t tileRight = tileLeft + MIN(_windowRight - tileLeft, VIVA_LABEL_TILE_WIDTH);
    CGFloat left = (CGFloat)tileLeft / _windowPixelScale;
    CGFloat right = tileRight >= full ? _frame.size.width : (CGFloat)tileRight / _windowPixelScale;
    return CGRectMake(_frame.origin.x + left, _frame.origin.y, right - left, _frame.size.height);
}

- (CGRect)textureFrameAtIndex:(NSUInteger)index {
    if (_bitmap != nil) return _frame;
    size_t full = (size_t)MAX(1.0, ceil(_frame.size.width * _windowPixelScale));
    size_t tileLeft = _windowLeft + index * VIVA_LABEL_TILE_WIDTH;
    size_t tileRight = tileLeft + MIN(_windowRight - tileLeft, VIVA_LABEL_TILE_WIDTH);
    size_t textureLeft = tileLeft > 0 ? tileLeft - 1 : 0;
    size_t textureRight = MIN(tileRight + 1, full);
    CGFloat left = (CGFloat)textureLeft / _windowPixelScale;
    CGFloat right = textureRight >= full ? _frame.size.width : (CGFloat)textureRight / _windowPixelScale;
    return CGRectMake(_frame.origin.x + left, _frame.origin.y, right - left, _frame.size.height);
}

@end

// Keep native windows and offscreen frames on the same retained-texture path.
// viewport is the size of the window or frame the labels are drawn in, logical pixels.
int viva_update_label_layers(
    NSArray<VivaTextLayer *> *previous,
    const viva_text_label *labels,
    NSUInteger count,
    id<MTLDevice> device,
    CGFloat scale,
    CGSize viewport,
    NSMutableArray<VivaTextLayer *> **updated
) {
    NSMutableArray<VivaTextLayer *> *replacement =
        [[NSMutableArray alloc] initWithCapacity:count];
    NSUInteger oldCount = previous.count;
    uint64_t *oldHashes = malloc(MAX(oldCount, 1) * sizeof(uint64_t));
    uint64_t *newHashes = malloc(MAX(count, 1) * sizeof(uint64_t));
    int64_t *source = malloc(MAX(count, 1) * sizeof(int64_t));
    if (oldHashes == NULL || newHashes == NULL || source == NULL) {
        free(oldHashes);
        free(newHashes);
        free(source);
        [replacement release];
        return 3;
    }
    for (NSUInteger index = 0; index < oldCount; index++) {
        oldHashes[index] = [previous[index] contentHash];
    }
    for (NSUInteger index = 0; index < count; index++) {
        newHashes[index] = viva_label_content_hash(labels[index]);
    }
    int status = viva_plan_label_reuse(oldHashes, oldCount, newHashes, count, source);
    free(oldHashes);
    free(newHashes);
    if (status != 0) {
        free(source);
        [replacement release];
        return status;
    }
    for (NSUInteger index = 0; index < count; index++) {
        VivaTextLayer *existing = nil;
        if (source[index] >= 0 && [previous[(NSUInteger)source[index]] matchesConfig:labels[index]]) {
            existing = previous[(NSUInteger)source[index]];
        }
        if (existing != nil) {
            [existing updateFrame:CGRectMake(
                labels[index].x, labels[index].y,
                labels[index].width, labels[index].height)];
            [existing updateClipFromConfig:labels[index]];
            [existing updateQuadFromConfig:labels[index]];
            // An image drawn at a new size may need a texture of another resolution, and a
            // line held in part another part of it.
            BOOL rebuilt = (labels[index].flags & VIVA_LABEL_BITMAP) != 0
                ? [existing rebuildWithDevice:device scale:scale viewport:viewport]
                : [existing refreshWindowWithDevice:device scale:scale viewport:viewport];
            if (!rebuilt) {
                free(source);
                [replacement release];
                return 3;
            }
            [replacement addObject:existing];
            continue;
        }
        VivaTextLayer *layer = [[VivaTextLayer alloc] initWithConfig:labels[index]];
        if (layer == nil || ![layer rebuildWithDevice:device scale:scale viewport:viewport]) {
            int failure = layer == nil ? 2 : 3;
            free(source);
            [layer release];
            [replacement release];
            return failure;
        }
        [layer markContentRebuild];
        [replacement addObject:layer];
        [layer release];
    }
    free(source);
    *updated = replacement;
    return 0;
}

// viva_label_box_quad writes the four corners of an axis-aligned box in the
// order the text vertex shader expects them: top-left, top-right, bottom-left,
// bottom-right.
void viva_label_box_quad(float *quad, float x, float y, float width, float height) {
    quad[0] = x;
    quad[1] = y;
    quad[2] = x + width;
    quad[3] = y;
    quad[4] = x;
    quad[5] = y + height;
    quad[6] = x + width;
    quad[7] = y + height;
}

// viva_label_plain_tint writes the colour that leaves a label's pixels as they
// are: opaque white at every corner.
void viva_label_plain_tint(float *tint) {
    for (int index = 0; index < 16; index++) {
        tint[index] = 1;
    }
}

// viva_label_tile_mesh maps one texture tile's part of the label's triangles. A
// label wider than a texture holds arrives in several tiles, each a horizontal
// slice of the same picture; clipping the original triangles preserves their
// affine texture coordinates and tints across tile boundaries.
size_t viva_label_tile_mesh(
    const float *quad,
    const float *tint,
    CGRect frame,
    CGRect tile,
    CGRect texture,
    viva_label_vertex *vertices
) {
    CGFloat width = frame.size.width > 0 ? frame.size.width : 1;
    float left = (float)((tile.origin.x - frame.origin.x) / width);
    float right = (float)((tile.origin.x + tile.size.width - frame.origin.x) / width);
    left = fminf(fmaxf(left, 0), 1);
    right = fminf(fmaxf(right, 0), 1);
    float textureLeft = (float)((texture.origin.x - frame.origin.x) / width);
    float textureRight = (float)((texture.origin.x + texture.size.width - frame.origin.x) / width);
    return viva_label_tile_vertices(quad, tint, left, right, textureLeft, textureRight, vertices);
}

// viva_text_label_uniform mirrors TextLabelUniform in scene.metal: where a
// label's box starts, its size, and the radius its picture's corners are cut by.
typedef struct {
    float originX, originY;
    float sizeX, sizeY;
    float radius;
    float padding;
} viva_text_label_uniform;

// viva_encode_input draws the native editor's texture.
void viva_encode_input(
    id<MTLRenderCommandEncoder> encoder,
    id<MTLRenderPipelineState> textPipeline,
    id<MTLTexture> inputTexture,
    CGRect inputFrame,
    CGSize viewport
) {
    float viewportSize[2] = {(float)viewport.width, (float)viewport.height};
    float quad[8];
    viva_label_box_quad(quad,
                        (float)inputFrame.origin.x, (float)inputFrame.origin.y,
                        (float)inputFrame.size.width, (float)inputFrame.size.height);
    float tint[16];
    viva_label_plain_tint(tint);
    viva_label_vertex vertices[6];
    viva_label_vertices(quad, tint, vertices);
    [encoder setRenderPipelineState:textPipeline];
    [encoder setVertexBytes:vertices length:sizeof(vertices) atIndex:2];
    viva_text_label_uniform labelUniform = {0, 0, 0, 0, 0, 0};
    [encoder setVertexBytes:&labelUniform length:sizeof(labelUniform) atIndex:3];
    [encoder setVertexBytes:&viewportSize length:sizeof(viewportSize) atIndex:1];
    [encoder setFragmentTexture:inputTexture atIndex:0];
    [encoder drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:6];
    viva_gpu_draw_call_count++;
}

// viva_encode_scene draws one frame: the rectangle and text-label batches in
// composite order, with the native editor's texture drawn after every batch of
// its own layer, so a higher layer (a popup over the field) covers it. The window and the
// headless renderer share it, so a screenshot is the pixels the window shows.
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
) {
    float viewportSize[2] = {(float)viewport.width, (float)viewport.height};
    BOOL inputDrawn = inputTexture == nil;
    // A label that a canvas clip cuts is drawn through a scissor, so the part
    // outside the clip is never painted. The scale maps logical pixels to the
    // target's.
    CGFloat targetScale = viewport.width > 0 ? (CGFloat)targetWidth / viewport.width : 1.0;
    float sceneViewport[4] = {(float)viewport.width, (float)viewport.height, (float)(1.0 / targetScale), 0};
    MTLScissorRect fullTarget = {0, 0, targetWidth, targetHeight};
    for (NSUInteger index = 0; index < commandCount; index++) {
        viva_composite_command command = commands[index];
        if (!inputDrawn && command.layer > inputLayer) {
            viva_encode_input(encoder, textPipeline, inputTexture, inputFrame, viewport);
            inputDrawn = YES;
        }
        if (command.kind == VIVA_COMPOSITE_RECTANGLES) {
            // A clipped batch is drawn through a scissor. An edge between two
            // pixel boundaries moves to the nearer one, as render.ScissorOf does.
            if (command.clipped != 0) {
                CGFloat leftEdge = floor(command.clip_x * targetScale + 0.5);
                CGFloat topEdge = floor(command.clip_y * targetScale + 0.5);
                CGFloat rightEdge = floor((command.clip_x + command.clip_width) * targetScale + 0.5);
                CGFloat bottomEdge = floor((command.clip_y + command.clip_height) * targetScale + 0.5);
                NSInteger left = (NSInteger)MIN(MAX(leftEdge, 0), (CGFloat)targetWidth);
                NSInteger top = (NSInteger)MIN(MAX(topEdge, 0), (CGFloat)targetHeight);
                NSInteger right = (NSInteger)MIN(MAX(rightEdge, 0), (CGFloat)targetWidth);
                NSInteger bottom = (NSInteger)MIN(MAX(bottomEdge, 0), (CGFloat)targetHeight);
                if (right <= left || bottom <= top) {
                    continue;
                }
                MTLScissorRect scissor = {(NSUInteger)left, (NSUInteger)top, (NSUInteger)(right - left), (NSUInteger)(bottom - top)};
                [encoder setScissorRect:scissor];
            }
            [encoder setRenderPipelineState:rectanglePipeline];
            [encoder setVertexBuffer:rectangleBuffer offset:0 atIndex:0];
            [encoder setVertexBytes:&sceneViewport length:sizeof(sceneViewport) atIndex:1];
            [encoder drawPrimitives:MTLPrimitiveTypeTriangle
                        vertexStart:0
                        vertexCount:6
                      instanceCount:command.count
                       baseInstance:command.first];
            viva_gpu_draw_call_count++;
            viva_rectangle_instance_count += command.count;
            if (command.clipped != 0) {
                [encoder setScissorRect:fullTarget];
            }
            continue;
        }

        VivaTextLayer *layer = [labels objectAtIndex:command.first];
        BOOL scissored = NO;
        if ([layer clipped]) {
            CGRect clip = [layer clip];
            NSInteger left = MAX((NSInteger)floor(clip.origin.x * targetScale), 0);
            NSInteger top = MAX((NSInteger)floor(clip.origin.y * targetScale), 0);
            NSInteger right = MIN((NSInteger)ceil((clip.origin.x + clip.size.width) * targetScale), (NSInteger)targetWidth);
            NSInteger bottom = MIN((NSInteger)ceil((clip.origin.y + clip.size.height) * targetScale), (NSInteger)targetHeight);
            if (right <= left || bottom <= top) {
                continue;
            }
            MTLScissorRect scissor = {(NSUInteger)left, (NSUInteger)top, (NSUInteger)(right - left), (NSUInteger)(bottom - top)};
            [encoder setScissorRect:scissor];
            scissored = YES;
        }
        [encoder setRenderPipelineState:textPipeline];
        [encoder setVertexBytes:&viewportSize length:sizeof(viewportSize) atIndex:1];
        CGRect layerFrame = [layer frame];
        viva_text_label_uniform labelUniform = {
            (float)layerFrame.origin.x, (float)layerFrame.origin.y,
            (float)layerFrame.size.width, (float)layerFrame.size.height,
            [layer imageRadius], 0,
        };
        [encoder setVertexBytes:&labelUniform length:sizeof(labelUniform) atIndex:3];
        for (NSUInteger tile = 0; tile < [layer textureCount]; tile++) {
            viva_label_vertex vertices[18];
            size_t count = viva_label_tile_mesh([layer quad], [layer tint], [layer frame],
                [layer quadAtIndex:tile], [layer textureFrameAtIndex:tile], vertices);
            if (count == 0) continue;
            [encoder setVertexBytes:vertices length:count * sizeof(viva_label_vertex) atIndex:2];
            [encoder setFragmentTexture:[layer textureAtIndex:tile] atIndex:0];
            [encoder drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:count];
            viva_gpu_draw_call_count++;
        }
        if (scissored) {
            [encoder setScissorRect:fullTarget];
        }
    }
    if (!inputDrawn) {
        viva_encode_input(encoder, textPipeline, inputTexture, inputFrame, viewport);
    }
}


// viva_make_scene_resources makes the Metal objects a window's first scene is
// drawn with: the device, its queue, the two pipelines the window's events and
// content call for, and the rectangle buffer. Every object it returns is
// retained for the caller. It answers 0, or the status the window reports when
// the object it names cannot be made (1 device, 2 queue, 3 library, 4 rectangle
// pipeline, 5 buffer, 6 text pipeline), with everything made so far released.
int viva_make_scene_resources(
    const viva_window_config *config_in,
    id<MTLDevice> *device_out,
    id<MTLCommandQueue> *queue_out,
    id<MTLRenderPipelineState> *rectangle_pipeline_out,
    id<MTLRenderPipelineState> *text_pipeline_out,
    id<MTLBuffer> *rectangle_buffer_out
) {
    const viva_window_config config = *config_in;
    *device_out = nil;
    *queue_out = nil;
    *rectangle_pipeline_out = nil;
    *text_pipeline_out = nil;
    *rectangle_buffer_out = nil;
    id<MTLDevice> device = MTLCreateSystemDefaultDevice();
    if (device == nil) {
        return 1;
    }

    id<MTLCommandQueue> commandQueue = [device newCommandQueue];
    if (commandQueue == nil) {
        [device release];
        return 2;
    }

    id<MTLRenderPipelineState> rectanglePipeline = nil;
    id<MTLRenderPipelineState> textPipeline = nil;
    id<MTLBuffer> rectangleBuffer = nil;
    if (config.shader_library != NULL && config.shader_library_length > 0) {
        viva_remember_shader_library(config.shader_library, config.shader_library_length);
        _Static_assert(sizeof(viva_rectangle) == 80, "rectangle ABI must be 80 bytes");
        _Static_assert(sizeof(viva_text_label) == 272, "text label ABI must be 272 bytes");
        _Static_assert(sizeof(viva_text_run) == 36, "text run ABI must be 36 bytes");
        _Static_assert(sizeof(viva_composite_command) == 36, "composite command ABI must be 36 bytes");
        dispatch_data_t libraryData = dispatch_data_create(
            config.shader_library,
            config.shader_library_length,
            dispatch_get_main_queue(),
            DISPATCH_DATA_DESTRUCTOR_DEFAULT
        );
        NSError *libraryError = nil;
        id<MTLLibrary> library = [device newLibraryWithData:libraryData
                                                     error:&libraryError];
        dispatch_release(libraryData);
        if (library == nil) {
            fprintf(stderr, "viva: Metal library error: %s\n",
                    libraryError.localizedDescription.UTF8String);
            [commandQueue release];
            [device release];
            return 3;
        }

        NSError *pipelineError = nil;
        if (config.rectangle_count > 0 || config.pointer_events_enabled ||
            config.resize_events_enabled || config.redraw_events_enabled) {
            rectanglePipeline = viva_create_pipeline(
                device,
                library,
                @"viva_rectangle_vertex",
                @"viva_rectangle_fragment",
                &pipelineError
            );
            if (rectanglePipeline == nil) {
                fprintf(stderr, "viva: rectangle pipeline error: %s\n",
                        pipelineError.localizedDescription.UTF8String);
                [library release];
                [commandQueue release];
                [device release];
                return 4;
            }
        }
        if (config.text_input.enabled || config.dynamic_text_input || config.text_label_count > 0 ||
            config.resize_events_enabled || config.redraw_events_enabled) {
            pipelineError = nil;
            textPipeline = viva_create_pipeline(
                device,
                library,
                @"viva_text_vertex",
                @"viva_text_fragment",
                &pipelineError
            );
            if (textPipeline == nil) {
                fprintf(stderr, "viva: text pipeline error: %s\n",
                        pipelineError.localizedDescription.UTF8String);
                [rectanglePipeline release];
                [library release];
                [commandQueue release];
                [device release];
                return 6;
            }
        }
        [library release];

        if (config.rectangle_count > 0) {
            rectangleBuffer = [device
                newBufferWithBytes:config.rectangles
                            length:config.rectangle_count * sizeof(viva_rectangle)
                           options:MTLResourceStorageModeShared];
            if (rectangleBuffer == nil) {
                [textPipeline release];
                [rectanglePipeline release];
                [commandQueue release];
                [device release];
                return 5;
            }
            viva_gpu_uploaded_bytes +=
                config.rectangle_count * sizeof(viva_rectangle);
            viva_gpu_full_rebuild_count++;
        }
    }
    *device_out = device;
    *queue_out = commandQueue;
    *rectangle_pipeline_out = rectanglePipeline;
    *text_pipeline_out = textPipeline;
    *rectangle_buffer_out = rectangleBuffer;
    return 0;
}

@interface VivaRenderer (Plan)
- (BOOL)replaceCompositePlanForRectangles:(const viva_rectangle *)rectangles
                                    clips:(const viva_clip *)clips
                            rectangleCount:(NSUInteger)rectangleCount
                                 textLayers:(const uint16_t *)textLayers
                           textShapesBefore:(const uint32_t *)textShapesBefore
                                  textCount:(NSUInteger)textCount;
@end

@implementation VivaRenderer

@synthesize layer = _layer;
@synthesize textLabels = _textLabels;
@synthesize rectangleCount = _rectangleCount;
@synthesize clearColor = _clearColor;
@synthesize textPipeline = _textPipeline;

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
                     viewport:(CGSize)viewport {
    self = [super init];
    if (self == nil) {
        return nil;
    }
    _layer = [layer retain];
    _commandQueue = [commandQueue retain];
    _rectanglePipeline = [rectanglePipeline retain];
    _rectangleBuffer = [rectangleBuffer retain];
    _rectangleCount = rectangleCount;
    _rectangleCapacity = rectangleCount;
    if (rectangleCount > 0) {
        _rectangleClips = malloc(rectangleCount * sizeof(viva_clip));
        if (_rectangleClips == NULL) {
            [self release];
            return nil;
        }
        memcpy(_rectangleClips, rectangleClips, rectangleCount * sizeof(viva_clip));
    }
    _textPipeline = [textPipeline retain];
    _clearColor = clearColor;
    if (textLabelCount > 0) {
        _textLabelLayers = malloc(textLabelCount * sizeof(uint16_t));
        _textLabelShapesBefore = malloc(textLabelCount * sizeof(uint32_t));
        if (_textLabelLayers == NULL || _textLabelShapesBefore == NULL) {
            [self release];
            return nil;
        }
        for (NSUInteger index = 0; index < textLabelCount; index++) {
            _textLabelLayers[index] = textLabels[index].layer;
            _textLabelShapesBefore[index] = textLabels[index].shapes_before;
        }
    }
    _textLabelCount = textLabelCount;
    _textLabels = [[NSMutableArray alloc] initWithCapacity:textLabelCount];
    for (NSUInteger index = 0; index < textLabelCount; index++) {
        VivaTextLayer *textLayer = [[VivaTextLayer alloc] initWithConfig:textLabels[index]];
        if (textLayer == nil) {
            [self release];
            return nil;
        }
        [_textLabels addObject:textLayer];
        [textLayer release];
    }
    if (![self rebuildTextLabelsWithViewport:viewport]) {
        [self release];
        return nil;
    }
    if (![self replaceCompositePlanForRectangles:
            rectangleBuffer == nil ? NULL : rectangleBuffer.contents
                                           clips:_rectangleClips
                                  rectangleCount:rectangleCount
                                       textLayers:_textLabelLayers
                                 textShapesBefore:_textLabelShapesBefore
                                        textCount:_textLabelCount]) {
        [self release];
        return nil;
    }
    viva_label_initial_count = textLabelCount;
    return self;
}

- (void)dealloc {
    free(_compositeCommands);
    free(_rectangleClips);
    free(_textLabelLayers);
    free(_textLabelShapesBefore);
    [_rectangleBuffer release];
    [_rectanglePipeline release];
    [_textLabels release];
    [_textPipeline release];
    [_commandQueue release];
    [_layer release];
    [super dealloc];
}

- (BOOL)replaceCompositePlanForRectangles:(const viva_rectangle *)rectangles
                                    clips:(const viva_clip *)clips
                            rectangleCount:(NSUInteger)rectangleCount
                                 textLayers:(const uint16_t *)textLayers
                           textShapesBefore:(const uint32_t *)textShapesBefore
                                  textCount:(NSUInteger)textCount {
    if (rectangleCount > SIZE_MAX - textCount) {
        return NO;
    }
    NSUInteger capacity = rectangleCount + textCount;
    viva_composite_command *replacement = NULL;
    if (capacity > 0) {
        if (capacity > SIZE_MAX / sizeof(viva_composite_command)) {
            return NO;
        }
        replacement = malloc(capacity * sizeof(viva_composite_command));
        if (replacement == NULL) {
            return NO;
        }
    }
    size_t count = 0;
    int status = viva_plan_composite(
        rectangles,
        clips,
        rectangleCount,
        textLayers,
        textShapesBefore,
        textCount,
        replacement,
        capacity,
        &count
    );
    if (status != 0) {
        free(replacement);
        return NO;
    }
    free(_compositeCommands);
    _compositeCommands = replacement;
    _compositeCommandCount = count;
    return YES;
}

- (BOOL)rebuildTextLabelsWithViewport:(CGSize)viewport {
    CGFloat scale = MAX(_layer.contentsScale, 1.0);
    for (VivaTextLayer *textLayer in _textLabels) {
        if (![textLayer rebuildWithDevice:_layer.device scale:scale viewport:viewport]) {
            return NO;
        }
    }
    return YES;
}

- (void)forceRebuildTextLabelsWithViewport:(CGSize)viewport {
    CGFloat scale = MAX(_layer.contentsScale, 1.0);
    for (VivaTextLayer *textLayer in _textLabels) {
        [textLayer forceTextureRebuildWithDevice:_layer.device scale:scale viewport:viewport];
    }
    viva_label_texture_rebuild_count += _textLabels.count;
}

- (id<MTLCommandBuffer>)renderToDrawable:(id<CAMetalDrawable>)drawable
                                viewport:(CGSize)viewport
                            inputTexture:(id<MTLTexture>)inputTexture
                              inputFrame:(CGRect)inputFrame
                              inputLayer:(uint16_t)inputLayer {
    MTLRenderPassDescriptor *pass = [MTLRenderPassDescriptor renderPassDescriptor];
    pass.colorAttachments[0].texture = drawable.texture;
    pass.colorAttachments[0].loadAction = MTLLoadActionClear;
    pass.colorAttachments[0].storeAction = MTLStoreActionStore;
    pass.colorAttachments[0].clearColor = _clearColor;

    id<MTLCommandBuffer> commandBuffer = [_commandQueue commandBuffer];
    id<MTLRenderCommandEncoder> encoder = [commandBuffer renderCommandEncoderWithDescriptor:pass];
    viva_encode_scene(encoder, _rectanglePipeline, _textPipeline, _rectangleBuffer,
                     _compositeCommands, _compositeCommandCount, _textLabels,
                     inputTexture, inputFrame, inputLayer, viewport,
                     drawable.texture.width, drawable.texture.height);
    [encoder endEncoding];
    [commandBuffer presentDrawable:drawable];
    [commandBuffer commit];
    viva_gpu_submission_count++;
    return commandBuffer;
}

- (BOOL)recoverWithViewport:(CGSize)viewport {
    if (viva_shader_library_bytes == NULL) {
        return NO;
    }
    @autoreleasepool {
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        if (device == nil) {
            return NO;
        }
        id<MTLCommandQueue> queue = [device newCommandQueue];
        if (queue == nil) {
            [device release];
            return NO;
        }
        dispatch_data_t libraryData = dispatch_data_create(
            viva_shader_library_bytes,
            viva_shader_library_length,
            dispatch_get_main_queue(),
            DISPATCH_DATA_DESTRUCTOR_DEFAULT
        );
        NSError *libraryError = nil;
        id<MTLLibrary> library = [device newLibraryWithData:libraryData error:&libraryError];
        dispatch_release(libraryData);
        if (library == nil) {
            [queue release];
            [device release];
            return NO;
        }
        // Both pipelines, whatever the window had: the frame that failed asked
        // for rectangles, and a label texture of the old device cannot be drawn
        // by the new one.
        id<MTLRenderPipelineState> rectanglePipeline = viva_create_pipeline(
            device, library, @"viva_rectangle_vertex", @"viva_rectangle_fragment", NULL);
        id<MTLRenderPipelineState> textPipeline = viva_create_pipeline(
            device, library, @"viva_text_vertex", @"viva_text_fragment", NULL);
        [library release];
        if (rectanglePipeline == nil || textPipeline == nil) {
            [rectanglePipeline release];
            [textPipeline release];
            [queue release];
            [device release];
            return NO;
        }
        _layer.device = device;
        [_commandQueue release];
        _commandQueue = queue;
        [_rectanglePipeline release];
        _rectanglePipeline = rectanglePipeline;
        [_textPipeline release];
        _textPipeline = textPipeline;
        // The instance buffers belonged to the old device; the application
        // sends every instance again, as a rebuild.
        [_rectangleBuffer release];
        _rectangleBuffer = nil;
        free(_rectangleClips);
        _rectangleClips = NULL;
        _rectangleCount = 0;
        _rectangleCapacity = 0;
        free(_compositeCommands);
        _compositeCommands = NULL;
        _compositeCommandCount = 0;
        CGFloat scale = MAX(_layer.contentsScale, 1.0);
        for (VivaTextLayer *textLayer in _textLabels) {
            if (![textLayer forceTextureRebuildWithDevice:device scale:scale viewport:viewport]) {
                [device release];
                return NO;
            }
        }
        viva_label_texture_rebuild_count += _textLabels.count;
        [device release];
        return YES;
    }
}

- (int)updateRectangles:(const viva_rectangle *)rectangles
                  clips:(const viva_clip *)clips
             totalCount:(NSUInteger)totalCount
                  first:(NSUInteger)first
                  count:(NSUInteger)count
                rebuild:(BOOL)rebuild {
    if (rebuild) {
        id<MTLBuffer> replacement = nil;
        viva_clip *replacementClips = NULL;
        if (totalCount > 0) {
            if (_rectanglePipeline == nil || clips == NULL) {
                return 3;
            }
            replacementClips = malloc(totalCount * sizeof(viva_clip));
            if (replacementClips == NULL) {
                return 3;
            }
            memcpy(replacementClips, clips, totalCount * sizeof(viva_clip));
            replacement = [_layer.device
                newBufferWithBytes:rectangles
                            length:totalCount * sizeof(viva_rectangle)
                           options:MTLResourceStorageModeShared];
            if (replacement == nil) {
                free(replacementClips);
                return 3;
            }
            // The staged instances carry their layer in reserved0, so the
            // bytes newBufferWithBytes copies are already complete.
        }
        if (![self replaceCompositePlanForRectangles:
                replacement == nil ? NULL : replacement.contents
                                               clips:replacementClips
                                      rectangleCount:totalCount
                                           textLayers:_textLabelLayers
                                     textShapesBefore:_textLabelShapesBefore
                                            textCount:_textLabelCount]) {
            [replacement release];
            free(replacementClips);
            return 3;
        }
        [_rectangleBuffer release];
        free(_rectangleClips);
        _rectangleClips = replacementClips;
        _rectangleBuffer = replacement;
        _rectangleCount = totalCount;
        _rectangleCapacity = totalCount;
        viva_gpu_uploaded_bytes += totalCount * sizeof(viva_rectangle);
        viva_gpu_full_rebuild_count++;
        return 0;
    }
    BOOL shrank = totalCount < _rectangleCount;
    if (shrank) {
        if (first + count != totalCount) {
            return 2;
        }
        _rectangleCount = totalCount;
    }
    if (count == 0) {
        if (shrank && ![self replaceCompositePlanForRectangles:
                _rectangleBuffer == nil ? NULL : _rectangleBuffer.contents
                                       clips:_rectangleClips
                              rectangleCount:_rectangleCount
                                   textLayers:_textLabelLayers
                             textShapesBefore:_textLabelShapesBefore
                                    textCount:_textLabelCount]) {
            return 3;
        }
        return 0;
    }
    if (_rectangleBuffer == nil || _rectangleClips == NULL || clips == NULL ||
        first > totalCount || count > totalCount - first) {
        return 2;
    }
    BOOL grew = totalCount > _rectangleCount;
    BOOL reallocated = NO;
    if (grew) {
        // The scene appended instances. A stream that appends every frame
        // must not allocate, copy and re-flush the whole buffer each time,
        // so the buffers carry headroom; a rebuild re-tightens them to the
        // exact count. The headroom is only made when the new count no
        // longer fits: growing on every append multiplied the capacity by
        // a quarter per frame while the scene appended one instance, and
        // the request passed what the device hands out within a few dozen
        // appends.
        if (first > _rectangleCount || first + count != totalCount) {
            return 2;
        }
        NSUInteger capacity = viva_rectangle_capacity(_rectangleCapacity, totalCount);
        if (capacity != _rectangleCapacity) {
            id<MTLBuffer> replacement = [_layer.device
                newBufferWithLength:capacity * sizeof(viva_rectangle)
                            options:MTLResourceStorageModeShared];
            if (replacement == nil) {
                return 3;
            }
            viva_clip *replacementClips = malloc(capacity * sizeof(viva_clip));
            if (replacementClips == NULL) {
                [replacement release];
                return 3;
            }
            memcpy(replacementClips, _rectangleClips, _rectangleCount * sizeof(viva_clip));
            memcpy(replacement.contents, _rectangleBuffer.contents, _rectangleCount * sizeof(viva_rectangle));
            [_rectangleBuffer release];
            free(_rectangleClips);
            _rectangleBuffer = replacement;
            _rectangleClips = replacementClips;
            _rectangleCapacity = capacity;
            reallocated = YES;
        }
        _rectangleCount = totalCount;
    }
    memcpy(_rectangleClips + first, clips, count * sizeof(viva_clip));
    memcpy(
        (uint8_t *)_rectangleBuffer.contents + first * sizeof(viva_rectangle),
        rectangles,
        count * sizeof(viva_rectangle)
    );
    // The staged instances carry their layer in reserved0, so the memcpy
    // above has already written every field this buffer holds.
#if TARGET_OS_OSX
    [_rectangleBuffer didModifyRange:NSMakeRange(
        reallocated ? 0 : first * sizeof(viva_rectangle),
        reallocated ? totalCount * sizeof(viva_rectangle) : count * sizeof(viva_rectangle)
    )];
#endif
    if (![self replaceCompositePlanForRectangles:_rectangleBuffer.contents
                                           clips:_rectangleClips
                                  rectangleCount:_rectangleCount
                                       textLayers:_textLabelLayers
                                 textShapesBefore:_textLabelShapesBefore
                                        textCount:_textLabelCount]) {
        return 3;
    }
    viva_gpu_uploaded_bytes += count * sizeof(viva_rectangle);
    viva_gpu_partial_update_count++;
    return 0;
}

- (int)updateLabels:(const viva_text_label *)labels
              count:(NSUInteger)count
           viewport:(CGSize)viewport {
    CGFloat scale = MAX(_layer.contentsScale, 1.0);
    uint16_t *replacementLayers = NULL;
    uint32_t *replacementShapesBefore = NULL;
    if (count > 0) {
        replacementLayers = malloc(count * sizeof(uint16_t));
        replacementShapesBefore = malloc(count * sizeof(uint32_t));
        if (replacementLayers == NULL || replacementShapesBefore == NULL) {
            free(replacementLayers);
            free(replacementShapesBefore);
            return 3;
        }
        for (NSUInteger index = 0; index < count; index++) {
            replacementLayers[index] = labels[index].layer;
            replacementShapesBefore[index] = labels[index].shapes_before;
        }
    }
    NSMutableArray<VivaTextLayer *> *replacement = nil;
    int status = viva_update_label_layers(_textLabels, labels, count, _layer.device, scale, viewport, &replacement);
    if (status != 0) {
        free(replacementLayers);
        free(replacementShapesBefore);
        return status;
    }
    if (![self replaceCompositePlanForRectangles:
            _rectangleBuffer == nil ? NULL : _rectangleBuffer.contents
                                           clips:_rectangleClips
                                  rectangleCount:_rectangleCount
                                       textLayers:replacementLayers
                                 textShapesBefore:replacementShapesBefore
                                        textCount:count]) {
        [replacement release];
        free(replacementLayers);
        free(replacementShapesBefore);
        return 3;
    }
    [_textLabels release];
    _textLabels = replacement;
    free(_textLabelLayers);
    free(_textLabelShapesBefore);
    _textLabelLayers = replacementLayers;
    _textLabelShapesBefore = replacementShapesBefore;
    _textLabelCount = count;
    viva_label_update_count++;
    return 0;
}

@end
