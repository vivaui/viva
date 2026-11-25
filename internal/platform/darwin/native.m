//go:build !ios

#import "core.h"
#include <pthread.h>
#include "cursor_owner_darwin.h"
#include "label_geometry.h"
#include "main_menu_darwin.h"
#import <objc/runtime.h>

static uint64_t viva_process_start_ticks;
static uint64_t viva_pointer_event_count;
static uint64_t viva_text_event_count;
static uint64_t viva_resize_event_count;
static uint64_t viva_key_event_count;
static uint64_t viva_accessibility_update_count;
static uint64_t viva_close_request_count;
static uint64_t viva_scroll_event_count;
static double viva_scroll_last_pixel_y;


/* Window chrome modes. */
enum {
    VIVA_WINDOW_TITLED = 0,
    VIVA_WINDOW_FRAMELESS = 1,
    VIVA_WINDOW_CUSTOM_TITLEBAR = 2,
    VIVA_WINDOW_PANEL = 3,
    VIVA_WINDOW_PREVIEW = 4,
    VIVA_WINDOW_NOTIFICATION = 5,
};

/* How far from the status item a panel opens, the margin it keeps from the edge
 * of the screen, and the radius of its corners, in points. */
#define VIVA_PANEL_GAP 4.0
#define VIVA_PANEL_MARGIN 8.0
#define VIVA_PANEL_CORNER_RADIUS 14.0

/* When a panel last closed itself for losing focus, on the clock of the system
 * uptime. A status item's click reads it to tell whether the press that
 * produced the click is what dismissed the panel. */
double viva_panel_dismissed_at = -1000.0;

void viva_application_close_window(void *window);
void viva_application_close_all(void);

/* The status item module (status_item_darwin.m) installs these when it creates
 * its first item, so the event loop can take an item away and clear them at the
 * end without depending on that module: an application with no status item
 * links neither, and the native tests include this file on its own. */
static void (*viva_status_remove_hook)(void *item);
static void (*viva_status_clear_hook)(void);

void viva_install_status_hooks(void (*remove)(void *item), void (*clear)(void)) {
    viva_status_remove_hook = remove;
    viva_status_clear_hook = clear;
}

/* The system titlebar's height in points. A custom titlebar strip of this
 * height leaves the traffic lights where the system puts them. */
#define VIVA_DEFAULT_TITLEBAR_HEIGHT 28.0
// How far the pointer drags, in points, before a press on a backdrop in the
// titlebar strip becomes a window move.
#define VIVA_TITLEBAR_DRAG_SLOP 4.0

// What a press at a point does (platform.PressKind).
enum {
    VIVA_PRESS_CLIENT = 0,
    VIVA_PRESS_CONTROL = 1,
    VIVA_PRESS_MOVE = 2,
    VIVA_PRESS_MOVE_ON_DRAG = 3,
};

/* The three traffic light buttons. */
static const NSWindowButton viva_traffic_buttons[3] = {
    NSWindowCloseButton, NSWindowMiniaturizeButton, NSWindowZoomButton,
};

/* The titlebar container is the view AppKit lays the traffic lights out in:
 * their button's superview's superview. */
static NSView *viva_titlebar_container(NSWindow *window) {
    NSView *close = [window standardWindowButton:NSWindowCloseButton];
    return close.superview.superview;
}

/* Where the traffic lights' centers sit, in points below the top of the
 * window; negative when the window has none. */
static CGFloat viva_traffic_light_center(NSWindow *window) {
    NSButton *close = [window standardWindowButton:NSWindowCloseButton];
    if (close == nil || close.superview == nil) {
        return -1;
    }
    NSRect button = [close convertRect:close.bounds toView:nil];
    return NSHeight(window.frame) - NSMidY(button);
}

/* Centers the traffic lights in a custom titlebar strip taller than the
 * system's own: the titlebar container grows to the strip, so the buttons stay
 * inside it and clickable, and each button is set that far below the top.
 * Idempotent. AppKit puts the buttons back at the system's height on every
 * layout, so callers place them again whenever a button or the container
 * reports a frame change (see keepTrafficLightsCentered). A scale in (0, 1)
 * shrinks the buttons around their original centers whether or not the strip
 * is taller; a strip no taller than the system's keeps the system's positions,
 * and a full screen window keeps the system's layout. */
static void viva_place_traffic_lights(NSWindow *window, CGFloat stripHeight, CGFloat scale) {
    if (window == nil || (window.styleMask & NSWindowStyleMaskFullScreen) != 0) {
        return;
    }
    BOOL taller = stripHeight > VIVA_DEFAULT_TITLEBAR_HEIGHT;
    NSView *container = viva_titlebar_container(window);
    if (container == nil || container.superview == nil) {
        return;
    }
    if (taller && fabs(NSHeight(container.frame) - stripHeight) > 0.25) {
        NSRect frame = container.frame;
        frame.size.height = stripHeight;
        frame.origin.y = NSHeight(container.superview.bounds) - stripHeight;
        container.frame = frame;
    }
    for (int index = 0; index < 3; index++) {
        NSView *button = [window standardWindowButton:viva_traffic_buttons[index]];
        if (button == nil || button.superview == nil) {
            continue;
        }
        // Keep the original drawing coordinates while shrinking the view frame.
        // Using bounds makes repeated AppKit layout notifications idempotent.
        if (isfinite(scale) && scale > 0 && scale < 1) {
            NSSize originalSize = button.bounds.size;
            NSSize targetSize = NSMakeSize(originalSize.width * scale, originalSize.height * scale);
            if (fabs(NSWidth(button.frame) - targetSize.width) > 0.01 ||
                fabs(NSHeight(button.frame) - targetSize.height) > 0.01) {
                NSRect frame = button.frame;
                frame.origin.x += (frame.size.width - targetSize.width) / 2.0;
                // Keep the vertical center too, so the shrink holds in a strip of the
                // system's height where the placement below does not run.
                frame.origin.y += (frame.size.height - targetSize.height) / 2.0;
                frame.size = targetSize;
                button.frame = frame;
                button.bounds = NSMakeRect(0, 0, originalSize.width, originalSize.height);
            }
        }
        if (!taller) {
            continue;
        }
        CGFloat y = NSHeight(button.superview.bounds) - stripHeight / 2.0 - NSHeight(button.frame) / 2.0;
        if (fabs(button.frame.origin.y - y) > 0.25) {
            NSPoint origin = button.frame.origin;
            origin.y = y;
            [button setFrameOrigin:origin];
        }
    }
}

enum {
    VIVA_POINTER_DOWN = 1,
    VIVA_POINTER_MOVE = 2,
    VIVA_POINTER_UP = 3,
    VIVA_POINTER_EXIT = 4,
    VIVA_POINTER_CANCEL = 5,
	VIVA_REDRAW_EVENT = 0x4752,
	VIVA_CLOSE_EVENT = 0x4743,
	VIVA_HIDE_EVENT = 0x4748,
	VIVA_SHOW_EVENT = 0x4753,
	VIVA_QUIT_EVENT = 0x4751,
	VIVA_STATUS_REMOVE_EVENT = 0x4754,
};


/* Key identities mirror input.Key values. */
enum {
    VIVA_KEY_TAB = 1,
    VIVA_KEY_ENTER = 2,
    VIVA_KEY_SPACE = 3,
    VIVA_KEY_ESCAPE = 4,
    VIVA_KEY_LEFT = 5,
    VIVA_KEY_RIGHT = 6,
    VIVA_KEY_UP = 7,
    VIVA_KEY_DOWN = 8,
    VIVA_KEY_HOME = 9,
    VIVA_KEY_END = 10,
    VIVA_KEY_BACKSPACE = 11,
    VIVA_KEY_DELETE = 12,
    VIVA_KEY_PAGE_UP = 13,
    VIVA_KEY_PAGE_DOWN = 14,
    VIVA_KEY_INSERT = 15,
    VIVA_KEY_F1 = 16, /* F2..F12 follow: VIVA_KEY_F1 + n - 1 */
    VIVA_KEY_SHIFT_LEFT = 28,
    VIVA_KEY_SHIFT_RIGHT = 29,
    VIVA_KEY_CONTROL_LEFT = 30,
    VIVA_KEY_CONTROL_RIGHT = 31,
    VIVA_KEY_ALT_LEFT = 32,
    VIVA_KEY_ALT_RIGHT = 33,
    VIVA_KEY_META_LEFT = 34,
    VIVA_KEY_META_RIGHT = 35,
};

/* Modifier bits mirror input.Modifiers values. */
enum {
    VIVA_MODIFIER_SHIFT = 1,
    VIVA_MODIFIER_CONTROL = 2,
    VIVA_MODIFIER_ALT = 4,
    VIVA_MODIFIER_META = 8,
};

enum {
    VIVA_TEXT_INSERT = 1,
    VIVA_TEXT_SET_MARKED = 2,
    VIVA_TEXT_UNMARK = 3,
    VIVA_TEXT_COMMAND_EVENT = 4,
    VIVA_TEXT_SET_SELECTION = 5,
    VIVA_TEXT_PASTE = 6,
    VIVA_TEXT_PASTE_NO_TEXT = 7,
    VIVA_TEXT_CANCEL_COMPOSITION = 8,
};

enum {
    VIVA_TEXT_DELETE_BACKWARD = 1,
    VIVA_TEXT_DELETE_FORWARD = 2,
    VIVA_TEXT_MOVE_LEFT = 3,
    VIVA_TEXT_MOVE_RIGHT = 4,
    VIVA_TEXT_MOVE_LEFT_AND_MODIFY_SELECTION = 5,
    VIVA_TEXT_MOVE_RIGHT_AND_MODIFY_SELECTION = 6,
    VIVA_TEXT_INSERT_NEWLINE = 7,
    VIVA_TEXT_INSERT_TAB = 8,
    VIVA_TEXT_INSERT_LINE_BREAK = 9,
    VIVA_TEXT_UNDO = 10,
    VIVA_TEXT_REDO = 11,
    /* The rest continue platform.TextCommand's numbering, which the Go side takes
     * as it is; TestNativeTextSelection checks every value against it. */
    VIVA_TEXT_MOVE_LINE_START = 13,
    VIVA_TEXT_MOVE_LINE_END = 14,
    VIVA_TEXT_MOVE_LINE_START_AND_MODIFY_SELECTION = 15,
    VIVA_TEXT_MOVE_LINE_END_AND_MODIFY_SELECTION = 16,
    VIVA_TEXT_MOVE_DOCUMENT_START = 17,
    VIVA_TEXT_MOVE_DOCUMENT_END = 18,
    VIVA_TEXT_MOVE_DOCUMENT_START_AND_MODIFY_SELECTION = 19,
    VIVA_TEXT_MOVE_DOCUMENT_END_AND_MODIFY_SELECTION = 20,
    VIVA_TEXT_MOVE_WORD_LEFT = 21,
    VIVA_TEXT_MOVE_WORD_RIGHT = 22,
    VIVA_TEXT_MOVE_WORD_LEFT_AND_MODIFY_SELECTION = 23,
    VIVA_TEXT_MOVE_WORD_RIGHT_AND_MODIFY_SELECTION = 24,
    VIVA_TEXT_DELETE_WORD_BACKWARD = 25,
    VIVA_TEXT_DELETE_WORD_FORWARD = 26,
    VIVA_TEXT_DELETE_TO_LINE_START = 27,
    VIVA_TEXT_DELETE_TO_LINE_END = 28,
};

/* The existing opt-in text.CompositionPhase values. */
enum {
    VIVA_COMPOSITION_NONE = 0,
    VIVA_COMPOSITION_UPDATE,
    VIVA_COMPOSITION_RESULT,
    VIVA_COMPOSITION_COMMIT,
    VIVA_COMPOSITION_CANCEL,
    VIVA_COMPOSITION_EMPTY_COMMIT,
};

static int viva_text_composition_phase(int kind, NSUInteger length, BOOL marked, NSRange replacement) {
    switch (kind) {
    case VIVA_TEXT_SET_MARKED:
        return length > 0 ? VIVA_COMPOSITION_UPDATE : marked ? VIVA_COMPOSITION_CANCEL : VIVA_COMPOSITION_NONE;
    case VIVA_TEXT_INSERT:
        if (marked) {
            return length > 0 ? VIVA_COMPOSITION_RESULT : VIVA_COMPOSITION_EMPTY_COMMIT;
        }
        return length == 0 && replacement.location != NSNotFound ? VIVA_COMPOSITION_EMPTY_COMMIT : VIVA_COMPOSITION_NONE;
    case VIVA_TEXT_UNMARK:
        return marked ? VIVA_COMPOSITION_COMMIT : VIVA_COMPOSITION_NONE;
    case VIVA_TEXT_CANCEL_COMPOSITION:
        return marked ? VIVA_COMPOSITION_CANCEL : VIVA_COMPOSITION_NONE;
    default:
        return VIVA_COMPOSITION_NONE;
    }
}

static int64_t viva_text_raw_coordinate(NSUInteger value) {
    return value > (NSUInteger)INT64_MAX ? INT64_MAX : (int64_t)value;
}

extern void vivaPointerEvent(
    uintptr_t callbackHandle,
    void *view,
    int kind,
    int button,
    int modifiers,
    int clickCount,
    double x,
    double y,
    double screenX,
    double screenY
);

extern void vivaViewResized(
    uintptr_t callbackHandle,
    void *view,
    double width,
    double height
);

extern void vivaRedrawRequested(
    uintptr_t callbackHandle,
    void *view
);

extern void vivaKeyEvent(
    uintptr_t callbackHandle,
    void *view,
    int key,
    int modifiers,
    int repeat,
    const unsigned char *utf8,
    size_t utf8Length
);

/* flags: 1 repeat, 2 release. code is the physical key's virtual key code plus
 * one, so zero keeps meaning "not followed". */
extern void vivaRawKeyEvent(
    uintptr_t callbackHandle,
    void *view,
    int key,
    int modifiers,
    int flags,
    unsigned int code,
    const unsigned char *utf8,
    size_t utf8Length,
    const unsigned char *typed,
    size_t typedLength
);

// vivaTerminalCells is how many terminal cells a character occupies, answered by the
// terminal's own table in Go.
extern int vivaTerminalCells(uint32_t character);
extern void vivaWindowActivated(uintptr_t callbackHandle);
extern void vivaWindowDeactivated(uintptr_t callbackHandle, void *view);
extern void vivaFocusLost(
    uintptr_t callbackHandle,
    void *view
);

// vivaAccessibilityAction carries one request from an accessibility client to
// the application. It returns nonzero when the node took it.
extern int vivaAccessibilityAction(
    uintptr_t callbackHandle,
    void *view,
    uint64_t nodeID,
    int action,
    double number,
    const unsigned char *text,
    size_t textLength,
    int start,
    int end
);

extern int vivaShortcutEvent(
    uintptr_t callbackHandle,
    void *view,
    int key,
    int modifiers,
    int repeat,
    const unsigned char *utf8,
    size_t utf8Length
);

// phase: 0 over, 1 leave, 2 drop. paths is the dropped paths joined by NUL, with
// length bytes; the result is nonzero when the point takes files.
extern void vivaOpenURL(const char *url);
extern void vivaOpenFile(const char *path);
extern int vivaApplicationQuitRequested(void);
extern int vivaApplicationReopenRequested(void);
extern int vivaFileDropEvent(
    uintptr_t handle,
    void *view,
    int phase,
    double x,
    double y,
    const char *paths,
    size_t length
);
extern void vivaScrollEvent(
    uintptr_t callbackHandle,
    void *view,
    double deltaX,
    double deltaY,
    double x,
    double y,
    int modifiers,
    int precise
);

extern void vivaCloseRequested(
    uintptr_t callbackHandle,
    void *view
);

extern void vivaWindowClosed(uintptr_t callbackHandle);

extern void vivaFrameChanged(
    uintptr_t callbackHandle,
    void *view,
    double x,
    double y,
    double width,
    double height
);

extern void vivaWindowMinimized(uintptr_t callbackHandle, void *view, int minimized);
extern void vivaWindowOccluded(uintptr_t callbackHandle, void *view, int visible);
extern void vivaWindowFullscreen(uintptr_t callbackHandle, void *view, int fullscreen);
extern int vivaWindowReopenRequested(uintptr_t callbackHandle, void *view);

extern void vivaTextInput(
    uintptr_t callbackHandle,
    void *view,
    int kind,
    const unsigned char *utf8,
    size_t utf8Length,
    int64_t selectedStart,
    int64_t selectedLength,
    int64_t replacementStart,
    int64_t replacementLength,
    int command
);

extern void vivaCompositionInput(
    uintptr_t callbackHandle,
    void *view,
    int kind,
    const unsigned char *utf8,
    size_t utf8Length,
    int64_t selectedStart,
    int64_t selectedLength,
    int64_t replacementStart,
    int64_t replacementLength,
    int command,
    int compositionPhase,
    int64_t replacementUTF16Start,
    int64_t replacementUTF16Length,
    int hasReplacementUTF16
);

extern unsigned char *vivaCopyText(uintptr_t callbackHandle, void *view, size_t *length);
extern void vivaTextCompositionLost(uintptr_t callbackHandle, void *view, int reason);

extern int vivaPressAt(
    uintptr_t callbackHandle,
    double x,
    double y
);

__attribute__((constructor))
static void viva_record_process_start(void) {
    viva_process_start_ticks = mach_continuous_time();
}

static BOOL viva_profile_enabled(void) {
    const char *value = getenv("VIVA_PROFILE_STARTUP");
    return value != NULL && value[0] == '1' && value[1] == '\0';
}

/* ---- Input latency probe ---------------------------------------------
 * VIVA_PROFILE_INPUT=1 records, for every key press, when each frame that
 * followed it reached the screen (the drawable's presented time, in the same
 * clock as NSEvent timestamps), how many frames that took, and how long the
 * window thread sat waiting for a free drawable. The summary prints on exit.
 * With the variable unset nothing here runs. */
static BOOL viva_input_probe_on;
/* The running window and its view, for the -tags vivahead input driver. */
static id viva_running_window;
static id viva_running_view;
static NSLock *viva_input_probe_lock;
enum { VIVA_PROBE_CAPACITY = 8192 };
static double viva_probe_keys[VIVA_PROBE_CAPACITY];
static int viva_probe_key_count;
static double viva_probe_frames[VIVA_PROBE_CAPACITY];
static int viva_probe_frame_count;
static double viva_probe_commits[VIVA_PROBE_CAPACITY];
static int viva_probe_commit_count;
static double viva_probe_blocked_total;
static double viva_probe_blocked_max;
static int viva_probe_blocked_over_1ms;
static int viva_probe_renders;

static BOOL viva_input_probe_enabled(void) {
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        const char *value = getenv("VIVA_PROFILE_INPUT");
        viva_input_probe_on = value != NULL && value[0] == '1' && value[1] == '\0';
        viva_input_probe_lock = [[NSLock alloc] init];
    });
    return viva_input_probe_on;
}

static void viva_probe_key(double timestamp) {
    if (!viva_input_probe_enabled()) {
        return;
    }
    [viva_input_probe_lock lock];
    if (viva_probe_key_count < VIVA_PROBE_CAPACITY) {
        viva_probe_keys[viva_probe_key_count++] = timestamp;
    }
    [viva_input_probe_lock unlock];
}

/* viva_probe_committed notes when the window thread finished handing a frame to
 * the GPU: everything before this instant is our own work. */
static void viva_probe_committed(void) {
    [viva_input_probe_lock lock];
    if (viva_probe_commit_count < VIVA_PROBE_CAPACITY) {
        viva_probe_commits[viva_probe_commit_count++] = NSProcessInfo.processInfo.systemUptime;
    }
    [viva_input_probe_lock unlock];
}

static void viva_probe_presented(double time) {
    [viva_input_probe_lock lock];
    if (viva_probe_frame_count < VIVA_PROBE_CAPACITY && time > 0) {
        viva_probe_frames[viva_probe_frame_count++] = time;
    }
    [viva_input_probe_lock unlock];
}

static int viva_probe_compare(const void *left, const void *right) {
    double a = *(const double *)left, b = *(const double *)right;
    return a < b ? -1 : (a > b ? 1 : 0);
}

static double viva_probe_percentile(double *sorted, int count, double fraction) {
    return count == 0 ? 0 : sorted[MIN(count - 1, (int)(fraction * count))];
}

static void viva_probe_report(void) {
    if (!viva_input_probe_on) {
        return;
    }
    [viva_input_probe_lock lock];
    qsort(viva_probe_frames, viva_probe_frame_count, sizeof(double), viva_probe_compare);
    double first[VIVA_PROBE_CAPACITY], settled[VIVA_PROBE_CAPACITY], count_per_key[VIVA_PROBE_CAPACITY];
    double ours[VIVA_PROBE_CAPACITY], display[VIVA_PROBE_CAPACITY];
    int measured = 0;
    // A key's frames are those presented after it and before the next key,
    // within a settle window long enough for an echo from a nearby server.
    const double settle = 0.1;
    for (int key = 0; key < viva_probe_key_count; key++) {
        double start = viva_probe_keys[key];
        double end = start + settle;
        if (key + 1 < viva_probe_key_count) {
            end = MIN(end, viva_probe_keys[key + 1]);
        }
        int frames = 0;
        double firstFrame = 0, lastFrame = 0;
        for (int frame = 0; frame < viva_probe_frame_count; frame++) {
            double time = viva_probe_frames[frame];
            if (time <= start) {
                continue;
            }
            if (time > end) {
                break;
            }
            if (frames == 0) {
                firstFrame = time;
            }
            lastFrame = time;
            frames++;
        }
        if (frames == 0) {
            continue;
        }
        double lastCommit = 0;
        for (int commit = 0; commit < viva_probe_commit_count; commit++) {
            double time = viva_probe_commits[commit];
            if (time > start && time <= end) {
                lastCommit = time;
            }
        }
        first[measured] = (firstFrame - start) * 1000.0;
        settled[measured] = (lastFrame - start) * 1000.0;
        ours[measured] = lastCommit > 0 ? (lastCommit - start) * 1000.0 : 0;
        display[measured] = lastCommit > 0 ? MAX(0.0, (lastFrame - lastCommit) * 1000.0) : 0;
        count_per_key[measured] = frames;
        measured++;
    }
    qsort(first, measured, sizeof(double), viva_probe_compare);
    qsort(settled, measured, sizeof(double), viva_probe_compare);
    qsort(ours, measured, sizeof(double), viva_probe_compare);
    qsort(display, measured, sizeof(double), viva_probe_compare);
    qsort(count_per_key, measured, sizeof(double), viva_probe_compare);
    fprintf(stderr, "viva_input_keys=%d measured=%d renders=%d\n", viva_probe_key_count, measured, viva_probe_renders);
    fprintf(stderr, "viva_input_frames_per_key median=%.0f p90=%.0f max=%.0f\n",
            viva_probe_percentile(count_per_key, measured, 0.5),
            viva_probe_percentile(count_per_key, measured, 0.9),
            viva_probe_percentile(count_per_key, measured, 1.0));
    fprintf(stderr, "viva_input_first_frame_ms median=%.2f p90=%.2f max=%.2f\n",
            viva_probe_percentile(first, measured, 0.5), viva_probe_percentile(first, measured, 0.9), viva_probe_percentile(first, measured, 1.0));
    fprintf(stderr, "viva_input_settled_ms median=%.2f p90=%.2f max=%.2f\n",
            viva_probe_percentile(settled, measured, 0.5), viva_probe_percentile(settled, measured, 0.9), viva_probe_percentile(settled, measured, 1.0));
    fprintf(stderr, "viva_input_ours_ms     median=%.2f p90=%.2f max=%.2f  (key to last commit)\n",
            viva_probe_percentile(ours, measured, 0.5), viva_probe_percentile(ours, measured, 0.9), viva_probe_percentile(ours, measured, 1.0));
    fprintf(stderr, "viva_input_display_ms  median=%.2f p90=%.2f max=%.2f  (commit to on screen)\n",
            viva_probe_percentile(display, measured, 0.5), viva_probe_percentile(display, measured, 0.9), viva_probe_percentile(display, measured, 1.0));
    fprintf(stderr, "viva_drawable_wait_ms total=%.2f max=%.2f over_1ms=%d of %d renders\n",
            viva_probe_blocked_total * 1000.0, viva_probe_blocked_max * 1000.0, viva_probe_blocked_over_1ms, viva_probe_renders);
    [viva_input_probe_lock unlock];
}

int viva_text_input_caret_should_blink(
    int enabled,
    size_t selected_length,
    int has_marked_text,
    int terminal,
    int hide_native_overlays
) {
    // A terminal draws its own cursor and keeps it steady, so an idle
    // terminal never wakes the application.
    return enabled != 0 && terminal == 0 && hide_native_overlays == 0 &&
        selected_length == 0 && has_marked_text == 0;
}

int viva_terminal_key_routes_raw(
    int composing,
    int named_key,
    int control,
    int command,
    int alt,
    int alt_sends_escape
) {
    if (composing != 0) {
        return 0; // the input method owns every key until it commits
    }
    return named_key != 0 || control != 0 || command != 0 ||
        (alt != 0 && alt_sends_escape != 0);
}

/* viva_control_tab reports whether a key equivalent is Tab (or Shift+Tab, which
 * AppKit types as the back-tab character) with Control held and Command not:
 * the chord the window reserves for key view navigation. */
static int viva_control_tab(NSEventModifierFlags modifiers, NSString *characters) {
    if ((modifiers & NSEventModifierFlagControl) == 0 || (modifiers & NSEventModifierFlagCommand) != 0 ||
        characters.length != 1) {
        return 0;
    }
    unichar character = [characters characterAtIndex:0];
    return character == NSTabCharacter || character == NSBackTabCharacter;
}

int viva_terminal_key_code(unsigned short character) {
    switch (character) {
        case NSTabCharacter:
        case NSBackTabCharacter: return VIVA_KEY_TAB;
        case NSCarriageReturnCharacter:
        case NSEnterCharacter: return VIVA_KEY_ENTER;
        case 0x1b: return VIVA_KEY_ESCAPE;
        case NSLeftArrowFunctionKey: return VIVA_KEY_LEFT;
        case NSRightArrowFunctionKey: return VIVA_KEY_RIGHT;
        case NSUpArrowFunctionKey: return VIVA_KEY_UP;
        case NSDownArrowFunctionKey: return VIVA_KEY_DOWN;
        case NSHomeFunctionKey: return VIVA_KEY_HOME;
        case NSEndFunctionKey: return VIVA_KEY_END;
        case NSPageUpFunctionKey: return VIVA_KEY_PAGE_UP;
        case NSPageDownFunctionKey: return VIVA_KEY_PAGE_DOWN;
        case NSInsertFunctionKey: return VIVA_KEY_INSERT;
        // The Mac Delete key is the terminal's Backspace; forward delete is Delete.
        case NSDeleteCharacter:
        case NSBackspaceCharacter: return VIVA_KEY_BACKSPACE;
        case NSDeleteFunctionKey: return VIVA_KEY_DELETE;
        default:
            if (character >= NSF1FunctionKey && character <= NSF12FunctionKey) {
                return VIVA_KEY_F1 + (character - NSF1FunctionKey);
            }
            return 0;
    }
}

/* viva_modifier_key_change names the modifier key a flagsChanged event for the
 * virtual key code is about, and whether it went down, from the device
 * dependent flag bits. It returns 0 for a key that is not a modifier. */
