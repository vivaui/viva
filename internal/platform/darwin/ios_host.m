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
extern void vivaWindowClosed(uintptr_t callbackHandle);
extern void vivaTouchEvent(uintptr_t callbackHandle, void *view, int kind, int identifier,
                           double x, double y, double screenX, double screenY);
extern int vivaIOSKeyEvent(uintptr_t callbackHandle, void *view, int usage, int modifiers, int repeat,
                           const unsigned char *utf8, size_t utf8Length);
extern void vivaIOSVisibility(uintptr_t callbackHandle, void *view, int visible);
extern void vivaIOSLifecycle(uintptr_t callbackHandle, void *view, int kind);
extern void vivaIOSAppearanceChanged(void);
extern void vivaIOSSafeArea(uintptr_t callbackHandle, void *view, double top, double left, double bottom, double right);
extern int vivaAccessibilityAction(uintptr_t callbackHandle, void *view, uint64_t node, int action, double number,
                                   const unsigned char *utf8, size_t utf8Length, int start, int end);
extern void vivaIOSTextOp(uintptr_t callbackHandle, void *view, int operation, const unsigned char *utf8, size_t utf8Length,
                          int64_t first, int64_t last);
extern void vivaRedrawRequested(uintptr_t callbackHandle, void *view);

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

/* What the keyboard does to the text input, as vivaIOSTextOp takes it. */
enum {
    VIVA_IOS_TEXT_COMMIT = 1,
    VIVA_IOS_TEXT_SET_MARKED = 2,
    VIVA_IOS_TEXT_FINISH = 3,
    VIVA_IOS_TEXT_REPLACE = 4,
    VIVA_IOS_TEXT_SELECT = 5,
    VIVA_IOS_TEXT_DELETE_BACKWARD = 6,
    VIVA_IOS_TEXT_ENTER = 7,
};

@class VivaIOSView;

/* A position in the text, as a number of UTF-16 units from its start. */
@interface VivaIOSTextPosition : UITextPosition {
@public
    NSInteger _offset;
}
+ (instancetype)positionAt:(NSInteger)offset;
@end

@implementation VivaIOSTextPosition
+ (instancetype)positionAt:(NSInteger)offset {
    VivaIOSTextPosition *position = [[VivaIOSTextPosition alloc] init];
    position->_offset = offset;
    return [position autorelease];
}
@end

@interface VivaIOSTextRange : UITextRange {
    VivaIOSTextPosition *_start;
    VivaIOSTextPosition *_end;
}
+ (instancetype)rangeFrom:(NSInteger)start to:(NSInteger)end;
@end

@implementation VivaIOSTextRange
+ (instancetype)rangeFrom:(NSInteger)start to:(NSInteger)end {
    VivaIOSTextRange *range = [[VivaIOSTextRange alloc] init];
    range->_start = [[VivaIOSTextPosition positionAt:MIN(start, end)] retain];
    range->_end = [[VivaIOSTextPosition positionAt:MAX(start, end)] retain];
    return [range autorelease];
}
- (void)dealloc {
    [_start release];
    [_end release];
    [super dealloc];
}
- (UITextPosition *)start {
    return _start;
}
- (UITextPosition *)end {
    return _end;
}
- (BOOL)isEmpty {
    return _start->_offset == _end->_offset;
}
@end

/* VivaIOSEditor is what the keyboard types into: an invisible view that holds a copy of
 * the application's focused text input and passes what the keyboard does to it back, as
 * ranges of UTF-16 units. The application owns the text; this is only read by UIKit. */
@interface VivaIOSEditor : UIView <UITextInput> {
    NSMutableString *_text;
    NSRange _selected;
    NSRange _marked;
    BOOL _hasMarked;
    BOOL _enabled;
    BOOL _multiline;
    BOOL _secure;
    uintptr_t _callbackHandle;
    VivaIOSView *_surface;
    CGRect _caret;
    id<UITextInputDelegate> _inputDelegate;
    UITextInputStringTokenizer *_tokenizer;
    UITextInputAssistantItem *_assistant;
}
@property(nonatomic, assign) VivaIOSView *surface;
@property(nonatomic, assign) uintptr_t callbackHandle;
@property(nonatomic, readonly) BOOL enabled;
- (void)setState:(const unsigned char *)utf8 length:(size_t)length selection:(NSRange)selection marked:(NSRange)marked hasMarked:(BOOL)hasMarked;
- (void)setEnabled:(BOOL)enabled multiline:(BOOL)multiline secure:(BOOL)secure;
- (void)setCaret:(CGRect)caret;
+ (BOOL)textSystemTakes:(UIKey *)key;
@end


