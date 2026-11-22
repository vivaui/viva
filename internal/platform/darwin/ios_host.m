//go:build ios

// The iOS host: UIKit's application, window and view around the shared renderer.
// It answers the same C interface (native.h) the AppKit host does, so the Go side
// that runs a window is the same code; what a phone has no use for answers that
// the system does not offer it. Everything runs on the main thread, where UIKit
// calls it and where the Go callbacks are made.

#import "core.h"
#import <UIKit/UIKit.h>
#include "ios_host.h"

/* The callbacks into Go. */
extern void vivaIOSLaunched(void);
extern void vivaViewResized(uintptr_t callbackHandle, void *view, double width, double height);
extern void vivaRedrawRequested(uintptr_t callbackHandle, void *view);
extern void vivaWindowClosed(uintptr_t callbackHandle);
extern void vivaTouchEvent(uintptr_t callbackHandle, void *view, int kind, int identifier,
                           double x, double y, double screenX, double screenY);

/* The pointer kinds, as platform.PointerEventKind numbers them. */
enum {
    VIVA_IOS_POINTER_DOWN = 1,
    VIVA_IOS_POINTER_MOVE = 2,
    VIVA_IOS_POINTER_UP = 3,
    VIVA_IOS_POINTER_CANCEL = 5,
};

/* What viva_application_open_window reports when it cannot make the window. */
enum {
    VIVA_IOS_STATUS_NO_APPLICATION = 8,
    VIVA_IOS_STATUS_ONE_WINDOW = 9,
    VIVA_IOS_STATUS_NO_VIEW = 7,
};

static int viva_application_status = 0;

@interface VivaIOSView : UIView {
    VivaRenderer *_renderer;
    uintptr_t _callbackHandle;
    BOOL _resizeEventsEnabled;
    BOOL _closed;
    CGSize _lastReportedSize;
    BOOL _reportedSize;
}
@property(nonatomic, readonly) uintptr_t callbackHandle;
@property(nonatomic, readonly) BOOL closed;
- (instancetype)initWithFrame:(CGRect)frame
                       device:(id<MTLDevice>)device
                 commandQueue:(id<MTLCommandQueue>)commandQueue
            rectanglePipeline:(id<MTLRenderPipelineState>)rectanglePipeline
              rectangleBuffer:(id<MTLBuffer>)rectangleBuffer
                 textPipeline:(id<MTLRenderPipelineState>)textPipeline
                       config:(const viva_window_config *)config;
- (void)render;
- (void)renderWaitingForSchedule:(BOOL)wait;
- (BOOL)recoverResources;
- (int)updateRectangles:(const viva_rectangle *)rectangles
                  clips:(const viva_clip *)clips
             totalCount:(NSUInteger)totalCount
                  first:(NSUInteger)first
                  count:(NSUInteger)count
                rebuild:(BOOL)rebuild
                 render:(BOOL)render;
- (int)updateLabels:(const viva_text_label *)labels count:(NSUInteger)count;
- (void)closeWindow;
@end

@implementation VivaIOSView

@synthesize callbackHandle = _callbackHandle;
@synthesize closed = _closed;

+ (Class)layerClass {
    return [CAMetalLayer class];
}

- (instancetype)initWithFrame:(CGRect)frame
                       device:(id<MTLDevice>)device
                 commandQueue:(id<MTLCommandQueue>)commandQueue
            rectanglePipeline:(id<MTLRenderPipelineState>)rectanglePipeline
              rectangleBuffer:(id<MTLBuffer>)rectangleBuffer
                 textPipeline:(id<MTLRenderPipelineState>)textPipeline
                       config:(const viva_window_config *)config {
    self = [super initWithFrame:frame];
    if (self == nil) {
        return nil;
    }
    _callbackHandle = config->callback_handle;
    _resizeEventsEnabled = config->resize_events_enabled != 0;
    // Fingers are separate pointers: a second one ends what the first was doing.
    self.multipleTouchEnabled = YES;
    self.opaque = config->clear_alpha >= 1.0;

    CAMetalLayer *layer = (CAMetalLayer *)self.layer;
    layer.device = device;
    layer.pixelFormat = MTLPixelFormatBGRA8Unorm;
    layer.framebufferOnly = YES;
    layer.opaque = config->clear_alpha >= 1.0;
    layer.maximumDrawableCount = 2;
    layer.allowsNextDrawableTimeout = YES;
    [self updateDrawableSize];

    _renderer = [[VivaRenderer alloc]
        initWithLayer:layer
         commandQueue:commandQueue
    rectanglePipeline:rectanglePipeline
      rectangleBuffer:rectangleBuffer
       rectangleCount:config->rectangle_count
       rectangleClips:config->rectangle_clips
         textPipeline:textPipeline
           textLabels:config->text_labels
       textLabelCount:config->text_label_count
           clearColor:MTLClearColorMake(config->clear_red, config->clear_green, config->clear_blue, config->clear_alpha)
             viewport:self.bounds.size];
    if (_renderer == nil) {
        [self release];
        return nil;
    }
    return self;
}