int viva_modifier_key_change(unsigned short key_code, unsigned long flags, int *pressed) {
    int key = 0;
    unsigned long mask = 0;
    switch (key_code) {
        case 56: key = VIVA_KEY_SHIFT_LEFT; mask = 0x2; break;
        case 60: key = VIVA_KEY_SHIFT_RIGHT; mask = 0x4; break;
        case 59: key = VIVA_KEY_CONTROL_LEFT; mask = 0x1; break;
        case 62: key = VIVA_KEY_CONTROL_RIGHT; mask = 0x2000; break;
        case 58: key = VIVA_KEY_ALT_LEFT; mask = 0x20; break;
        case 61: key = VIVA_KEY_ALT_RIGHT; mask = 0x40; break;
        case 55: key = VIVA_KEY_META_LEFT; mask = 0x8; break;
        case 54: key = VIVA_KEY_META_RIGHT; mask = 0x10; break;
        default: return 0;
    }
    *pressed = (flags & mask) != 0;
    return key;
}

int viva_text_input_row_key_is_raw(int placed, int composing, int key, int command) {
    if (placed == 0 || composing != 0) {
        return 0; // the input method owns every key until it commits
    }
    switch (key) {
        case VIVA_KEY_UP:
        case VIVA_KEY_DOWN:
        case VIVA_KEY_HOME:
        case VIVA_KEY_END:
        case VIVA_KEY_PAGE_UP:
        case VIVA_KEY_PAGE_DOWN:
            return 1;
        case VIVA_KEY_LEFT:
        case VIVA_KEY_RIGHT:
            return command != 0;
    }
    return 0;
}

int viva_text_input_navigation_key_is_raw(int navigation_keys, int composing, int key) {
    if (navigation_keys == 0 || composing != 0) {
        return 0;
    }
    switch (key) {
        case VIVA_KEY_UP:
        case VIVA_KEY_DOWN:
        case VIVA_KEY_PAGE_UP:
        case VIVA_KEY_PAGE_DOWN:
            return 1;
    }
    return 0;
}

int viva_text_input_should_interpret_key(int enabled, int multiline, int composing, int key) {
    if (enabled == 0 || key == VIVA_KEY_TAB || (key == VIVA_KEY_ESCAPE && composing == 0)) {
        return 0;
    }
    // The function keys belong to the application, as they do on Windows and
    // Linux: a focused field has no editing command for F1-F12, so sending them
    // to the text system only drops them. An input method that does use them
    // (a Japanese conversion on F6-F10) holds the keyboard while it composes, so
    // they stay with the text system for as long as it does.
    if (composing == 0 && key >= VIVA_KEY_F1 && key <= VIVA_KEY_F1 + 11) {
        return 0;
    }
    return key != VIVA_KEY_ENTER || multiline != 0 || composing != 0;
}

static BOOL viva_test_pointer_enabled(void) {
    const char *value = getenv("VIVA_TEST_POINTER_DOWN");
    return value != NULL && value[0] == '1' && value[1] == '\0';
}

static BOOL viva_test_click_point(NSPoint *point) {
    const char *value = getenv("VIVA_TEST_CLICK_POINT");
    if (value == NULL) {
        return NO;
    }
    double x = 0;
    double y = 0;
    if (sscanf(value, "%lf,%lf", &x, &y) != 2) {
        return NO;
    }
    point->x = (CGFloat)x;
    point->y = (CGFloat)y;
    return YES;
}

static BOOL viva_test_text_input_enabled(void) {
    const char *value = getenv("VIVA_TEST_TEXT_INPUT");
    return value != NULL && value[0] == '1' && value[1] == '\0';
}

static BOOL viva_test_key_enabled(void) {
    const char *value = getenv("VIVA_TEST_KEY");
    return value != NULL && value[0] == '1' && value[1] == '\0';
}

static BOOL viva_test_close_request_enabled(void) {
    const char *value = getenv("VIVA_TEST_CLOSE_REQUEST");
    return value != NULL && value[0] == '1' && value[1] == '\0';
}

static BOOL viva_test_scroll_enabled(void) {
    const char *value = getenv("VIVA_TEST_SCROLL");
    return value != NULL && value[0] == '1' && value[1] == '\0';
}

static BOOL viva_test_autoclose_enabled(void) {
    const char *value = getenv("VIVA_TEST_AUTOCLOSE");
    return value != NULL && value[0] == '1' && value[1] == '\0';
}

static BOOL viva_test_resize_enabled(void) {
    const char *value = getenv("VIVA_TEST_RESIZE");
    return value != NULL && value[0] == '1' && value[1] == '\0';
}

static BOOL viva_test_live_resize_enabled(void) {
    const char *value = getenv("VIVA_TEST_LIVE_RESIZE");
    return value != NULL && value[0] == '1' && value[1] == '\0';
}

static double viva_elapsed_milliseconds(uint64_t start, uint64_t end) {
    mach_timebase_info_data_t timebase;
    mach_timebase_info(&timebase);
    double nanoseconds = (double)(end - start) *
                         (double)timebase.numer /
                         (double)timebase.denom;
    return nanoseconds / 1000000.0;
}

static BOOL viva_key_is_named(unichar character) {
    switch (character) {
        case NSTabCharacter:
        case NSBackTabCharacter:
        case NSCarriageReturnCharacter:
        case NSEnterCharacter:
        case 0x1b:
        case NSBackspaceCharacter:
        case NSDeleteCharacter:
            return YES;
        default:
            // The function keys (arrows, Home, End, page, insert, F1-F35) keep
            // their NSEvent character above U+F700.
            return character >= NSUpArrowFunctionKey;
    }
}

static NSString *viva_key_characters(NSEvent *event) {
    // Command's character belongs to the shortcut layout, not the input
    // method's text layout. Preserve Command and Shift when removing Control
    // codes and Option-produced text; AppKit resolves the active layout.
    NSEventModifierFlags modifiers = event.modifierFlags;
    if (!(modifiers & NSEventModifierFlagCommand)) {
        return event.charactersIgnoringModifiers;
    }
    NSEventModifierFlags textModifiers = NSEventModifierFlagControl | NSEventModifierFlagOption;
    if (!(modifiers & textModifiers)) {
        return event.characters;
    }
    NSString *characters = event.charactersIgnoringModifiers;
    if (characters.length > 0 && viva_key_is_named([characters characterAtIndex:0])) {
        // Re-applying the modifiers would turn a named key into the control
        // code with the same low bits (Up becomes U+001E, Forward Delete
        // becomes U+007F) and lose the key's identity.
        return characters;
    }
    return [event charactersByApplyingModifiers:modifiers & ~textModifiers];
}

// viva_key_base_characters is the character a key names with no modifier
// applied: "c" for Control+Shift+C, never "C". A raw terminal key carries this
// unmodified character in KeyEvent.Text as the key's identity, with what was
// held in Modifiers and what the key types in Typed (see gfx.KeyEvent). The
// layout's own character for the key is the fallback when it has no unmodified
// one.
static NSString *viva_key_base_characters(NSEvent *event) {
    NSString *characters = [event charactersByApplyingModifiers:0];
    if (characters.length == 0) {
        characters = event.charactersIgnoringModifiers;
    }
    return characters;
}

static int viva_key_code(unichar key) {
    switch (key) {
        case NSTabCharacter: return VIVA_KEY_TAB;
        case NSCarriageReturnCharacter: return VIVA_KEY_ENTER;
        case ' ': return VIVA_KEY_SPACE;
        case 0x1b: return VIVA_KEY_ESCAPE;
        case NSLeftArrowFunctionKey: return VIVA_KEY_LEFT;
        case NSRightArrowFunctionKey: return VIVA_KEY_RIGHT;
        case NSUpArrowFunctionKey: return VIVA_KEY_UP;
        case NSDownArrowFunctionKey: return VIVA_KEY_DOWN;
        case NSHomeFunctionKey: return VIVA_KEY_HOME;
        case NSEndFunctionKey: return VIVA_KEY_END;
        case NSBackspaceCharacter: return VIVA_KEY_BACKSPACE;
        case NSDeleteCharacter: return VIVA_KEY_DELETE;
        default: return 0;
    }
}

static int viva_modifier_flags(NSEventModifierFlags flags) {
    int modifiers = 0;
    if ((flags & NSEventModifierFlagShift) != 0) {
        modifiers |= VIVA_MODIFIER_SHIFT;
    }
    if ((flags & NSEventModifierFlagControl) != 0) {
        modifiers |= VIVA_MODIFIER_CONTROL;
    }
    if ((flags & NSEventModifierFlagOption) != 0) {
        modifiers |= VIVA_MODIFIER_ALT;
    }
    if ((flags & NSEventModifierFlagCommand) != 0) {
        modifiers |= VIVA_MODIFIER_META;
    }
    return modifiers;
}

// viva_test_pasteboard replaces the general pasteboard in native tests, which
// must not read or overwrite the user's clipboard.
static NSPasteboard *viva_test_pasteboard;

static NSPasteboard *viva_pasteboard(void) {
    if (viva_test_pasteboard != nil) {
        return viva_test_pasteboard;
    }
    // VIVA_TEST_PASTEBOARD=1 keeps a manual run's copy and paste on a private
    // pasteboard, like the other VIVA_TEST_* switches.
    const char *value = getenv("VIVA_TEST_PASTEBOARD");
    if (value != NULL && value[0] == '1' && value[1] == '\0') {
        return [NSPasteboard pasteboardWithName:@"viva.test.pasteboard"];
    }
    return [NSPasteboard generalPasteboard];
}

// viva_single_line_paste prepares clipboard text for a single-line field: line
// breaks cannot live in one line, and leading or trailing ones (copied whole
// lines, shell output) must not turn into stray spaces.
static NSString *viva_single_line_paste(NSString *value) {
    // CRLF is two characters of the newline set, so splitting leaves an empty
    // piece between them; empty pieces are dropped rather than joined as spaces.
    NSMutableArray<NSString *> *lines = [NSMutableArray array];
    for (NSString *line in [value componentsSeparatedByCharactersInSet:[NSCharacterSet newlineCharacterSet]]) {
        if (line.length > 0) {
            [lines addObject:line];
        }
    }
    return [lines componentsJoinedByString:@" "];
}

@interface VivaAccessibleElement : NSAccessibilityElement {
    NSString *_label;
    NSString *_valueText;
    NSRect _frameInWindow;
    BOOL _focused;
    BOOL _configured;
    uint64_t _nodeID;
    int _live;
    uintptr_t _callbackHandle;
    // The view that owns the element, not retained: the callback is refused once
    // the window is closed, and the pointer is only handed back to Go.
    void *_view;
}
- (instancetype)initWithConfig:(viva_accessibility_node)config
                   windowFrame:(NSRect)windowFrame
                       focused:(BOOL)focused
                callbackHandle:(uintptr_t)callbackHandle
                          view:(void *)view;
@property(readonly) uint64_t nodeID;
@property(readonly) int live;
@property(readonly) BOOL nodeFocused;
@property(readonly, copy) NSString *valueText;
@end

@implementation VivaAccessibleElement

@synthesize nodeID = _nodeID;
@synthesize live = _live;
@synthesize valueText = _valueText;

- (BOOL)nodeFocused {
    return _focused;
}

- (instancetype)initWithConfig:(viva_accessibility_node)config
                   windowFrame:(NSRect)windowFrame
                       focused:(BOOL)focused
                callbackHandle:(uintptr_t)callbackHandle
                          view:(void *)view {
    self = [super init];
    if (self != nil) {
        _nodeID = config.id;
        _live = config.live;
        _callbackHandle = callbackHandle;
        _view = view;
        _valueText = [@"" retain];
        if (config.label_length > 0) {
            _label = [[NSString alloc] initWithBytes:config.label
                                              length:config.label_length
                                            encoding:NSUTF8StringEncoding];
            if (_label == nil) {
                _label = [@"" retain];
            }
        } else {
            _label = [@"" retain];
        }
        // Window content uses a top-left origin; accessibility frames are
        // bottom-left in window coordinates.
        CGFloat flippedY = windowFrame.size.height - config.y - config.height;
        _frameInWindow = NSMakeRect(config.x, flippedY, config.width, config.height);
        _focused = config.focused != 0;
        switch (config.role) {
            case 3: self.accessibilityRole = NSAccessibilityButtonRole; break;
            case 4: self.accessibilityRole = NSAccessibilityStaticTextRole; break;
            case 5: self.accessibilityRole = NSAccessibilityTextFieldRole; break;
            case 6: self.accessibilityRole = NSAccessibilityCheckBoxRole; break;
            case 7: self.accessibilityRole = NSAccessibilitySliderRole; break;
            case 8: self.accessibilityRole = NSAccessibilityImageRole; break;
            case 9: self.accessibilityRole = NSAccessibilityLinkRole; break;
            case 10: self.accessibilityRole = NSAccessibilityRadioButtonRole; break;
            case 11: self.accessibilityRole = NSAccessibilityButtonRole; break;
            case 12: self.accessibilityRole = NSAccessibilityMenuItemRole; break;
            case 13: self.accessibilityRole = NSAccessibilityPopUpButtonRole; break;
            case 14: self.accessibilityRole = NSAccessibilityCheckBoxRole; break;
            case 15: self.accessibilityRole = NSAccessibilityMenuRole; break;
            case 16: self.accessibilityRole = NSAccessibilityMenuBarRole; break;
            case 17: self.accessibilityRole = NSAccessibilityListRole; break;
            case 18: self.accessibilityRole = NSAccessibilityGroupRole; break;
            case 19: self.accessibilityRole = NSAccessibilityGroupRole; break;
            case 20: self.accessibilityRole = NSAccessibilityGroupRole; break;
            case 21: self.accessibilityRole = NSAccessibilityStaticTextRole; break;
            case 22: self.accessibilityRole = NSAccessibilityStaticTextRole; break;
            case 23: self.accessibilityRole = NSAccessibilitySplitterRole; break;
            case 24: self.accessibilityRole = NSAccessibilityProgressIndicatorRole; break;
            default: self.accessibilityRole = NSAccessibilityGroupRole; break;
        }
        if (config.secure) {
            self.accessibilitySubrole = NSAccessibilitySecureTextFieldSubrole;
        }
        self.accessibilityLabel = _label;
        NSString *hint = config.hint_length > 0
            ? [[[NSString alloc] initWithBytes:config.hint
                                        length:config.hint_length
                                      encoding:NSUTF8StringEncoding] autorelease]
            : nil;
        NSString *description = config.description_length > 0
            ? [[[NSString alloc] initWithBytes:config.description
                                        length:config.description_length
                                      encoding:NSUTF8StringEncoding] autorelease]
            : nil;
        if (hint != nil && description != nil) {
            self.accessibilityHelp = [NSString stringWithFormat:@"%@. %@", hint, description];
        } else if (hint != nil || description != nil) {
            self.accessibilityHelp = hint != nil ? hint : description;
        }
        NSString *valueText = config.value_length > 0
            ? [[[NSString alloc] initWithBytes:config.value
                                        length:config.value_length
                                      encoding:NSUTF8StringEncoding] autorelease]
            : nil;
        if (valueText != nil) {
            [_valueText release];
            _valueText = [valueText copy];
        }
        if (config.role == 6 || config.role == 10 || config.role == 14) {
            // NSAccessibility uses -1 for a mixed checkbox.
            self.accessibilityValue = @(config.mixed ? -1 : (config.checked != 0 ? 1 : 0));
        } else if (config.range_set) {
            self.accessibilityValue = @(config.range_now);
            self.accessibilityMinValue = @(config.range_min);
            self.accessibilityMaxValue = @(config.range_max);
            if (valueText != nil) {
                self.accessibilityValueDescription = valueText;
            }
        } else if (config.role == 21 && config.level > 0) {
            // A heading's value is its level.
            self.accessibilityValue = @(config.level);
        } else if (valueText != nil) {
            self.accessibilityValue = valueText;
        }
        if (config.selection_valid) {
            int low = config.selection_start < config.selection_end ? config.selection_start : config.selection_end;
            int high = config.selection_start < config.selection_end ? config.selection_end : config.selection_start;
            self.accessibilitySelectedTextRange = NSMakeRange((NSUInteger)low, (NSUInteger)(high - low));
            self.accessibilityNumberOfCharacters = config.text_length;
        }
        self.accessibilityFrame = _frameInWindow;
        self.accessibilityFocused = _focused;
        self.accessibilityEnabled = config.disabled == 0;
        self.accessibilitySelected = config.selected != 0;
        if (config.expandable || config.expanded) {
            self.accessibilityExpanded = config.expanded != 0;
        }
        _configured = YES;
    }
    return self;
}

- (void)dealloc {
    [_label release];
    [_valueText release];
    [super dealloc];
}

// perform hands a client's request to the application. The element may be
// replaced by the very frame the request causes, so it is kept alive until the
// call returns.
- (BOOL)perform:(int)action number:(double)number text:(NSString *)text start:(int)start end:(int)end {
    if (_callbackHandle == 0 || _nodeID == 0) {
        return NO;
    }
    [[self retain] autorelease];
    NSData *data = text != nil ? [text dataUsingEncoding:NSUTF8StringEncoding] : nil;
    return vivaAccessibilityAction(_callbackHandle, _view, _nodeID, action, number,
                                  (const unsigned char *)data.bytes, data.length, start, end) != 0;
}

- (BOOL)accessibilityPerformPress {
    return [self perform:VIVA_AX_PRESS number:0 text:nil start:0 end:0];
}

- (BOOL)accessibilityPerformIncrement {
    return [self perform:VIVA_AX_INCREMENT number:0 text:nil start:0 end:0];
}

- (BOOL)accessibilityPerformDecrement {
    return [self perform:VIVA_AX_DECREMENT number:0 text:nil start:0 end:0];
}

// The setters below are how a client changes the element. The initializer
// writes the same properties, so they act only once it is done.
- (void)setAccessibilityFocused:(BOOL)focused {
    [super setAccessibilityFocused:focused];
    if (_configured && focused) {
        [self perform:VIVA_AX_FOCUS number:0 text:nil start:0 end:0];
    }
}

- (void)setAccessibilityValue:(id)value {
    if (!_configured) {
        [super setAccessibilityValue:value];
        return;
    }
    if ([value isKindOfClass:[NSNumber class]]) {
        [self perform:VIVA_AX_SET_NUMBER number:[value doubleValue] text:nil start:0 end:0];
    } else if ([value isKindOfClass:[NSString class]]) {
        [self perform:VIVA_AX_SET_TEXT number:0 text:value start:0 end:0];
    }
}

- (void)setAccessibilitySelectedTextRange:(NSRange)range {
    if (!_configured) {
        [super setAccessibilitySelectedTextRange:range];
        return;
    }
    [self perform:VIVA_AX_SET_SELECTION number:0 text:nil start:(int)range.location end:(int)(range.location + range.length)];
}

@end

@class VivaMetalView;
@class VivaFileDragSource;

// VivaFileDragSource reports the end of a session the view started. It is not
// retained by the view after the session ends; the system keeps it while the
// session runs.
@interface VivaFileDragSource : NSObject <NSDraggingSource> {
    VivaMetalView *_view; // not retained: the view owns this source
    intptr_t _drag;
    NSDragOperation _allowed;
}
- (instancetype)initWithView:(VivaMetalView *)view handle:(uintptr_t)handle drag:(intptr_t)drag allowed:(NSDragOperation)allowed;
@end

@interface VivaMetalView : NSView <NSTextInputClient> {
    CAMetalLayer *_metalLayer;
    // What the surface draws: the rectangles, the label layers, the pipelines and
    // the queue they go through (VivaRenderer, shared with the other surfaces).
    VivaRenderer *_renderer;
    id<MTLTexture> _textTexture;
    NSString *_text;
    NSRect _textFrame;
    uint16_t _textLayer;
    CGFloat _textFontSize;
    CGFloat _textFontWeight;
    int _textAlignment;
    NSColor *_textColor;
    NSRange _selectedRange;
    NSRange _markedRange;
    BOOL _hasMarkedText;
    BOOL _cancelingComposition;
    BOOL _compositionCancelEmitted;
    BOOL _textInputEnabled;
    BOOL _textInputMultiline;
    BOOL _textPlaced;
    BOOL _textNavigationKeys;
    NSRect _textCaretFrame;
    BOOL _textSecure;
    BOOL _textCopyOverride;
    BOOL _textTerminal;
    BOOL _textHideNativeOverlays;
    int _textMonoFace;
    CGFloat _textCellWidth; // a terminal's cell, which its preedit is laid on
    CGFloat _textBaseline;  // the row of that cell its preedit stands on
    BOOL _textAltSendsEscape;
    BOOL _textKeyReleases;
    // The surface runs the input method of the machine it shows, not this
    // one's (platform.TextInput.DisableIME): keys never reach the host's text
    // system and no composition session is opened.
    BOOL _textDisableIME;
    BOOL _tentativeComposition;
    BOOL _discardingComposition;
    uint64_t _textInputGeneration;
    BOOL _interpretingKey;
    BOOL _interpretedConsumed;
    BOOL _interpretedInsert;
    NSRange _interpretedReplacement;
    NSString *_interpretedPlain;
    BOOL _secureEventInputOn;
    BOOL _dynamicTextInput;
    BOOL _caretVisible;
    NSTimer *_caretTimer;
    NSRange _textSelectionAnchor;
    BOOL _textSelectionDragging;
    BOOL _pointerEventsEnabled;
    BOOL _resizeEventsEnabled;
    BOOL _frameEventsEnabled;
    BOOL _minimizeEventsEnabled;
    BOOL _keyEventsEnabled;
    BOOL _scrollEventsEnabled;
    BOOL _fileDropEnabled;
    BOOL _modifierTapsEnabled;
    int _windowMode;
    CGFloat _titlebarHeight;
    BOOL _titlebarDragPending;
    NSPoint _titlebarDragOrigin;
    uintptr_t _callbackHandle;
        NSMutableArray<VivaAccessibleElement *> *_accessibilityElements;
    NSSize _lastReportedSize;
    BOOL _liveResizeRelaxed;
    BOOL _reportedSize;
    BOOL _reportedFirstFrame;
    BOOL _profileEnabled;
    NSCursor *_cursor;
    BOOL _nativeCursorActive;
    // The drag out of this window while a session is running: the source that
    // reports its end and the promise delegates the session asks for content.
    VivaFileDragSource *_fileDragSource;
    NSArray *_fileDragDelegates;
}

- (instancetype)initWithFrame:(NSRect)frame
                        device:(id<MTLDevice>)device
                  commandQueue:(id<MTLCommandQueue>)commandQueue
             rectanglePipeline:(id<MTLRenderPipelineState>)rectanglePipeline
               rectangleBuffer:(id<MTLBuffer>)rectangleBuffer
                rectangleCount:(NSUInteger)rectangleCount
                rectangleClips:(const viva_clip *)rectangleClips
                    textPipeline:(id<MTLRenderPipelineState>)textPipeline
                      textLabels:(const viva_text_label *)textLabels
                  textLabelCount:(NSUInteger)textLabelCount
                    accessibility:(const viva_accessibility_node *)accessibility
                accessibilityCount:(NSUInteger)accessibilityCount
                         textInput:(viva_text_input_config)textInput
            dynamicTextInputEnabled:(BOOL)dynamicTextInputEnabled
              pointerEventsEnabled:(BOOL)pointerEventsEnabled
                resizeEventsEnabled:(BOOL)resizeEventsEnabled
                  keyEventsEnabled:(BOOL)keyEventsEnabled
                scrollEventsEnabled:(BOOL)scrollEventsEnabled
                      windowMode:(int)windowMode
                     callbackHandle:(uintptr_t)callbackHandle
                        clearColor:(MTLClearColor)clearColor;
- (void)enableFileDrops;
- (void)setModifierTapsEnabled:(BOOL)enabled;
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
- (int)updateLabels:(const viva_text_label *)labels
              count:(NSUInteger)count;
- (int)updateAccessibility:(const viva_accessibility_node *)nodes
                     count:(NSUInteger)count;
- (int)updateText:(const unsigned char *)utf8
            length:(NSUInteger)length
     selectedRange:(NSRange)selectedRange
       markedRange:(NSRange)markedRange
     hasMarkedText:(BOOL)hasMarkedText
 tentativeComposition:(BOOL)tentativeComposition
    endsComposition:(BOOL)endsComposition;
- (int)updateTextInput:(viva_text_input_config)textInput;
- (int)updateTextInput:(viva_text_input_config)textInput keepComposition:(BOOL)keep;
- (void)setTitlebarHeight:(CGFloat)height;
- (void)cancelTitlebarDrag;
- (int)moveTextInputToFrame:(NSRect)frame;
- (void)restartCaretBlink;
- (void)stopCaretBlink;
- (void)shutdownTextInput;
- (void)detachCallbacks;
- (void)notifyCompositionLost;
- (void)endOwnedNativeComposition;
- (BOOL)rebuildTextTexture;
- (BOOL)rebuildTextLabels;
- (viva_editor_line)newTextLine;
- (CGFloat)textOffsetForIndex:(NSUInteger)index line:(viva_editor_line)line;
- (NSString *)displayedText;
- (void)syncSecureEventInput;
- (void)keyWindowChanged:(NSNotification *)notification;
- (void)emitTextKind:(int)kind
                text:(NSString *)text
       selectedRange:(NSRange)selectedRange
     replacementRange:(NSRange)replacementRange
             command:(int)command;
- (void)cancelMarkedComposition;
- (void)interpretKeyEvent:(NSEvent *)event;
- (NSUInteger)textIndexForEvent:(NSEvent *)event;
- (NSUInteger)textIndexForLocalPoint:(NSPoint)point;
- (BOOL)eventIsInsideTextInput:(NSEvent *)event;
- (void)selectTextToIndex:(NSUInteger)index;
- (void)selectTextRange:(NSRange)range;
- (void)selectAll:(id)sender;
- (void)copy:(id)sender;
- (void)cut:(id)sender;
- (void)paste:(id)sender;
@end

/* The selectors AppKit's key bindings send for the navigation and deleting chords:
 * Option with Left, Right, Backspace and Delete by word; Command with Left and
 * Right to the ends of the line, with Up and Down (and Home and End, which AppKit
 * scrolls with) to the ends of the text, Command+Backspace and Control+K removing
 * to the ends of the line. A line is the text up to a newline. 0 for any other
 * selector. */