/* The accessibility roles, in the numbering of platform.A11yRole*. */
enum {
    VIVA_A11Y_GROUP = 2, VIVA_A11Y_BUTTON = 3, VIVA_A11Y_TEXT = 4, VIVA_A11Y_TEXT_FIELD = 5, VIVA_A11Y_CHECKBOX = 6,
    VIVA_A11Y_SLIDER = 7, VIVA_A11Y_IMAGE = 8, VIVA_A11Y_LINK = 9, VIVA_A11Y_RADIO = 10, VIVA_A11Y_TAB = 11,
    VIVA_A11Y_MENU_ITEM = 12, VIVA_A11Y_SWITCH = 14, VIVA_A11Y_OPTION = 18, VIVA_A11Y_GRID_CELL = 20,
    VIVA_A11Y_HEADING = 21, VIVA_A11Y_STATUS = 22, VIVA_A11Y_PROGRESS = 24,
};

/* One node of the program's accessibility tree, as VoiceOver reads it. It holds what the
 * node said when it was last sent; VoiceOver's requests go back to the program by the
 * node's identity. */
@interface VivaIOSAccessibleElement : UIAccessibilityElement {
    uint64_t _identity;
    int _role;
    BOOL _operable;
    uintptr_t _callbackHandle;
    void *_surface;
    double _rangeNow;
    double _rangeMin;
    double _rangeMax;
}
- (instancetype)initWithNode:(const viva_accessibility_node *)node container:(UIView *)container handle:(uintptr_t)handle;
@end

@implementation VivaIOSAccessibleElement

static NSString *viva_string(const char *chars, size_t length) {
    if (chars == NULL || length == 0) {
        return nil;
    }
    return [[[NSString alloc] initWithBytes:chars length:length encoding:NSUTF8StringEncoding] autorelease];
}

- (instancetype)initWithNode:(const viva_accessibility_node *)node container:(UIView *)container handle:(uintptr_t)handle {
    self = [super initWithAccessibilityContainer:container];
    if (self == nil) {
        return nil;
    }
    _identity = node->id;
    _role = node->role;
    _callbackHandle = handle;
    _surface = container;
    _rangeNow = node->range_now;
    _rangeMin = node->range_min;
    _rangeMax = node->range_max;
    _operable = !node->disabled;
    self.accessibilityLabel = viva_string(node->label, node->label_length);
    NSString *value = viva_string(node->value, node->value_length);
    if (_role == VIVA_A11Y_CHECKBOX || _role == VIVA_A11Y_SWITCH || _role == VIVA_A11Y_RADIO) {
        value = node->mixed ? @"2" : node->checked ? @"1" : @"0";
    } else if (node->range_set && value == nil) {
        double span = node->range_max - node->range_min;
        value = span > 0 ? [NSString stringWithFormat:@"%.0f%%", (node->range_now - node->range_min) / span * 100] : nil;
    }
    self.accessibilityValue = value;
    NSString *hint = viva_string(node->hint, node->hint_length);
    if (hint == nil) {
        hint = viva_string(node->description, node->description_length);
    }
    self.accessibilityHint = hint;
    self.accessibilityFrameInContainerSpace = CGRectMake(node->x, node->y, node->width, node->height);
    UIAccessibilityTraits traits = UIAccessibilityTraitNone;
    switch (_role) {
    case VIVA_A11Y_BUTTON:
    case VIVA_A11Y_MENU_ITEM:
    case VIVA_A11Y_TAB:
    case VIVA_A11Y_OPTION:
    case VIVA_A11Y_RADIO:
    case VIVA_A11Y_GRID_CELL:
    case VIVA_A11Y_CHECKBOX:
    case VIVA_A11Y_SWITCH:
        traits |= UIAccessibilityTraitButton;
        break;
    case VIVA_A11Y_LINK:
        traits |= UIAccessibilityTraitLink;
        break;
    case VIVA_A11Y_SLIDER:
        traits |= UIAccessibilityTraitAdjustable;
        break;
    case VIVA_A11Y_HEADING:
        traits |= UIAccessibilityTraitHeader | UIAccessibilityTraitStaticText;
        break;
    case VIVA_A11Y_TEXT:
    case VIVA_A11Y_STATUS:
        traits |= UIAccessibilityTraitStaticText;
        break;
    case VIVA_A11Y_IMAGE:
        traits |= UIAccessibilityTraitImage;
        break;
    default:
        break;
    }
    if (node->selected) {
        traits |= UIAccessibilityTraitSelected;
    }
    if (node->disabled) {
        traits |= UIAccessibilityTraitNotEnabled;
    }
    if (node->live != 0) {
        traits |= UIAccessibilityTraitUpdatesFrequently;
    }
    self.accessibilityTraits = traits;
    // A group only holds others; what it contains is what is read.
    self.isAccessibilityElement = _role != 1 && _role != VIVA_A11Y_GROUP && (self.accessibilityLabel != nil || value != nil || traits != UIAccessibilityTraitNone);
    return self;
}