- (void)dealloc {
    [_renderer release];
    [super dealloc];
}

// updateDrawableSize makes the layer as many pixels as the view has points times
// the screen's scale, so the picture is not stretched.
- (void)updateDrawableSize {
    CGFloat scale = self.window.screen.scale ?: UIScreen.mainScreen.scale;
    CAMetalLayer *layer = (CAMetalLayer *)self.layer;
    CGSize size = self.bounds.size;
    layer.contentsScale = scale;
    layer.drawableSize = CGSizeMake(size.width * scale, size.height * scale);
}

// layoutSubviews runs when the view gets its size, the first time and on every
// rotation: the application is told, and answers with the scene at that size.
- (void)layoutSubviews {
    [super layoutSubviews];
    if (_closed) {
        return;
    }
    [self updateDrawableSize];
    CGSize size = self.bounds.size;
    if (_resizeEventsEnabled && (!_reportedSize || !CGSizeEqualToSize(size, _lastReportedSize))) {
        _reportedSize = YES;
        _lastReportedSize = size;
        vivaViewResized(_callbackHandle, self, size.width, size.height);
    }
}

- (void)render {
    [self renderWaitingForSchedule:NO];
}

- (void)renderWaitingForSchedule:(BOOL)wait {
    if (_closed) {
        return;
    }
    @autoreleasepool {
        CAMetalLayer *layer = (CAMetalLayer *)self.layer;
        if (layer.drawableSize.width < 1 || layer.drawableSize.height < 1) {
            return;
        }
        id<CAMetalDrawable> drawable = [layer nextDrawable];
        if (drawable == nil) {
            return;
        }
        id<MTLCommandBuffer> commandBuffer = [_renderer renderToDrawable:drawable
                                                                viewport:self.bounds.size
                                                            inputTexture:nil
                                                              inputFrame:CGRectZero
                                                              inputLayer:0];
        if (wait) {
            [commandBuffer waitUntilScheduled];
        }
    }
}

- (BOOL)recoverResources {
    @autoreleasepool {
        [self updateDrawableSize];
        return [_renderer recoverWithViewport:self.bounds.size];
    }
}

- (int)updateRectangles:(const viva_rectangle *)rectangles
                  clips:(const viva_clip *)clips
             totalCount:(NSUInteger)totalCount
                  first:(NSUInteger)first
                  count:(NSUInteger)count
                rebuild:(BOOL)rebuild
                 render:(BOOL)render {
    int status = [_renderer updateRectangles:rectangles
                                       clips:clips
                                  totalCount:totalCount
                                       first:first
                                       count:count
                                     rebuild:rebuild];
    if (status == 0 && render) {
        [self render];
    }
    return status;
}

- (int)updateLabels:(const viva_text_label *)labels count:(NSUInteger)count {
    return [_renderer updateLabels:labels count:count viewport:self.bounds.size];
}

// closeWindow ends the window for the application: iOS windows are not closed
// by the user, so this is the application's own doing, after a failure.
- (void)closeWindow {
    if (_closed) {
        return;
    }
    _closed = YES;
    vivaWindowClosed(_callbackHandle);
}

#pragma mark Touches

// emitTouches reports each finger of a phase. A finger is its UITouch for as long
// as it touches, so the object's address names it.
- (void)emitTouches:(NSSet<UITouch *> *)touches kind:(int)kind {
    if (_closed) {
        return;
    }
    for (UITouch *touch in touches) {
        CGPoint point = [touch locationInView:self];
        CGPoint onScreen = [touch locationInView:nil];
        int identifier = (int)(((uintptr_t)touch >> 4) & 0x7fffffff);
        vivaTouchEvent(_callbackHandle, self, kind, identifier, point.x, point.y, onScreen.x, onScreen.y);
    }
}