static int viva_navigation_command(SEL selector) {
    static const struct { const char *name; int command; } table[] = {
        {"moveWordLeft:", VIVA_TEXT_MOVE_WORD_LEFT},
        {"moveWordRight:", VIVA_TEXT_MOVE_WORD_RIGHT},
        {"moveWordLeftAndModifySelection:", VIVA_TEXT_MOVE_WORD_LEFT_AND_MODIFY_SELECTION},
        {"moveWordRightAndModifySelection:", VIVA_TEXT_MOVE_WORD_RIGHT_AND_MODIFY_SELECTION},
        {"deleteWordBackward:", VIVA_TEXT_DELETE_WORD_BACKWARD},
        {"deleteWordForward:", VIVA_TEXT_DELETE_WORD_FORWARD},
        {"moveToBeginningOfLine:", VIVA_TEXT_MOVE_LINE_START},
        {"moveToLeftEndOfLine:", VIVA_TEXT_MOVE_LINE_START},
        {"moveToEndOfLine:", VIVA_TEXT_MOVE_LINE_END},
        {"moveToRightEndOfLine:", VIVA_TEXT_MOVE_LINE_END},
        {"moveToBeginningOfLineAndModifySelection:", VIVA_TEXT_MOVE_LINE_START_AND_MODIFY_SELECTION},
        {"moveToLeftEndOfLineAndModifySelection:", VIVA_TEXT_MOVE_LINE_START_AND_MODIFY_SELECTION},
        {"moveToEndOfLineAndModifySelection:", VIVA_TEXT_MOVE_LINE_END_AND_MODIFY_SELECTION},
        {"moveToRightEndOfLineAndModifySelection:", VIVA_TEXT_MOVE_LINE_END_AND_MODIFY_SELECTION},
        {"moveToBeginningOfDocument:", VIVA_TEXT_MOVE_DOCUMENT_START},
        {"scrollToBeginningOfDocument:", VIVA_TEXT_MOVE_DOCUMENT_START},
        {"moveToEndOfDocument:", VIVA_TEXT_MOVE_DOCUMENT_END},
        {"scrollToEndOfDocument:", VIVA_TEXT_MOVE_DOCUMENT_END},
        {"moveToBeginningOfDocumentAndModifySelection:", VIVA_TEXT_MOVE_DOCUMENT_START_AND_MODIFY_SELECTION},
        {"moveToEndOfDocumentAndModifySelection:", VIVA_TEXT_MOVE_DOCUMENT_END_AND_MODIFY_SELECTION},
        {"deleteToBeginningOfLine:", VIVA_TEXT_DELETE_TO_LINE_START},
        {"deleteToEndOfLine:", VIVA_TEXT_DELETE_TO_LINE_END},
        {"deleteToBeginningOfParagraph:", VIVA_TEXT_DELETE_TO_LINE_START},
        {"deleteToEndOfParagraph:", VIVA_TEXT_DELETE_TO_LINE_END},
    };
    const char *name = sel_getName(selector);
    for (size_t index = 0; index < sizeof(table) / sizeof(table[0]); index++) {
        if (strcmp(name, table[index].name) == 0) {
            return table[index].command;
        }
    }
    return 0;
}

@implementation VivaMetalView

- (instancetype)initWithFrame:(NSRect)frame
                        device:(id<MTLDevice>)device
                  commandQueue:(id<MTLCommandQueue>)commandQueue
             rectanglePipeline:(id<MTLRenderPipelineState>)rectanglePipeline
               rectangleBuffer:(id<MTLBuffer>)rectangleBuffer
                rectangleCount:(NSUInteger)rectangleCount
                rectangleClips:(const viva_clip *)rectangleClips
                    textPipeline:(id<MTLRenderPipelineState>)textPipeline
                      textLabels:(const viva_text_label *)textLabels
                  textLabelCount:(NSUInteger)textLabelCount
                    accessibility:(const viva_accessibility_node *)accessibility
                accessibilityCount:(NSUInteger)accessibilityCount
                         textInput:(viva_text_input_config)textInput
            dynamicTextInputEnabled:(BOOL)dynamicTextInputEnabled
             pointerEventsEnabled:(BOOL)pointerEventsEnabled
               resizeEventsEnabled:(BOOL)resizeEventsEnabled
                 keyEventsEnabled:(BOOL)keyEventsEnabled
                scrollEventsEnabled:(BOOL)scrollEventsEnabled
                     windowMode:(int)windowMode
                    callbackHandle:(uintptr_t)callbackHandle
                       clearColor:(MTLClearColor)clearColor {
    self = [super initWithFrame:frame];
    if (self != nil) {
        _textInputEnabled = textInput.enabled != 0;
        _textInputMultiline = textInput.multiline != 0;
        _textPlaced = textInput.placed != 0;
        _textNavigationKeys = textInput.navigation_keys != 0;
        _textLayer = textInput.layer;
        _textCaretFrame = NSMakeRect(textInput.caret_x, textInput.caret_y, textInput.caret_width, textInput.caret_height);
        _textSecure = textInput.secure != 0;
        _textCopyOverride = textInput.copy_override != 0;
        _textTerminal = textInput.terminal != 0;
        _textHideNativeOverlays = textInput.hide_native_overlays != 0;
        _textMonoFace = textInput.mono_face;
        _textCellWidth = textInput.cell_width;
        _textBaseline = textInput.baseline;
        _textAltSendsEscape = textInput.alt_sends_escape != 0;
        _textKeyReleases = textInput.key_releases != 0;
        _textDisableIME = textInput.disable_ime != 0;
        [[NSNotificationCenter defaultCenter] addObserver:self selector:@selector(keyWindowChanged:) name:NSWindowDidBecomeKeyNotification object:nil];
        [[NSNotificationCenter defaultCenter] addObserver:self selector:@selector(keyWindowChanged:) name:NSWindowDidResignKeyNotification object:nil];
        _dynamicTextInput = dynamicTextInputEnabled;
        _caretVisible = YES;
        _pointerEventsEnabled = pointerEventsEnabled;
        _resizeEventsEnabled = resizeEventsEnabled;
        _keyEventsEnabled = keyEventsEnabled;
        _scrollEventsEnabled = scrollEventsEnabled;
        _windowMode = windowMode;
        _titlebarHeight = VIVA_DEFAULT_TITLEBAR_HEIGHT;
        _textFrame = NSMakeRect(
            textInput.x,
            textInput.y,
            textInput.width,
            textInput.height
        );
        _textFontSize = textInput.font_size;
        _textFontWeight = textInput.font_weight;
        _textAlignment = textInput.placed || textInput.terminal ? 0 : textInput.alignment;
        _textColor = [[NSColor colorWithSRGBRed:textInput.red
                                          green:textInput.green
                                           blue:textInput.blue
                                          alpha:textInput.alpha] retain];
        _selectedRange = NSMakeRange(textInput.selected_start, textInput.selected_length);
        _markedRange = NSMakeRange(textInput.marked_start, textInput.marked_length);
        _hasMarkedText = textInput.has_marked_text != 0;
        _tentativeComposition = textInput.tentative_composition != 0;
        if (_textInputEnabled) {
            _text = [[NSString alloc] initWithBytes:textInput.utf8
                                             length:textInput.utf8_length
                                           encoding:NSUTF8StringEncoding];
            if (_text == nil) {
                _text = [@"" retain];
            }
        } else {
            _text = [@"" retain];
        }
        _callbackHandle = callbackHandle;
        _lastReportedSize = frame.size;
        _reportedSize = YES;
        _profileEnabled = viva_profile_enabled();

        _metalLayer = [[CAMetalLayer alloc] init];
        _metalLayer.device = device;
        _metalLayer.pixelFormat = MTLPixelFormatBGRA8Unorm;
        _metalLayer.framebufferOnly = YES;
        /* A transparent clear color must reach the desktop through a frameless
         * surface. Opaque titled windows keep the normal fast path. */
        _metalLayer.opaque = clearColor.alpha >= 1.0 && windowMode != VIVA_WINDOW_PANEL && windowMode != VIVA_WINDOW_PREVIEW;
        _metalLayer.maximumDrawableCount = 2;
        _metalLayer.allowsNextDrawableTimeout = YES;

        self.wantsLayer = YES;
        self.layer = _metalLayer;
        if (windowMode == VIVA_WINDOW_PANEL || windowMode == VIVA_WINDOW_NOTIFICATION) {
            _metalLayer.cornerRadius = VIVA_PANEL_CORNER_RADIUS;
            _metalLayer.masksToBounds = YES;
        }
        [self updateDrawableSize];
        /* A panel opens while the application is not the active one, and its
         * rows still react to the pointer. */
        NSTrackingAreaOptions activeness = (windowMode == VIVA_WINDOW_PANEL || windowMode == VIVA_WINDOW_NOTIFICATION) ? NSTrackingActiveAlways : NSTrackingActiveInActiveApp;
        NSTrackingArea *tracking = [[NSTrackingArea alloc]
            initWithRect:self.bounds
                 options:NSTrackingMouseMoved |
                         NSTrackingMouseEnteredAndExited |
                         activeness |
                         NSTrackingInVisibleRect
                   owner:self
                userInfo:nil];
        [self addTrackingArea:tracking];
        [tracking release];
        _renderer = [[VivaRenderer alloc] initWithLayer:_metalLayer
                                           commandQueue:commandQueue
                                      rectanglePipeline:rectanglePipeline
                                        rectangleBuffer:rectangleBuffer
                                         rectangleCount:rectangleCount
                                         rectangleClips:rectangleClips
                                           textPipeline:textPipeline
                                             textLabels:textLabels
                                         textLabelCount:textLabelCount
                                             clearColor:clearColor
                                               viewport:self.bounds.size];
        if (_renderer == nil) {
            [self release];
            return nil;
        }
        _accessibilityElements = [[NSMutableArray alloc] init];
        for (NSUInteger index = 0; index < accessibilityCount; index++) {
            VivaAccessibleElement *element =
                [[VivaAccessibleElement alloc] initWithConfig:accessibility[index]
                                                 windowFrame:self.bounds
                                                     focused:NO
                                              callbackHandle:_callbackHandle
                                                        view:self];
            if (element == nil) {
                [self release];
                return nil;
            }
            [_accessibilityElements addObject:element];
            [element release];
        }
        if (_textInputEnabled && ![self rebuildTextTexture]) {
            [self release];
            return nil;
        }
        [self restartCaretBlink];
    }
    return self;
}

- (void)dealloc {
    [[NSNotificationCenter defaultCenter] removeObserver:self];
    _textInputEnabled = NO;
    [self syncSecureEventInput];
    [self stopCaretBlink];
    [_cursor release];
    [_textTexture release];
    [_accessibilityElements release];
    [_text release];
    [_textColor release];
    [_renderer release];
    [_metalLayer release];
    [super dealloc];
}


- (BOOL)isOpaque {
    return _windowMode != VIVA_WINDOW_PANEL && _windowMode != VIVA_WINDOW_PREVIEW && _windowMode != VIVA_WINDOW_NOTIFICATION;
}

- (BOOL)acceptsFirstResponder {
    return YES;
}

// A page taking the keyboard is the window losing it as far as the keys that were down go.
- (BOOL)resignFirstResponder {
    uint64_t generation = _textInputGeneration;
    if (_callbackHandle != 0 && _keyEventsEnabled && self.window.isKeyWindow) {
        vivaFocusLost(_callbackHandle, self);
    }
    if (_textInputGeneration == generation && _textInputEnabled && _tentativeComposition) {
        [self endOwnedNativeComposition];
    }
    return YES;
}

// acceptsFirstMouse lets the click that brings an inactive window forward also
// act on the control under it. Without it AppKit spends that click on activating
// the window, so after a native panel or another app had the focus a button took
// two presses to respond.
- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    (void)event;
    return YES;
}

// vivaSetCursor records the pointer shape the application chose and has AppKit
// rebuild the view's cursor rectangles around it.
- (void)vivaSetCursor:(NSCursor *)cursor {
    if (cursor != _cursor) {
        [cursor retain];
        [_cursor release];
        _cursor = cursor;
    }
    [self.window invalidateCursorRectsForView:self];
}

// resetCursorRects keeps the chosen shape across layout changes, after which
// AppKit calls it having discarded the old rectangles.
- (void)resetCursorRects {
    NSPoint point = [self convertPoint:self.window.mouseLocationOutsideOfEventStream fromView:nil];
    // A full-window cursor rectangle would override WebKit's hand or text cursor.
    if (_cursor != nil && !viva_view_has_native_cursor(self, point)) {
        [self addCursorRect:self.bounds cursor:_cursor];
    }
}

- (void)emitPointerEvent:(int)kind fromEvent:(NSEvent *)event {
    if (_callbackHandle == 0 || !_pointerEventsEnabled) {
        return;
    }
    viva_pointer_event_count++;
    NSPoint point = [self convertPoint:event.locationInWindow fromView:nil];
    NSPoint screen = event.window != nil ? [event.window convertPointToScreen:event.locationInWindow] : NSEvent.mouseLocation;
    CGFloat primaryHeight = NSScreen.screens.firstObject.frame.size.height;
    // clickCount is defined only for button events; reading it from a move
    // or drag raises.
    int clickCount = (kind == VIVA_POINTER_DOWN || kind == VIVA_POINTER_UP)
        ? (int)MAX(event.clickCount, 1) : 0;
    // NSEvent counts left 0, right 1, middle 2, then the side buttons: Back 3
    // and Forward 4. A further button has no meaning and is not reported.
    if (event.buttonNumber > 4) {
        return;
    }
    vivaPointerEvent(
        _callbackHandle,
        self,
        kind,
        (int)event.buttonNumber == 0 ? 1 : (int)event.buttonNumber + 1,
        viva_modifier_flags(event.modifierFlags),
        clickCount,
        point.x,
        self.bounds.size.height - point.y,
        screen.x,
        primaryHeight - screen.y
    );
}

- (void)setTitlebarHeight:(CGFloat)height {
    _titlebarHeight = height > 0 ? height : VIVA_DEFAULT_TITLEBAR_HEIGHT;
}

- (void)cancelTitlebarDrag {
    _titlebarDragPending = NO;
}

- (void)mouseDown:(NSEvent *)event {
    // A press in Viva's own content takes the keyboard back from a page that had it.
    if (self.window.firstResponder != self) {
        [self.window makeFirstResponder:self];
    }
    int press = VIVA_PRESS_CLIENT;
    if ((self.window.styleMask & NSWindowStyleMaskFullScreen) == 0 &&
        (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR || _windowMode == VIVA_WINDOW_FRAMELESS || _windowMode == VIVA_WINDOW_NOTIFICATION) && _callbackHandle != 0) {
        // The application declares where the window moves from (a move region).
        NSPoint local = [self convertPoint:event.locationInWindow fromView:nil];
        press = vivaPressAt(_callbackHandle, local.x, self.bounds.size.height - local.y);
    }
    if (press == VIVA_PRESS_MOVE) {
        if (event.clickCount >= 2 && _windowMode != VIVA_WINDOW_NOTIFICATION) {
            // Standard titlebar behavior: double-click zooms the window.
            [[self window] performZoom:event];
            return;
        }
        [self.window performWindowDragWithEvent:event];
        return;
    }
    // A backdrop over a move region dismisses its overlay on a click, so the
    // application has the press; the window moves if the pointer drags.
    _titlebarDragPending = press == VIVA_PRESS_MOVE_ON_DRAG;
    _titlebarDragOrigin = event.locationInWindow;
    [self emitPointerEvent:VIVA_POINTER_DOWN fromEvent:event];
    if (_textInputEnabled && !_textTerminal && [self eventIsInsideTextInput:event]) {
        [self.window makeFirstResponder:self];
        if (!_textDisableIME) {
            [self.inputContext activate];
        }
        if (_textPlaced) {
            // The application puts the caret where the press landed, by rows.
            _textSelectionDragging = NO;
            return;
        }
        NSUInteger index = [self textIndexForEvent:event];
        _textSelectionAnchor = NSMakeRange(index, 0);
        if (event.clickCount >= 2 && _text.length > 0) {
            if (event.clickCount >= 3 || _textSecure) {
                _textSelectionAnchor = NSMakeRange(0, _text.length);
            } else {
                NSAttributedString *value = [[NSAttributedString alloc] initWithString:_text];
                _textSelectionAnchor = [value doubleClickAtIndex:MIN(index, _text.length - 1)];
                [value release];
            }
        }
        _textSelectionDragging = YES;
        [self selectTextToIndex:index];
    } else {
        _textSelectionDragging = NO;
    }
}

- (void)mouseUp:(NSEvent *)event {
    _titlebarDragPending = NO;
    if (_textSelectionDragging && _textInputEnabled) {
        [self selectTextToIndex:[self textIndexForEvent:event]];
    }
    _textSelectionDragging = NO;
    [self emitPointerEvent:VIVA_POINTER_UP fromEvent:event];
}

- (void)mouseDragged:(NSEvent *)event {
    if (_titlebarDragPending) {
        NSPoint at = event.locationInWindow;
        if (hypot(at.x - _titlebarDragOrigin.x, at.y - _titlebarDragOrigin.y) >= VIVA_TITLEBAR_DRAG_SLOP) {
            // The system's drag loop swallows the button release, so the
            // application is told the press is over. It is a cancel, not an
            // exit: an exit only says the pointer left the window, and a press
            // keeps the pointer until its button comes up.
            _titlebarDragPending = NO;
            [self emitPointerEvent:VIVA_POINTER_CANCEL fromEvent:event];
            [self.window performWindowDragWithEvent:event];
            return;
        }
    }
    if (_textSelectionDragging && _textInputEnabled) {
        [self selectTextToIndex:[self textIndexForEvent:event]];
    }
    [self emitPointerEvent:VIVA_POINTER_MOVE fromEvent:event];
}

- (void)mouseMoved:(NSEvent *)event {
    NSPoint point = [self convertPoint:event.locationInWindow fromView:nil];
    BOOL native = viva_view_has_native_cursor(self, point);
    BOOL enteringNative = !_nativeCursorActive && native;
    BOOL returningToCanvas = _nativeCursorActive && !native;
    if (native != _nativeCursorActive) {
        _nativeCursorActive = native;
        [self.window invalidateCursorRectsForView:self];
    }
    // Host tracking areas also cover subviews; native content must not emit canvas moves or set its cursor.
    if (native) {
        // The pointer left Viva's content for the page, where moves are not emitted; the control it was over
        // must not stay hovered.
        if (enteringNative) {
            [self emitPointerExit];
        }
        return;
    }
    [self emitPointerEvent:VIVA_POINTER_MOVE fromEvent:event];
    // Force restoration only when returning ownership; ordinary moves retain the Go cursor cache.
    if (returningToCanvas && _cursor != nil) {
        [_cursor set];
    }
}

- (void)rightMouseDown:(NSEvent *)event {
    if (self.window.firstResponder != self) {
        [self.window makeFirstResponder:self];
    }
    [self emitPointerEvent:VIVA_POINTER_DOWN fromEvent:event];
}
- (void)rightMouseUp:(NSEvent *)event { [self emitPointerEvent:VIVA_POINTER_UP fromEvent:event]; }
- (void)rightMouseDragged:(NSEvent *)event { [self emitPointerEvent:VIVA_POINTER_MOVE fromEvent:event]; }
- (void)otherMouseDown:(NSEvent *)event { [self emitPointerEvent:VIVA_POINTER_DOWN fromEvent:event]; }
- (void)otherMouseUp:(NSEvent *)event { [self emitPointerEvent:VIVA_POINTER_UP fromEvent:event]; }
- (void)otherMouseDragged:(NSEvent *)event { [self emitPointerEvent:VIVA_POINTER_MOVE fromEvent:event]; }

- (void)mouseExited:(NSEvent *)event {
    (void)event;
    [self emitPointerExit];
}

- (void)emitPointerExit {
    if (_callbackHandle == 0 || !_pointerEventsEnabled) {
        return;
    }
    viva_pointer_event_count++;
    vivaPointerEvent(_callbackHandle, self, VIVA_POINTER_EXIT, 0, 0, 0, -1, -1, 0, 0);
}

// ---- Files dragged from Finder ----

- (void)setModifierTapsEnabled:(BOOL)enabled {
    _modifierTapsEnabled = enabled;
}

- (void)enableFileDrops {
    _fileDropEnabled = YES;
    [self registerForDraggedTypes:@[ NSPasteboardTypeFileURL ]];
}

// dropPhase reports a step of the drag to the application and returns the
// operation the system should show: a copy where the point takes files, none
// where it does not.
- (NSDragOperation)dropPhase:(int)phase info:(id<NSDraggingInfo>)info paths:(const char *)paths length:(size_t)length {
    if (_callbackHandle == 0 || !_fileDropEnabled) {
        return NSDragOperationNone;
    }
    NSPoint point = [self convertPoint:info.draggingLocation fromView:nil];
    int accepted = vivaFileDropEvent(_callbackHandle, self, phase, point.x, self.bounds.size.height - point.y, paths, length);
    return accepted != 0 ? NSDragOperationCopy : NSDragOperationNone;
}

- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {
    return [self dropPhase:0 info:sender paths:NULL length:0];
}

- (NSDragOperation)draggingUpdated:(id<NSDraggingInfo>)sender {
    return [self dropPhase:0 info:sender paths:NULL length:0];
}

- (void)draggingExited:(id<NSDraggingInfo>)sender {
    if (_callbackHandle != 0 && _fileDropEnabled) {
        vivaFileDropEvent(_callbackHandle, self, 1, 0, 0, NULL, 0);
    }
    (void)sender;
}

- (BOOL)prepareForDragOperation:(id<NSDraggingInfo>)sender {
    (void)sender;
    return _fileDropEnabled;
}

- (BOOL)performDragOperation:(id<NSDraggingInfo>)sender {
    NSDictionary *options = @{ NSPasteboardURLReadingFileURLsOnlyKey : @YES };
    NSArray *urls = [sender.draggingPasteboard readObjectsForClasses:@[ [NSURL class] ] options:options];
    NSMutableData *joined = [NSMutableData data];
    for (NSURL *url in urls) {
        const char *path = url.path.fileSystemRepresentation;
        if (path == NULL || path[0] == 0) {
            continue;
        }
        [joined appendBytes:path length:strlen(path) + 1];
    }
    if (joined.length == 0) {
        return NO;
    }
    return [self dropPhase:2 info:sender paths:(const char *)joined.bytes length:joined.length] != NSDragOperationNone;
}

// ---- Files dragged out of this window ----

// emitPointerCancel ends the application's press when the system took it: the
// dragging session swallows the button release, so the application would
// otherwise keep a press the pointer can no longer follow.
- (void)emitPointerCancel {
    if (_callbackHandle == 0 || !_pointerEventsEnabled) {
        return;
    }
    viva_pointer_event_count++;
    vivaPointerEvent(_callbackHandle, self, VIVA_POINTER_CANCEL, 0, 0, 0, -1, -1, 0, 0);
}

// startFileDragWithItems begins a dragging session for the press that is being
// handled now. The system owns the pointer until the drop or Escape reports
// through the source, which is why the application must not keep its own press
// open. drag and delegates are the promised drag's registry key and its
// delegates; both are 0/nil for a drag of paths.
- (int)startFileDragWithItems:(NSArray *)items allowed:(NSDragOperation)allowed drag:(intptr_t)drag delegates:(NSArray *)delegates {
    NSEvent *event = [NSApp currentEvent];
    if (event == nil) {
        return -1;
    }
    switch (event.type) {
        case NSEventTypeLeftMouseDown:
        case NSEventTypeLeftMouseDragged:
        case NSEventTypeRightMouseDragged:
        case NSEventTypeOtherMouseDragged:
            break;
        default:
            return -2;
    }
    if (_fileDragSource != nil) {
        return -3;
    }
    VivaFileDragSource *source = [[VivaFileDragSource alloc] initWithView:self handle:_callbackHandle drag:drag allowed:allowed];
    _fileDragSource = source;
    _fileDragDelegates = [delegates retain];
    NSDraggingSession *session = [self beginDraggingSessionWithItems:items event:event source:source];
    session.animatesToStartingPositionsOnCancelOrFail = NO;
    return 0;
}

- (void)fileDragSessionEnded {
    [_fileDragSource release];
    _fileDragSource = nil;
    [_fileDragDelegates release];
    _fileDragDelegates = nil;
    [self emitPointerCancel];
}

- (void)scrollWheel:(NSEvent *)event {
    if (_callbackHandle == 0 || !_scrollEventsEnabled) {
        return;
    }
    // AppKit reports the direction the content should move, already flipped
    // by the user's "Natural scrolling" setting: positive Y is content moving
    // down (revealing what is above). The platform contract is the opposite
    // scroll-offset sign shared with Win32 and X11: positive deltas reveal
    // content below and to the right, so negate both axes and leave honoring
    // the system setting to AppKit.
    //
    // Precise deltas are trackpad pixels. Line-based deltas (classic mice)
    // are typically ~10x coarser, so scale them to an approximate pixel
    // distance and report both readings.
    CGFloat pixelX = -event.scrollingDeltaX;
    CGFloat pixelY = -event.scrollingDeltaY;
    BOOL precise = event.hasPreciseScrollingDeltas;
    if (!precise) {
        pixelX *= 10.0;
        pixelY *= 10.0;
    }
    NSPoint point = [self convertPoint:event.locationInWindow fromView:nil];
    [self emitScrollPixelX:pixelX
                    pixelY:pixelY
                         x:point.x
                         y:self.bounds.size.height - point.y
                 modifiers:viva_modifier_flags(event.modifierFlags)
                   precise:precise];
}

- (void)emitScrollPixelX:(CGFloat)pixelX
                  pixelY:(CGFloat)pixelY
                       x:(CGFloat)x
                       y:(CGFloat)y
               modifiers:(int)modifiers
                 precise:(BOOL)precise {
    if (_callbackHandle == 0 || !_scrollEventsEnabled) {
        return;
    }
    viva_scroll_event_count++;
    viva_scroll_last_pixel_y = (double)pixelY;
    vivaScrollEvent(
        _callbackHandle,
        self,
        (double)pixelX,
        (double)pixelY,
        (double)x,
        (double)y,
        modifiers,
        precise ? 1 : 0
    );
}