- (BOOL)accessibilityActivate {
    if (!_operable) {
        return NO;
    }
    return vivaAccessibilityAction(_callbackHandle, _surface, _identity, 1, 0, NULL, 0, 0, 0) != 0;
}

- (void)accessibilityIncrement {
    vivaAccessibilityAction(_callbackHandle, _surface, _identity, 3, 0, NULL, 0, 0, 0);
}

- (void)accessibilityDecrement {
    vivaAccessibilityAction(_callbackHandle, _surface, _identity, 4, 0, NULL, 0, 0, 0);
}

@end

@interface VivaIOSView : UIView {
    NSMutableArray *_accessibleElements;
    VivaIOSEditor *_editor;
    VivaRenderer *_renderer;
    uintptr_t _callbackHandle;
    BOOL _resizeEventsEnabled;
    BOOL _closed;
    CGSize _lastReportedSize;
    BOOL _reportedSize;
    UIEdgeInsets _safeArea;
    UIEdgeInsets _lastReportedSafeArea;
}
@property(nonatomic, readonly) uintptr_t callbackHandle;
/* How far the system covers each edge (status bar, sensor housing, home indicator), which
 * the view controller sets; the next layout tells the application. */
@property(nonatomic) UIEdgeInsets safeArea;
@property(nonatomic, readonly) BOOL closed;
@property(nonatomic, readonly) VivaIOSEditor *editor;
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
- (int)updateAccessibility:(const viva_accessibility_node *)nodes count:(NSUInteger)count;
- (void)closeWindow;
@end

@implementation VivaIOSView

@synthesize callbackHandle = _callbackHandle;
@synthesize closed = _closed;
@synthesize editor = _editor;
@synthesize safeArea = _safeArea;

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
    _editor = [[VivaIOSEditor alloc] initWithFrame:CGRectMake(0, 0, 1, 1)];
    _editor.surface = self;
    _editor.callbackHandle = _callbackHandle;
    [self addSubview:_editor];
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
    [_accessibleElements release];
    [_editor release];
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
    if (_resizeEventsEnabled && (!_reportedSize || !CGSizeEqualToSize(size, _lastReportedSize) ||
                                 !UIEdgeInsetsEqualToEdgeInsets(_safeArea, _lastReportedSafeArea))) {
        _reportedSize = YES;
        _lastReportedSize = size;
        _lastReportedSafeArea = _safeArea;
        // The insets come first: the resize that follows lays the page out inside them.
        vivaIOSSafeArea(_callbackHandle, self, _safeArea.top, _safeArea.left, _safeArea.bottom, _safeArea.right);
        vivaViewResized(_callbackHandle, self, size.width, size.height);
    }
}

- (void)render {
    [self renderWaitingForSchedule:NO];
}