- (void)touchesBegan:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
    [self emitTouches:touches kind:VIVA_IOS_POINTER_DOWN];
}

- (void)touchesMoved:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
    [self emitTouches:touches kind:VIVA_IOS_POINTER_MOVE];
}

- (void)touchesEnded:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
    [self emitTouches:touches kind:VIVA_IOS_POINTER_UP];
}

- (void)touchesCancelled:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
    [self emitTouches:touches kind:VIVA_IOS_POINTER_CANCEL];
}

@end

@interface VivaIOSViewController : UIViewController
@end

@implementation VivaIOSViewController

- (UIInterfaceOrientationMask)supportedInterfaceOrientations {
    return UIInterfaceOrientationMaskAll;
}

@end

@interface VivaIOSAppDelegate : UIResponder <UIApplicationDelegate>
@end

@implementation VivaIOSAppDelegate

- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions {
    vivaIOSLaunched();
    return YES;
}

@end

/* The one window an iPhone application shows (see viva_application_open_window). */
static UIWindow *viva_ios_window;
static VivaIOSView *viva_ios_view;

int viva_ios_main(void) {
    @autoreleasepool {
        char *argv[] = {"viva", NULL};
        return UIApplicationMain(1, argv, nil, NSStringFromClass([VivaIOSAppDelegate class]));
    }
}

void viva_ios_terminate(int status) {
    exit(status);
}

int viva_ios_is_simulator(void) {
#if TARGET_OS_SIMULATOR
    return 1;
#else
    return 0;
#endif
}

int viva_application_last_status(void) {
    return viva_application_status;
}

/* viva_application_open_window makes the application's window: one that fills the
 * screen, as a phone's does. A second one is refused, since there is no screen to
 * put it on; how an application shows a second surface there is a decision the
 * framework has yet to make (ADR 0095). It must be called from the launch
 * callback, on the main thread. */
void *viva_application_open_window(viva_window_config config) {
    @autoreleasepool {
        if (viva_ios_view != nil) {
            viva_application_status = VIVA_IOS_STATUS_ONE_WINDOW;
            return NULL;
        }
        id<MTLDevice> device = nil;
        id<MTLCommandQueue> commandQueue = nil;
        id<MTLRenderPipelineState> rectanglePipeline = nil;
        id<MTLRenderPipelineState> textPipeline = nil;
        id<MTLBuffer> rectangleBuffer = nil;
        int status = viva_make_scene_resources(
            &config, &device, &commandQueue, &rectanglePipeline, &textPipeline, &rectangleBuffer);
        if (status != 0) {
            viva_application_status = status;
            return NULL;
        }
        CGRect screen = UIScreen.mainScreen.bounds;
        VivaIOSView *view = [[VivaIOSView alloc] initWithFrame:screen
                                                        device:device
                                                  commandQueue:commandQueue
                                             rectanglePipeline:rectanglePipeline
                                               rectangleBuffer:rectangleBuffer
                                                  textPipeline:textPipeline
                                                        config:&config];
        [rectangleBuffer release];
        [textPipeline release];
        [rectanglePipeline release];
        [commandQueue release];
        [device release];
        if (view == nil) {
            viva_application_status = VIVA_IOS_STATUS_NO_VIEW;
            return NULL;
        }
        VivaIOSViewController *controller = [[VivaIOSViewController alloc] init];
        controller.view = view;
        UIWindow *window = [[UIWindow alloc] initWithFrame:screen];
        window.rootViewController = controller;
        [window makeKeyAndVisible];
        [controller release];
        viva_ios_window = window;
        viva_ios_view = view;
        return view;
    }
}

/* viva_request_redraw asks, from any thread, for the application to paint again. */
void viva_request_redraw(uintptr_t callback_handle) {
    dispatch_async(dispatch_get_main_queue(), ^{
        VivaIOSView *view = viva_ios_view;
        if (view != nil && !view.closed && view.callbackHandle == callback_handle) {
            vivaRedrawRequested(callback_handle, view);
        }
    });
}

/* The window always fills the screen: where it is and how big, in the points of
 * the screen, and that it is full screen, are not the application's to change. */