- (void)keyDown:(NSEvent *)event {
    viva_probe_key(event.timestamp);
    NSString *characters = viva_key_characters(event);
	BOOL control = (event.modifierFlags & NSEventModifierFlagControl) != 0;
	BOOL command = (event.modifierFlags & NSEventModifierFlagCommand) != 0;
	int shortcutKey = characters.length == 0 ? 0 : viva_terminal_key_code([characters characterAtIndex:0]);
    if (_textInputEnabled && !_hasMarkedText && !control && !command &&
        shortcutKey == 0 && characters.length > 0 && _keyEventsEnabled && _callbackHandle != 0) {
        NSString *typed = event.characters;
        NSRange replacement = NSMakeRange(NSNotFound, 0);
        if (!_textDisableIME) {
            // Let the input method start a composition before considering a plain
            // or Option shortcut. An unchanged direct insertion is held until the
            // application declines the chord; a composition or converted commit
            // goes straight to the text service and never becomes a shortcut.
            _interpretingKey = YES;
            _interpretedConsumed = NO;
            _interpretedInsert = NO;
            _interpretedPlain = typed;
            // handleEvent also reports asynchronous ownership: an input method
            // can take the first key before it publishes any marked text.
            NSTextInputContext *context = self.inputContext;
            BOOL interpreted = context != nil && [context handleEvent:event];
            if (!interpreted) {
                [self interpretKeyEvent:event];
            }
            _interpretingKey = NO;
            _interpretedPlain = nil;
            if (_interpretedConsumed || _hasMarkedText || (interpreted && !_interpretedInsert)) {
                return;
            }
            replacement = _interpretedReplacement;
        }
        NSData *base = [characters dataUsingEncoding:NSUTF8StringEncoding];
        if (base.length > 0 && vivaShortcutEvent(_callbackHandle, self, 0,
                viva_modifier_flags(event.modifierFlags), event.isARepeat ? 1 : 0, base.bytes, base.length)) {
            return;
        }
        BOOL alt = (event.modifierFlags & NSEventModifierFlagOption) != 0;
        viva_key_event_count++;
        if (_textTerminal && (_textKeyReleases || (alt && _textAltSendsEscape))) {
            if (_textKeyReleases) {
                /* The key's text stays the unshifted character, as the rest of
                 * the keyboard protocol expects; what the key types goes to
                 * typed. */
                NSData *keyText = [viva_key_base_characters(event) dataUsingEncoding:NSUTF8StringEncoding];
                NSData *typedText = alt ? nil : [typed dataUsingEncoding:NSUTF8StringEncoding];
                vivaRawKeyEvent(_callbackHandle, self, 0, viva_modifier_flags(event.modifierFlags),
                    event.isARepeat ? 1 : 0, (unsigned int)event.keyCode + 1, keyText.bytes, keyText.length,
                    typedText.length > 0 ? typedText.bytes : NULL, typedText.length);
            } else {
                vivaKeyEvent(_callbackHandle, self, 0, viva_modifier_flags(event.modifierFlags),
                    event.isARepeat ? 1 : 0, base.bytes, base.length);
            }
        } else if (_textDisableIME) {
            // The host's input method never saw the key, so what the layout
            // types is the key's text: an ordinary field takes it the way one
            // on a platform whose raw keyboard delivers the character without
            // a method in between.
            [self emitTextKind:VIVA_TEXT_INSERT text:typed selectedRange:NSMakeRange(0, 0)
                replacementRange:NSMakeRange(NSNotFound, 0) command:0];
        } else if (_interpretedInsert) {
            [self emitTextKind:VIVA_TEXT_INSERT text:typed selectedRange:NSMakeRange(0, 0)
                replacementRange:replacement command:0];
        }
        return;
    }
	if (_keyEventsEnabled && _callbackHandle != 0 && !_hasMarkedText &&
        characters.length > 0) {
		int named = viva_terminal_key_code([characters characterAtIndex:0]);
		if (named == 0) {
			NSData *text = [characters dataUsingEncoding:NSUTF8StringEncoding];
			if (text.length > 0 && vivaShortcutEvent(
					_callbackHandle,
					self,
					0,
					viva_modifier_flags(event.modifierFlags),
					event.isARepeat ? 1 : 0,
					text.bytes,
					text.length)) {
				return;
			}
		} else if (named == VIVA_KEY_ENTER && (control || command)) {
			/* Control or Command with Return: a multiline field would make it a
			 * line break, so the application is offered it first. */
			if (vivaShortcutEvent(
					_callbackHandle,
					self,
					named,
					viva_modifier_flags(event.modifierFlags),
					event.isARepeat ? 1 : 0,
					NULL,
					0)) {
				return;
			}
		}
	}
    if (_textInputEnabled && _textTerminal && _keyEventsEnabled && _callbackHandle != 0) {
        // A terminal gets every key that is not text itself, raw: named keys,
        // Control or Command chords, and optionally printable Option chords.
        // Other printable characters, dead keys and input-method composition
        // go through the text system.
        int named = characters.length == 0 ? 0 : viva_terminal_key_code([characters characterAtIndex:0]);
        BOOL alt = (event.modifierFlags & NSEventModifierFlagOption) != 0;
        if (viva_terminal_key_routes_raw(
                _hasMarkedText,
                named,
                control,
                command,
                alt,
                _textAltSendsEscape || _textKeyReleases)) {
            // For a chord the application wants the character itself ("c" for
            // Control+C, and for Control+Shift+C too: Shift is in Modifiers);
            // named keys carry no text.
            NSString *chord = viva_key_base_characters(event);
            NSData *text = named != 0 || chord.length == 0 ? nil
                : [chord dataUsingEncoding:NSUTF8StringEncoding];
            viva_key_event_count++;
            if (_textKeyReleases) {
                vivaRawKeyEvent(_callbackHandle, self, named, viva_modifier_flags(event.modifierFlags),
                    event.isARepeat ? 1 : 0, (unsigned int)event.keyCode + 1,
                    text.length > 0 ? text.bytes : NULL, text.length, NULL, 0);
                return;
            }
            vivaKeyEvent(
                _callbackHandle,
                self,
                named,
                viva_modifier_flags(event.modifierFlags),
                event.isARepeat ? 1 : 0,
                text.length > 0 ? text.bytes : NULL,
                text.length
            );
            return;
        }
        if (_textKeyReleases && !_hasMarkedText && characters.length > 0) {
            NSString *typed = event.characters;
            _interpretedConsumed = NO;
            if (!_textDisableIME) {
                _interpretingKey = YES;
                _interpretedPlain = typed;
                [self interpretKeyEvent:event];
                _interpretingKey = NO;
                _interpretedPlain = nil;
            }
            if (!_interpretedConsumed && !_hasMarkedText) {
                NSData *base = [viva_key_base_characters(event) dataUsingEncoding:NSUTF8StringEncoding];
                NSData *typedText = [typed dataUsingEncoding:NSUTF8StringEncoding];
                viva_key_event_count++;
                vivaRawKeyEvent(_callbackHandle, self, 0, viva_modifier_flags(event.modifierFlags),
                    event.isARepeat ? 1 : 0, (unsigned int)event.keyCode + 1, base.bytes, base.length,
                    typedText.length > 0 ? typedText.bytes : NULL, typedText.length);
            }
            return;
        }
        if (!_textDisableIME) {
            [self interpretKeyEvent:event];
        }
        return;
    }
    int key = characters.length == 0 ? 0 : viva_key_code([characters characterAtIndex:0]);
    if (key == 0 && characters.length > 0) {
        key = viva_terminal_key_code([characters characterAtIndex:0]);
    }
    if (!_textDisableIME &&
        !viva_text_input_row_key_is_raw(_textInputEnabled && _textPlaced, _hasMarkedText, key, command) &&
        !viva_text_input_navigation_key_is_raw(_textInputEnabled && _textNavigationKeys, _hasMarkedText, key) &&
        viva_text_input_should_interpret_key(_textInputEnabled, _textInputMultiline, _hasMarkedText, key)) {
        [self interpretKeyEvent:event];
        return;
    }
    if (_keyEventsEnabled && _callbackHandle != 0) {
        if (key != 0) {
            viva_key_event_count++;
            vivaKeyEvent(
                _callbackHandle,
                self,
                key,
                viva_modifier_flags(event.modifierFlags),
                event.isARepeat ? 1 : 0,
                NULL,
                0
            );
            return;
        }
    }
    [super keyDown:event];
}

- (void)keyUp:(NSEvent *)event {
    if (_textInputEnabled && _textTerminal && _textKeyReleases && _keyEventsEnabled && _callbackHandle != 0) {
        // The press carried the key's identity; the runtime pairs by code and
        // drops a release it has no press for (a key the input method took).
        vivaRawKeyEvent(_callbackHandle, self, 0, viva_modifier_flags(event.modifierFlags), 2,
            (unsigned int)event.keyCode + 1, NULL, 0, NULL, 0);
        return;
    }
    [super keyUp:event];
}

- (void)flagsChanged:(NSEvent *)event {
    if (_textInputEnabled && _textTerminal && _textKeyReleases && _keyEventsEnabled && _callbackHandle != 0) {
        int pressed = 0;
        int key = viva_modifier_key_change(event.keyCode, (unsigned long)event.modifierFlags, &pressed);
        if (key != 0) {
            vivaRawKeyEvent(_callbackHandle, self, key, viva_modifier_flags(event.modifierFlags),
                pressed ? 0 : 2, (unsigned int)event.keyCode + 1, NULL, 0, NULL, 0);
            return;
        }
    }
    if (_modifierTapsEnabled && _keyEventsEnabled && _callbackHandle != 0) {
        /* The application listens for a modifier pressed on its own: report the
         * press and the release, and still let AppKit and the input method have
         * the event (a bare Shift switches an input method's language). */
        int pressed = 0;
        int key = viva_modifier_key_change(event.keyCode, (unsigned long)event.modifierFlags, &pressed);
        if (key != 0) {
            vivaRawKeyEvent(_callbackHandle, self, key, viva_modifier_flags(event.modifierFlags),
                pressed ? 0 : 2, (unsigned int)event.keyCode + 1, NULL, 0, NULL, 0);
        }
    }
    [super flagsChanged:event];
}

- (BOOL)performKeyEquivalent:(NSEvent *)event {
    NSEventModifierFlags modifiers = event.modifierFlags &
        (NSEventModifierFlagCommand | NSEventModifierFlagControl |
         NSEventModifierFlagOption | NSEventModifierFlagShift);
    NSString *characters = viva_key_characters(event);
    if (_keyEventsEnabled && _callbackHandle != 0 && !_hasMarkedText && viva_control_tab(modifiers, characters)) {
        // AppKit's window keeps Control+Tab and Control+Shift+Tab to move the
        // key view and never passes them to keyDown:, so an application would
        // not see the chord browsers and terminals switch tabs with. It is
        // delivered here, before the window can take it. No release is
        // promised: the window may swallow the matching keyUp too.
        viva_key_event_count++;
        vivaKeyEvent(_callbackHandle, self, VIVA_KEY_TAB, viva_modifier_flags(event.modifierFlags),
            event.isARepeat ? 1 : 0, NULL, 0);
        return YES;
    }
    if (_textInputEnabled && _textTerminal && modifiers == NSEventModifierFlagCommand) {
        // A terminal has no selection to edit yet; only paste is meant here.
        if ([characters isEqualToString:@"v"] || [characters isEqualToString:@"V"]) {
            [self paste:nil];
            return YES;
        }
        return [super performKeyEquivalent:event];
    }
    if (_textInputEnabled && modifiers == NSEventModifierFlagCommand) {
        // The window's menu has no Edit items, so the editing shortcuts are
        // handled here, before the menu would have been asked.
        if ([characters isEqualToString:@"a"] || [characters isEqualToString:@"A"]) {
            [self selectAll:nil];
            return YES;
        }
        if ([characters isEqualToString:@"c"] || [characters isEqualToString:@"C"]) {
            [self copy:nil];
            return YES;
        }
        if ([characters isEqualToString:@"x"] || [characters isEqualToString:@"X"]) {
            [self cut:nil];
            return YES;
        }
        if ([characters isEqualToString:@"v"] || [characters isEqualToString:@"V"]) {
            [self paste:nil];
            return YES;
        }
        if ([characters isEqualToString:@"z"] || [characters isEqualToString:@"Z"]) {
            [self emitTextKind:VIVA_TEXT_COMMAND_EVENT
                          text:@""
                 selectedRange:NSMakeRange(0, 0)
               replacementRange:NSMakeRange(NSNotFound, 0)
                       command:VIVA_TEXT_UNDO];
            return YES;
        }
    }
    if (_textInputEnabled && modifiers == (NSEventModifierFlagCommand | NSEventModifierFlagShift) &&
        ([characters isEqualToString:@"z"] || [characters isEqualToString:@"Z"])) {
        [self emitTextKind:VIVA_TEXT_COMMAND_EVENT
                      text:@""
             selectedRange:NSMakeRange(0, 0)
           replacementRange:NSMakeRange(NSNotFound, 0)
                   command:VIVA_TEXT_REDO];
        return YES;
    }
    return [super performKeyEquivalent:event];
}

- (BOOL)wantsUpdateLayer {
    return YES;
}

- (void)updateLayer {
    [self render];
}

- (void)setFrameSize:(NSSize)newSize {
    [super setFrameSize:newSize];
    [self updateDrawableSize];
    if (_resizeEventsEnabled && _callbackHandle != 0 &&
        newSize.width >= 1 && newSize.height >= 1 &&
        (!_reportedSize || newSize.width != _lastReportedSize.width ||
         newSize.height != _lastReportedSize.height)) {
        _lastReportedSize = newSize;
        _reportedSize = YES;
        viva_resize_event_count++;
        vivaViewResized(_callbackHandle, self, newSize.width, newSize.height);
    } else {
        [self setNeedsDisplay:YES];
    }
}

- (void)beginLiveResizeRelaxation {
    _liveResizeRelaxed = YES;
    /* AppKit resizes the layer before the drawable drawn for the new bounds is
     * presented; the default resize gravity would stretch the previous drawable
     * across the new bounds. With presentsWithTransaction the drawable is shown
     * in the same Core Animation transaction as the layer's new bounds, so the
     * window never shows a scaled previous frame. The resize path waits for the
     * frame to be scheduled before returning. */
    _metalLayer.presentsWithTransaction = YES;
}

- (void)endLiveResizeRelaxation {
    if (!_liveResizeRelaxed) {
        return;
    }
    _liveResizeRelaxed = NO;
    [_renderer forceRebuildTextLabelsWithViewport:self.bounds.size];
    // setNeedsDisplay is not honored without drawRect; render explicitly so
    // the refreshed textures actually reach the screen.
    [self render];
    _metalLayer.presentsWithTransaction = NO;
}

- (void)viewDidChangeBackingProperties {
    [super viewDidChangeBackingProperties];
    [self updateDrawableSize];
    if (_textInputEnabled) {
        [self rebuildTextTexture];
    }
    [self rebuildTextLabels];
    // Moving between displays of different scale lands here with no resize
    // and no input, so nothing else would draw the rebuilt frame; and
    // setNeedsDisplay is not honored without drawRect. Render explicitly or
    // the window shows the old scale's pixels until the next hover.
    [self render];
}

- (BOOL)rebuildTextLabels {
    return [_renderer rebuildTextLabelsWithViewport:self.bounds.size];
}

- (void)updateDrawableSize {
    NSScreen *screen = self.window.screen ?: NSScreen.mainScreen;
    CGFloat scale = screen != nil ? screen.backingScaleFactor : 1.0;
    NSSize size = self.bounds.size;
    _metalLayer.contentsScale = scale;
    _metalLayer.drawableSize = CGSizeMake(size.width * scale, size.height * scale);
}

// displayedText is what the platform draws and what services may read back:
// the real text, or for a password field one bullet per UTF-16 unit, so every
// range and caret offset still lines up with the model.
- (NSString *)displayedText {
    if (!_textSecure) {
        return _text;
    }
    return [@"" stringByPaddingToLength:_text.length withString:@"\u2022" startingAtIndex:0];
}

static void viva_set_secure_event_input(BOOL on) {
    // Resolved at runtime so no application links Carbon for a defensive
    // feature; the symbols come with AppKit's own dependencies.
    typedef OSStatus (*toggle)(void);
    toggle function = (toggle)dlsym(RTLD_DEFAULT, on ? "EnableSecureEventInput" : "DisableSecureEventInput");
    if (function != NULL) {
        function();
    }
}

// syncSecureEventInput keeps other processes from reading keystrokes while a
// password field has the keyboard. The mode is process-wide, so it follows the
// key window and is released as soon as the field or the window loses focus.
- (void)syncSecureEventInput {
    BOOL want = _textInputEnabled && _textSecure && self.window.isKeyWindow;
    if (want == _secureEventInputOn) {
        return;
    }
    _secureEventInputOn = want;
    viva_set_secure_event_input(want);
}

- (void)keyWindowChanged:(NSNotification *)notification {
    uint64_t generation = _textInputGeneration;
    [self syncSecureEventInput];
    if ([notification.name isEqualToString:NSWindowDidBecomeKeyNotification] &&
        notification.object == self.window && _callbackHandle != 0) {
        vivaWindowActivated(_callbackHandle);
    }
    // The keys that were down will be released to whichever window has the
    // keyboard now, so the application must let go of them here.
    if ([notification.name isEqualToString:NSWindowDidResignKeyNotification] &&
        notification.object == self.window && _callbackHandle != 0 && _keyEventsEnabled) {
        vivaFocusLost(_callbackHandle, self);
    }
    if ([notification.name isEqualToString:NSWindowDidResignKeyNotification] &&
        notification.object == self.window && _callbackHandle != 0) {
        vivaWindowDeactivated(_callbackHandle, self);
    }
    if (_textInputGeneration == generation && [notification.name isEqualToString:NSWindowDidResignKeyNotification] &&
        notification.object == self.window && _textInputEnabled && _tentativeComposition) {
        [self endOwnedNativeComposition];
    }
    if (notification.object == self.window && _textInputEnabled) {
        BOOL visible = _caretVisible;
        [self restartCaretBlink];
        if (_caretVisible != visible && [self rebuildTextTexture]) {
            [self render];
        }
    }
}

- (viva_editor_line)newTextLine {
    return viva_new_editor_line([self displayedText], _textFontSize, _textFontWeight, _textTerminal, _textCellWidth);
}

- (BOOL)rebuildTextTexture {
    if (!_textInputEnabled || _renderer.textPipeline == nil) {
        return !_textInputEnabled;
    }
    id<MTLTexture> texture = viva_text_input_texture(
        _metalLayer.device, MAX(_metalLayer.contentsScale, 1.0), [self displayedText],
        _textFontSize, _textFontWeight, _textColor, _textFrame.size, _selectedRange, _hasMarkedText,
        _markedRange, _caretVisible, _textTerminal, _textHideNativeOverlays, _textMonoFace, _textCellWidth, _textBaseline, _textAlignment);
    if (texture == nil) {
        return NO;
    }
    [_textTexture release];
    _textTexture = texture;
    return YES;
}

// caretHasKeyboard reports whether the caret belongs on screen at all: as a native
// text view's insertion point, only while its window is key. A view not yet in a
// window has nothing to lose it to.
- (BOOL)caretHasKeyboard {
    NSWindow *window = self.window;
    return window == nil || window.isKeyWindow;
}

- (void)caretBlinkTimerFired:(NSTimer *)timer {
    if (timer != _caretTimer || ![self caretHasKeyboard] || !viva_text_input_caret_should_blink(
            _textInputEnabled, _selectedRange.length, _hasMarkedText, _textTerminal, _textHideNativeOverlays)) {
        [self stopCaretBlink];
        return;
    }
    _caretVisible = !_caretVisible;
    if ([self rebuildTextTexture]) {
        [self render];
    }
}

- (void)stopCaretBlink {
    if (_caretTimer == nil) {
        return;
    }
    [_caretTimer invalidate];
    [_caretTimer release];
    _caretTimer = nil;
}

- (void)restartCaretBlink {
    [self stopCaretBlink];
    // A window in the background shows no caret and keeps no timer, so it wakes
    // for nothing; becoming key restarts the blink from the visible phase.
    _caretVisible = [self caretHasKeyboard];
    if (!_caretVisible || !viva_text_input_caret_should_blink(
            _textInputEnabled, _selectedRange.length, _hasMarkedText, _textTerminal, _textHideNativeOverlays)) {
        return;
    }
    _caretTimer = [[NSTimer timerWithTimeInterval:0.5
                                           target:self
                                         selector:@selector(caretBlinkTimerFired:)
                                         userInfo:nil
                                          repeats:YES] retain];
    [[NSRunLoop mainRunLoop] addTimer:_caretTimer forMode:NSRunLoopCommonModes];
}

- (void)notifyCompositionLost {
    if (_callbackHandle != 0 && _textInputEnabled && _tentativeComposition) {
        vivaTextCompositionLost(_callbackHandle, self, 1);
    }
}

- (void)endOwnedNativeComposition {
    if (!_hasMarkedText || !_tentativeComposition) {
        return;
    }
    uint64_t generation = _textInputGeneration;
    _hasMarkedText = NO;
    _markedRange = NSMakeRange(0, 0);
    BOOL previousDiscarding = _discardingComposition;
    _discardingComposition = YES;
    [self.inputContext discardMarkedText];
    _discardingComposition = previousDiscarding;
    if (_textInputGeneration == generation) {
        [self notifyCompositionLost];
    }
}

- (void)shutdownTextInput {
    _textInputGeneration++;
    BOOL wasEnabled = _textInputEnabled;
    _textInputEnabled = NO;
    [self syncSecureEventInput];
    _textSelectionDragging = NO;
    [self stopCaretBlink];
    if (wasEnabled) {
        [self.inputContext discardMarkedText];
        [self.inputContext deactivate];
    }
    _hasMarkedText = NO;
    _textAltSendsEscape = NO;
    _textKeyReleases = NO;
    _textDisableIME = NO;
    _tentativeComposition = NO;
}

- (void)detachCallbacks {
    // The view can outlive its Go window and receive a final mouseExited:.
    _callbackHandle = 0;
    [self shutdownTextInput];
}

- (void)render {
    [self renderWaitingForSchedule:NO];
}

- (void)renderWaitingForSchedule:(BOOL)wait {
    @autoreleasepool {
        if (_metalLayer.drawableSize.width < 1 || _metalLayer.drawableSize.height < 1) {
            return;
        }

        BOOL probe = viva_input_probe_enabled();
        uint64_t waitStart = probe ? mach_continuous_time() : 0;
        id<CAMetalDrawable> drawable = [_metalLayer nextDrawable];
        if (probe) {
            double waited = viva_elapsed_milliseconds(waitStart, mach_continuous_time()) / 1000.0;
            [viva_input_probe_lock lock];
            viva_probe_renders++;
            viva_probe_blocked_total += waited;
            viva_probe_blocked_max = MAX(viva_probe_blocked_max, waited);
            if (waited > 0.001) {
                viva_probe_blocked_over_1ms++;
            }
            [viva_input_probe_lock unlock];
        }
        if (drawable == nil) {
            return;
        }
        if (probe) {
            [drawable addPresentedHandler:^(id<MTLDrawable> presented) {
                viva_probe_presented(presented.presentedTime);
            }];
        }

        id<MTLCommandBuffer> commandBuffer = [_renderer renderToDrawable:drawable
                                                                viewport:self.bounds.size
                                                            inputTexture:_textTexture
                                                              inputFrame:_textFrame
                                                              inputLayer:_textLayer];
        if (probe) {
            viva_probe_committed();
        }

        if (wait) {
            // A live resize must not return to AppKit before the frame is
            // scheduled: with presentsWithTransaction set, the drawable is
            // presented in the same transaction as the layer's new bounds, so
            // the window never shows the previous frame stretched. A scroll
            // gesture that can outrun the display uses the same path.
            [commandBuffer waitUntilScheduled];
        } else if (_profileEnabled && !_reportedFirstFrame) {
            [commandBuffer waitUntilScheduled];
            uint64_t now = mach_continuous_time();
            fprintf(
                stderr,
                "viva_first_frame_ms=%.3f\n",
                viva_elapsed_milliseconds(viva_process_start_ticks, now)
            );
            fflush(stderr);
            _reportedFirstFrame = YES;
        }
    }
}

/* recoverResources makes the window's Metal resources again after the device
 * stopped serving them: the driver was reset, the GPU hung, an external GPU was
 * unplugged or switched, or an allocation failed under pressure. What the
 * window shows lives in the application, so the scene is sent again as a
 * rebuild; the label layers rebuild their textures on the new device here.
 * Returns NO when the device, its queue or the pipelines cannot be made, which
 * is when the window cannot go on. */