- (void)renderWaitingForSchedule:(BOOL)wait {
    // The system kills an application that submits GPU work from the background.
    if (_closed || UIApplication.sharedApplication.applicationState == UIApplicationStateBackground) {
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

#pragma mark Accessibility

// updateAccessibility makes the program's tree what VoiceOver reads. The surface is the
// container of the elements: it is not itself read.
- (int)updateAccessibility:(const viva_accessibility_node *)nodes count:(NSUInteger)count {
    NSMutableArray *elements = [[NSMutableArray alloc] initWithCapacity:count];
    for (NSUInteger index = 0; index < count; index++) {
        VivaIOSAccessibleElement *element = [[VivaIOSAccessibleElement alloc] initWithNode:&nodes[index] container:self handle:_callbackHandle];
        if (element != nil) {
            [elements addObject:element];
            [element release];
        }
    }
    [_accessibleElements release];
    _accessibleElements = elements;
    UIAccessibilityPostNotification(UIAccessibilityLayoutChangedNotification, nil);
    return 0;
}

- (BOOL)isAccessibilityElement {
    return NO;
}

- (NSArray *)accessibilityElements {
    return _accessibleElements;
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

#pragma mark Hardware keyboard

- (BOOL)canBecomeFirstResponder {
    return YES;
}

- (void)didMoveToWindow {
    [super didMoveToWindow];
    if (self.window != nil) {
        [self becomeFirstResponder];
    }
}

// pressesBegan reports the keys of a connected keyboard. The keys that are not
// text arrive as key events and the chords with Control or Command as shortcut
// offers; what is typed is the text input's (see UITextInput).
- (void)pressesBegan:(NSSet<UIPress *> *)presses withEvent:(UIPressesEvent *)event {
    NSMutableSet<UIPress *> *others = [NSMutableSet set];
    for (UIPress *press in presses) {
        UIKey *key = press.key;
        if (key == nil || _closed || (_editor.enabled && [VivaIOSEditor textSystemTakes:key])) {
            [others addObject:press];
            continue;
        }
        const char *chars = key.charactersIgnoringModifiers.UTF8String;
        size_t length = chars != NULL ? strlen(chars) : 0;
        if (vivaIOSKeyEvent(_callbackHandle, self, (int)key.keyCode, (int)key.modifierFlags, 0, (const unsigned char *)chars, length) == 0) {
            [others addObject:press];
        }
    }
    if (others.count > 0) {
        [super pressesBegan:others withEvent:event];
    }
}

@end


@implementation VivaIOSEditor

@synthesize surface = _surface;
@synthesize callbackHandle = _callbackHandle;
@synthesize enabled = _enabled;
@synthesize markedTextStyle;
@synthesize inputDelegate = _inputDelegate;

- (instancetype)initWithFrame:(CGRect)frame {
    self = [super initWithFrame:frame];
    if (self != nil) {
        _text = [[NSMutableString alloc] init];
        _selected = NSMakeRange(0, 0);
        _marked = NSMakeRange(NSNotFound, 0);
        self.alpha = 0;
    }
    return self;
}

- (void)dealloc {
    [_text release];
    [_tokenizer release];
    [markedTextStyle release];
    [super dealloc];
}

/* textSystemTakes says a hardware key is the text system's while a field is focused: it
 * arrives as insertText, deleteBackward or a change of the selection, not as a key. */
+ (BOOL)textSystemTakes:(UIKey *)key {
    switch (key.keyCode) {
    case UIKeyboardHIDUsageKeyboardReturnOrEnter:
    case UIKeyboardHIDUsageKeyboardDeleteOrBackspace:
    case UIKeyboardHIDUsageKeyboardDeleteForward:
    case UIKeyboardHIDUsageKeyboardLeftArrow:
    case UIKeyboardHIDUsageKeyboardRightArrow:
    case UIKeyboardHIDUsageKeyboardHome:
    case UIKeyboardHIDUsageKeyboardEnd:
    case UIKeyboardHIDUsageKeyboardSpacebar:
        return YES;
    default:
        return NO;
    }
}

- (BOOL)canBecomeFirstResponder {
    return _enabled;
}

/* The editor is not there to be touched: the surface under it is. */
- (BOOL)pointInside:(CGPoint)point withEvent:(UIEvent *)event {
    return NO;
}

- (UIView *)textInputView {
    return self;
}


- (void)setEnabled:(BOOL)enabled multiline:(BOOL)multiline secure:(BOOL)secure {
    BOOL restart = _enabled != enabled || _multiline != multiline || _secure != secure;
    _enabled = enabled;
    _multiline = multiline;
    _secure = secure;
    if (!restart) {
        return;
    }
    if (enabled) {
        if (self.isFirstResponder) {
            [self reloadInputViews];
        } else {
            [self becomeFirstResponder];
        }
    } else {
        [self resignFirstResponder];
        // The keys of a connected keyboard go on to the surface.
        [_surface becomeFirstResponder];
    }
}

- (void)setState:(const unsigned char *)utf8 length:(size_t)length selection:(NSRange)selection marked:(NSRange)marked hasMarked:(BOOL)hasMarked {
    NSString *value = length > 0 ? [[NSString alloc] initWithBytes:utf8 length:length encoding:NSUTF8StringEncoding] : [[NSString alloc] init];
    BOOL textChanged = ![_text isEqualToString:value];
    BOOL selectionChanged = !NSEqualRanges(_selected, selection) || _hasMarked != hasMarked || (hasMarked && !NSEqualRanges(_marked, marked));
    if (textChanged) {
        [_inputDelegate textWillChange:self];
    }
    if (selectionChanged) {
        [_inputDelegate selectionWillChange:self];
    }
    [_text setString:value];
    [value release];
    _selected = NSMakeRange(MIN(selection.location, _text.length), MIN(selection.length, _text.length - MIN(selection.location, _text.length)));
    _hasMarked = hasMarked;
    _marked = hasMarked ? NSMakeRange(MIN(marked.location, _text.length), MIN(marked.length, _text.length - MIN(marked.location, _text.length))) : NSMakeRange(NSNotFound, 0);
    if (selectionChanged) {
        [_inputDelegate selectionDidChange:self];
    }
    if (textChanged) {
        [_inputDelegate textDidChange:self];
    }
}

- (void)setCaret:(CGRect)caret {
    _caret = caret;
}

- (void)sendOperation:(int)operation text:(NSString *)text first:(NSInteger)first last:(NSInteger)last {
    NSData *data = [text dataUsingEncoding:NSUTF8StringEncoding];
    vivaIOSTextOp(_callbackHandle, _surface, operation, data.bytes, data.length, first, last);
}

#pragma mark UIKeyInput

- (BOOL)hasText {
    return _text.length > 0;
}

- (void)insertText:(NSString *)text {
    if ([text isEqualToString:@"\n"] && !_multiline) {
        [self sendOperation:VIVA_IOS_TEXT_ENTER text:@"" first:0 last:0];
        return;
    }
    [self sendOperation:VIVA_IOS_TEXT_COMMIT text:text first:0 last:0];
}

- (void)deleteBackward {
    [self sendOperation:VIVA_IOS_TEXT_DELETE_BACKWARD text:@"" first:0 last:0];
}

#pragma mark UITextInput: text

- (NSString *)textInRange:(UITextRange *)range {
    VivaIOSTextRange *r = (VivaIOSTextRange *)range;
    NSInteger start = MIN(((VivaIOSTextPosition *)r.start)->_offset, (NSInteger)_text.length);
    NSInteger end = MIN(((VivaIOSTextPosition *)r.end)->_offset, (NSInteger)_text.length);
    return [_text substringWithRange:NSMakeRange(start, MAX(end - start, 0))];
}

- (void)replaceRange:(UITextRange *)range withText:(NSString *)text {
    VivaIOSTextRange *r = (VivaIOSTextRange *)range;
    [self sendOperation:VIVA_IOS_TEXT_REPLACE text:text first:((VivaIOSTextPosition *)r.start)->_offset last:((VivaIOSTextPosition *)r.end)->_offset];
}

- (UITextRange *)selectedTextRange {
    return [VivaIOSTextRange rangeFrom:_selected.location to:_selected.location + _selected.length];
}

- (void)setSelectedTextRange:(UITextRange *)range {
    VivaIOSTextRange *r = (VivaIOSTextRange *)range;
    if (r == nil) {
        return;
    }
    [self sendOperation:VIVA_IOS_TEXT_SELECT text:@"" first:((VivaIOSTextPosition *)r.start)->_offset last:((VivaIOSTextPosition *)r.end)->_offset];
}

- (UITextRange *)markedTextRange {
    return _hasMarked ? [VivaIOSTextRange rangeFrom:_marked.location to:_marked.location + _marked.length] : nil;
}

- (void)setMarkedText:(NSString *)markedText selectedRange:(NSRange)selectedRange {
    NSInteger start = selectedRange.location == NSNotFound ? (NSInteger)markedText.length : (NSInteger)selectedRange.location;
    [self sendOperation:VIVA_IOS_TEXT_SET_MARKED text:markedText ?: @"" first:start last:start + (NSInteger)selectedRange.length];
}

- (void)unmarkText {
    [self sendOperation:VIVA_IOS_TEXT_FINISH text:@"" first:0 last:0];
}

#pragma mark UITextInput: positions

- (UITextPosition *)beginningOfDocument {
    return [VivaIOSTextPosition positionAt:0];
}

- (UITextPosition *)endOfDocument {
    return [VivaIOSTextPosition positionAt:_text.length];
}

- (UITextRange *)textRangeFromPosition:(UITextPosition *)from toPosition:(UITextPosition *)to {
    return [VivaIOSTextRange rangeFrom:((VivaIOSTextPosition *)from)->_offset to:((VivaIOSTextPosition *)to)->_offset];
}

- (UITextPosition *)positionFromPosition:(UITextPosition *)position offset:(NSInteger)offset {
    NSInteger target = ((VivaIOSTextPosition *)position)->_offset + offset;
    if (target < 0 || target > (NSInteger)_text.length) {
        return nil;
    }
    return [VivaIOSTextPosition positionAt:target];
}

- (UITextPosition *)positionFromPosition:(UITextPosition *)position inDirection:(UITextLayoutDirection)direction offset:(NSInteger)offset {
    NSInteger from = ((VivaIOSTextPosition *)position)->_offset;
    NSInteger target = from;
    switch (direction) {
    case UITextLayoutDirectionLeft:
        target = from - offset;
        break;
    case UITextLayoutDirectionRight:
        target = from + offset;
        break;
    case UITextLayoutDirectionUp:
        target = 0;
        break;
    case UITextLayoutDirectionDown:
        target = _text.length;
        break;
    }
    return [VivaIOSTextPosition positionAt:MIN(MAX(target, 0), (NSInteger)_text.length)];
}

- (NSComparisonResult)comparePosition:(UITextPosition *)position toPosition:(UITextPosition *)other {
    NSInteger a = ((VivaIOSTextPosition *)position)->_offset;
    NSInteger b = ((VivaIOSTextPosition *)other)->_offset;
    return a < b ? NSOrderedAscending : a > b ? NSOrderedDescending : NSOrderedSame;
}

- (NSInteger)offsetFromPosition:(UITextPosition *)from toPosition:(UITextPosition *)to {
    return ((VivaIOSTextPosition *)to)->_offset - ((VivaIOSTextPosition *)from)->_offset;
}

- (id<UITextInputTokenizer>)tokenizer {
    if (_tokenizer == nil) {
        _tokenizer = [[UITextInputStringTokenizer alloc] initWithTextInput:self];
    }
    return _tokenizer;
}

- (UITextPosition *)positionWithinRange:(UITextRange *)range farthestInDirection:(UITextLayoutDirection)direction {
    BOOL end = direction == UITextLayoutDirectionRight || direction == UITextLayoutDirectionDown;
    return end ? range.end : range.start;
}

- (UITextRange *)characterRangeByExtendingPosition:(UITextPosition *)position inDirection:(UITextLayoutDirection)direction {
    NSInteger offset = ((VivaIOSTextPosition *)position)->_offset;
    BOOL forward = direction == UITextLayoutDirectionRight || direction == UITextLayoutDirectionDown;
    return [VivaIOSTextRange rangeFrom:offset to:forward ? _text.length : 0];
}

- (NSWritingDirection)baseWritingDirectionForPosition:(UITextPosition *)position inDirection:(UITextStorageDirection)direction {
    return NSWritingDirectionNatural;
}

- (void)setBaseWritingDirection:(NSWritingDirection)direction forRange:(UITextRange *)range {
}

#pragma mark UITextInput: geometry

- (CGRect)firstRectForRange:(UITextRange *)range {
    return [self convertRect:_caret fromView:_surface];
}

- (CGRect)caretRectForPosition:(UITextPosition *)position {
    return [self convertRect:CGRectMake(CGRectGetMinX(_caret), CGRectGetMinY(_caret), 2, CGRectGetHeight(_caret)) fromView:_surface];
}

- (NSArray<UITextSelectionRect *> *)selectionRectsForRange:(UITextRange *)range {
    return @[];
}

- (UITextPosition *)closestPositionToPoint:(CGPoint)point {
    return [VivaIOSTextPosition positionAt:_selected.location + _selected.length];
}

- (UITextPosition *)closestPositionToPoint:(CGPoint)point withinRange:(UITextRange *)range {
    return range.end;
}

- (UITextRange *)characterRangeAtPoint:(CGPoint)point {
    return nil;
}

#pragma mark UITextInputTraits

- (UIKeyboardType)keyboardType {
    return UIKeyboardTypeDefault;
}

- (BOOL)isSecureTextEntry {
    return _secure;
}

- (UIReturnKeyType)returnKeyType {
    return _multiline ? UIReturnKeyDefault : UIReturnKeyDone;
}

- (UITextAutocorrectionType)autocorrectionType {
    return _secure ? UITextAutocorrectionTypeNo : UITextAutocorrectionTypeDefault;
}

- (UITextAutocapitalizationType)autocapitalizationType {
    return UITextAutocapitalizationTypeNone;
}

- (UITextSpellCheckingType)spellCheckingType {
    return _secure ? UITextSpellCheckingTypeNo : UITextSpellCheckingTypeDefault;
}

@end


@interface VivaIOSViewController : UIViewController {
    VivaIOSView *_surface;
}
- (void)setSurface:(VivaIOSView *)surface;
@end

@implementation VivaIOSViewController

- (void)loadView {
    UIView *root = [[UIView alloc] init];
    root.backgroundColor = UIColor.systemBackgroundColor;
    self.view = root;
    [root release];
}

- (void)setSurface:(VivaIOSView *)surface {
    _surface = surface;
    [self.view addSubview:surface];
    [[NSNotificationCenter defaultCenter] addObserver:self
                                             selector:@selector(keyboardWillChange:)
                                                 name:UIKeyboardWillChangeFrameNotification
                                               object:nil];
}

- (void)dealloc {
    [[NSNotificationCenter defaultCenter] removeObserver:self];
    [super dealloc];
}

// keyboardWillChange moves the surface with the keyboard, in the keyboard's own
// animation.
- (void)keyboardWillChange:(NSNotification *)notification {
    NSTimeInterval duration = [notification.userInfo[UIKeyboardAnimationDurationUserInfoKey] doubleValue];
    UIViewAnimationOptions curve = [notification.userInfo[UIKeyboardAnimationCurveUserInfoKey] unsignedIntegerValue] << 16;
    [self.view setNeedsLayout];
    [UIView animateWithDuration:duration delay:0 options:curve animations:^{
        [self.view layoutIfNeeded];
    } completion:nil];
}

// viewDidLayoutSubviews makes the surface the whole window, as far as the keyboard
// allows, and tells it how far the system covers each edge: the status bar and the
// sensor housing, the home indicator. The application draws under them and keeps what
// matters inside them (ADR 0096). The keyboard shrinks the surface from the bottom, and
// with it up nothing covers the bottom edge but the keyboard itself.
- (void)viewDidLayoutSubviews {
    [super viewDidLayoutSubviews];
    CGRect bounds = self.view.bounds;
    UIEdgeInsets safe = self.view.safeAreaInsets;
    CGFloat keyboardTop = CGRectGetMinY(self.view.keyboardLayoutGuide.layoutFrame);
    // With no keyboard the guide's top is the safe area's bottom edge, the home indicator's
    // line; the keyboard is up when it is higher than that.
    if (keyboardTop < CGRectGetMaxY(bounds) - safe.bottom - 0.5) {
        bounds.size.height = MAX(keyboardTop - bounds.origin.y, 0);
        safe.bottom = 0;
    }
    _surface.safeArea = safe;
    _surface.frame = bounds;
    [_surface setNeedsLayout];
}

- (UIInterfaceOrientationMask)supportedInterfaceOrientations {
    return UIInterfaceOrientationMaskAll;
}

// traitCollectionDidChange tells the application when the system switches between
// its light and dark looks.
- (void)traitCollectionDidChange:(UITraitCollection *)previous {
    [super traitCollectionDidChange:previous];
    if (previous != nil && previous.userInterfaceStyle != self.traitCollection.userInterfaceStyle) {
        vivaIOSAppearanceChanged();
    }
}

@end

@interface VivaIOSAppDelegate : UIResponder <UIApplicationDelegate> {
    BOOL _inBackground;
}
@end

/* The one window an iPhone application shows (see viva_application_open_window). */
static UIWindow *viva_ios_window;
static VivaIOSView *viva_ios_view;

@implementation VivaIOSAppDelegate

- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions {
    vivaIOSLaunched();
    return YES;
}

// The application goes behind another and comes back: it is told. The window is drawn
// again only once the application is active, as the system gives no GPU to one that
// has only begun to return; what the layer shows may not be what it should.
- (void)applicationDidEnterBackground:(UIApplication *)application {
    if (viva_ios_view != nil && !viva_ios_view.closed) {
        _inBackground = YES;
        vivaIOSLifecycle(viva_ios_view.callbackHandle, viva_ios_view, 1); // Background
        vivaIOSVisibility(viva_ios_view.callbackHandle, viva_ios_view, 0);
    }
}

- (void)applicationWillEnterForeground:(UIApplication *)application {
    if (viva_ios_view != nil && !viva_ios_view.closed) {
        vivaIOSLifecycle(viva_ios_view.callbackHandle, viva_ios_view, 2); // Foreground
    }
}

- (void)applicationDidBecomeActive:(UIApplication *)application {
    if (_inBackground && viva_ios_view != nil && !viva_ios_view.closed) {
        _inBackground = NO;
        vivaIOSVisibility(viva_ios_view.callbackHandle, viva_ios_view, 1);
    }
}

- (void)applicationDidReceiveMemoryWarning:(UIApplication *)application {
    if (viva_ios_view != nil && !viva_ios_view.closed) {
        vivaIOSLifecycle(viva_ios_view.callbackHandle, viva_ios_view, 3); // MemoryWarning
    }
}

@end

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
        if (config.accessibility_count > 0) {
            [view updateAccessibility:config.accessibility count:config.accessibility_count];
        }
        VivaIOSViewController *controller = [[VivaIOSViewController alloc] init];
        [controller loadViewIfNeeded];
        [controller setSurface:view];
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
    if (view == NULL) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaIOSView *)view updateAccessibility:nodes count:count];
    }
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
    VivaIOSView *surface = (VivaIOSView *)view;
    if (surface == nil || surface.editor == nil) {
        return 1;
    }
    [surface.editor setState:utf8
                      length:utf8_length
                   selection:NSMakeRange(selected_start, selected_length)
                      marked:NSMakeRange(marked_start, marked_length)
                   hasMarked:has_marked_text != 0];
    return 0;
}

