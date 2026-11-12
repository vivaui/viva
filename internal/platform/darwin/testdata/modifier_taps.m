#include "../native.m"
#include "../main_menu_darwin.m"
#include <assert.h>

// The callbacks native.m reports to Go. The raw key one records what it is told.
static int raw_calls, raw_key, raw_modifiers, raw_flags;
static unsigned int raw_code;
static int plain_key_calls;

void vivaOpenURL(const char *url) {}
void vivaOpenFile(const char *path) {}
int vivaApplicationQuitRequested(void) { return 0; }
int vivaApplicationReopenRequested(void) { return 0; }
int vivaFileDropEvent(uintptr_t handle, void *view, int phase, double x, double y, const char *paths, size_t length) { return 0; }
void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {}
void vivaViewResized(uintptr_t handle, void *view, double width, double height) {}
void vivaRedrawRequested(uintptr_t handle, void *view) {}
void vivaKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) { plain_key_calls++; }
void vivaRawKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int flags, unsigned int code, const unsigned char *utf8, size_t utf8Length, const unsigned char *typed, size_t typedLength) {
    raw_calls++;
    raw_key = key;
    raw_modifiers = modifiers;
    raw_flags = flags;
    raw_code = code;
}
void vivaWindowActivated(uintptr_t handle) {}
void vivaFocusLost(uintptr_t handle, void *view) {}
int vivaTerminalCells(uint32_t character) { return 1; }
int vivaAccessibilityAction(uintptr_t handle, void *view, uint64_t node, int action, double number, const unsigned char *utf8, size_t utf8Length, int start, int end) { return 0; }
viva_text_line *vivaShapedLine(const unsigned char *utf8, size_t length, double size, double weight) { return viva_text_line_create(utf8, length, size, weight); }
viva_text_line *vivaShapedLineStyle(const unsigned char *utf8, size_t length, double size, double weight, int italic) { return viva_text_line_create_style(utf8, length, size, weight, italic); }
int vivaShortcutEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) { return 0; }
void vivaScrollEvent(uintptr_t handle, void *view, double deltaX, double deltaY, double x, double y, int modifiers, int precise) {}
void vivaCloseRequested(uintptr_t handle, void *view) {}
void vivaWindowClosed(uintptr_t handle) {}
void vivaFrameChanged(uintptr_t handle, void *view, double x, double y, double width, double height) {}
void vivaWindowMinimized(uintptr_t handle, void *view, int minimized) {}
void vivaWindowOccluded(uintptr_t handle, void *view, int visible) {}
void vivaWindowFullscreen(uintptr_t handle, void *view, int fullscreen) {}
int vivaWindowReopenRequested(uintptr_t handle, void *view) { return 0; }
int vivaPressAt(uintptr_t handle, double x, double y) { return 0; }
void vivaTextInput(uintptr_t handle, void *view, int kind, const unsigned char *utf8,
                  size_t length, int64_t start, int64_t selectedLength,
                  int64_t replacementStart, int64_t replacementLength, int command) {}
unsigned char *vivaCopyText(uintptr_t handle, void *view, size_t *length) { *length = 0; return NULL; }


// A flagsChanged event for a modifier key, as AppKit makes one: the device
// dependent bit of the key says whether it went down.
static NSEvent *flags_event(unsigned short key_code, NSEventModifierFlags flags) {
    return [NSEvent keyEventWithType:NSEventTypeFlagsChanged location:NSZeroPoint modifierFlags:flags
                           timestamp:0 windowNumber:0 context:nil characters:@"" charactersIgnoringModifiers:@""
                           isARepeat:NO keyCode:key_code];
}

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        // A view that is never put in a window: nothing appears on screen.
        VivaMetalView *view = [[VivaMetalView alloc] initWithFrame:NSMakeRect(0, 0, 400, 300)];
        [view setValue:[NSNumber numberWithUnsignedLong:7] forKey:@"_callbackHandle"];
        [view setValue:[NSNumber numberWithBool:YES] forKey:@"_keyEventsEnabled"];
        NSEventModifierFlags shiftDown = NSEventModifierFlagShift | 0x2; /* left Shift */

        // A window that did not ask for taps hears nothing of a modifier.
        [view flagsChanged:flags_event(56, shiftDown)];
        [view flagsChanged:flags_event(56, 0)];
        assert(raw_calls == 0);

        [view setModifierTapsEnabled:YES];
        [view flagsChanged:flags_event(56, shiftDown)];
        assert(raw_calls == 1);
        assert(raw_key == VIVA_KEY_SHIFT_LEFT);
        assert(raw_flags == 0);          /* a press */
        assert(raw_code == 56 + 1);       /* the virtual key code plus one */
        assert(raw_modifiers != 0);       /* the modifier is in the set, as for a terminal */

        [view flagsChanged:flags_event(56, 0)];
        assert(raw_calls == 2);
        assert(raw_key == VIVA_KEY_SHIFT_LEFT);
        assert(raw_flags == 2);           /* a release */
        assert(raw_code == 56 + 1);       /* the same code as the press, so they pair */

        /* The right-hand key is a key of its own. */
        [view flagsChanged:flags_event(60, NSEventModifierFlagShift | 0x4)];
        assert(raw_calls == 3 && raw_key == VIVA_KEY_SHIFT_RIGHT && raw_flags == 0);
        [view flagsChanged:flags_event(60, 0)];
        assert(raw_calls == 4 && raw_key == VIVA_KEY_SHIFT_RIGHT && raw_flags == 2);

        /* Caps Lock is not a modifier key Viva names. */
        [view flagsChanged:flags_event(57, NSEventModifierFlagCapsLock)];
        assert(raw_calls == 4);

        /* The text system still hears of the change: no key event was made of it. */
        assert(plain_key_calls == 0);

        [view setModifierTapsEnabled:NO];
        [view flagsChanged:flags_event(56, shiftDown)];
        assert(raw_calls == 4);
    }
    printf("modifier taps: ok\n");
    return 0;
}