- (BOOL)recoverResources {
    @autoreleasepool {
        [self updateDrawableSize];
        if (![_renderer recoverWithViewport:self.bounds.size]) {
            return NO;
        }
        if (_textInputEnabled && ![self rebuildTextTexture]) {
            return NO;
        }
        return YES;
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

- (int)updateLabels:(const viva_text_label *)labels
              count:(NSUInteger)count {
    return [_renderer updateLabels:labels count:count viewport:self.bounds.size];
}

- (int)updateAccessibility:(const viva_accessibility_node *)nodes
                     count:(NSUInteger)count {
    NSMutableArray<VivaAccessibleElement *> *replacement =
        [[NSMutableArray alloc] initWithCapacity:count];
    NSRect windowFrame = self.bounds;
    for (NSUInteger index = 0; index < count; index++) {
        VivaAccessibleElement *element =
            [[VivaAccessibleElement alloc] initWithConfig:nodes[index]
                                             windowFrame:windowFrame
                                                 focused:NO
                                          callbackHandle:_callbackHandle
                                                    view:self];
        if (element == nil) {
            [replacement release];
            return 2;
        }
        [replacement addObject:element];
        [element release];
    }
    NSArray<VivaAccessibleElement *> *previous = _accessibilityElements;
    _accessibilityElements = replacement;
    [self announceAccessibilityChangesFrom:previous];
    [previous release];
    viva_accessibility_update_count++;
    return 0;
}

// announceAccessibilityChangesFrom tells assistive technology what the new
// elements did that it cannot see for itself: the element that took the
// keyboard, and a live region whose words changed.
- (void)announceAccessibilityChangesFrom:(NSArray<VivaAccessibleElement *> *)previous {
    if (self.window == nil) {
        return;
    }
    uint64_t previousFocus = 0;
    for (VivaAccessibleElement *element in previous) {
        if (element.nodeFocused) {
            previousFocus = element.nodeID;
        }
    }
    // A live region is told apart by where it is: a status area that is only
    // text has no identity of its own.
    NSMutableDictionary<NSString *, VivaAccessibleElement *> *before = nil;
    for (VivaAccessibleElement *element in _accessibilityElements) {
        if (element.nodeFocused && element.nodeID != previousFocus) {
            NSAccessibilityPostNotification(element, NSAccessibilityFocusedUIElementChangedNotification);
        }
        if (element.live == 0) {
            continue;
        }
        if (before == nil) {
            before = [NSMutableDictionary dictionary];
            for (VivaAccessibleElement *old in previous) {
                if (old.live != 0) {
                    before[NSStringFromRect(old.accessibilityFrame)] = old;
                }
            }
        }
        VivaAccessibleElement *old = before[NSStringFromRect(element.accessibilityFrame)];
        NSString *words = element.valueText.length > 0 ? element.valueText : element.accessibilityLabel;
        NSString *oldWords = old.valueText.length > 0 ? old.valueText : old.accessibilityLabel;
        if (old != nil && words.length > 0 && ![words isEqualToString:oldWords]) {
            NSAccessibilityPostNotificationWithUserInfo(
                self.window, NSAccessibilityAnnouncementRequestedNotification,
                @{NSAccessibilityAnnouncementKey: words,
                  NSAccessibilityPriorityKey: @(element.live == 2 ? NSAccessibilityPriorityHigh : NSAccessibilityPriorityMedium)});
        }
    }
}

- (BOOL)isAccessibilityElement {
    return NO;
}

- (NSArray *)accessibilityChildren {
    return _accessibilityElements;
}

- (int)updateTextInput:(viva_text_input_config)textInput {
    return [self updateTextInput:textInput keepComposition:NO];
}

// updateTextInput:keepComposition: mirrors a new editor configuration into the
// native surface. With keep set, the editor is the one that was already
// focused and the application acknowledges the same text and selection: a
// field that grows a row or moves keeps the input method's composition, where
// an unkept update discards it. The frame, caret, texture and character
// coordinates still follow, so the candidate window moves with the caret.
- (int)updateTextInput:(viva_text_input_config)textInput keepComposition:(BOOL)keep {
    if (!_dynamicTextInput) {
        return 4;
    }
    if (textInput.enabled &&
        (textInput.width <= 0 || textInput.height <= 0 || textInput.font_size <= 0 ||
         (textInput.utf8 == NULL && textInput.utf8_length > 0))) {
        return 1;
    }

    NSString *replacement = nil;
    NSColor *replacementColor = nil;
    if (textInput.enabled) {
        replacement = [[NSString alloc] initWithBytes:textInput.utf8
                                               length:textInput.utf8_length
                                             encoding:NSUTF8StringEncoding];
        if (replacement == nil) {
            return 2;
        }
        replacementColor = [[NSColor colorWithSRGBRed:textInput.red
                                                green:textInput.green
                                                 blue:textInput.blue
                                                alpha:textInput.alpha] retain];
    }

    BOOL wasEnabled = _textInputEnabled;
    keep = keep && wasEnabled && textInput.enabled != 0;
    if (!keep) {
        _textInputGeneration++;
        _textInputEnabled = NO;
        [self syncSecureEventInput];
        _textSelectionDragging = NO;
        [self stopCaretBlink];
        if (wasEnabled) {
            [self.inputContext discardMarkedText];
            [self.inputContext deactivate];
        }
        _hasMarkedText = NO;
    }

    if (!textInput.enabled) {
        _textSecure = NO;
        _textCopyOverride = NO;
        _textTerminal = NO;
        _textHideNativeOverlays = NO;
        _textMonoFace = 0;
        _textCellWidth = 0;
        _textBaseline = 0;
        _textAltSendsEscape = NO;
        _textKeyReleases = NO;
        _textDisableIME = NO;
        _tentativeComposition = NO;
        [_text release];
        _text = [@"" retain];
        [_textColor release];
        _textColor = nil;
        [_textTexture release];
        _textTexture = nil;
        _textFrame = NSZeroRect;
        _textFontSize = 0;
        _textFontWeight = 0;
        _textAlignment = 0;
        _selectedRange = NSMakeRange(0, 0);
        _markedRange = NSMakeRange(0, 0);
        _textInputMultiline = NO;
        _textPlaced = NO;
        _textNavigationKeys = NO;
        _textLayer = 0;
        _textCaretFrame = NSZeroRect;
        _caretVisible = NO;
        return 0;
    }

    [_text release];
    _text = replacement;
    _textCopyOverride = textInput.copy_override != 0;
    [_textColor release];
    _textColor = replacementColor;
    _textFrame = NSMakeRect(textInput.x, textInput.y, textInput.width, textInput.height);
    _textFontSize = textInput.font_size;
    _textFontWeight = textInput.font_weight;
    _textAlignment = textInput.placed || textInput.terminal ? 0 : textInput.alignment;
    _selectedRange = viva_clamp_range(
        NSMakeRange(textInput.selected_start, textInput.selected_length),
        _text.length
    );
    _markedRange = viva_clamp_range(
        NSMakeRange(textInput.marked_start, textInput.marked_length),
        _text.length
    );
    _textAltSendsEscape = textInput.alt_sends_escape != 0;
    _textKeyReleases = textInput.key_releases != 0;
    _textDisableIME = textInput.disable_ime != 0;
    // A surface that runs the other machine's input method has no composition
    // of this one to mirror.
    _hasMarkedText = textInput.has_marked_text != 0 && !_textDisableIME;
    _tentativeComposition = textInput.tentative_composition != 0;
    _textInputMultiline = textInput.multiline != 0;
    _textPlaced = textInput.placed != 0;
    _textNavigationKeys = textInput.navigation_keys != 0;
    _textLayer = textInput.layer;
    _textCaretFrame = NSMakeRect(textInput.caret_x, textInput.caret_y, textInput.caret_width, textInput.caret_height);
    _textSecure = textInput.secure != 0;
    _textTerminal = textInput.terminal != 0;
    _textHideNativeOverlays = textInput.hide_native_overlays != 0;
    _textMonoFace = textInput.mono_face;
    _textCellWidth = textInput.cell_width;
    _textBaseline = textInput.baseline;
    _textInputEnabled = YES;
    [self syncSecureEventInput];
    _caretVisible = YES;
    if (![self rebuildTextTexture]) {
        return 3;
    }
    if (keep) {
        // The input context stays active and composing; only where it points
        // changed.
        if (!_textDisableIME) {
            [self.inputContext invalidateCharacterCoordinates];
        }
        [self restartCaretBlink];
        return 0;
    }
    [self.window makeFirstResponder:self];
    if (!_textDisableIME) {
        // A surface that runs the other machine's input method opens no
        // session here: the method would compose the keys that belong there.
        [self.inputContext activate];
        [self.inputContext invalidateCharacterCoordinates];
    }
    [self restartCaretBlink];
    return 0;
}

// moveTextInputToFrame follows a terminal's cursor. Unlike updateTextInput it
// leaves a composition in progress alone, so output arriving while someone is
// typing through an input method does not cancel it.
- (int)moveTextInputToFrame:(NSRect)frame {
    if (!_textInputEnabled || !_textTerminal || frame.size.width <= 0 || frame.size.height <= 0) {
        return 1;
    }
    BOOL resized = frame.size.width != _textFrame.size.width || frame.size.height != _textFrame.size.height;
    _textFrame = frame;
    if (resized && ![self rebuildTextTexture]) {
        return 3;
    }
    [self.inputContext invalidateCharacterCoordinates];
    return 0;
}

- (int)updateText:(const unsigned char *)utf8
            length:(NSUInteger)length
     selectedRange:(NSRange)selectedRange
       markedRange:(NSRange)markedRange
     hasMarkedText:(BOOL)hasMarkedText
 tentativeComposition:(BOOL)tentativeComposition
    endsComposition:(BOOL)endsComposition {
    if (!_textInputEnabled || (utf8 == NULL && length > 0)) {
        return 1;
    }
    NSString *replacement = [[NSString alloc] initWithBytes:utf8
                                                        length:length
                                                      encoding:NSUTF8StringEncoding];
    if (replacement == nil) {
        return 2;
    }
    BOOL lostComposition = _hasMarkedText && _tentativeComposition && tentativeComposition && !hasMarkedText && !endsComposition;
    [_text release];
    _text = replacement;
    _selectedRange = viva_clamp_range(selectedRange, _text.length);
    _markedRange = viva_clamp_range(markedRange, _text.length);
    _hasMarkedText = hasMarkedText;
    _tentativeComposition = tentativeComposition;
    _caretVisible = YES;
    if (![self rebuildTextTexture]) {
        if (lostComposition) {
            // The editor's texture is gone, but the composition it lost is not:
            // record it before the early return, because the retry re-evaluates
            // against the state this call already installed.
            [self notifyCompositionLost];
        }
        return 3;
    }
    [self restartCaretBlink];
    if (lostComposition) {
        [self notifyCompositionLost];
    }
    return 0;
}

- (void)emitTextKind:(int)kind
                text:(NSString *)text
       selectedRange:(NSRange)selectedRange
     replacementRange:(NSRange)replacementRange
             command:(int)command {
    if (_discardingComposition || _callbackHandle == 0 || !_textInputEnabled) {
        return;
    }
    if (_interpretingKey) {
        // A key a terminal gets as a raw press: the text system answering with
        // exactly the typed character is the key itself, anything else (a dead
        // key, composition, a commit) belongs to the input method.
        if (kind == VIVA_TEXT_INSERT && !_hasMarkedText && _interpretedPlain != nil && [text isEqualToString:_interpretedPlain]) {
            _interpretedInsert = YES;
            _interpretedReplacement = replacementRange;
            return;
        }
        _interpretedConsumed = YES;
    }
    NSData *data = [text dataUsingEncoding:NSUTF8StringEncoding];
    int phase = _tentativeComposition ? viva_text_composition_phase(kind, text.length, _hasMarkedText, replacementRange) : VIVA_COMPOSITION_NONE;
    // AppKit names ranges in the view text, which still holds the preedit; the
    // phased engine applies a replacement to the text after the tentative
    // rollback. A range that names the marked text is the composition
    // continuing or committing, so the engine must act on the composition
    // itself; a range beyond it shifts left by the preedit.
    NSRange phasedReplacement = replacementRange;
    if (phase != VIVA_COMPOSITION_NONE && _hasMarkedText && phasedReplacement.location != NSNotFound) {
        NSRange marked = _markedRange;
        NSUInteger markedEnd = marked.location + marked.length;
        NSUInteger end = phasedReplacement.length > NSUIntegerMax - phasedReplacement.location
            ? NSUIntegerMax
            : phasedReplacement.location + phasedReplacement.length;
        BOOL namesComposition = phasedReplacement.location >= marked.location && end <= markedEnd;
        BOOL caretInComposition = phasedReplacement.length == 0 &&
            phasedReplacement.location >= marked.location && phasedReplacement.location <= markedEnd;
        if (namesComposition || caretInComposition) {
            phasedReplacement = NSMakeRange(NSNotFound, 0);
        } else {
            NSUInteger start = phasedReplacement.location >= markedEnd
                ? phasedReplacement.location - marked.length
                : phasedReplacement.location;
            NSUInteger stop = end >= markedEnd
                ? end - marked.length
                : end > marked.location ? marked.location : end;
            if (stop < start) {
                stop = start;
            }
            phasedReplacement = NSMakeRange(start, stop - start);
        }
    }
    int64_t replacementStart = -1;
    int64_t replacementLength = 0;
    if (phase == VIVA_COMPOSITION_NONE && replacementRange.location != NSNotFound) {
        replacementStart = (int64_t)replacementRange.location;
        replacementLength = (int64_t)replacementRange.length;
    }
    BOOL raw = phase != VIVA_COMPOSITION_NONE && phasedReplacement.location != NSNotFound;
    viva_text_event_count++;
    if (phase == VIVA_COMPOSITION_NONE) {
        vivaTextInput(_callbackHandle, self, kind,
            data.length > 0 ? data.bytes : NULL, data.length,
            (int64_t)selectedRange.location, (int64_t)selectedRange.length,
            replacementStart, replacementLength, command);
        return;
    }
    vivaCompositionInput(
        _callbackHandle,
        self,
        kind,
        data.length > 0 ? data.bytes : NULL,
        data.length,
        (int64_t)selectedRange.location,
        (int64_t)selectedRange.length,
        replacementStart,
        replacementLength,
        command,
        phase,
        raw ? viva_text_raw_coordinate(phasedReplacement.location) : 0,
        raw ? viva_text_raw_coordinate(phasedReplacement.length) : 0,
        raw ? 1 : 0
    );
}

- (void)cancelMarkedComposition {
    if (_compositionCancelEmitted) {
        return;
    }
    _compositionCancelEmitted = YES;
    [self emitTextKind:VIVA_TEXT_CANCEL_COMPOSITION
                  text:@""
         selectedRange:NSMakeRange(0, 0)
       replacementRange:NSMakeRange(NSNotFound, 0)
               command:0];
}

/* interpretKeyEvent is the one place that knows the key being interpreted is
 * Escape during an input-method composition. Inside that window an empty mark,
 * an empty insert or cancelOperation: is the input method's explicit
 * cancellation; outside it they keep their ordinary meaning. */
- (void)interpretKeyEvent:(NSEvent *)event {
    BOOL previousCanceling = _cancelingComposition;
    BOOL previousEmitted = _compositionCancelEmitted;
    NSString *characters = event.characters;
    _cancelingComposition = _hasMarkedText && characters.length > 0 &&
        viva_key_code([characters characterAtIndex:0]) == VIVA_KEY_ESCAPE;
    _compositionCancelEmitted = NO;
    [self interpretKeyEvents:@[event]];
    _cancelingComposition = previousCanceling;
    _compositionCancelEmitted = previousEmitted;
}

- (void)insertText:(id)string replacementRange:(NSRange)replacementRange {
    NSString *plain = [string isKindOfClass:[NSAttributedString class]]
        ? [(NSAttributedString *)string string]
        : (NSString *)string;
    if (_cancelingComposition) {
        if (_compositionCancelEmitted) {
            return;
        }
        if (plain.length == 0) {
            [self cancelMarkedComposition];
            return;
        }
        _cancelingComposition = NO;
    }
    [self emitTextKind:VIVA_TEXT_INSERT
                  text:plain ?: @""
         selectedRange:NSMakeRange(0, 0)
       replacementRange:replacementRange
               command:0];
}

- (void)setMarkedText:(id)string
         selectedRange:(NSRange)selectedRange
       replacementRange:(NSRange)replacementRange {
    NSString *plain = [string isKindOfClass:[NSAttributedString class]]
        ? [(NSAttributedString *)string string]
        : (NSString *)string;
    if (_cancelingComposition) {
        if (_compositionCancelEmitted) {
            return;
        }
        if (plain.length == 0) {
            [self cancelMarkedComposition];
            return;
        }
        _cancelingComposition = NO;
    }
    [self emitTextKind:VIVA_TEXT_SET_MARKED
                  text:plain ?: @""
         selectedRange:selectedRange
       replacementRange:replacementRange
               command:0];
}

- (void)unmarkText {
    [self emitTextKind:VIVA_TEXT_UNMARK
                  text:@""
         selectedRange:NSMakeRange(0, 0)
       replacementRange:NSMakeRange(NSNotFound, 0)
               command:0];
}

- (BOOL)hasMarkedText {
    return _hasMarkedText;
}

- (NSRange)markedRange {
    return _hasMarkedText ? _markedRange : NSMakeRange(NSNotFound, 0);
}

- (NSRange)selectedRange {
    return _selectedRange;
}

- (NSArray<NSAttributedStringKey> *)validAttributesForMarkedText {
    return @[];
}

- (NSAttributedString *)attributedSubstringForProposedRange:(NSRange)range
                                                 actualRange:(NSRangePointer)actualRange {
    NSRange safe = viva_clamp_range(range, _text.length);
    if (actualRange != NULL) {
        *actualRange = safe;
    }
    return [[[NSAttributedString alloc] initWithString:[[self displayedText] substringWithRange:safe]] autorelease];
}

- (NSUInteger)characterIndexForPoint:(NSPoint)point {
    NSPoint windowPoint = [self.window convertPointFromScreen:point];
    NSPoint local = [self convertPoint:windowPoint fromView:nil];
    return [self textIndexForLocalPoint:local];
}

- (NSUInteger)textIndexForEvent:(NSEvent *)event {
    NSPoint local = [self convertPoint:event.locationInWindow fromView:nil];
    return [self textIndexForLocalPoint:local];
}

- (NSUInteger)textIndexForLocalPoint:(NSPoint)point {
    CGFloat x = point.x - _textFrame.origin.x;
    viva_editor_line line = [self newTextLine];
    if (!viva_editor_line_made(line)) {
        return _selectedRange.location;
    }
    x -= viva_editor_alignment_inset(line, _textFrame.size.width, _textAlignment, _textTerminal);
    NSUInteger position = viva_text_index_at(line, [self displayedText], x, _textTerminal);
    viva_editor_line_free(line);
    return MIN(position, _text.length);
}

- (CGFloat)textOffsetForIndex:(NSUInteger)index line:(viva_editor_line)line {
    return viva_text_offset(line, index, _textTerminal);
}

- (BOOL)eventIsInsideTextInput:(NSEvent *)event {
    NSPoint local = [self convertPoint:event.locationInWindow fromView:nil];
    NSPoint topLeft = NSMakePoint(local.x, self.bounds.size.height - local.y);
    return NSPointInRect(topLeft, _textFrame);
}

- (void)selectTextToIndex:(NSUInteger)index {
    // Keep the word under a double press selected through the release and drag.
    NSUInteger start = MIN(_textSelectionAnchor.location, index);
    NSUInteger end = MAX(NSMaxRange(_textSelectionAnchor), index);
    [self selectTextRange:NSMakeRange(start, end - start)];
}

- (void)selectTextRange:(NSRange)range {
    range = viva_clamp_range(range, _text.length);
    if (!_textInputEnabled || (NSEqualRanges(range, _selectedRange) && !_hasMarkedText)) {
        return;
    }
    if (_hasMarkedText) {
        if (_tentativeComposition) {
            uint64_t generation = _textInputGeneration;
            [self endOwnedNativeComposition];
            if (_textInputGeneration != generation || !_textInputEnabled) {
                return;
            }
            range = viva_clamp_range(range, _text.length);
        } else {
            [self.inputContext discardMarkedText];
        }
    }
    [self emitTextKind:VIVA_TEXT_SET_SELECTION
                  text:@""
         selectedRange:range
       replacementRange:NSMakeRange(NSNotFound, 0)
               command:0];
}

- (void)selectAll:(id)sender {
    (void)sender;
    _textSelectionDragging = NO;
    [self selectTextRange:NSMakeRange(0, _text.length)];
}

- (NSRect)firstRectForCharacterRange:(NSRange)range actualRange:(NSRangePointer)actualRange {
    NSRange safe = viva_clamp_range(range, _text.length);
    if (actualRange != NULL) {
        *actualRange = safe;
    }
    CGFloat offset = 0;
    viva_editor_line line = [self newTextLine];
    if (viva_editor_line_made(line)) {
        offset = viva_editor_visible_caret(
            viva_editor_alignment_inset(line, _textFrame.size.width, _textAlignment, _textTerminal) +
                [self textOffsetForIndex:safe.location line:line],
            _textFrame.size.width, _textAlignment, _textTerminal);
        viva_editor_line_free(line);
    }
    NSRect local = NSMakeRect(
        _textFrame.origin.x + offset,
        self.bounds.size.height - NSMaxY(_textFrame),
        1,
        _textFrame.size.height
    );
    if (_textPlaced && _textCaretFrame.size.height > 0) {
        // The field lays its rows out itself: the caret is where it painted it.
        local = NSMakeRect(
            _textCaretFrame.origin.x,
            self.bounds.size.height - NSMaxY(_textCaretFrame),
            MAX(_textCaretFrame.size.width, 1),
            _textCaretFrame.size.height
        );
    }
    NSRect windowRect = [self convertRect:local toView:nil];
    return [self.window convertRectToScreen:windowRect];
}

// A failed copy never emits a cut edit. A custom selection is supplied by the
// application while the native primary selection remains an ordinary caret.
- (BOOL)copyToPasteboard {
    if (!_textInputEnabled || _textSecure) {
        return NO;
    }
    NSString *value = nil;
    if (_textCopyOverride) {
        size_t length = 0;
        unsigned char *utf8 = vivaCopyText(_callbackHandle, self, &length);
        if (utf8 == NULL || length == 0) {
            free(utf8);
            return NO;
        }
        value = [[NSString alloc] initWithBytes:utf8 length:length encoding:NSUTF8StringEncoding];
        free(utf8);
        if (value == nil) {
            return NO;
        }
    } else {
        NSRange selected = viva_clamp_range(_selectedRange, _text.length);
        if (selected.length == 0) {
            return NO;
        }
        value = [_text substringWithRange:selected];
    }
    NSPasteboard *pasteboard = viva_pasteboard();
    [pasteboard clearContents];
    BOOL copied = [pasteboard setString:value forType:NSPasteboardTypeString];
    if (_textCopyOverride) {
        [value release];
    }
    return copied;
}

- (void)copy:(id)sender {
    [self copyToPasteboard];
}

// cut: copies, then asks the application to remove its selection.
- (void)cut:(id)sender {
    if (![self copyToPasteboard]) {
        return;
    }
    [self emitTextKind:VIVA_TEXT_PASTE
                  text:@""
         selectedRange:NSMakeRange(0, 0)
       replacementRange:NSMakeRange(NSNotFound, 0)
               command:0];
}

// paste: inserts the pasteboard's text in place of the selection. Passwords may
// be pasted; single-line fields get their line breaks flattened.
- (void)paste:(id)sender {
    if (!_textInputEnabled) {
        return;
    }
    NSString *value = [viva_pasteboard() stringForType:NSPasteboardTypeString];
    if (_textTerminal) {
        // The application decides how pasted text reaches the program (for
        // example inside bracketed-paste markers), so tell it this is a paste.
        if (value.length > 0) {
            [self emitTextKind:VIVA_TEXT_PASTE
                          text:value
                 selectedRange:NSMakeRange(0, 0)
               replacementRange:NSMakeRange(NSNotFound, 0)
                       command:0];
        } else {
            [self emitTextKind:VIVA_TEXT_PASTE_NO_TEXT
                          text:@""
                 selectedRange:NSMakeRange(0, 0)
               replacementRange:NSMakeRange(NSNotFound, 0)
                       command:0];
        }
        return;
    }
    if (value.length == 0) {
        // Nothing to insert. An application that takes pictures looks at the
        // pasteboard itself; for any other editor the event changes nothing.
        [self emitTextKind:VIVA_TEXT_PASTE_NO_TEXT
                      text:@""
             selectedRange:NSMakeRange(0, 0)
           replacementRange:NSMakeRange(NSNotFound, 0)
                   command:0];
        return;
    }
    if (!_textInputMultiline) {
        value = viva_single_line_paste(value);
        if (value.length == 0) {
            return;
        }
    }
    [self emitTextKind:VIVA_TEXT_PASTE
                  text:value
         selectedRange:NSMakeRange(0, 0)
       replacementRange:NSMakeRange(NSNotFound, 0)
               command:0];
}

/* The modifiers held with the key being handled. A method so a test can say
 * which are down: AppKit supplies them only through the event it is dispatching. */
- (NSEventModifierFlags)returnKeyModifierFlags {
    return NSApp.currentEvent.modifierFlags;
}

- (void)doCommandBySelector:(SEL)selector {
    int command = 0;
    if (selector == @selector(deleteBackward:)) {
        command = VIVA_TEXT_DELETE_BACKWARD;
    } else if (selector == @selector(deleteForward:)) {
        command = VIVA_TEXT_DELETE_FORWARD;
    } else if (selector == @selector(moveLeft:)) {
        command = VIVA_TEXT_MOVE_LEFT;
    } else if (selector == @selector(moveRight:)) {
        command = VIVA_TEXT_MOVE_RIGHT;
    } else if (selector == @selector(moveLeftAndModifySelection:)) {
        command = VIVA_TEXT_MOVE_LEFT_AND_MODIFY_SELECTION;
    } else if (selector == @selector(moveRightAndModifySelection:)) {
        command = VIVA_TEXT_MOVE_RIGHT_AND_MODIFY_SELECTION;
    } else if (selector == @selector(insertLineBreak:) || selector == @selector(insertNewlineIgnoringFieldEditor:)) {
        command = VIVA_TEXT_INSERT_LINE_BREAK;
    } else if (selector == @selector(insertNewline:)) {
        // Return and Shift+Return both arrive as insertNewline:, so the key
        // that is down decides: Shift, Option or Control asks for a line break.
        BOOL lineBreak = ([self returnKeyModifierFlags] & (NSEventModifierFlagShift | NSEventModifierFlagOption | NSEventModifierFlagControl)) != 0;
        command = lineBreak ? VIVA_TEXT_INSERT_LINE_BREAK : VIVA_TEXT_INSERT_NEWLINE;
    } else if (selector == @selector(insertTab:)) {
        command = VIVA_TEXT_INSERT_TAB;
    } else if (selector == @selector(selectAll:)) {
        [self selectAll:nil];
        return;
    } else if (selector == @selector(copy:)) {
        [self copy:nil];
        return;
    } else if (selector == @selector(cut:)) {
        [self cut:nil];
        return;
    } else if (selector == @selector(paste:)) {
        [self paste:nil];
        return;
    } else if (selector == @selector(undo:)) {
        command = VIVA_TEXT_UNDO;
    } else if (selector == @selector(redo:)) {
        command = VIVA_TEXT_REDO;
    } else if (selector == @selector(cancelOperation:)) {
        if (_hasMarkedText) {
            if (!_cancelingComposition) {
                _compositionCancelEmitted = NO;
            }
            [self cancelMarkedComposition];
            return;
        }
        command = viva_navigation_command(selector);
    } else {
        command = viva_navigation_command(selector);
    }
    if (command != 0) {
        [self emitTextKind:VIVA_TEXT_COMMAND_EVENT
                      text:@""
             selectedRange:NSMakeRange(0, 0)
           replacementRange:NSMakeRange(NSNotFound, 0)
                   command:command];
    }
}

@end

/* ---- Files dragged out of this window ---- */

extern int vivaFilePromiseContent(intptr_t drag, int item, const char *path);
extern void vivaFileDragFinished(intptr_t drag);

// viva_drag_operation turns the platform effect bits (1 copy, 2 move, 4 link)
// into the NSDragOperation a dragging source reports.
static NSDragOperation viva_drag_operation(unsigned allowed) {
    NSDragOperation operation = NSDragOperationNone;
    if (allowed & 1) {
        operation |= NSDragOperationCopy;
    }
    if (allowed & 2) {
        operation |= NSDragOperationMove;
    }
    if (allowed & 4) {
        operation |= NSDragOperationLink;
    }
    return operation;
}

// viva_drag_view is the view of the window handling the current press: a drag
// starts from the mouse event that is being handled, and the session takes the
// pointer over from it.
static VivaMetalView *viva_drag_view(NSEvent **outEvent) {
    NSEvent *event = [NSApp currentEvent];
    if (outEvent != NULL) {
        *outEvent = event;
    }
    if (event == nil || event.window == nil) {
        return nil;
    }
    id view = event.window.contentView;
    if (![view isKindOfClass:[VivaMetalView class]]) {
        return nil;
    }
    return (VivaMetalView *)view;
}

// viva_drag_image is the picture that follows the pointer: the file's own icon
// where there is a path, a folder or a stack of documents for promised content.
static NSImage *viva_drag_image(NSString *path, BOOL directory) {
    NSImage *image = nil;
    if (path != nil) {
        image = [[NSWorkspace sharedWorkspace] iconForFile:path];
    }
    if (image == nil) {
        image = directory ? [NSImage imageNamed:NSImageNameFolder] : [NSImage imageNamed:NSImageNameMultipleDocuments];
    }
    image.size = NSMakeSize(32, 32);
    return image;
}

static NSRect viva_drag_frame(VivaMetalView *view, NSEvent *event) {
    NSPoint local = [view convertPoint:event.locationInWindow fromView:nil];
    return NSMakeRect(local.x - 16, local.y - 16, 32, 32);
}

// VivaFileDragContext is the promised drag's content registry as long as any
// provider may ask for it. The providers keep their delegates (through
// associated objects); the delegates keep this; its deallocation is the moment
// the application can forget the promises.
@interface VivaFileDragContext : NSObject {
    intptr_t _drag;
}
- (instancetype)initWithDrag:(intptr_t)drag;
- (intptr_t)drag;
@end

@implementation VivaFileDragContext
- (instancetype)initWithDrag:(intptr_t)drag {
    if ((self = [super init]) != nil) {
        _drag = drag;
    }
    return self;
}
- (intptr_t)drag {
    return _drag;
}
- (void)dealloc {
    if (_drag != 0) {
        vivaFileDragFinished(_drag);
    }
    [super dealloc];
}
@end

// VivaFileDragSource reports the end of a session the view started. It is not
// retained by the view after the session ends; the system keeps it while the
// session runs.
@implementation VivaFileDragSource
- (instancetype)initWithView:(VivaMetalView *)view handle:(uintptr_t)handle drag:(intptr_t)drag allowed:(NSDragOperation)allowed {
    if ((self = [super init]) != nil) {
        _view = view;
        _drag = drag;
        _allowed = allowed;
    }
    return self;
}

- (NSDragOperation)draggingSession:(NSDraggingSession *)session sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
    (void)session;
    (void)context;
    return _allowed;
}

- (void)draggingSession:(NSDraggingSession *)session endedAtPoint:(NSPoint)screenPoint operation:(NSDragOperation)operation {
    (void)session;
    (void)screenPoint;
    (void)operation;
    // The view releases this source here, so keep it alive across the call.
    [self retain];
    [_view fileDragSessionEnded];
    [self release];
}

// A drag to another application that also accepts drops is the only thing a
// promised drag does; a link or a move would have to be produced by the source.
- (BOOL)ignoreModifierKeysForDraggingSession:(NSDraggingSession *)session {
    (void)session;
    return YES;
}
@end

// VivaFilePromiseDelegate produces one promised item when the target asks. Its
// provider keeps it alive through the association set when the item is built,
// and the context keeps the application's promises alive for as long as any
// provider may ask.
@interface VivaFilePromiseDelegate : NSObject <NSFilePromiseProviderDelegate> {
    NSString *_name;
    int _item;
    VivaFileDragContext *_context; // retained
}
- (instancetype)initWithName:(NSString *)name item:(int)item context:(VivaFileDragContext *)context;
@end

@implementation VivaFilePromiseDelegate
- (instancetype)initWithName:(NSString *)name item:(int)item context:(VivaFileDragContext *)context {
    if ((self = [super init]) != nil) {
        _name = [name copy];
        _item = item;
        _context = [context retain];
    }
    return self;
}
- (void)dealloc {
    [_name release];
    [_context release];
    [super dealloc];
}

- (NSString *)filePromiseProvider:(NSFilePromiseProvider *)provider fileNameForType:(NSString *)fileType {
    (void)provider;
    (void)fileType;
    return _name;
}

- (NSOperationQueue *)operationQueueForFilePromiseProvider:(NSFilePromiseProvider *)provider {
    (void)provider;
    static NSOperationQueue *queue;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        queue = [[NSOperationQueue alloc] init];
        queue.name = @"viva-file-promise";
    });
    return queue;
}