/* viva_view_update_text_input follows the application's wish for its text input: another
 * one, the same with other text, or none, which has the keyboard away. */
static int viva_ios_text_input(void *view, viva_text_input_config text_input) {
    VivaIOSView *surface = (VivaIOSView *)view;
    if (surface == nil || surface.editor == nil) {
        return 1;
    }
    if (text_input.enabled == 0) {
        [surface.editor setEnabled:NO multiline:NO secure:NO];
        return 0;
    }
    [surface.editor setCaret:CGRectMake(text_input.x, text_input.y, text_input.width, text_input.height)];
    [surface.editor setEnabled:YES multiline:text_input.multiline != 0 secure:text_input.secure != 0];
    return viva_view_update_text(view, text_input.utf8, text_input.utf8_length, text_input.selected_start, text_input.selected_length,
                                 text_input.marked_start, text_input.marked_length, text_input.has_marked_text, 0, 0, 0);
}

int viva_view_update_text_input(void *view, viva_text_input_config text_input) {
    return viva_ios_text_input(view, text_input);
}

int viva_view_update_text_input_keeping_composition(void *view, viva_text_input_config text_input) {
    return viva_ios_text_input(view, text_input);
}

int viva_view_move_text_input(void *view, float x, float y, float width, float height) {
    VivaIOSView *surface = (VivaIOSView *)view;
    if (surface == nil || surface.editor == nil) {
        return 1;
    }
    [surface.editor setCaret:CGRectMake(x, y, width, height)];
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
