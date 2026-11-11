#include "../native.m"
#include "../main_menu_darwin.m"
#include <assert.h>
#include <math.h>

// The callbacks native.m reports to Go; this test listens to none of them.
void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {}
void vivaViewResized(uintptr_t handle, void *view, double width, double height) {}
void vivaRedrawRequested(uintptr_t handle, void *view) {}
void vivaKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) {}
void vivaRawKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int flags, unsigned int code, const unsigned char *utf8, size_t utf8Length, const unsigned char *typed, size_t typedLength) {}
void vivaFocusLost(uintptr_t handle, void *view) {}
int vivaTerminalCells(uint32_t character) { return 1; }
int vivaAccessibilityAction(uintptr_t handle, void *view, uint64_t node, int action, double number, const unsigned char *utf8, size_t utf8Length, int start, int end) { return 0; }
viva_text_line *vivaShapedLine(const unsigned char *utf8, size_t length, double size, double weight) { return viva_text_line_create(utf8, length, size, weight); }
viva_text_line *vivaShapedLineStyle(const unsigned char *utf8, size_t length, double size, double weight, int italic) { return viva_text_line_create_style(utf8, length, size, weight, italic); }
int vivaShortcutEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) { return 0; }
void vivaOpenURL(const char *url) {}
void vivaOpenFile(const char *path) {}
int vivaApplicationQuitRequested(void) { return 0; }
int vivaApplicationReopenRequested(void) { return 0; }
int vivaFileDropEvent(uintptr_t handle, void *view, int phase, double x, double y, const char *paths, size_t length) { return 0; }
void vivaScrollEvent(uintptr_t handle, void *view, double deltaX, double deltaY, double x, double y, int modifiers, int precise) {}
void vivaCloseRequested(uintptr_t handle, void *view) {}
void vivaWindowClosed(uintptr_t handle) {}
void vivaFrameChanged(uintptr_t handle, void *view, double x, double y, double width, double height) {}
void vivaWindowMinimized(uintptr_t handle, void *view, int minimized) {}
void vivaWindowOccluded(uintptr_t handle, void *view, int visible) {}
void vivaWindowFullscreen(uintptr_t handle, void *view, int fullscreen) {}
int vivaWindowReopenRequested(uintptr_t handle, void *view) { return 0; }
uint16_t vivaRectangleLayer(uintptr_t handle, size_t index) { return 0; }
int vivaPressAt(uintptr_t handle, double x, double y) { return 0; }
void vivaTextInput(uintptr_t handle, void *view, int kind, const unsigned char *utf8,
                   size_t length, int64_t start, int64_t selectedLength,
                   int64_t replacementStart, int64_t replacementLength, int command) {}
unsigned char *vivaCopyText(uintptr_t handle, void *view, size_t *length) { *length = 0; return NULL; }

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        assert(device != nil);

        VivaMetalView *view = [[VivaMetalView alloc]
            initWithFrame:NSMakeRect(0, 0, 320, 200)
                   device:device
             commandQueue:nil
        rectanglePipeline:nil
          rectangleBuffer:nil
           rectangleCount:0
           rectangleClips:NULL
             textPipeline:nil
               textLabels:NULL
           textLabelCount:0
             accessibility:NULL
         accessibilityCount:0
                textInput:(viva_text_input_config){0}
   dynamicTextInputEnabled:NO
      pointerEventsEnabled:NO
       resizeEventsEnabled:NO
         keyEventsEnabled:NO
      scrollEventsEnabled:NO
               windowMode:0
           callbackHandle:0
               clearColor:(MTLClearColor){0, 0, 0, 1}];
        assert(view != nil);
        CAMetalLayer *layer = (CAMetalLayer *)view.layer;
        assert([layer isKindOfClass:[CAMetalLayer class]]);

        // A live resize must present the new drawable inside the same Core
        // Animation transaction that changes the layer's bounds. Otherwise the
        // compositor stretches the previous frame across the new bounds, which
        // is what makes cards, labels and the side bar wobble while dragging.
        assert(layer.presentsWithTransaction == NO);
        [view beginLiveResizeRelaxation];
        assert(layer.presentsWithTransaction == YES);
        [view endLiveResizeRelaxation];
        assert(layer.presentsWithTransaction == NO);

        // The drawable still follows the view at the screen's scale, so the
        // presented frame is one texel per device pixel.
        CGFloat scale = MAX(layer.contentsScale, 1.0);
        assert(fabs(layer.drawableSize.width - 320 * scale) < 0.5);
        assert(fabs(layer.drawableSize.height - 200 * scale) < 0.5);
        [view setFrameSize:NSMakeSize(640, 400)];
        assert(fabs(layer.drawableSize.width - 640 * scale) < 0.5);
        assert(fabs(layer.drawableSize.height - 400 * scale) < 0.5);

        printf("live resize: transactional presentation on, drawable 640x400 at %.1fx\n", scale);
        [view release];
    }
    return 0;
}