- (void)filePromiseProvider:(NSFilePromiseProvider *)provider writePromiseToURL:(NSURL *)url completionHandler:(void (^)(NSError *))completionHandler {
    (void)provider;
    NSString *path = url.path;
    const char *representation = path == nil ? NULL : path.fileSystemRepresentation;
    int status = representation == NULL ? -1 : vivaFilePromiseContent([_context drag], _item, representation);
    if (status == 0) {
        completionHandler(nil);
        return;
    }
    NSError *error = [NSError errorWithDomain:@"VivaFilePromiseErrorDomain"
                                         code:status
                                     userInfo:@{ NSLocalizedDescriptionKey : @"The file could not be produced." }];
    completionHandler(error);
}
@end

int viva_start_file_drag(const char *const *paths, size_t count, unsigned allowed) {
    @autoreleasepool {
        NSEvent *event = nil;
        VivaMetalView *view = viva_drag_view(&event);
        if (view == nil) {
            return -1;
        }
        NSMutableArray *items = [NSMutableArray arrayWithCapacity:count];
        NSRect frame = viva_drag_frame(view, event);
        for (size_t index = 0; index < count; index++) {
            if (paths[index] == NULL) {
                continue;
            }
            NSString *path = [NSString stringWithUTF8String:paths[index]];
            if (path == nil) {
                continue;
            }
            NSPasteboardItem *writer = [[NSPasteboardItem alloc] init];
            [writer setString:[[NSURL fileURLWithPath:path] absoluteString] forType:NSPasteboardTypeFileURL];
            NSDraggingItem *item = [[NSDraggingItem alloc] initWithPasteboardWriter:writer];
            [writer release];
            [item setDraggingFrame:frame contents:viva_drag_image(path, NO)];
            [items addObject:item];
            [item release];
        }
        if (items.count == 0) {
            return -2;
        }
        return [view startFileDragWithItems:items allowed:viva_drag_operation(allowed) drag:0 delegates:nil];
    }
}

int viva_start_file_promise_drag(intptr_t drag, const viva_drag_item *items, size_t count) {
    @autoreleasepool {
        NSEvent *event = nil;
        VivaMetalView *view = viva_drag_view(&event);
        if (view == nil) {
            return -1;
        }
        static char delegateKey;
        VivaFileDragContext *context = [[VivaFileDragContext alloc] initWithDrag:drag];
        NSMutableArray *dragging = [NSMutableArray arrayWithCapacity:count];
        NSMutableArray *delegates = [NSMutableArray arrayWithCapacity:count];
        NSRect frame = viva_drag_frame(view, event);
        for (size_t index = 0; index < count; index++) {
            if (items[index].name == NULL) {
                continue;
            }
            NSString *name = [NSString stringWithUTF8String:items[index].name];
            if (name == nil) {
                continue;
            }
            NSString *type = items[index].directory ? @"public.directory" : @"public.data";
            VivaFilePromiseDelegate *delegate = [[VivaFilePromiseDelegate alloc] initWithName:name item:items[index].item context:context];
            NSFilePromiseProvider *provider = [[NSFilePromiseProvider alloc] initWithFileType:type delegate:delegate];
            // The provider's delegate is not retained by it; keep it exactly as
            // long as the provider, which the target holds while it copies.
            objc_setAssociatedObject(provider, &delegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
            NSDraggingItem *item = [[NSDraggingItem alloc] initWithPasteboardWriter:provider];
            [provider release];
            [item setDraggingFrame:frame contents:viva_drag_image(nil, items[index].directory != 0)];
            [dragging addObject:item];
            [item release];
            [delegates addObject:delegate];
            [delegate release];
        }
        [context release];
        if (dragging.count == 0) {
            return -2;
        }
        return [view startFileDragWithItems:dragging allowed:NSDragOperationCopy drag:drag delegates:delegates];
    }
}

/* A frameless workspace still owns keyboard focus and the application's main
 * window role. A plain borderless NSWindow declines both by default. */
@interface VivaFramelessWindow : NSWindow
@end

@implementation VivaFramelessWindow
- (BOOL)canBecomeKeyWindow {
    return YES;
}
- (BOOL)canBecomeMainWindow {
    return YES;
}
@end

/* VivaPanel is the window of a WindowPanel: it takes keyboard focus although it
 * has no titlebar, and never becomes the application's main window. Its style
 * mask keeps it from bringing the application forward when it appears. */
@interface VivaPanel : NSPanel
@end

@implementation VivaPanel
- (BOOL)canBecomeKeyWindow {
    return YES;
}
- (BOOL)canBecomeMainWindow {
    return NO;
}
@end

/* VivaPreview is the window of a WindowPreview: it is never key or main, and
 * its style mask keeps it from bringing the application forward. */
@interface VivaPreview : NSPanel
@end

@implementation VivaPreview
- (BOOL)canBecomeKeyWindow {
    return NO;
}
- (BOOL)canBecomeMainWindow {
    return NO;
}
@end

@interface VivaAppDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate> {
    NSRect _anchor;
    BOOL _hasAnchor;
    BOOL _hasOrigin;
    NSPoint _origin;
    NSWindow *_window;
    NSString *_title;
    int _width;
    int _height;
    int _minWidth;
    int _minHeight;
    MTLClearColor _clearColor;
    id<MTLDevice> _device;
    id<MTLCommandQueue> _commandQueue;
    id<MTLRenderPipelineState> _rectanglePipeline;
    id<MTLBuffer> _rectangleBuffer;
    NSUInteger _rectangleCount;
    const viva_clip *_rectangleClips;
    id<MTLRenderPipelineState> _textPipeline;
    const viva_text_label *_textLabels;
    NSUInteger _textLabelCount;
    const viva_accessibility_node *_accessibility;
    NSUInteger _accessibilityCount;
    viva_text_input_config _textInput;
    BOOL _dynamicTextInput;
    BOOL _pointerEventsEnabled;
    BOOL _resizeEventsEnabled;
    BOOL _frameEventsEnabled;
    BOOL _minimizeEventsEnabled;
    BOOL _visibilityEventsEnabled;
    BOOL _keyEventsEnabled;
    BOOL _scrollEventsEnabled;
    int _windowMode;
    CGFloat _titlebarHeight;
    CGFloat _trafficLightScale;
    BOOL _placingTrafficLights;
    NSMutableArray<NSView *> *_titlebarWatched; /* views whose frame changes re-center the traffic lights */
    BOOL _fullscreenTarget;
    BOOL _fullscreenTransition;
    BOOL _alwaysOnTop;
    BOOL _closeInterceptEnabled;
    BOOL _fileDropEnabled;
    BOOL _modifierTapsEnabled;
    BOOL _closeConfirmed;
    uintptr_t _callbackHandle;
    BOOL _running;
    int _status;
}

- (void)updateMinimumSize;
- (void)setVivaContentSize:(NSSize)size;

- (instancetype)initWithConfig:(viva_window_config)config
                         device:(id<MTLDevice>)device
                   commandQueue:(id<MTLCommandQueue>)commandQueue
              rectanglePipeline:(id<MTLRenderPipelineState>)rectanglePipeline
                rectangleBuffer:(id<MTLBuffer>)rectangleBuffer
                 rectangleCount:(NSUInteger)rectangleCount
                 rectangleClips:(const viva_clip *)rectangleClips
                     textPipeline:(id<MTLRenderPipelineState>)textPipeline
                      textLabels:(const viva_text_label *)textLabels
                  textLabelCount:(NSUInteger)textLabelCount
                   accessibility:(const viva_accessibility_node *)accessibility
              accessibilityCount:(NSUInteger)accessibilityCount
                       textInput:(viva_text_input_config)textInput
          dynamicTextInputEnabled:(BOOL)dynamicTextInputEnabled
                   pointerEventsEnabled:(BOOL)pointerEventsEnabled
                     resizeEventsEnabled:(BOOL)resizeEventsEnabled
                       keyEventsEnabled:(BOOL)keyEventsEnabled
                scrollEventsEnabled:(BOOL)scrollEventsEnabled
                           windowMode:(int)windowMode
                   closeInterceptEnabled:(BOOL)closeInterceptEnabled
                        callbackHandle:(uintptr_t)callbackHandle;
- (void)setFileDropEnabled:(BOOL)enabled;
- (void)setModifierTapsEnabled:(BOOL)enabled;
- (void)confirmClose;
- (void)hideWindow;
- (void)showWindow;
- (void)setFullscreen:(BOOL)fullscreen;
- (void)setWindowTitle:(NSString *)title;
- (void)setAlwaysOnTop:(BOOL)enabled;
- (BOOL)isFullscreen;
- (void)reopenWindow;
- (BOOL)isPanel;
- (BOOL)isHidden;
- (BOOL)isMiniaturized;
- (void)requestRedraw:(uintptr_t)callbackHandle;
- (BOOL)isRunning;
- (int)nativeStatus;
@end

@implementation VivaAppDelegate

- (instancetype)initWithConfig:(viva_window_config)config
                         device:(id<MTLDevice>)device
                   commandQueue:(id<MTLCommandQueue>)commandQueue
              rectanglePipeline:(id<MTLRenderPipelineState>)rectanglePipeline
                rectangleBuffer:(id<MTLBuffer>)rectangleBuffer
                 rectangleCount:(NSUInteger)rectangleCount
                 rectangleClips:(const viva_clip *)rectangleClips
                     textPipeline:(id<MTLRenderPipelineState>)textPipeline
                        textLabels:(const viva_text_label *)textLabels
                    textLabelCount:(NSUInteger)textLabelCount
                         accessibility:(const viva_accessibility_node *)accessibility
                    accessibilityCount:(NSUInteger)accessibilityCount
                             textInput:(viva_text_input_config)textInput
                dynamicTextInputEnabled:(BOOL)dynamicTextInputEnabled
                   pointerEventsEnabled:(BOOL)pointerEventsEnabled
                     resizeEventsEnabled:(BOOL)resizeEventsEnabled
                       keyEventsEnabled:(BOOL)keyEventsEnabled
                scrollEventsEnabled:(BOOL)scrollEventsEnabled
                           windowMode:(int)windowMode
                 closeInterceptEnabled:(BOOL)closeInterceptEnabled
                            callbackHandle:(uintptr_t)callbackHandle {
    self = [super init];
    if (self != nil) {
        _title = [[NSString alloc] initWithUTF8String:config.title];
        _width = config.width;
        _height = config.height;
        _minWidth = config.min_width;
        _minHeight = config.min_height;
        _clearColor = MTLClearColorMake(
            config.clear_red,
            config.clear_green,
            config.clear_blue,
            config.clear_alpha
        );
        _device = [device retain];
        _commandQueue = [commandQueue retain];
        _rectanglePipeline = [rectanglePipeline retain];
        _rectangleBuffer = [rectangleBuffer retain];
        _rectangleCount = rectangleCount;
        _rectangleClips = rectangleClips;
        _textPipeline = [textPipeline retain];
        _textLabels = textLabels;
        _textLabelCount = textLabelCount;
        _accessibility = accessibility;
        _accessibilityCount = accessibilityCount;
        _textInput = textInput;
        _dynamicTextInput = dynamicTextInputEnabled;
        _pointerEventsEnabled = pointerEventsEnabled;
        _resizeEventsEnabled = resizeEventsEnabled;
        _frameEventsEnabled = config.frame_events_enabled != 0;
        _minimizeEventsEnabled = config.minimize_events_enabled != 0;
        _visibilityEventsEnabled = config.visibility_events_enabled != 0;
        _keyEventsEnabled = keyEventsEnabled;
        _scrollEventsEnabled = scrollEventsEnabled;
        _windowMode = windowMode;
        _titlebarHeight = config.titlebar_height > 0 ? config.titlebar_height : VIVA_DEFAULT_TITLEBAR_HEIGHT;
        _trafficLightScale = config.traffic_light_scale;
        _closeInterceptEnabled = closeInterceptEnabled;
        _callbackHandle = callbackHandle;
        _hasAnchor = config.has_anchor != 0;
        _hasOrigin = config.has_origin != 0;
        _origin = NSMakePoint(config.origin_x, config.origin_y);
        _anchor = NSMakeRect(config.anchor_x, config.anchor_y, config.anchor_width, config.anchor_height);
        _running = YES;
        _status = 0;
    }
    return self;
}

- (void)setFileDropEnabled:(BOOL)enabled {
    _fileDropEnabled = enabled;
}

- (void)setModifierTapsEnabled:(BOOL)enabled {
    _modifierTapsEnabled = enabled;
}

- (void)dealloc {
    [[NSNotificationCenter defaultCenter] removeObserver:self];
    if ([_window.contentView isKindOfClass:[VivaMetalView class]]) {
        [(VivaMetalView *)_window.contentView detachCallbacks];
    }
    for (NSView *view in _titlebarWatched) {
        [[NSNotificationCenter defaultCenter] removeObserver:self name:NSViewFrameDidChangeNotification object:view];
    }
    [_titlebarWatched release];
    [_window release];
    [_title release];
    [_rectangleBuffer release];
    [_rectanglePipeline release];
    [_textPipeline release];
    [_commandQueue release];
    [_device release];
    [super dealloc];
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    (void)notification;

    NSRect frame = NSMakeRect(0, 0, _width, _height);
    NSWindowStyleMask style = NSWindowStyleMaskTitled |
                              NSWindowStyleMaskClosable |
                              NSWindowStyleMaskMiniaturizable |
                              NSWindowStyleMaskResizable;
    if (_windowMode == VIVA_WINDOW_FRAMELESS) {
        style = NSWindowStyleMaskBorderless;
    } else if (_windowMode == VIVA_WINDOW_PANEL || _windowMode == VIVA_WINDOW_PREVIEW || _windowMode == VIVA_WINDOW_NOTIFICATION) {
        style = NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel;
    } else if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
        style |= NSWindowStyleMaskFullSizeContentView;
    }

    Class windowClass = _windowMode == VIVA_WINDOW_PANEL ? [VivaPanel class] : [NSWindow class];
    if (_windowMode == VIVA_WINDOW_PREVIEW || _windowMode == VIVA_WINDOW_NOTIFICATION) {
        windowClass = [VivaPreview class];
    }
    if (_windowMode == VIVA_WINDOW_FRAMELESS) {
        windowClass = [VivaFramelessWindow class];
    }
    _window = [[windowClass alloc] initWithContentRect:frame
                                             styleMask:style
                                               backing:NSBackingStoreBuffered
                                                 defer:NO];
    _window.title = _title;
    _window.delegate = self;
    [[NSNotificationCenter defaultCenter] addObserver:self selector:@selector(screenParametersChanged:)
                                                 name:NSApplicationDidChangeScreenParametersNotification object:nil];
    _window.releasedWhenClosed = NO;
    if (_windowMode != VIVA_WINDOW_PANEL && _windowMode != VIVA_WINDOW_PREVIEW && _windowMode != VIVA_WINDOW_NOTIFICATION) {
        _window.collectionBehavior |= NSWindowCollectionBehaviorFullScreenPrimary;
    }
    if (_windowMode == VIVA_WINDOW_PREVIEW) {
        /* Over every window and every Space, and invisible to the pointer: a
         * picture that follows a drag, which must not take any of its events. */
        _window.minSize = NSMakeSize(1, 1);
        _window.backgroundColor = [NSColor clearColor];
        _window.opaque = NO;
        _window.hasShadow = NO;
        _window.ignoresMouseEvents = YES;
        _window.animationBehavior = NSWindowAnimationBehaviorNone;
        _window.level = NSStatusWindowLevel;
        _window.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces |
                                     NSWindowCollectionBehaviorFullScreenAuxiliary |
                                     NSWindowCollectionBehaviorTransient |
                                     NSWindowCollectionBehaviorIgnoresCycle;
        ((NSPanel *)_window).hidesOnDeactivate = NO;
    } else if (_windowMode == VIVA_WINDOW_PANEL || _windowMode == VIVA_WINDOW_NOTIFICATION) {
        _window.minSize = NSMakeSize(1, 1);
        _window.backgroundColor = [NSColor clearColor];
        _window.opaque = NO;
        _window.hasShadow = YES;
        _window.level = NSPopUpMenuWindowLevel;
        _window.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces |
                                     NSWindowCollectionBehaviorFullScreenAuxiliary |
                                     NSWindowCollectionBehaviorTransient;
        ((NSPanel *)_window).floatingPanel = YES;
        ((NSPanel *)_window).hidesOnDeactivate = NO;
    } else if (_windowMode == VIVA_WINDOW_FRAMELESS) {
        _window.minSize = NSMakeSize(1, 1);
        _window.backgroundColor = [NSColor clearColor];
        _window.opaque = NO;
        _window.hasShadow = YES;
    } else if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
        _window.minSize = NSMakeSize(320, 200);
        if (_clearColor.alpha < 1.0) {
            _window.backgroundColor = [NSColor clearColor];
            _window.opaque = NO;
        }
        _window.titlebarAppearsTransparent = YES;
        _window.titleVisibility = NSWindowTitleHidden;
        _window.movableByWindowBackground = NO;
    } else {
        _window.minSize = NSMakeSize(320, 200);
        if (_clearColor.alpha < 1.0) {
            _window.backgroundColor = [NSColor clearColor];
            _window.opaque = NO;
        }
    }

    VivaMetalView *view = [[VivaMetalView alloc] initWithFrame:frame
                                                    device:_device
                                              commandQueue:_commandQueue
                                         rectanglePipeline:_rectanglePipeline
                                           rectangleBuffer:_rectangleBuffer
                                            rectangleCount:_rectangleCount
                                             rectangleClips:_rectangleClips
                                                textPipeline:_textPipeline
                                                  textLabels:_textLabels
                                              textLabelCount:_textLabelCount
                                                accessibility:_accessibility
                                            accessibilityCount:_accessibilityCount
                                         textInput:_textInput
                            dynamicTextInputEnabled:_dynamicTextInput
                              pointerEventsEnabled:_pointerEventsEnabled
                                resizeEventsEnabled:_resizeEventsEnabled
                                  keyEventsEnabled:_keyEventsEnabled
                                    scrollEventsEnabled:_scrollEventsEnabled
                                      windowMode:_windowMode
                                     callbackHandle:_callbackHandle
                                        clearColor:_clearColor];
    if (view == nil) {
        _status = 7;
        _running = NO;
        return;
    }
    _window.contentView = view;
    if (_fileDropEnabled) {
        [view enableFileDrops];
    }
    [view setModifierTapsEnabled:_modifierTapsEnabled];
    [view setTitlebarHeight:_titlebarHeight];
    if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
        [self keepTrafficLightsCentered];
    }

    if (_windowMode == VIVA_WINDOW_PANEL || _windowMode == VIVA_WINDOW_NOTIFICATION) {
        [self placePanel];
    } else if (_hasOrigin) {
        [self placeAtOrigin];
    } else {
        [_window center];
    }
    [self updateMinimumSize];
    if (_minWidth > 0 || _minHeight > 0) {
        NSScreen *screen = _window.screen ?: [NSScreen mainScreen];
        if (screen != nil) {
            NSRect fitted = _window.frame;
            fitted.size.width = MIN(fitted.size.width, screen.visibleFrame.size.width);
            fitted.size.height = MIN(fitted.size.height, screen.visibleFrame.size.height);
            [_window setFrame:[_window constrainFrameRect:fitted toScreen:screen] display:NO];
        }
    }
    if (_windowMode == VIVA_WINDOW_PREVIEW || _windowMode == VIVA_WINDOW_NOTIFICATION) {
        /* It appears without taking the key or bringing the application forward. */
        [_window orderFrontRegardless];
    } else {
        [_window makeKeyAndOrderFront:nil];
        [_window makeFirstResponder:view];
        if (_windowMode != VIVA_WINDOW_PANEL) {
            // A panel appears without bringing the application forward.
            [NSApp activateIgnoringOtherApps:YES];
        }
        viva_running_window = _window;
        viva_running_view = view;
    }
    [view render];
    if (viva_test_resize_enabled() && _resizeEventsEnabled && _window != nil) {
        NSRect frame = _window.frame;
        [_window setFrame:NSMakeRect(frame.origin.x, frame.origin.y, 640, 420)
                   display:YES];
    }
    if (viva_test_live_resize_enabled() && _resizeEventsEnabled && _window != nil) {
        // Emulate a drag: many small frames inside one live-resize session.
        NSRect frame = _window.frame;
        for (int step = 1; step <= 8; step++) {
            CGFloat width = 1180 + (CGFloat)step * ((1420.0 - 1180.0) / 8.0);
            CGFloat height = 760 + (CGFloat)step * ((920.0 - 760.0) / 8.0);
            [_window setFrame:NSMakeRect(frame.origin.x, frame.origin.y, width, height)
                       display:YES];
        }
        for (int step = 1; step <= 8; step++) {
            CGFloat width = 1420 - (CGFloat)step * ((1420.0 - 980.0) / 8.0);
            CGFloat height = 920 - (CGFloat)step * ((920.0 - 640.0) / 8.0);
            [_window setFrame:NSMakeRect(frame.origin.x, frame.origin.y, width, height)
                       display:YES];
        }
    }
    if (viva_test_text_input_enabled() && _textInput.enabled) {
        [view setMarkedText:@"ni"
              selectedRange:NSMakeRange(2, 0)
            replacementRange:NSMakeRange(NSNotFound, 0)];
        [view setMarkedText:@"你好"
              selectedRange:NSMakeRange(2, 0)
            replacementRange:NSMakeRange(NSNotFound, 0)];
        [view insertText:@"你好 👩‍💻"
            replacementRange:NSMakeRange(NSNotFound, 0)];
        if (viva_test_autoclose_enabled()) {
            [_window close];
        }
    } else if (viva_test_autoclose_enabled()) {
        [_window close];
    }
    NSPoint clickPoint = NSMakePoint(0, 0);
    BOOL hasClickPoint = viva_test_click_point(&clickPoint);
    if (viva_test_scroll_enabled() && _scrollEventsEnabled && view != nil) {
        // A real AppKit scroll event, so the test covers the NSEvent to
        // platform-sign conversion: the wheel moves 16 device pixels down
        // (CGEvent wheel deltas are positive upward), which must arrive as a
        // positive delta that reveals the content below.
        CGEventRef scrollEvent = CGEventCreateScrollWheelEvent2(
            NULL, kCGScrollEventUnitPixel, 2, -16, 0, 0);
        if (scrollEvent != NULL) {
            [(VivaMetalView *)view scrollWheel:[NSEvent eventWithCGEvent:scrollEvent]];
            CFRelease(scrollEvent);
        }
    }
    if (viva_test_close_request_enabled() && _window != nil) {
        [_window close];
        if (!_closeInterceptEnabled || !_running) {
            // Window either closed through or interception stopped the run
            // loop only when the handler confirmed; nothing else to do.
        }
    }
    if (viva_test_key_enabled() && _keyEventsEnabled) {
        NSInteger eventNumber = 1;
        NSEvent *tab = [NSEvent keyEventWithType:NSEventTypeKeyDown
                                         location:NSZeroPoint
                                    modifierFlags:0
                                        timestamp:NSProcessInfo.processInfo.systemUptime
                                     windowNumber:_window.windowNumber
                                          context:nil
                                       characters:@"\t"
                     charactersIgnoringModifiers:@"\t"
                                       isARepeat:NO
                                         keyCode:48];
        [view keyDown:tab];
        if (viva_test_autoclose_enabled()) {
            [_window close];
        }
    } else if ((!hasClickPoint && viva_test_pointer_enabled() && _callbackHandle != 0) ||
               (hasClickPoint && _callbackHandle != 0)) {
        if (!hasClickPoint) {
            clickPoint = NSMakePoint(
                _window.contentView.bounds.size.width / 2.0,
                _window.contentView.bounds.size.height / 2.0
            );
        }
        CGFloat height = _window.contentView.bounds.size.height;
        NSInteger eventNumber = 1;
        NSEvent *down = [NSEvent mouseEventWithType:NSEventTypeLeftMouseDown
                                          location:NSMakePoint(clickPoint.x, height - clickPoint.y)
                                     modifierFlags:0
                                         timestamp:NSProcessInfo.processInfo.systemUptime
                                      windowNumber:_window.windowNumber
                                           context:nil
                                       eventNumber:eventNumber
                                        clickCount:1
                                          pressure:1.0];
        NSEvent *up = [NSEvent mouseEventWithType:NSEventTypeLeftMouseUp
                                          location:NSMakePoint(clickPoint.x, height - clickPoint.y)
                                     modifierFlags:0
                                         timestamp:NSProcessInfo.processInfo.systemUptime
                                      windowNumber:_window.windowNumber
                                           context:nil
                                       eventNumber:eventNumber + 1
                                        clickCount:1
                                          pressure:0.0];
        [view mouseDown:down];
        [view mouseUp:up];
    }
    [view release];
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
    (void)sender;
    return YES;
}

- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    if ([_window.contentView isKindOfClass:[VivaMetalView class]]) {
        [(VivaMetalView *)_window.contentView shutdownTextInput];
    }
    _running = NO;
    NSEvent *wakeEvent = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                            location:NSZeroPoint
                                       modifierFlags:0
                                           timestamp:0
                                        windowNumber:0
                                             context:nil
                                             subtype:0
                                               data1:0
                                               data2:0];
    [sender postEvent:wakeEvent atStart:NO];
    return NSTerminateCancel;
}

- (BOOL)windowShouldClose:(NSWindow *)sender {
    (void)sender;
    if (_closeInterceptEnabled && !_closeConfirmed && _callbackHandle != 0) {
        viva_close_request_count++;
        vivaCloseRequested(_callbackHandle, _window.contentView);
        return NO;
    }
    return YES;
}

- (void)windowWillClose:(NSNotification *)notification {
    (void)notification;
    if ([_window.contentView isKindOfClass:[VivaMetalView class]]) {
        [(VivaMetalView *)_window.contentView detachCallbacks];
    }
    _running = NO;
    if (_callbackHandle != 0) {
        uintptr_t callbackHandle = _callbackHandle;
        _callbackHandle = 0;
        vivaWindowClosed(callbackHandle);
    }
}

/* Centers the traffic lights in the app's titlebar strip and keeps them there.
 * AppKit moves the buttons back every time it lays the titlebar out (a resize,
 * becoming key, a theme change), and reports it as a frame change of the button
 * or the container, so the placement is redone in that very pass and the
 * buttons are never drawn at the system's height. Entering and leaving full
 * screen replaces the container, so the watched views are collected again. */
// Frame notifications may fire synchronously between setting frame and bounds.
// Suppress reentry until the scaled drawing coordinates are fully restored.
- (void)placeTrafficLights {
    if (_placingTrafficLights) {
        return;
    }
    _placingTrafficLights = YES;
    @try {
        viva_place_traffic_lights(_window, _titlebarHeight, _trafficLightScale);
    } @finally {
        _placingTrafficLights = NO;
    }
}

- (void)keepTrafficLightsCentered {
    [self placeTrafficLights];
    if (_titlebarWatched == nil) {
        _titlebarWatched = [[NSMutableArray alloc] init];
    }
    NSNotificationCenter *center = [NSNotificationCenter defaultCenter];
    for (NSView *view in _titlebarWatched) {
        [center removeObserver:self name:NSViewFrameDidChangeNotification object:view];
    }
    [_titlebarWatched removeAllObjects];
    NSView *container = viva_titlebar_container(_window);
    if (container != nil) {
        [_titlebarWatched addObject:container];
    }
    for (int index = 0; index < 3; index++) {
        NSView *button = [_window standardWindowButton:viva_traffic_buttons[index]];
        if (button != nil) {
            [_titlebarWatched addObject:button];
        }
    }
    for (NSView *view in _titlebarWatched) {
        view.postsFrameChangedNotifications = YES;
        [center addObserver:self selector:@selector(titlebarChanged:) name:NSViewFrameDidChangeNotification object:view];
    }
}