int viva_application_window_frame(void *window, double *out) {
    VivaIOSView *view = (VivaIOSView *)window;
    if (view == nil || view != viva_ios_view) {
        return 0;
    }
    CGRect bounds = view.window.bounds;
    out[0] = 0;
    out[1] = 0;
    out[2] = bounds.size.width;
    out[3] = bounds.size.height;
    return 1;
}

void viva_application_move_window(void *window, double x, double y) {}
void viva_application_set_window_size(void *window, double width, double height) {}
int viva_application_set_fullscreen(void *window, int fullscreen) { return 0; }
int viva_application_fullscreen(void *window) { return 1; }
int viva_application_set_always_on_top(void *window, int enabled) { return 0; }
void viva_application_hide_window(void *window) {}
void viva_application_show_window(void *window) {}

int viva_application_set_title(void *window, const char *title) {
    VivaIOSView *view = (VivaIOSView *)window;
    NSString *text = title != NULL ? [NSString stringWithUTF8String:title] : nil;
    view.window.windowScene.title = text;
    return 0;
}

void viva_application_close_all(void) {
    [viva_ios_view closeWindow];
}

void viva_application_close_window(void *window) {
    [(VivaIOSView *)window closeWindow];
}

int viva_view_update_rectangles(
    void *view,
    const viva_rectangle *rectangles,
    const viva_clip *clips,
    size_t total_count,
    size_t first,
    size_t count,
    int rebuild,
    int render
) {
    if (view == NULL) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaIOSView *)view updateRectangles:rectangles
                                               clips:clips
                                          totalCount:total_count
                                               first:first
                                               count:count
                                             rebuild:rebuild != 0
                                              render:render != 0];
    }
}

int viva_view_update_labels(void *view, const viva_text_label *labels, size_t count) {
    if (view == NULL) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaIOSView *)view updateLabels:labels count:count];
    }
}

int viva_view_recover(void *view) {
    if (view == NULL) {
        return 1;
    }
    return [(VivaIOSView *)view recoverResources] ? 0 : 1;
}

void viva_view_render(void *view) {
    [(VivaIOSView *)view render];
}

void viva_view_render_sync(void *view) {
    [(VivaIOSView *)view renderWaitingForSchedule:YES];
}

void viva_view_close(void *view) {
    [(VivaIOSView *)view closeWindow];
}

void viva_view_close_confirmed(void *view) {
    [(VivaIOSView *)view closeWindow];
}

/* What an iOS view does not take yet (ADR 0095): the system keyboard, the
 * accessibility tree and the web views come with their own work packages. They
 * accept what the application sends and show nothing of it. */
int viva_view_update_accessibility(void *view, const viva_accessibility_node *nodes, size_t count) {
    return 0;
}

int viva_view_update_text(
    void *view,
    const unsigned char *utf8,
    size_t utf8_length,
    size_t selected_start,
    size_t selected_length,
    size_t marked_start,
    size_t marked_length,
    int has_marked_text,
    int tentative_composition,
    int ends_composition,
    int render
) {
    return 0;
}

int viva_view_update_text_input(void *view, viva_text_input_config text_input) {
    return 0;
}

int viva_view_update_text_input_keeping_composition(void *view, viva_text_input_config text_input) {
    return 0;
}

int viva_view_move_text_input(void *view, float x, float y, float width, float height) {
    return 0;
}

void viva_web_ready_later(uintptr_t callback_handle) {}

/* The AppKit text input's key routing is a Mac keyboard's: an iOS one comes
 * through UIKit's own text input, so nothing here asks for raw keys. */
int viva_text_input_row_key_is_raw(int placed, int composing, int key, int command) { return 0; }
int viva_text_input_navigation_key_is_raw(int navigation_keys, int composing, int key) { return 0; }
int viva_text_input_should_interpret_key(int enabled, int multiline, int composing, int key) { return 0; }
int viva_text_input_caret_should_blink(
    int enabled,
    size_t selected_length,
    int composing,
    int terminal,
    int hide_native_overlays
) {
    return 0;
}
int viva_terminal_key_routes_raw(
    int composing,
    int named_key,
    int control,
    int command,
    int alt,
    int alt_sends_escape
) {
    return 0;
}
int viva_terminal_key_code(unsigned short character) { return 0; }