- (void)titlebarChanged:(NSNotification *)notification {
    (void)notification;
    [self placeTrafficLights];
}

/* AppKit reports some of the buttons' moves a turn of the run loop late, so the
 * resize itself is also a moment to place them. */
- (void)windowDidResize:(NSNotification *)notification {
    (void)notification;
    if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
        [self placeTrafficLights];
    }
    [self reportFrameChange];
}

/* AppKit uses logical points; the backing scale must not be applied twice.
 * A smaller display's visible content area is the upper bound for the limit. */
- (void)updateMinimumSize {
    if (_window == nil || (_minWidth <= 0 && _minHeight <= 0)) {
        return;
    }
    NSSize minimum = _window.contentMinSize;
    if (_minWidth > 0) minimum.width = _minWidth;
    if (_minHeight > 0) minimum.height = _minHeight;
    NSScreen *screen = _window.screen ?: [NSScreen mainScreen];
    if (screen != nil) {
        NSSize available = [_window contentRectForFrameRect:screen.visibleFrame].size;
        minimum.width = MIN(minimum.width, MAX(1, available.width));
        minimum.height = MIN(minimum.height, MAX(1, available.height));
    }
    if (!NSEqualSizes(_window.contentMinSize, minimum)) {
        _window.contentMinSize = minimum;
    }
}

- (void)windowDidChangeScreen:(NSNotification *)notification {
    (void)notification;
    [self updateMinimumSize];
}

- (void)windowDidChangeScreenProfile:(NSNotification *)notification {
    (void)notification;
    [self updateMinimumSize];
}

/* A resolution, arrangement, Dock or menu-bar change does not change the
 * window's screen, so windowDidChangeScreen does not fire for it. */
- (void)screenParametersChanged:(NSNotification *)notification {
    (void)notification;
    [self updateMinimumSize];
}

/* Reports the content rectangle on screen after a move or a resize. The frame is
 * the same one contentFrame: gives Frame: top-left origin, y down, logical
 * pixels. A live drag reports every intermediate position. */
- (void)reportFrameChange {
    if (!_frameEventsEnabled || !_running || _callbackHandle == 0 || _window == nil ||
        _fullscreenTransition || [self isFullscreen]) {
        return;
    }
    double frame[4];
    if (![self contentFrame:frame]) {
        return;
    }
    vivaFrameChanged(_callbackHandle, _window.contentView, frame[0], frame[1], frame[2], frame[3]);
}

- (void)windowDidMove:(NSNotification *)notification {
    (void)notification;
    [self reportFrameChange];
}

- (void)windowDidMiniaturize:(NSNotification *)notification {
    (void)notification;
    if (_minimizeEventsEnabled && _running && _callbackHandle != 0) {
        vivaWindowMinimized(_callbackHandle, _window.contentView, 1);
    }
}

- (void)windowDidDeminiaturize:(NSNotification *)notification {
    (void)notification;
    if (_minimizeEventsEnabled && _running && _callbackHandle != 0) {
        vivaWindowMinimized(_callbackHandle, _window.contentView, 0);
    }
}

// windowDidChangeOcclusionState covers what the user can see: a window that is covered, minimized,
// hidden or on a space that is not showing is not visible.
- (void)windowDidChangeOcclusionState:(NSNotification *)notification {
    (void)notification;
    if (_visibilityEventsEnabled && _running && _callbackHandle != 0) {
        vivaWindowOccluded(_callbackHandle, _window.contentView, (_window.occlusionState & NSWindowOcclusionStateVisible) != 0);
    }
}

- (BOOL)isFullscreen {
    return (_window.styleMask & NSWindowStyleMaskFullScreen) != 0;
}

- (void)setWindowTitle:(NSString *)title {
    [_title release];
    _title = [title copy];
    if (_window != nil) {
        _window.title = _title;
        /* A new title lays the titlebar out again, and AppKit sets the zoom
         * button back at the system's height without reporting the move as a
         * frame change, so keepTrafficLightsCentered never hears of it. */
        if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
            viva_place_traffic_lights(_window, _titlebarHeight, _trafficLightScale);
        }
    }
}

- (void)setFullscreen:(BOOL)fullscreen {
    _fullscreenTarget = fullscreen;
    if (!_fullscreenTransition && [self isFullscreen] != fullscreen) {
        _fullscreenTransition = YES;
        [_window toggleFullScreen:nil];
    }
}

- (void)setAlwaysOnTop:(BOOL)enabled {
    _alwaysOnTop = enabled;
    _window.level = enabled ? NSFloatingWindowLevel : NSNormalWindowLevel;
}

- (void)windowWillEnterFullScreen:(NSNotification *)notification {
    (void)notification;
    if ([_window.contentView isKindOfClass:[VivaMetalView class]]) {
        [(VivaMetalView *)_window.contentView cancelTitlebarDrag];
    }
    if (!_fullscreenTransition) {
        _fullscreenTarget = YES;
    }
    _fullscreenTransition = YES;
    if (_running && _callbackHandle != 0) {
        vivaWindowFullscreen(_callbackHandle, _window.contentView, 1);
    }
}

- (void)windowWillExitFullScreen:(NSNotification *)notification {
    (void)notification;
    if (!_fullscreenTransition) {
        _fullscreenTarget = NO;
    }
    _fullscreenTransition = YES;
    if (_running && _callbackHandle != 0) {
        vivaWindowFullscreen(_callbackHandle, _window.contentView, 0);
    }
}

- (void)windowDidEnterFullScreen:(NSNotification *)notification {
    (void)notification;
    _fullscreenTransition = NO;
    [self setFullscreen:_fullscreenTarget];
}

- (void)windowDidExitFullScreen:(NSNotification *)notification {
    (void)notification;
    _fullscreenTransition = NO;
    if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
        [self keepTrafficLightsCentered];
    }
    [self setAlwaysOnTop:_alwaysOnTop];
    [self setFullscreen:_fullscreenTarget];
    [self reportFrameChange];
}

- (void)windowDidFailToEnterFullScreen:(NSWindow *)window {
    (void)window;
    _fullscreenTransition = NO;
    _fullscreenTarget = NO;
    if (_running && _callbackHandle != 0) {
        vivaWindowFullscreen(_callbackHandle, _window.contentView, 0);
    }
}

- (void)windowDidFailToExitFullScreen:(NSWindow *)window {
    (void)window;
    _fullscreenTransition = NO;
    _fullscreenTarget = YES;
    if (_running && _callbackHandle != 0) {
        vivaWindowFullscreen(_callbackHandle, _window.contentView, 1);
    }
}

- (void)windowWillStartLiveResize:(NSNotification *)notification {
    (void)notification;
    if ([_window.contentView isKindOfClass:[VivaMetalView class]]) {
        [(VivaMetalView *)_window.contentView beginLiveResizeRelaxation];
    }
}

- (void)windowDidEndLiveResize:(NSNotification *)notification {
    (void)notification;
    if ([_window.contentView isKindOfClass:[VivaMetalView class]]) {
        [(VivaMetalView *)_window.contentView endLiveResizeRelaxation];
    }
}

- (void)confirmClose {
    if (_window == nil) {
        return;
    }
    _closeConfirmed = YES;
    [_window close];
    _closeConfirmed = NO;
}

/* Whether this window is one the application's ordinary window handling
 * leaves alone: no Dock reopen, no full screen. */
- (BOOL)isPanel {
    return _windowMode == VIVA_WINDOW_PANEL || _windowMode == VIVA_WINDOW_PREVIEW || _windowMode == VIVA_WINDOW_NOTIFICATION;
}

- (BOOL)isHidden {
    return _window != nil && !_window.isVisible && !_window.isMiniaturized;
}

- (BOOL)isMiniaturized {
    return _window != nil && _window.isMiniaturized;
}

- (void)hideWindow {
    [_window orderOut:nil];
}

- (void)showWindow {
    if (_window == nil || !_running) {
        return;
    }
    if (_windowMode == VIVA_WINDOW_NOTIFICATION) {
        [_window orderFrontRegardless];
        return;
    }
    if (_window.isMiniaturized) {
        [_window deminiaturize:nil];
    }
    [_window makeKeyAndOrderFront:nil];
    if (_windowMode != VIVA_WINDOW_PANEL) {
        [NSApp activateIgnoringOtherApps:YES];
    }
}

/* A Dock reopen must let the owner reset its hidden state before showing. */
- (void)reopenWindow {
    if (_window == nil || !_running) {
        return;
    }
    // A handled request queues Window.Show, which clears the Go hidden state and
    // preserves ordering with any Hide already posted by the application.
    if (_callbackHandle == 0 || !vivaWindowReopenRequested(_callbackHandle, _window.contentView)) {
        [self showWindow];
    }
}

/* Puts the content area's top-left at the requested screen point, kept on a
 * screen. */
- (void)placeAtOrigin {
    CGFloat primaryHeight = NSScreen.screens.firstObject.frame.size.height;
    NSRect content = NSMakeRect(_origin.x, primaryHeight - _origin.y - _height, _width, _height);
    NSRect frame = [_window frameRectForContentRect:content];
    if (_windowMode == VIVA_WINDOW_PREVIEW) {
        /* A drag's picture goes wherever the pointer is, even half off a screen. */
        [_window setFrame:frame display:NO];
        return;
    }
    NSScreen *screen = nil;
    if (_windowMode == VIVA_WINDOW_NOTIFICATION) {
        NSPoint topLeft = NSMakePoint(NSMinX(frame), NSMaxY(frame));
        CGFloat nearestDistance = CGFLOAT_MAX;
        for (NSScreen *candidate in NSScreen.screens) {
            NSRect bounds = candidate.frame;
            CGFloat dx = topLeft.x - MIN(MAX(topLeft.x, NSMinX(bounds)), NSMaxX(bounds));
            CGFloat dy = topLeft.y - MIN(MAX(topLeft.y, NSMinY(bounds)), NSMaxY(bounds));
            CGFloat distance = dx * dx + dy * dy;
            if (distance < nearestDistance) {
                nearestDistance = distance;
                screen = candidate;
            }
        }
        if (screen != nil) {
            NSRect area = screen.visibleFrame;
            frame.size.width = MIN(frame.size.width, area.size.width);
            frame.size.height = MIN(frame.size.height, area.size.height);
            frame.origin.x = MIN(MAX(NSMinX(area), frame.origin.x), NSMaxX(area) - frame.size.width);
            frame.origin.y = MIN(MAX(NSMinY(area), frame.origin.y), NSMaxY(area) - frame.size.height);
        }
        [_window setFrame:frame display:NO];
        return;
    }
    NSPoint middle = NSMakePoint(NSMidX(frame), NSMidY(frame));
    for (NSScreen *candidate in NSScreen.screens) {
        if (NSPointInRect(middle, candidate.frame)) {
            screen = candidate;
            break;
        }
    }
    if (screen == nil) {
        screen = NSScreen.mainScreen ?: NSScreen.screens.firstObject;
    }
    [_window setFrame:[_window constrainFrameRect:frame toScreen:screen] display:NO];
}

/* Puts the top-left of the content area at origin, in top-left screen
 * coordinates. */
- (void)moveContentTo:(NSPoint)origin {
    if (_window == nil || !_running) {
        return;
    }
    CGFloat primaryHeight = NSScreen.screens.firstObject.frame.size.height;
    NSRect content = [_window contentRectForFrameRect:_window.frame];
    content.origin = NSMakePoint(origin.x, primaryHeight - origin.y - content.size.height);
    [_window setFrame:[_window frameRectForContentRect:content] display:NO];
}

/* Changes the content area while keeping its current top-left origin. */
- (void)setVivaContentSize:(NSSize)size {
    if (_window == nil || !_running || _window.miniaturized || _window.zoomed ||
        (_window.styleMask & NSWindowStyleMaskFullScreen) != 0) {
        return;
    }
    NSRect current = [_window contentRectForFrameRect:_window.frame];
    NSSize minimum = _window.contentMinSize;
    size.width = MAX(size.width, minimum.width);
    size.height = MAX(size.height, minimum.height);
    NSScreen *screen = _window.screen ?: NSScreen.mainScreen ?: NSScreen.screens.firstObject;
    if (screen != nil) {
        NSRect available = [_window contentRectForFrameRect:screen.visibleFrame];
        size.width = MIN(size.width, available.size.width);
        size.height = MIN(size.height, available.size.height);
    }
    NSRect wanted = NSMakeRect(current.origin.x, NSMaxY(current) - size.height, size.width, size.height);
    NSRect frame = [_window frameRectForContentRect:wanted];
    if (screen != nil) {
        NSRect work = screen.visibleFrame;
        frame.origin.x = MAX(NSMinX(work), MIN(frame.origin.x, NSMaxX(work) - frame.size.width));
        frame.origin.y = MAX(NSMinY(work), MIN(frame.origin.y, NSMaxY(work) - frame.size.height));
    }
    [_window setFrame:frame display:YES];
}

/* The content area in top-left screen coordinates; NO before the window exists. */
- (BOOL)contentFrame:(double *)out {
    if (_window == nil || !_running) {
        return NO;
    }
    NSView *view = _window.contentView;
    NSRect inWindow = [view convertRect:view.bounds toView:nil];
    NSRect screen = [_window convertRectToScreen:inWindow];
    CGFloat primaryHeight = NSScreen.screens.firstObject.frame.size.height;
    out[0] = screen.origin.x;
    out[1] = primaryHeight - screen.origin.y - screen.size.height;
    out[2] = screen.size.width;
    out[3] = screen.size.height;
    return YES;
}

/* Puts the panel beside its anchor: centered under it, since the status area is
 * the menu bar at the top of the screen, and kept inside the visible part of the
 * screen the anchor is on. Without an anchor it is centered on the screen. */
- (void)placePanel {
    if (_windowMode == VIVA_WINDOW_NOTIFICATION) {
        if (_hasOrigin) {
            [self placeAtOrigin];
            return;
        }
        NSPoint point = NSEvent.mouseLocation;
        NSScreen *screen = NSScreen.mainScreen ?: NSScreen.screens.firstObject;
        for (NSScreen *candidate in NSScreen.screens) {
            if (NSPointInRect(point, candidate.frame)) {
                screen = candidate;
                break;
            }
        }
        NSRect area = screen.visibleFrame;
        NSSize size = _window.frame.size;
        [_window setFrameOrigin:NSMakePoint(MAX(NSMinX(area), NSMaxX(area) - size.width - 12),
                                            MIN(NSMaxY(area) - size.height, NSMinY(area) + 12))];
        return;
    }
    if (!_hasAnchor) {
        [_window center];
        return;
    }
    NSPoint middle = NSMakePoint(NSMidX(_anchor), NSMidY(_anchor));
    NSScreen *screen = nil;
    for (NSScreen *candidate in NSScreen.screens) {
        if (NSPointInRect(middle, candidate.frame)) {
            screen = candidate;
            break;
        }
    }
    if (screen == nil) {
        screen = NSScreen.mainScreen ?: NSScreen.screens.firstObject;
    }
    NSRect area = screen.visibleFrame;
    NSSize size = _window.frame.size;
    CGFloat x = NSMidX(_anchor) - size.width / 2.0;
    CGFloat y = NSMinY(_anchor) - VIVA_PANEL_GAP - size.height;
    x = MIN(x, NSMaxX(area) - size.width - VIVA_PANEL_MARGIN);
    x = MAX(x, NSMinX(area) + VIVA_PANEL_MARGIN);
    y = MAX(y, NSMinY(area) + VIVA_PANEL_MARGIN);
    [_window setFrameOrigin:NSMakePoint(x, y)];
}

/* A panel is a transient popup: losing focus, to another window or another
 * application, dismisses it. The close is posted rather than done here, since
 * the window is in the middle of changing key status. */
- (void)windowDidResignKey:(NSNotification *)notification {
    (void)notification;
    if (_windowMode != VIVA_WINDOW_PANEL || !_running || _closeConfirmed) {
        return;
    }
    viva_panel_dismissed_at = NSProcessInfo.processInfo.systemUptime;
    viva_application_close_window(self);
}

- (void)requestRedraw:(uintptr_t)callbackHandle {
    if (callbackHandle == 0 || callbackHandle != _callbackHandle || _window == nil) {
        return;
    }
    vivaRedrawRequested(_callbackHandle, _window.contentView);
}

- (BOOL)isRunning {
    return _running;
}

- (int)nativeStatus {
    return _status;
}

@end

static NSMutableArray<VivaAppDelegate *> *viva_application_windows;
static int viva_application_status;

static void viva_reset_profile_counters(void) {
        viva_gpu_submission_count = 0;
        viva_gpu_draw_call_count = 0;
        viva_rectangle_instance_count = 0;
        viva_gpu_uploaded_bytes = 0;
        viva_gpu_full_rebuild_count = 0;
        viva_gpu_partial_update_count = 0;
        viva_pointer_event_count = 0;
        viva_text_event_count = 0;
        viva_text_texture_rebuild_count = 0;
        viva_label_texture_rebuild_count = 0;
        viva_label_update_count = 0;
        viva_label_initial_count = 0;
        viva_resize_event_count = 0;
        viva_key_event_count = 0;
        viva_accessibility_update_count = 0;
        viva_close_request_count = 0;
        viva_scroll_event_count = 0;
        viva_scroll_last_pixel_y = 0;
}

/* VivaApplicationDelegate answers the Dock icon: opening the application again
 * while its windows are hidden puts them back. */
@interface VivaApplicationDelegate : NSObject <NSApplicationDelegate>
@end

@implementation VivaApplicationDelegate
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    (void)sender;
    if (vivaApplicationQuitRequested()) {
        return NSTerminateCancel;
    }
    viva_application_close_all();
    return NSTerminateCancel;
}

- (BOOL)applicationShouldHandleReopen:(NSApplication *)sender hasVisibleWindows:(BOOL)flag {
    (void)sender;
    (void)flag;
    if (vivaApplicationReopenRequested()) {
        return NO;
    }
    for (VivaAppDelegate *delegate in viva_application_windows) {
        if (![delegate isPanel] && ([delegate isHidden] || [delegate isMiniaturized])) {
            [delegate reopenWindow];
        }
    }
    return NO;
}

/* A link the system opened this application with (a registered URL scheme). */
- (void)application:(NSApplication *)application openURLs:(NSArray<NSURL *> *)urls {
    (void)application;
    for (NSURL *url in urls) {
        const char *text = url.absoluteString.UTF8String;
        if (text != NULL) {
            vivaOpenURL(text);
        }
    }
}

/* Documents the system opened this application with, for the file types the
 * bundle registers. AppKit sends one file to openFile and a batch to openFiles. */
- (BOOL)application:(NSApplication *)application openFile:(NSString *)filename {
    (void)application;
    const char *text = filename.UTF8String;
    if (text == NULL) {
        return NO;
    }
    vivaOpenFile(text);
    return YES;
}

- (void)application:(NSApplication *)application openFiles:(NSArray<NSString *> *)filenames {
    (void)application;
    for (NSString *path in filenames) {
        const char *text = path.UTF8String;
        if (text != NULL) {
            vivaOpenFile(text);
        }
    }
    [NSApp replyToOpenOrPrint:NSApplicationDelegateReplySuccess];
}
@end

static VivaApplicationDelegate *viva_application_delegate;

int viva_application_check_thread(void) {
    return pthread_main_np() ? 0 : VIVA_APPLICATION_NOT_MAIN_THREAD;
}

int viva_application_begin(void) {
    int status = viva_application_check_thread();
    if (status != 0) {
        return status;
    }
    @autoreleasepool {
        if (viva_application_windows != nil) {
            return 8;
        }
        viva_reset_profile_counters();
        viva_application_status = 0;
        viva_application_windows = [[NSMutableArray alloc] init];
        NSApplication *application = [NSApplication sharedApplication];
        application.activationPolicy = NSApplicationActivationPolicyRegular;
        if (application.delegate == nil) {
            viva_application_delegate = [[VivaApplicationDelegate alloc] init];
            application.delegate = viva_application_delegate;
        }
        if (!application.isRunning) {
            [application finishLaunching];
        }
        return 0;
    }
}

int viva_application_last_status(void) {
    return viva_application_status;
}

void *viva_application_open_window(viva_window_config config) {
    @autoreleasepool {
        if (viva_application_windows == nil) {
            viva_application_status = 8;
            return NULL;
        }
        id<MTLDevice> device = nil;
        id<MTLCommandQueue> commandQueue = nil;
        id<MTLRenderPipelineState> rectanglePipeline = nil;
        id<MTLRenderPipelineState> textPipeline = nil;
        id<MTLBuffer> rectangleBuffer = nil;
        int resourceStatus = viva_make_scene_resources(
            &config, &device, &commandQueue, &rectanglePipeline, &textPipeline, &rectangleBuffer);
        if (resourceStatus != 0) {
            viva_application_status = resourceStatus;
            return NULL;
        }

        VivaAppDelegate *delegate = [[VivaAppDelegate alloc] initWithConfig:config
                                                                  device:device
                                                            commandQueue:commandQueue
                                                       rectanglePipeline:rectanglePipeline
                                                         rectangleBuffer:rectangleBuffer
                                                          rectangleCount:config.rectangle_count
                                                          rectangleClips:config.rectangle_clips
                                                              textPipeline:textPipeline
                                                                textLabels:config.text_labels
                                                            textLabelCount:config.text_label_count
                                                                accessibility:config.accessibility
                                                            accessibilityCount:config.accessibility_count
                                                                    textInput:config.text_input
                                                      dynamicTextInputEnabled:config.dynamic_text_input != 0
                                                          pointerEventsEnabled:config.pointer_events_enabled != 0
                                                            resizeEventsEnabled:config.resize_events_enabled != 0
                                                              keyEventsEnabled:config.key_events_enabled != 0
                                                                scrollEventsEnabled:config.scroll_events_enabled != 0
                                                                  windowMode:config.window_mode
                                                        closeInterceptEnabled:config.close_intercept_enabled != 0
                                                                   callbackHandle:config.callback_handle];
        [delegate setFileDropEnabled:config.file_drop_enabled != 0];
        [delegate setModifierTapsEnabled:config.modifier_taps_enabled != 0];
        if (delegate == nil) {
            [rectangleBuffer release];
            [textPipeline release];
            [rectanglePipeline release];
            [commandQueue release];
            [device release];
            viva_application_status = 7;
            return NULL;
        }
        if (viva_application_windows.count == 0) {
            NSString *applicationName = [NSString stringWithUTF8String:config.title];
            viva_install_main_menu(applicationName);
        }
        [delegate applicationDidFinishLaunching:[NSNotification notificationWithName:NSApplicationDidFinishLaunchingNotification object:NSApp]];
        int nativeStatus = [delegate nativeStatus];
        if (nativeStatus != 0 || ![delegate isRunning]) {
            [delegate release];
            [rectangleBuffer release];
            [textPipeline release];
            [rectanglePipeline release];
            [commandQueue release];
            [device release];
            viva_application_status = nativeStatus != 0 ? nativeStatus : 7;
            return NULL;
        }
        [viva_application_windows addObject:delegate];
        void *token = delegate;
        [delegate release];
        [rectangleBuffer release];
        [textPipeline release];
        [rectanglePipeline release];
        [commandQueue release];
        [device release];
        return token;
    }
}

static VivaAppDelegate *viva_find_window(void *token) {
    NSUInteger count = viva_application_windows.count;
    for (NSUInteger index = 0; index < count; index++) {
        VivaAppDelegate *delegate = [viva_application_windows objectAtIndex:index];
        if ((void *)delegate == token) {
            return delegate;
        }
    }
    return nil;
}

@interface VivaAppDelegate (VivaWebHost)
- (NSView *)webHostView;
@end

@implementation VivaAppDelegate (VivaWebHost)
- (NSView *)webHostView {
    return _window.contentView;
}
@end

void *viva_application_window_view(void *token) {
    return [viva_find_window(token) webHostView];
}

static void viva_collect_closed_windows(void) {
    for (NSInteger index = (NSInteger)viva_application_windows.count - 1; index >= 0; index--) {
        VivaAppDelegate *delegate = [viva_application_windows objectAtIndex:(NSUInteger)index];
        if (![delegate isRunning]) {
            int status = [delegate nativeStatus];
            if (viva_application_status == 0 && status != 0) {
                viva_application_status = status;
            }
            [viva_application_windows removeObjectAtIndex:(NSUInteger)index];
        }
    }
}

int viva_application_run(void) {
    @autoreleasepool {
        NSApplication *application = [NSApplication sharedApplication];
        while (viva_application_windows.count > 0) {
            @autoreleasepool {
                NSEvent *event = [application
                    nextEventMatchingMask:NSEventMaskAny
                                  untilDate:[NSDate distantFuture]
                                     inMode:NSDefaultRunLoopMode
                                    dequeue:YES];
                if (event.type == NSEventTypeApplicationDefined && event.subtype == VIVA_REDRAW_EVENT) {
                    NSUInteger count = viva_application_windows.count;
                    for (NSUInteger index = 0; index < count; index++) {
                        [[viva_application_windows objectAtIndex:index] requestRedraw:(uintptr_t)event.data1];
                    }
                } else if (event.type == NSEventTypeApplicationDefined && event.subtype == VIVA_CLOSE_EVENT) {
                    VivaAppDelegate *delegate = viva_find_window((void *)(uintptr_t)event.data1);
                    if (delegate != nil) {
                        [delegate confirmClose];
                    }
                } else if (event.type == NSEventTypeApplicationDefined && event.subtype == VIVA_HIDE_EVENT) {
                    [viva_find_window((void *)(uintptr_t)event.data1) hideWindow];
                } else if (event.type == NSEventTypeApplicationDefined && event.subtype == VIVA_SHOW_EVENT) {
                    [viva_find_window((void *)(uintptr_t)event.data1) showWindow];
                } else if (event.type == NSEventTypeApplicationDefined && event.subtype == VIVA_QUIT_EVENT) {
                    viva_application_close_all();
                } else if (event.type == NSEventTypeApplicationDefined && event.subtype == VIVA_STATUS_REMOVE_EVENT) {
                    if (viva_status_remove_hook != NULL) {
                        viva_status_remove_hook((void *)(uintptr_t)event.data1);
                    }
                } else if (event != nil) {
                    [application sendEvent:event];
                }
                [application updateWindows];
                viva_collect_closed_windows();
            }
        }

        viva_probe_report();
        if (viva_profile_enabled()) {
            fprintf(stderr, "viva_gpu_submissions=%llu\n", viva_gpu_submission_count);
            fprintf(stderr, "viva_gpu_draw_calls=%llu\n", viva_gpu_draw_call_count);
            fprintf(stderr, "viva_rectangle_instances=%llu\n", viva_rectangle_instance_count);
            fprintf(stderr, "viva_gpu_uploaded_bytes=%llu\n", viva_gpu_uploaded_bytes);
            fprintf(stderr, "viva_gpu_full_rebuilds=%llu\n", viva_gpu_full_rebuild_count);
            fprintf(stderr, "viva_gpu_partial_updates=%llu\n", viva_gpu_partial_update_count);
            fprintf(stderr, "viva_pointer_events=%llu\n", viva_pointer_event_count);
            fprintf(stderr, "viva_text_events=%llu\n", viva_text_event_count);
            fprintf(stderr, "viva_text_texture_rebuilds=%llu\n", viva_text_texture_rebuild_count);
            fprintf(stderr, "viva_label_texture_rebuilds=%llu\n", viva_label_texture_rebuild_count);
            fprintf(stderr, "viva_label_updates=%llu\n", viva_label_update_count);
            fprintf(stderr, "viva_label_initial_count=%lu\n", (unsigned long)viva_label_initial_count);
            fprintf(stderr, "viva_resize_events=%llu\n", viva_resize_event_count);
            fprintf(stderr, "viva_key_events=%llu\n", viva_key_event_count);
            fprintf(stderr, "viva_accessibility_updates=%llu\n", viva_accessibility_update_count);
            fprintf(stderr, "viva_close_requests=%llu\n", viva_close_request_count);
            fprintf(stderr, "viva_scroll_events=%llu\n", viva_scroll_event_count);
            fprintf(stderr, "viva_scroll_last_y=%g\n", viva_scroll_last_pixel_y);
            fflush(stderr);
        }

        if (viva_status_clear_hook != NULL) {
            viva_status_clear_hook();
        }
        int nativeStatus = viva_application_status;
        [viva_application_windows release];
        viva_application_windows = nil;
        viva_running_window = nil;
        viva_running_view = nil;
        return nativeStatus;
    }
}

void viva_application_close_all(void) {
    @autoreleasepool {
        NSArray<VivaAppDelegate *> *windows = [[viva_application_windows copy] autorelease];
        for (VivaAppDelegate *delegate in windows) {
            [delegate confirmClose];
        }
        viva_collect_closed_windows();
    }
}

void viva_application_close_window(void *window) {
    if (window == NULL) {
        return;
    }
    @autoreleasepool {
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                            location:NSZeroPoint
                                       modifierFlags:0
                                           timestamp:0
                                        windowNumber:0
                                             context:nil
                                             subtype:VIVA_CLOSE_EVENT
                                               data1:(NSInteger)(uintptr_t)window
                                               data2:0];
        [[NSApplication sharedApplication] postEvent:event atStart:NO];
    }
}

static void viva_post_window_event(int subtype, void *token) {
    if (token == NULL) {
        return;
    }
    @autoreleasepool {
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                            location:NSZeroPoint
                                       modifierFlags:0
                                           timestamp:0
                                        windowNumber:0
                                             context:nil
                                             subtype:subtype
                                               data1:(NSInteger)(uintptr_t)token
                                               data2:0];
        [[NSApplication sharedApplication] postEvent:event atStart:NO];
    }
}

int viva_application_window_frame(void *window, double *out) {
    __block int found = 0;
    void (^work)(void) = ^{
        VivaAppDelegate *delegate = viva_find_window(window);
        found = delegate != nil && [delegate contentFrame:out] ? 1 : 0;
    };
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
    return found;
}

void viva_application_move_window(void *window, double x, double y) {
    void (^work)(void) = ^{
        VivaAppDelegate *delegate = viva_find_window(window);
        [delegate moveContentTo:NSMakePoint(x, y)];
    };
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
}

void viva_application_set_window_size(void *window, double width, double height) {
    dispatch_async(dispatch_get_main_queue(), ^{
        VivaAppDelegate *delegate = viva_find_window(window);
        [delegate setVivaContentSize:NSMakeSize(width, height)];
    });
}

void viva_application_hide_window(void *window) {
    viva_post_window_event(VIVA_HIDE_EVENT, window);
}

int viva_application_set_fullscreen(void *window, int fullscreen) {
    __block int found = 0;
    void (^work)(void) = ^{
        VivaAppDelegate *delegate = viva_find_window(window);
        if (delegate != nil && [delegate isPanel]) {
            found = -1;
        } else if (delegate != nil) {
            found = 1;
            [delegate setFullscreen:fullscreen != 0];
        }
    };
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
    return found;
}

int viva_application_set_title(void *window, const char *title) {
    __block int found = 0;
    NSString *value = [[NSString alloc] initWithUTF8String:title != NULL ? title : ""];
    if (value == nil) {
        value = [@"" retain];
    }
    void (^work)(void) = ^{
        VivaAppDelegate *delegate = viva_find_window(window);
        if (delegate != nil) {
            found = 1;
            [delegate setWindowTitle:value];
        }
    };
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
    [value release];
    return found;
}

int viva_application_fullscreen(void *window) {
    __block int fullscreen = 0;
    void (^work)(void) = ^{
        VivaAppDelegate *delegate = viva_find_window(window);
        fullscreen = delegate != nil && [delegate isFullscreen];
    };
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
    return fullscreen;
}

int viva_application_set_always_on_top(void *window, int enabled) {
    __block int found = 0;
    void (^work)(void) = ^{
        VivaAppDelegate *delegate = viva_find_window(window);
        if (delegate != nil && [delegate isPanel]) {
            found = -1;
        } else if (delegate != nil) {
            [delegate setAlwaysOnTop:enabled != 0];
            found = 1;
        }
    };
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
    return found;
}

void viva_application_show_window(void *window) {
    viva_post_window_event(VIVA_SHOW_EVENT, window);
}

void viva_application_quit(void) {
    @autoreleasepool {
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                            location:NSZeroPoint
                                       modifierFlags:0
                                           timestamp:0
                                        windowNumber:0
                                             context:nil
                                             subtype:VIVA_QUIT_EVENT
                                               data1:0
                                               data2:0];
        [[NSApplication sharedApplication] postEvent:event atStart:NO];
    }
}

void viva_application_post_status_remove(void *item) {
    viva_post_window_event(VIVA_STATUS_REMOVE_EVENT, item);
}

int viva_run(viva_window_config config) {
    int status = viva_application_begin();
    if (status != 0) {
        return status;
    }
    if (viva_application_open_window(config) == NULL) {
        status = viva_application_status;
        [viva_application_windows release];
        viva_application_windows = nil;
        return status;
    }
    return viva_application_run();
}

void viva_request_redraw(uintptr_t callback_handle) {
    if (callback_handle == 0) {
        return;
    }
    @autoreleasepool {
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                            location:NSZeroPoint
                                       modifierFlags:0
                                           timestamp:0
                                        windowNumber:0
                                             context:nil
                                             subtype:VIVA_REDRAW_EVENT
                                               data1:(NSInteger)callback_handle
                                               data2:0];
        [[NSApplication sharedApplication] postEvent:event atStart:NO];
    }
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
        return [(VivaMetalView *)view
            updateRectangles:rectangles
                       clips:clips
                  totalCount:total_count
                       first:first
                       count:count
                     rebuild:rebuild != 0
                      render:render != 0];
    }
}

int viva_view_recover(void *view) {
    if (view == NULL) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaMetalView *)view recoverResources] ? 0 : 1;
    }
}

int viva_view_update_labels(
    void *view,
    const viva_text_label *labels,
    size_t count
) {
    if (view == NULL || (labels == NULL && count > 0)) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaMetalView *)view updateLabels:labels count:count];
    }
}

int viva_view_update_accessibility(
    void *view,
    const viva_accessibility_node *nodes,
    size_t count
) {
    if (view == NULL || (nodes == NULL && count > 0)) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaMetalView *)view updateAccessibility:nodes count:count];
    }
}

void viva_view_render(void *view) {
    if (view == NULL) {
        return;
    }
    @autoreleasepool {
        [(VivaMetalView *)view render];
    }
}

void viva_view_render_sync(void *view) {
    if (view == NULL) {
        return;
    }
    @autoreleasepool {
        [(VivaMetalView *)view renderWaitingForSchedule:YES];
    }
}

void viva_view_close(void *view) {
    if (view == NULL) {
        return;
    }
    @autoreleasepool {
        [[(VivaMetalView *)view window] close];
    }
}

void viva_view_close_confirmed(void *view) {
    if (view == NULL) {
        return;
    }
    @autoreleasepool {
        NSWindow *window = [(VivaMetalView *)view window];
        id<NSWindowDelegate> delegate = window.delegate;
        if ([delegate respondsToSelector:@selector(confirmClose)]) {
            [(VivaAppDelegate *)delegate confirmClose];
        } else {
            [window close];
        }
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
    if (view == NULL) {
        return 1;
    }
    @autoreleasepool {
        int status = [(VivaMetalView *)view
            updateText:utf8
                  length:utf8_length
           selectedRange:NSMakeRange(selected_start, selected_length)
             markedRange:NSMakeRange(marked_start, marked_length)
           hasMarkedText:has_marked_text != 0
       tentativeComposition:tentative_composition != 0
          endsComposition:ends_composition != 0];
        if (status == 0 && render != 0) {
            [(VivaMetalView *)view render];
        }
        return status;
    }
}

int viva_view_update_text_input(void *view, viva_text_input_config text_input) {
    if (view == NULL) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaMetalView *)view updateTextInput:text_input];
    }
}

int viva_view_update_text_input_keeping_composition(void *view, viva_text_input_config text_input) {
    if (view == NULL) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaMetalView *)view updateTextInput:text_input keepComposition:YES];
    }
}

int viva_view_move_text_input(void *view, float x, float y, float width, float height) {
    if (view == NULL) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaMetalView *)view moveTextInputToFrame:NSMakeRect(x, y, width, height)];
    }
}

#ifdef VIVA_OFFSCREEN  /* defined by offscreen.go, which builds only with -tags vivahead */

/* ---- Fonts a headless run carries along --------------------------------
 * A headless run or a test can hand the text stack a font file without
 * installing it in the user's font library: process registration makes the
 * family visible to CoreText, and so to the fallback that picks a family for a
 * character no installed one draws. */

int viva_register_font_file(const char *path) {
    @autoreleasepool {
        if (path == NULL || path[0] == '\0') {
            return 1;
        }
        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        CFErrorRef error = NULL;
        BOOL registered = CTFontManagerRegisterFontsForURL((CFURLRef)url, kCTFontManagerScopeProcess, &error);
        if (error != NULL) {
            CFRelease(error);
        }
        return registered ? 0 : 2;
    }
}

int viva_unregister_font_file(const char *path) {
    @autoreleasepool {
        if (path == NULL || path[0] == '\0') {
            return 1;
        }
        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        CFErrorRef error = NULL;
        BOOL removed = CTFontManagerUnregisterFontsForURL((CFURLRef)url, kCTFontManagerScopeProcess, &error);
        if (error != NULL) {
            CFRelease(error);
        }
        return removed ? 0 : 2;
    }
}

/* ---- Offscreen rendering ------------------------------------------------
 * VivaOffscreen draws a scene with the window's own pipelines, label rasterizer
 * and native-editor rasterizer, into a texture instead of a drawable, with no
 * window, view or run loop. Screenshots from it are the pixels the window
 * shows. */

@interface VivaOffscreen : NSObject {
    id<MTLDevice> _device;
    id<MTLCommandQueue> _queue;
    id<MTLRenderPipelineState> _rectanglePipeline;
    id<MTLRenderPipelineState> _textPipeline;
    NSSize _size;
    CGFloat _scale;
    MTLClearColor _clear;
    id<MTLBuffer> _rectangleBuffer;
    NSUInteger _rectangleCount;
    viva_clip *_rectangleClips;
    NSMutableArray<VivaTextLayer *> *_labels;
    uint16_t *_labelLayers;
    uint32_t *_labelShapesBefore;
    NSUInteger _labelCount;
    viva_composite_command *_commands;
    NSUInteger _commandCount;
    id<MTLTexture> _inputTexture;
    NSRect _inputFrame;
    uint16_t _inputLayer;
}
- (instancetype)initWithLibrary:(const unsigned char *)library
                         length:(size_t)length
                           size:(NSSize)size
                          scale:(CGFloat)scale
                          clear:(MTLClearColor)clear;
- (BOOL)setRectangles:(const viva_rectangle *)rectangles clips:(const viva_clip *)clips count:(NSUInteger)count;
- (BOOL)setLabels:(const viva_text_label *)labels count:(NSUInteger)count;
- (BOOL)setTextInput:(viva_text_input_config)config;
- (size_t)labelTextureBytes;
- (BOOL)renderInto:(unsigned char *)pixels capacity:(size_t)capacity
             width:(size_t *)width height:(size_t *)height;
@end

@implementation VivaOffscreen

- (size_t)labelTextureBytes {
    size_t bytes = 0;
    for (VivaTextLayer *layer in _labels) {
        for (NSUInteger index = 0; index < [layer textureCount]; index++) {
            id<MTLTexture> texture = [layer textureAtIndex:index];
            bytes += texture.width * texture.height * 4;
        }
    }
    return bytes;
}

- (instancetype)initWithLibrary:(const unsigned char *)library
                         length:(size_t)length
                           size:(NSSize)size
                          scale:(CGFloat)scale
                          clear:(MTLClearColor)clear {
    self = [super init];
    if (self == nil) {
        return nil;
    }
    _size = size;
    _scale = MAX(scale, 1.0);
    _clear = clear;
    NSArray<id<MTLDevice>> *devices = MTLCopyAllDevices();
    for (id<MTLDevice> candidate in devices) {
        if (candidate.isLowPower) {
            _device = [candidate retain];
            break;
        }
    }
    [devices release];
    if (_device == nil) {
        _device = MTLCreateSystemDefaultDevice();
    }
    _queue = _device != nil ? [_device newCommandQueue] : nil;
    if (_queue == nil || library == NULL || length == 0) {
        [self release];
        return nil;
    }
    dispatch_data_t data = dispatch_data_create(
        library, length, dispatch_get_global_queue(0, 0), DISPATCH_DATA_DESTRUCTOR_DEFAULT);
    NSError *error = nil;
    id<MTLLibrary> shaders = [_device newLibraryWithData:data error:&error];
    dispatch_release(data);
    if (shaders == nil) {
        fprintf(stderr, "viva: Metal library error: %s\n", error.localizedDescription.UTF8String);
        [self release];
        return nil;
    }
    _rectanglePipeline = viva_create_pipeline(
        _device, shaders, @"viva_rectangle_vertex", @"viva_rectangle_fragment", &error);
    _textPipeline = viva_create_pipeline(
        _device, shaders, @"viva_text_vertex", @"viva_text_fragment", &error);
    [shaders release];
    if (_rectanglePipeline == nil || _textPipeline == nil) {
        fprintf(stderr, "viva: pipeline error: %s\n", error.localizedDescription.UTF8String);
        [self release];
        return nil;
    }
    _labels = [[NSMutableArray alloc] init];
    return self;
}

- (void)dealloc {
    free(_commands);
    free(_rectangleClips);
    free(_labelLayers);
    free(_labelShapesBefore);
    [_inputTexture release];
    [_labels release];
    [_rectangleBuffer release];
    [_textPipeline release];
    [_rectanglePipeline release];
    [_queue release];
    [_device release];
    [super dealloc];
}

- (BOOL)replan {
    free(_commands);
    _commands = NULL;
    _commandCount = 0;
    size_t capacity = _rectangleCount + _labelCount;
    if (capacity == 0) {
        return YES;
    }
    viva_composite_command *replacement = calloc(capacity, sizeof(viva_composite_command));
    if (replacement == NULL) {
        return NO;
    }
    size_t count = 0;
    int status = viva_plan_composite(
        _rectangleBuffer == nil ? NULL : _rectangleBuffer.contents, _rectangleClips, _rectangleCount,
        _labelLayers, _labelShapesBefore, _labelCount, replacement, capacity, &count);
    if (status != 0) {
        free(replacement);
        return NO;
    }
    _commands = replacement;
    _commandCount = count;
    return YES;
}

- (BOOL)setRectangles:(const viva_rectangle *)rectangles clips:(const viva_clip *)clips count:(NSUInteger)count {
    [_rectangleBuffer release];
    _rectangleBuffer = nil;
    free(_rectangleClips);
    _rectangleClips = NULL;
    _rectangleCount = count;
    if (count > 0) {
        _rectangleClips = malloc(count * sizeof(viva_clip));
        if (_rectangleClips == NULL) {
            _rectangleCount = 0;
            return NO;
        }
        memcpy(_rectangleClips, clips, count * sizeof(viva_clip));
        _rectangleBuffer = [_device newBufferWithBytes:rectangles
                                                length:count * sizeof(viva_rectangle)
                                               options:MTLResourceStorageModeShared];
        if (_rectangleBuffer == nil) {
            _rectangleCount = 0;
            return NO;
        }
    }
    return [self replan];
}

- (BOOL)setLabels:(const viva_text_label *)labels count:(NSUInteger)count {
    uint16_t *replacementLayers = NULL;
    uint32_t *replacementShapesBefore = NULL;
    if (count > 0) {
        replacementLayers = malloc(count * sizeof(uint16_t));
        replacementShapesBefore = malloc(count * sizeof(uint32_t));
        if (replacementLayers == NULL || replacementShapesBefore == NULL) {
            free(replacementLayers);
            free(replacementShapesBefore);
            return NO;
        }
    }
    NSMutableArray<VivaTextLayer *> *replacement = nil;
    if (viva_update_label_layers(_labels, labels, count, _device, _scale, _size, &replacement) != 0) {
        free(replacementLayers);
        free(replacementShapesBefore);
        return NO;
    }
    for (NSUInteger index = 0; index < count; index++) {
        replacementLayers[index] = labels[index].layer;
        replacementShapesBefore[index] = labels[index].shapes_before;
    }
    [_labels release];
    _labels = replacement;
    free(_labelLayers);
    free(_labelShapesBefore);
    _labelLayers = replacementLayers;
    _labelShapesBefore = replacementShapesBefore;
    _labelCount = count;
    return [self replan];
}

- (BOOL)setTextInput:(viva_text_input_config)config {
    [_inputTexture release];
    _inputTexture = nil;
    if (!config.enabled) {
        return YES;
    }
    NSString *text = [[[NSString alloc] initWithBytes:config.utf8
                                               length:config.utf8_length
                                             encoding:NSUTF8StringEncoding] autorelease];
    if (text == nil) {
        text = @"";
    }
    // A password field shows one bullet per UTF-16 unit, as the window does.
    NSString *shown = config.secure
        ? [@"" stringByPaddingToLength:text.length withString:@"•" startingAtIndex:0]
        : text;
    NSColor *color = [NSColor colorWithSRGBRed:config.red green:config.green
                                          blue:config.blue alpha:config.alpha];
    _inputFrame = NSMakeRect(config.x, config.y, config.width, config.height);
    _inputLayer = config.layer;
    _inputTexture = viva_text_input_texture(
        _device, _scale, shown, config.font_size, config.font_weight, color, _inputFrame.size,
        NSMakeRange(config.selected_start, config.selected_length),
        config.has_marked_text != 0,
        NSMakeRange(config.marked_start, config.marked_length), YES, config.terminal != 0,
        config.hide_native_overlays != 0, config.mono_face, config.cell_width, config.baseline,
        config.placed || config.terminal ? 0 : config.alignment);
    return _inputTexture != nil;
}

- (BOOL)renderInto:(unsigned char *)pixels capacity:(size_t)capacity
             width:(size_t *)width height:(size_t *)height {
    size_t pixelWidth = (size_t)MAX(1.0, ceil(_size.width * _scale));
    size_t pixelHeight = (size_t)MAX(1.0, ceil(_size.height * _scale));
    *width = pixelWidth;
    *height = pixelHeight;
    if (pixels == NULL || pixelWidth > 16384 || pixelHeight > 16384 ||
        capacity < pixelWidth * pixelHeight * 4) {
        return NO;
    }
    MTLTextureDescriptor *descriptor = [MTLTextureDescriptor
        texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                     width:pixelWidth
                                    height:pixelHeight
                                 mipmapped:NO];
    descriptor.storageMode = MTLStorageModeShared;
    descriptor.usage = MTLTextureUsageRenderTarget;
    id<MTLTexture> target = [_device newTextureWithDescriptor:descriptor];
    if (target == nil) {
        return NO;
    }
    MTLRenderPassDescriptor *pass = [MTLRenderPassDescriptor renderPassDescriptor];
    pass.colorAttachments[0].texture = target;
    pass.colorAttachments[0].loadAction = MTLLoadActionClear;
    pass.colorAttachments[0].storeAction = MTLStoreActionStore;
    pass.colorAttachments[0].clearColor = _clear;
    id<MTLCommandBuffer> commandBuffer = [_queue commandBuffer];
    id<MTLRenderCommandEncoder> encoder = [commandBuffer renderCommandEncoderWithDescriptor:pass];
    // Rounding the backing texture up must add spare pixels, not stretch the
    // requested logical scene independently on x/y. Match the window's scale
    // and the D3D/Vulkan offscreen mapping, including the half-pixel AA fringe.
    NSSize viewport = NSMakeSize((CGFloat)pixelWidth / _scale, (CGFloat)pixelHeight / _scale);
    viva_encode_scene(encoder, _rectanglePipeline, _textPipeline, _rectangleBuffer,
                     _commands, _commandCount, _labels, _inputTexture, _inputFrame, _inputLayer, viewport,
                     pixelWidth, pixelHeight);
    [encoder endEncoding];
    [commandBuffer commit];
    [commandBuffer waitUntilCompleted];
    BOOL ok = commandBuffer.status == MTLCommandBufferStatusCompleted;
    if (ok) {
        [target getBytes:pixels
             bytesPerRow:pixelWidth * 4
              fromRegion:MTLRegionMake2D(0, 0, pixelWidth, pixelHeight)
             mipmapLevel:0];
    }
    [target release];
    return ok;
}

@end

void *viva_offscreen_create(
    const unsigned char *shader_library,
    size_t shader_library_length,
    double width,
    double height,
    double scale,
    double red,
    double green,
    double blue,
    double alpha
) {
    @autoreleasepool {
        return [[VivaOffscreen alloc] initWithLibrary:shader_library
                                              length:shader_library_length
                                                size:NSMakeSize(width, height)
                                               scale:scale
                                               clear:MTLClearColorMake(red, green, blue, alpha)];
    }
}

int viva_offscreen_set_rectangles(void *offscreen, const viva_rectangle *rectangles, const viva_clip *clips, size_t count) {
    @autoreleasepool {
        return [(VivaOffscreen *)offscreen setRectangles:rectangles clips:clips count:count] ? 0 : 1;
    }
}

int viva_offscreen_set_labels(void *offscreen, const viva_text_label *labels, size_t count) {
    @autoreleasepool {
        return [(VivaOffscreen *)offscreen setLabels:labels count:count] ? 0 : 1;
    }
}

size_t viva_offscreen_label_texture_bytes(void *offscreen) {
    return [(VivaOffscreen *)offscreen labelTextureBytes];
}

int viva_offscreen_set_text_input(void *offscreen, viva_text_input_config config) {
    @autoreleasepool {
        return [(VivaOffscreen *)offscreen setTextInput:config] ? 0 : 1;
    }
}

int viva_offscreen_render(
    void *offscreen,
    unsigned char *pixels,
    size_t capacity,
    size_t *width,
    size_t *height
) {
    @autoreleasepool {
        return [(VivaOffscreen *)offscreen renderInto:pixels capacity:capacity
                                               width:width height:height] ? 0 : 1;
    }
}

void viva_offscreen_release(void *offscreen) {
    @autoreleasepool {
        [(VivaOffscreen *)offscreen release];
    }
}

/* ---- In-process input driver ----------------------------------------------
 * Delivers key presses to the running window's view from inside the process,
 * the way AppKit would after the window server handed it the event: no
 * system-level event is created, so nothing can reach another application.
 * Present only in -tags vivahead builds. */
void viva_probe_type(const char *utf8) {
    NSString *characters = [[NSString alloc] initWithUTF8String:utf8 == NULL ? "" : utf8];
    dispatch_async(dispatch_get_main_queue(), ^{
        @autoreleasepool {
            if (viva_running_view != nil && viva_running_window != nil && characters.length > 0) {
                NSEvent *event = [NSEvent keyEventWithType:NSEventTypeKeyDown
                                                  location:NSZeroPoint
                                             modifierFlags:0
                                                 timestamp:NSProcessInfo.processInfo.systemUptime
                                              windowNumber:[(NSWindow *)viva_running_window windowNumber]
                                                   context:nil
                                                characters:characters
                               charactersIgnoringModifiers:characters
                                                 isARepeat:NO
                                                   keyCode:0];
                [(VivaMetalView *)viva_running_view keyDown:event];
            }
            [characters release];
        }
    });
}

/* Keeps the window above every other, so a probe is never measured while another
 * application covers it (a covered window gets no display refreshes, and
 * drawables then take a second each to come back). */
void viva_probe_front(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *window = viva_running_window;
        window.level = NSStatusWindowLevel;
        [window orderFrontRegardless];
    });
}

void viva_probe_close(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [(NSWindow *)viva_running_window close];
    });
}

// viva_reference_text draws value the way a macOS application draws a line of text, in
// one CTLineDraw of the whole string at the left edge, into a bitmap context set up
// like a label's (premultiplied BGRA at `scale`), in white, on the baseline a label
// uses. It is the reference for the sharpness of the label drawing (tools/visualparity).
int viva_reference_text(const unsigned char *utf8, size_t length, double size, double weight, double scale,
                       size_t width, size_t height, unsigned char *pixels) {
    @autoreleasepool {
        memset(pixels, 0, width * height * 4);
        CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
        CGContextRef context = CGBitmapContextCreate(pixels, width, height, 8, width * 4, colorSpace,
                                                     kCGImageAlphaPremultipliedFirst | kCGBitmapByteOrder32Little);
        CGColorSpaceRelease(colorSpace);
        if (context == NULL) {
            return 1;
        }
        CGContextScaleCTM(context, scale, scale);
        CGContextSetTextMatrix(context, CGAffineTransformIdentity);
        CTFontRef font = (CTFontRef)CFRetain([NSFont systemFontOfSize:size weight:weight]);
        CGColorRef white = CGColorCreateSRGB(1, 1, 1, 1);
        NSString *value = [[NSString alloc] initWithBytes:utf8 length:length encoding:NSUTF8StringEncoding];
        const void *keys[] = {kCTFontAttributeName, kCTForegroundColorAttributeName};
        const void *values[] = {font, white};
        CFDictionaryRef attributes = CFDictionaryCreate(
            kCFAllocatorDefault, keys, values, 2, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
        CFAttributedStringRef attributed = CFAttributedStringCreate(kCFAllocatorDefault, (CFStringRef)value, attributes);
        CTLineRef line = CTLineCreateWithAttributedString(attributed);
        CGContextSetTextPosition(context, 0, viva_baseline((CGFloat)height / scale, size, ceil(scale)));
        CTLineDraw(line, context);
        CFRelease(line);
        CFRelease(attributed);
        CFRelease(attributes);
        [value release];
        CGColorRelease(white);
        CFRelease(font);
        CGContextRelease(context);
        return 0;
    }
}

// viva_override_default_integer sets, or with remove clears, an integer user default for this
// process only (the argument domain, as `-AppleFontSmoothing 0` on a command line does),
// so a test can see what a user's setting changes without writing the user's preferences.
void viva_override_default_integer(const char *key, int value, int remove) {
    @autoreleasepool {
        NSUserDefaults *defaults = [NSUserDefaults standardUserDefaults];
        NSMutableDictionary *domain = [[defaults volatileDomainForName:NSArgumentDomain] mutableCopy];
        if (domain == nil) {
            domain = [[NSMutableDictionary alloc] init];
        }
        NSString *name = [NSString stringWithUTF8String:key];
        if (remove) {
            [domain removeObjectForKey:name];
        } else {
            domain[name] = @(value);
        }
        [defaults removeVolatileDomainForName:NSArgumentDomain];
        [defaults setVolatileDomain:domain forName:NSArgumentDomain];
        [domain release];
    }
}

#endif /* VIVA_OFFSCREEN */
