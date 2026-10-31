#import <Cocoa/Cocoa.h>
#include <dlfcn.h>
#import <CoreText/CoreText.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>

#include <mach/mach_time.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "native.h"

static uint64_t viva_process_start_ticks;
static uint64_t viva_gpu_submission_count;
static uint64_t viva_gpu_draw_call_count;
static uint64_t viva_rectangle_instance_count;
static uint64_t viva_gpu_uploaded_bytes;
static uint64_t viva_gpu_full_rebuild_count;
static uint64_t viva_gpu_partial_update_count;
static uint64_t viva_pointer_event_count;
static uint64_t viva_text_event_count;
static uint64_t viva_text_texture_rebuild_count;
static uint64_t viva_label_texture_rebuild_count;
static uint64_t viva_label_update_count;
static uint64_t viva_resize_event_count;
static uint64_t viva_key_event_count;
static uint64_t viva_accessibility_update_count;
static uint64_t viva_close_request_count;
static uint64_t viva_scroll_event_count;
static double viva_scroll_last_pixel_y;
static NSUInteger viva_label_initial_count;

/* Window chrome modes. */
enum {
    VIVA_WINDOW_TITLED = 0,
    VIVA_WINDOW_FRAMELESS = 1,
    VIVA_WINDOW_CUSTOM_TITLEBAR = 2,
    VIVA_WINDOW_PANEL = 3,
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
 * reports a frame change (see keepTrafficLightsCentered). A strip no taller
 * than the system's, and a full screen window, keep the system's layout. */
static void viva_place_traffic_lights(NSWindow *window, CGFloat stripHeight) {
    if (window == nil || stripHeight <= VIVA_DEFAULT_TITLEBAR_HEIGHT ||
        (window.styleMask & NSWindowStyleMaskFullScreen) != 0) {
        return;
    }
    NSView *container = viva_titlebar_container(window);
    if (container == nil || container.superview == nil) {
        return;
    }
    if (fabs(NSHeight(container.frame) - stripHeight) > 0.25) {
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
	VIVA_REDRAW_EVENT = 0x4752,
	VIVA_CLOSE_EVENT = 0x4743,
	VIVA_HIDE_EVENT = 0x4748,
	VIVA_SHOW_EVENT = 0x4753,
	VIVA_QUIT_EVENT = 0x4751,
	VIVA_STATUS_REMOVE_EVENT = 0x4754,
};

static uint16_t viva_rectangle_layer(viva_rectangle rectangle) {
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

int viva_plan_composite(
    const viva_rectangle *rectangles,
    const viva_clip *clips,
    size_t rectangle_count,
    const uint16_t *text_layers,
    size_t text_count,
    viva_composite_command *commands,
    size_t command_capacity,
    size_t *command_count
) {
    if (command_count == NULL ||
        (rectangle_count > 0 && (rectangles == NULL || clips == NULL)) ||
        (text_count > 0 && text_layers == NULL) ||
        rectangle_count > UINT32_MAX || text_count > UINT32_MAX) {
        return 1;
    }

    size_t required = text_count;
    for (size_t index = 0; index < rectangle_count;) {
        size_t head = index;
        index++;
        while (index < rectangle_count &&
               viva_same_batch(rectangles, clips, head, index)) {
            index++;
        }
        required++;
    }
    *command_count = required;
    if (required > command_capacity || (required > 0 && commands == NULL)) {
        return 2;
    }

    size_t command_index = 0;
    for (size_t first = 0; first < rectangle_count;) {
        uint16_t layer = viva_rectangle_layer(rectangles[first]);
        size_t end = first + 1;
        while (end < rectangle_count &&
               viva_same_batch(rectangles, clips, first, end)) {
            end++;
        }
        viva_composite_command command = {
            .layer = layer,
            .kind = VIVA_COMPOSITE_RECTANGLES,
            .first = (uint32_t)first,
            .count = (uint32_t)(end - first),
        };
        if (clips[first].clipped != 0) {
            command.clipped = 1;
            command.clip_x = clips[first].x;
            command.clip_y = clips[first].y;
            command.clip_width = clips[first].width;
            command.clip_height = clips[first].height;
        }
        commands[command_index++] = command;
        first = end;
    }
    for (size_t index = 0; index < text_count; index++) {
        commands[command_index++] = (viva_composite_command){
            .layer = text_layers[index],
            .kind = VIVA_COMPOSITE_TEXT,
            .first = (uint32_t)index,
            .count = 1,
        };
    }
    qsort(commands, required, sizeof(viva_composite_command),
          viva_compare_composite_commands);
    return 0;
}

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
    int modifiers,
    int repeat,
    const unsigned char *utf8,
    size_t utf8Length
);

// phase: 0 over, 1 leave, 2 drop. paths is the dropped paths joined by NUL, with
// length bytes; the result is nonzero when the point takes files.
extern void vivaOpenURL(const char *url);
extern int vivaApplicationQuitRequested(void);
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

extern uint16_t vivaRectangleLayer(
    uintptr_t callbackHandle,
    size_t index
);

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
    int terminal
) {
    // A terminal draws its own cursor and keeps it steady, so an idle
    // terminal never wakes the application.
    return enabled != 0 && terminal == 0 && selected_length == 0 && has_marked_text == 0;
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

int viva_text_input_should_interpret_key(int enabled, int multiline, int key) {
    if (enabled == 0 || key == VIVA_KEY_TAB || key == VIVA_KEY_ESCAPE) {
        return 0;
    }
    return key != VIVA_KEY_ENTER || multiline != 0;
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

static NSRange viva_clamp_range(NSRange range, NSUInteger length) {
    if (range.location == NSNotFound) {
        return NSMakeRange(length, 0);
    }
    NSUInteger start = MIN(range.location, length);
    NSUInteger remaining = length - start;
    return NSMakeRange(start, MIN(range.length, remaining));
}

// A line of proportional text is shaped once by CoreText, in the system font with its font
// fallback, and everything that needs a position asks that line: the width
// layout uses, the glyphs a label draws, the caret, the selection and the character a click
// lands on. The engine in Go (text_engine.go) caches the lines; vivaShapedLine is how the
// drawing here reaches the same cache.
extern viva_text_line *vivaShapedLine(const unsigned char *utf8, size_t length, double size, double weight);

// viva_system_font is the proportional font that measurement, labels and the editor all
// use (a retained CTFont). It is an NSFont, which is a CTFont already.
static CTFontRef viva_system_font(CGFloat size, CGFloat weight) {
    NSFont *font = [NSFont systemFontOfSize:size weight:weight];
    return font != nil ? (CTFontRef)CFRetain(font) : NULL;
}

viva_text_line *viva_text_line_create(const unsigned char *utf8, size_t length, double font_size, double font_weight) {
    @autoreleasepool {
        static const unsigned char empty[1] = {0};
        CFStringRef string = CFStringCreateWithBytes(
            kCFAllocatorDefault, utf8 != NULL ? utf8 : empty, utf8 != NULL ? (CFIndex)length : 0, kCFStringEncodingUTF8, false);
        if (string == NULL) {
            return NULL;
        }
        CTFontRef font = viva_system_font(font_size, font_weight);
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
        CFRelease(font);
        return shaped;
    }
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
static CFIndex viva_cluster_start(CFStringRef string, size_t offset) {
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
static viva_text_line *viva_shared_text_line(NSString *shown, CGFloat fontSize, CGFloat weight) {
    NSData *bytes = [shown dataUsingEncoding:NSUTF8StringEncoding];
    return vivaShapedLine(bytes != nil ? (const unsigned char *)bytes.bytes : NULL, bytes != nil ? bytes.length : 0, fontSize, weight);
}

// viva_draw_text_line draws the shaped line in one call, its origin at x on baseline, in
// color (the line carries none, so the same line serves every color).
static void viva_draw_text_line(CGContextRef context, const viva_text_line *line, CGColorRef color, CGFloat x, CGFloat baseline) {
    CGContextSaveGState(context);
    CGContextSetFillColorWithColor(context, color);
    CGContextSetTextPosition(context, x, baseline);
    CTLineDraw((CTLineRef)line->line, context);
    CGContextRestoreGState(context);
}

// The geometry of the text a native editor shows: the shaped line of proportional text, or
// the fixed-pitch line of a terminal's preedit, whose cells are the grid's own.
typedef struct {
    viva_text_line *shaped;
    CTLineRef mono;
} viva_editor_line;

static void viva_editor_line_free(viva_editor_line line) {
    viva_text_line_release(line.shaped);
    if (line.mono != NULL) {
        CFRelease(line.mono);
    }
}

// viva_text_offset is where the editor draws the boundary before the UTF-16 unit `index`
// of shown, from the left of the text.
static CGFloat viva_text_offset(viva_editor_line line, NSUInteger index, BOOL terminal) {
    if (terminal) {
        return CTLineGetOffsetForStringIndex(line.mono, index, NULL);
    }
    return viva_text_line_caret(line.shaped, index);
}

// viva_text_spans is the extents of range in the editor, left to right.
static size_t viva_text_spans(viva_editor_line line, NSRange range, BOOL terminal, double *spans, size_t capacity) {
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
static NSUInteger viva_text_index_at(viva_editor_line line, NSString *shown, CGFloat x, BOOL terminal) {
    if (!terminal) {
        return viva_text_line_index_at(line.shaped, x);
    }
    CFIndex index = CTLineGetStringIndexForPosition(line.mono, CGPointMake(x, 0));
    if (index == kCFNotFound) {
        return x <= 0 ? 0 : shown.length;
    }
    NSUInteger position = MIN((NSUInteger)index, shown.length);
    if (position < shown.length) {
        NSRange cluster = [shown rangeOfComposedCharacterSequenceAtIndex:position];
        if (position != cluster.location) {
            CGFloat start = CTLineGetOffsetForStringIndex(line.mono, cluster.location, NULL);
            CGFloat end = CTLineGetOffsetForStringIndex(line.mono, NSMaxRange(cluster), NULL);
            position = fabs(x - start) <= fabs(x - end) ? cluster.location : NSMaxRange(cluster);
        }
    }
    return position;
}

// viva_mono_font is the system fixed-pitch font, the one every fixed-pitch label
// and the cell metrics are built from.
//
// A face from 2 up stands for an installed family and the families that draw what it
// lacks; viva_register_mono_family teaches the numbers, in registration order.
#define VIVA_MONO_FAMILY_SLOTS 4098 /* the 2 built-in faces and ui.MaxMonoFaces */
static NSString *viva_mono_families[VIVA_MONO_FAMILY_SLOTS];
static NSArray<NSString *> *viva_mono_fallbacks[VIVA_MONO_FAMILY_SLOTS];
static BOOL viva_mono_classic[VIVA_MONO_FAMILY_SLOTS];
static NSObject *viva_mono_family_lock(void) {
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
static void viva_mono_face_traits(int face, NSString **family, BOOL *classic, NSArray<NSString *> **fallbacks) {
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

char *viva_list_mono_families(void) {
    @autoreleasepool {
        NSFontManager *manager = [NSFontManager sharedFontManager];
        NSMutableArray<NSString *> *names = [NSMutableArray array];
        for (NSString *family in [manager availableFontFamilies]) {
            if ([family hasPrefix:@"."]) {
                continue;
            }
            NSFont *font = [manager fontWithFamily:family traits:0 weight:5 size:12];
            if (font != nil && font.isFixedPitch) {
                [names addObject:family];
            }
        }
        [names sortUsingSelector:@selector(localizedCaseInsensitiveCompare:)];
        return strdup([[names componentsJoinedByString:@"\n"] UTF8String]);
    }
}

// viva_mono_base_font is the primary font of a face, without its fallbacks.
static NSFont *viva_mono_base_font(CGFloat size, CGFloat weight, NSString *family, BOOL classic) {
    if (family != nil) {
        NSFont *named = [[NSFontManager sharedFontManager] fontWithFamily:family
                                                                   traits:(weight >= 0.34 ? NSBoldFontMask : 0)
                                                                   weight:5
                                                                     size:size];
        if (named != nil) {
            return named;
        }
    }
    if (classic) {
        NSFont *classicFont = [NSFont userFixedPitchFontOfSize:size];
        if (classicFont != nil) {
            return classicFont;
        }
    }
    NSFont *font = [NSFont monospacedSystemFontOfSize:size weight:weight];
    return font != nil ? font : [NSFont userFixedPitchFontOfSize:size];
}

// viva_mono_font_primary is the primary font of a face alone, the one the cell is measured in.
static NSFont *viva_mono_font_primary(CGFloat size, CGFloat weight, int face) {
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
static NSFont *viva_mono_font(CGFloat size, CGFloat weight, int face) {
    NSString *family = nil;
    BOOL classic = NO;
    NSArray<NSString *> *fallbacks = nil;
    viva_mono_face_traits(face, &family, &classic, &fallbacks);
    NSFont *base = viva_mono_base_font(size, weight, family, classic);
    if (base == nil || fallbacks.count == 0) {
        return base;
    }
    NSMutableArray<NSFontDescriptor *> *cascade = [NSMutableArray arrayWithCapacity:fallbacks.count];
    for (NSString *name in fallbacks) {
        [cascade addObject:[NSFontDescriptor fontDescriptorWithFontAttributes:@{NSFontFamilyAttribute: name}]];
    }
    NSFontDescriptor *descriptor = [base.fontDescriptor fontDescriptorByAddingAttributes:@{NSFontCascadeListAttribute: cascade}];
    NSFont *stacked = [NSFont fontWithDescriptor:descriptor size:size];
    return stacked != nil ? stacked : base;
}

// viva_font_smoothing_enabled is whether the user leaves font smoothing on. AppKit turns it
// off for its own text when AppleFontSmoothing is 0 (or not a number); a Core Graphics
// bitmap context does not look at the setting, so the text Viva rasterizes into one has to
// (measured against AppKit text: off draws the glyphs 13 to 28 percent lighter,
// the lighter the text color the more, and every other value leaves smoothing on).
static BOOL viva_font_smoothing_enabled(void) {
    id value = [[NSUserDefaults standardUserDefaults] objectForKey:@"AppleFontSmoothing"];
    if (value == nil) {
        return YES;
    }
    // A number or a numeric string, as integerForKey: reads it; anything else is 0.
    return [value respondsToSelector:@selector(integerValue)] && [value integerValue] != 0;
}

// viva_prepare_text_context sets a bitmap context up to draw text the way AppKit draws it.
static void viva_prepare_text_context(CGContextRef context, CGFloat scale) {
    CGContextScaleCTM(context, scale, scale);
    CGContextSetTextMatrix(context, CGAffineTransformIdentity);
    CGContextSetShouldSmoothFonts(context, viva_font_smoothing_enabled());
}

// viva_new_mono_line shapes a terminal's preedit text in the fixed-pitch font at fontSize.
// The native editor and the headless renderer both build it here, so they can only ever
// look the same.
static CTLineRef viva_new_mono_line(NSString *shown, CGFloat fontSize, NSColor *color, int monoFace) {
    // The NSFont itself, not its name looked up again: the name loses the fallback families.
    CTFontRef font = (CTFontRef)CFRetain(viva_mono_font(fontSize, 0, monoFace));
    if (font == NULL) {
        return NULL;
    }
    const void *keys[] = {kCTFontAttributeName, kCTForegroundColorAttributeName};
    const void *values[] = {font, color.CGColor};
    CFDictionaryRef attributes = CFDictionaryCreate(
        kCFAllocatorDefault,
        keys,
        values,
        2,
        &kCFTypeDictionaryKeyCallBacks,
        &kCFTypeDictionaryValueCallBacks
    );
    CFAttributedStringRef attributed = CFAttributedStringCreate(
        kCFAllocatorDefault,
        (CFStringRef)shown,
        attributes
    );
    CTLineRef line = CTLineCreateWithAttributedString(attributed);
    CFRelease(attributed);
    CFRelease(attributes);
    CFRelease(font);
    return line;
}

// viva_new_editor_line is the geometry of the text the editor shows: one shaped line
// shared with layout for proportional text, the fixed-pitch line for a terminal. A line
// whose members are both NULL could not be made.
static viva_editor_line viva_new_editor_line(NSString *shown, CGFloat fontSize, NSColor *color, BOOL terminal, int monoFace) {
    viva_editor_line line = {NULL, NULL};
    if (terminal) {
        line.mono = viva_new_mono_line(shown, fontSize, color, monoFace);
    } else {
        line.shaped = viva_shared_text_line(shown, fontSize, 0);
    }
    return line;
}

// viva_baseline is the height of the baseline above the bottom of a label's texture,
// in points. The baseline is the same row every backend uses (platform.LabelBaseline):
// a fixed distance below the middle of the line box, on a whole physical pixel, so
// the font's own ascent and descent do not move the text.
static CGFloat viva_baseline(CGFloat height, CGFloat fontSize, CGFloat pixelScale) {
    CGFloat rows = MAX(1.0, ceil(height * pixelScale));
    CGFloat fromTop = floor(rows * 0.5 + VIVA_BASELINE_BELOW_CENTER * fontSize * pixelScale + 1e-6);
    return (rows - fromTop) / pixelScale;
}

// viva_text_input_texture rasterizes the native editor's current appearance:
// the shown text (bullets for a password), the selection or composition, and
// the caret when it is in its visible phase. It returns a retained texture, or
// nil when the frame cannot be rasterized.
static id<MTLTexture> viva_text_input_texture(
    id<MTLDevice> device,
    CGFloat scale,
    NSString *shown,
    CGFloat fontSize,
    NSColor *color,
    NSSize frameSize,
    NSRange selectedRange,
    BOOL hasMarked,
    NSRange markedRange,
    BOOL caretVisible,
    BOOL terminal,
    int monoFace
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
    viva_editor_line line = viva_new_editor_line(shown, fontSize, color, terminal, monoFace);
    if (line.shaped == NULL && line.mono == NULL) {
        CGContextRelease(context);
        free(pixels);
        return nil;
    }

    CGFloat ascent = 0;
    CGFloat descent = 0;
    CGFloat baseline = 0;
    if (terminal) {
        // Sit on the same baseline as the terminal's own text, whatever the
        // composed characters' fallback fonts measure.
        // The primary font alone: the fallbacks never take part in the metrics.
        CTFontRef gridFont = CTFontCreateWithName((CFStringRef)viva_mono_font_primary(fontSize, 0, monoFace).fontName, fontSize, NULL);
        ascent = CTFontGetAscent(gridFont);
        descent = CTFontGetDescent(gridFont);
        CFRelease(gridFont);
        CGFloat pixelScale = ceil(scale);
        baseline = ceil(((frameSize.height - ascent - descent) * 0.5 + descent) * pixelScale) / pixelScale;
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
    NSColor *textColor = [color colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
    BOOL invisible = textColor != nil && textColor.alphaComponent <= 0.0;
    double spans[2 * VIVA_SELECTION_PIECES];
    if (selected.length > 0 && !terminal && !invisible) {
        size_t count = viva_text_spans(line, selected, terminal, spans, VIVA_SELECTION_PIECES);
        CGContextSetRGBFillColor(context, 0.18, 0.42, 0.90, 0.55);
        for (size_t piece = 0; piece < count; piece++) {
            CGContextFillRect(
                context,
                CGRectMake(spans[2 * piece], baseline - descent, MAX(1.0, spans[2 * piece + 1]), ascent + descent)
            );
        }
    }

    if (terminal) {
        CGContextSetTextPosition(context, 0, baseline);
        CTLineDraw(line.mono, context);
    } else {
        // The text is drawn as the one line the offsets above and below are read from, so
        // selection, caret and glyphs agree.
        viva_draw_text_line(context, line.shaped, color.CGColor, 0, baseline);
    }

    if (invisible) {
        // nothing more to draw
    } else if (hasMarked) {
        NSRange marked = viva_clamp_range(markedRange, textLength);
        size_t count = viva_text_spans(line, marked, terminal, spans, VIVA_SELECTION_PIECES);
        CGContextSetRGBFillColor(context, 0.55, 0.72, 1.0, 1.0);
        for (size_t piece = 0; piece < count; piece++) {
            CGContextFillRect(context, CGRectMake(spans[2 * piece], baseline - descent - 2.0, MAX(1.0, spans[2 * piece + 1]), 1.0));
        }
    } else if (selected.length == 0 && caretVisible && !terminal) {
        CGFloat caret = viva_text_offset(line, selected.location, terminal);
        NSColor *caretColor = [color colorUsingColorSpace:[NSColorSpace sRGBColorSpace]];
        CGContextSetRGBFillColor(
            context,
            caretColor.redComponent,
            caretColor.greenComponent,
            caretColor.blueComponent,
            MAX(caretColor.alphaComponent, 0.9)
        );
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

// viva_fragment_blends_premultiplied says how a fragment function's output must
// be blended. Rectangles output a straight color and blend with SourceAlpha.
// Text returns texels CoreGraphics already multiplied by coverage, so it blends
// with One; SourceAlpha would multiply by alpha twice and thin every glyph edge
// by roughly a fifth.
int viva_fragment_blends_premultiplied(const char *fragmentName) {
    return fragmentName != NULL && strcmp(fragmentName, "viva_text_fragment") == 0;
}

static id<MTLRenderPipelineState> viva_create_pipeline(
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

static uint64_t viva_hash_bytes(uint64_t hash, const void *data, size_t length) {
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
static float viva_quantized_origin(float value) {
    return roundf(value * 16.0f) / 16.0f;
}

// viva_label_content_hash identifies what a label looks like: its text, runs,
// font and color. Position and layer are not part of it. Labels with equal
// hashes are candidates for sharing one texture; matchesConfig confirms.
static uint64_t viva_label_content_hash(viva_text_label config) {
    uint64_t hash = 14695981039346656037ULL;
    hash = viva_hash_bytes(hash, config.utf8, config.utf8_length);
    float values[] = {config.width, config.height, config.font_size, config.font_weight,
                      config.red, config.green, config.blue, config.alpha, config.cell_width,
                      viva_quantized_origin(config.origin_x), viva_quantized_origin(config.origin_y)};
    hash = viva_hash_bytes(hash, values, sizeof(values));
    hash = viva_hash_bytes(hash, &config.flags, sizeof(config.flags));
    hash = viva_hash_bytes(hash, &config.mono_face, sizeof(config.mono_face));
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

@interface VivaTextLayer : NSObject {
    NSString *_value;
    NSRect _frame;
    NSRect _clip;     // where the label may paint, when _clipped
    BOOL _clipped;
    CGFloat _fontSize;
    CGFloat _fontWeight;
    CGFloat _red;
    CGFloat _green;
    CGFloat _blue;
    CGFloat _alpha;
    CGFloat _lastScale;
    CGFloat _originX;  // where the text starts within the frame, logical pixels
    CGFloat _originY;
    uint64_t _hash;
    BOOL _mono;
    int _monoFace;
    CGFloat _cellWidth;
    NSData *_utf8;       // the text as bytes, to slice into runs; only for run labels
    viva_text_run *_runs; // owned copy; NULL for an ordinary label
    size_t _runCount;
    NSData *_bitmap;
    uint32_t _bitmapWidth;
    uint32_t _bitmapHeight;
    uint32_t _bitmapStride;
    uint64_t _bitmapID;
    uint32_t _bitmapVersion;
    id<MTLTexture> _texture;
}
- (instancetype)initWithConfig:(viva_text_label)config;
- (BOOL)matchesConfig:(viva_text_label)config;
- (uint64_t)contentHash;
- (BOOL)contentMatchesSizeAgnostic:(viva_text_label)config;
- (void)updateFrame:(NSRect)frame;
- (void)updateClipFromConfig:(viva_text_label)config;
- (BOOL)clipped;
- (NSRect)clip;
- (void)forceTextureRebuildWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale;
- (void)markContentRebuild;
- (BOOL)rebuildWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale;
- (void)drawLineInContext:(CGContextRef)context pixelScale:(CGFloat)pixelScale;
- (void)drawRunsInContext:(CGContextRef)context pixelScale:(CGFloat)pixelScale;
- (id<MTLTexture>)texture;
- (NSRect)frame;
@end

@implementation VivaTextLayer

- (instancetype)initWithConfig:(viva_text_label)config {
    self = [super init];
    if (self != nil) {
        _value = [[NSString alloc] initWithBytes:config.utf8
                                          length:config.utf8_length
                                        encoding:NSUTF8StringEncoding];
        if (_value == nil) {
            [self release];
            return nil;
        }
        _frame = NSMakeRect(config.x, config.y, config.width, config.height);
        [self updateClipFromConfig:config];
        _fontSize = config.font_size;
        _fontWeight = config.font_weight;
        _red = config.red;
        _green = config.green;
        _blue = config.blue;
        _alpha = config.alpha;
        _lastScale = 0;
        _originX = viva_quantized_origin(config.origin_x);
        _originY = viva_quantized_origin(config.origin_y);
        _hash = viva_label_content_hash(config);
        _mono = (config.flags & VIVA_LABEL_MONO) != 0;
        _monoFace = config.mono_face;
        _cellWidth = config.cell_width;
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
        if ((config.flags & VIVA_LABEL_BITMAP) != 0 && config.bitmap != NULL) {
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
    if (_fontSize != config.font_size || _fontWeight != config.font_weight) {
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
        _monoFace != config.mono_face || _cellWidth != config.cell_width ||
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
        if (_bitmapID == 0 && memcmp(_bitmap.bytes, config.bitmap, config.bitmap_length) != 0) {
            return NO;
        }
    }
    if (config.utf8_length != [_value lengthOfBytesUsingEncoding:NSUTF8StringEncoding]) {
        return NO;
    }
    return memcmp(config.utf8, [_value UTF8String], config.utf8_length) == 0;
}

- (void)updateClipFromConfig:(viva_text_label)config {
    _clipped = (config.flags & VIVA_LABEL_CLIPPED) != 0;
    _clip = _clipped ? NSMakeRect(config.clip_x, config.clip_y, config.clip_width, config.clip_height) : NSZeroRect;
}

- (BOOL)clipped {
    return _clipped;
}

- (NSRect)clip {
    return _clip;
}

- (void)updateFrame:(NSRect)frame {
    _frame = frame;
}

- (void)forceTextureRebuildWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale {
    _lastScale = 0;
    [self rebuildWithDevice:device scale:scale];
}

- (void)markContentRebuild {
    viva_label_texture_rebuild_count++;
}

- (BOOL)rebuildWithDevice:(id<MTLDevice>)device scale:(CGFloat)scale {
    if (_texture != nil && (_bitmap != nil || _lastScale == scale)) {
        return YES;
    }
    if (_bitmap != nil) {
        if (_bitmapWidth == 0 || _bitmapHeight == 0 ||
            _bitmapStride < (size_t)_bitmapWidth * 4 || _bitmapHeight - 1 > SIZE_MAX / _bitmapStride ||
            _bitmap.length < (size_t)(_bitmapHeight - 1) * _bitmapStride + (size_t)_bitmapWidth * 4) {
            return NO;
        }
        MTLTextureDescriptor *descriptor = [MTLTextureDescriptor
            texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm
                                         width:_bitmapWidth
                                        height:_bitmapHeight
                                     mipmapped:NO];
        descriptor.storageMode = MTLStorageModeShared;
        descriptor.usage = MTLTextureUsageShaderRead;
        id<MTLTexture> texture = [device newTextureWithDescriptor:descriptor];
        if (texture != nil) {
            [texture replaceRegion:MTLRegionMake2D(0, 0, _bitmapWidth, _bitmapHeight)
                       mipmapLevel:0
                         withBytes:_bitmap.bytes
                       bytesPerRow:_bitmapStride];
        }
        if (texture == nil) {
            return NO;
        }
        [_texture release];
        _texture = texture;
        _lastScale = scale;
        return YES;
    }
    // Rasterize on the integer physical-pixel grid: a fractional frame origin
    // must not bleed anti-aliasing across texel boundaries.
    CGFloat pixelScale = ceil(scale);
    size_t pixelWidth = (size_t)MAX(1.0, ceil(_frame.size.width * pixelScale));
    size_t pixelHeight = (size_t)MAX(1.0, ceil(_frame.size.height * pixelScale));
    if (pixelWidth > 16384 || pixelHeight > 16384 || pixelWidth > SIZE_MAX / 4) {
        return NO;
    }
    size_t rowBytes = pixelWidth * 4;
    if (pixelHeight > SIZE_MAX / rowBytes) {
        return NO;
    }
    void *pixels = calloc(pixelHeight, rowBytes);
    if (pixels == NULL) {
        return NO;
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
        return NO;
    }

    // The bitmap is on the ceil(scale) grid and the texture is sampled down to the frame, so
    // the context is scaled to that grid: scaling by the display scale draws the text too small
    // wherever it is fractional.
    viva_prepare_text_context(context, pixelScale);
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
    id<MTLTexture> texture = [device newTextureWithDescriptor:descriptor];
    if (texture != nil) {
        [texture replaceRegion:MTLRegionMake2D(0, 0, pixelWidth, pixelHeight)
                   mipmapLevel:0
                     withBytes:pixels
                   bytesPerRow:rowBytes];
    }

    CGContextRelease(context);
    free(pixels);
    if (texture == nil) {
        return NO;
    }
    [_texture release];
    _texture = texture;
    _lastScale = scale;
    return YES;
}

// drawLineInContext draws an ordinary label: one line in one font and color,
// vertically centered.
- (void)drawLineInContext:(CGContextRef)context pixelScale:(CGFloat)pixelScale {
    CGColorRef foreground = CGColorCreateSRGB(_red, _green, _blue, _alpha);
    CGFloat baseline = viva_baseline(_frame.size.height, _fontSize, pixelScale);
    // Text laid out at a fraction of a pixel below the frame: Core Graphics puts a baseline
    // on the next device row down (it floors in its y-up space, so AppKit does), and a
    // glyph at a fraction of a pixel across (it quantizes the position, in drawing the line).
    baseline -= ceil(_originY * pixelScale - 1e-4) / pixelScale;
    if (_mono) {
        // A fixed-pitch label is not proportional text: it keeps its own advances.
        // The NSFont is a CTFont already. Looking it up again by name fails for the
        // monospaced system font at a weight (its name is private to that instance) and
        // CoreText answers with Times New Roman.
        CTFontRef font = (CTFontRef)CFRetain(viva_mono_font(_fontSize, _fontWeight, _monoFace));
        const void *keys[] = {kCTFontAttributeName, kCTForegroundColorAttributeName};
        const void *values[] = {font, foreground};
        CFDictionaryRef attributes = CFDictionaryCreate(
            kCFAllocatorDefault, keys, values, 2,
            &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
        CFAttributedStringRef attributed = CFAttributedStringCreate(kCFAllocatorDefault, (CFStringRef)_value, attributes);
        CTLineRef line = CTLineCreateWithAttributedString(attributed);
        CGContextSetTextPosition(context, _originX, baseline);
        CTLineDraw(line, context);
        CFRelease(line);
        CFRelease(attributed);
        CFRelease(attributes);
        CFRelease(font);
    } else {
        viva_text_line *shaped = viva_shared_text_line(_value, _fontSize, _fontWeight);
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
    NSFont *baseFont = viva_mono_font(_fontSize, _fontWeight, _monoFace);
    CGFloat baseline = viva_baseline(_frame.size.height, _fontSize, pixelScale);
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
        BOOL italic = (run->flags & VIVA_RUN_ITALIC) != 0;
        CTFontRef font = NULL;
        for (int slot = 0; slot < cacheCount; slot++) {
            if (cachedWeight[slot] == run->weight && cachedItalic[slot] == italic) {
                font = cachedFont[slot];
                break;
            }
        }
        if (font == NULL) {
            NSFont *native = viva_mono_font(_fontSize, run->weight, _monoFace);
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
        BOOL ascii = count <= kMaxGlyphRun;
        for (size_t at = 0; ascii && at < count; at++) {
            ascii = start[at] >= 0x20 && start[at] < 0x7F;
        }
        if ((run->flags & VIVA_RUN_LIGATURES) != 0 && count > 0) {
            // A ligature run is shaped as one line and scaled to its cells, so
            // the font's substitutions survive and the run still ends where
            // the grid says it does.
            NSString *shown = [[NSString alloc] initWithBytes:start
                                                       length:count
                                                     encoding:NSUTF8StringEncoding];
            if (shown != nil) {
                const void *keys[] = {kCTFontAttributeName, kCTForegroundColorAttributeName};
                const void *values[] = {font, foreground};
                CFDictionaryRef attributes = CFDictionaryCreate(
                    kCFAllocatorDefault, keys, values, 2,
                    &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
                CFAttributedStringRef attributed = CFAttributedStringCreate(
                    kCFAllocatorDefault, (CFStringRef)shown, attributes);
                CTLineRef line = CTLineCreateWithAttributedString(attributed);
                if (line != NULL) {
                    CGFloat width = CTLineGetTypographicBounds(line, NULL, NULL, NULL);
                    if (width > 0 && span > 0) {
                        CGContextSaveGState(context);
                        CGContextTranslateCTM(context, cellLeft, baseline);
                        CGContextScaleCTM(context, span / width, 1);
                        CGContextSetTextPosition(context, 0, 0);
                        CTLineDraw(line, context);
                        CGContextRestoreGState(context);
                    }
                    CFRelease(line);
                }
                CFRelease(attributed);
                CFRelease(attributes);
                [shown release];
            }
        } else if (ascii) {
            // Plain ASCII needs no shaping in a fixed-pitch font: map the
            // characters to glyphs and put glyph i exactly on cell i.
            UniChar characters[kMaxGlyphRun];
            CGGlyph glyphs[kMaxGlyphRun];
            CGPoint positions[kMaxGlyphRun];
            for (size_t at = 0; at < count; at++) {
                characters[at] = start[at];
                positions[at] = CGPointMake(cellLeft + (CGFloat)at * _cellWidth, baseline);
            }
            CTFontGetGlyphsForCharacters(font, characters, glyphs, (CFIndex)count);
            CGContextSetFillColorWithColor(context, foreground);
            // Glyph positions are relative to the text position.
            CGContextSetTextPosition(context, 0, 0);
            CTFontDrawGlyphs(font, glyphs, positions, (CFIndex)count, context);
        } else {
            NSString *shown = [[NSString alloc] initWithBytes:start
                                                       length:count
                                                     encoding:NSUTF8StringEncoding];
            if (shown == nil) {
                CFRelease(foreground);
                continue;
            }
            const void *keys[] = {kCTFontAttributeName, kCTForegroundColorAttributeName};
            const void *values[] = {font, foreground};
            CFDictionaryRef attributes = CFDictionaryCreate(
                kCFAllocatorDefault, keys, values, 2,
                &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
            CFAttributedStringRef attributed = CFAttributedStringCreate(
                kCFAllocatorDefault, (CFStringRef)shown, attributes);
            CTLineRef line = CTLineCreateWithAttributedString(attributed);
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
            CFRelease(attributed);
            CFRelease(attributes);
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

- (id<MTLTexture>)texture {
    return _texture;
}

- (NSRect)frame {
    return _frame;
}

@end

// viva_encode_input draws the native editor's texture.
static void viva_encode_input(
    id<MTLRenderCommandEncoder> encoder,
    id<MTLRenderPipelineState> textPipeline,
    id<MTLTexture> inputTexture,
    NSRect inputFrame,
    NSSize viewport
) {
    float viewportSize[2] = {(float)viewport.width, (float)viewport.height};
    float bounds[4] = {
        (float)inputFrame.origin.x,
        (float)inputFrame.origin.y,
        (float)inputFrame.size.width,
        (float)inputFrame.size.height,
    };
    [encoder setRenderPipelineState:textPipeline];
    [encoder setVertexBytes:&bounds length:sizeof(bounds) atIndex:0];
    [encoder setVertexBytes:&viewportSize length:sizeof(viewportSize) atIndex:1];
    [encoder setFragmentTexture:inputTexture atIndex:0];
    [encoder drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:6];
    viva_gpu_draw_call_count++;
}

// viva_encode_scene draws one frame: the rectangle and text-label batches in
// composite order, with the native editor's texture drawn after every batch of
// its own layer, so a higher layer (a popup over the field) covers it. The window and the
// headless renderer share it, so a screenshot is the pixels the window shows.
static void viva_encode_scene(
    id<MTLRenderCommandEncoder> encoder,
    id<MTLRenderPipelineState> rectanglePipeline,
    id<MTLRenderPipelineState> textPipeline,
    id<MTLBuffer> rectangleBuffer,
    const viva_composite_command *commands,
    NSUInteger commandCount,
    NSArray<VivaTextLayer *> *labels,
    id<MTLTexture> inputTexture,
    NSRect inputFrame,
    uint16_t inputLayer,
    NSSize viewport,
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
        NSRect frame = [layer frame];
        BOOL scissored = NO;
        if ([layer clipped]) {
            NSRect clip = [layer clip];
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
        float bounds[4] = {
            (float)frame.origin.x,
            (float)frame.origin.y,
            (float)frame.size.width,
            (float)frame.size.height,
        };
        [encoder setRenderPipelineState:textPipeline];
        [encoder setVertexBytes:&bounds length:sizeof(bounds) atIndex:0];
        [encoder setVertexBytes:&viewportSize length:sizeof(viewportSize) atIndex:1];
        [encoder setFragmentTexture:[layer texture] atIndex:0];
        [encoder drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:6];
        viva_gpu_draw_call_count++;
        if (scissored) {
            [encoder setScissorRect:fullTarget];
        }
    }
    if (!inputDrawn) {
        viva_encode_input(encoder, textPipeline, inputTexture, inputFrame, viewport);
    }
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

@interface VivaMetalView : NSView <NSTextInputClient> {
    CAMetalLayer *_metalLayer;
    id<MTLCommandQueue> _commandQueue;
    id<MTLRenderPipelineState> _rectanglePipeline;
    id<MTLBuffer> _rectangleBuffer;
    NSUInteger _rectangleCount;
    viva_clip *_rectangleClips;
    id<MTLRenderPipelineState> _textPipeline;
    NSMutableArray<VivaTextLayer *> *_textLabels;
    uint16_t *_textLabelLayers;
    NSUInteger _textLabelCount;
    viva_composite_command *_compositeCommands;
    NSUInteger _compositeCommandCount;
    id<MTLTexture> _textTexture;
    NSString *_text;
    NSRect _textFrame;
    uint16_t _textLayer;
    CGFloat _textFontSize;
    NSColor *_textColor;
    NSRange _selectedRange;
    NSRange _markedRange;
    BOOL _hasMarkedText;
    BOOL _textInputEnabled;
    BOOL _textInputMultiline;
    BOOL _textPlaced;
    BOOL _textNavigationKeys;
    NSRect _textCaretFrame;
    BOOL _textSecure;
    BOOL _textTerminal;
    int _textMonoFace;
    BOOL _textAltSendsEscape;
    BOOL _textKeyReleases;
    BOOL _interpretingKey;
    BOOL _interpretedConsumed;
    NSString *_interpretedPlain;
    BOOL _secureEventInputOn;
    BOOL _dynamicTextInput;
    BOOL _caretVisible;
    NSTimer *_caretTimer;
    NSUInteger _textSelectionAnchor;
    BOOL _textSelectionDragging;
    BOOL _pointerEventsEnabled;
    BOOL _resizeEventsEnabled;
    BOOL _frameEventsEnabled;
    BOOL _minimizeEventsEnabled;
    BOOL _keyEventsEnabled;
    BOOL _scrollEventsEnabled;
    BOOL _fileDropEnabled;
    int _windowMode;
    CGFloat _titlebarHeight;
    BOOL _titlebarDragPending;
    NSPoint _titlebarDragOrigin;
    uintptr_t _callbackHandle;
    MTLClearColor _clearColor;
    NSMutableArray<VivaAccessibleElement *> *_accessibilityElements;
    NSSize _lastReportedSize;
    BOOL _liveResizeRelaxed;
    BOOL _reportedSize;
    BOOL _reportedFirstFrame;
    BOOL _profileEnabled;
    NSCursor *_cursor;
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
- (void)render;
- (void)renderWaitingForSchedule:(BOOL)wait;
- (int)updateRectangles:(const viva_rectangle *)rectangles
                  clips:(const viva_clip *)clips
              totalCount:(NSUInteger)totalCount
                   first:(NSUInteger)first
                   count:(NSUInteger)count
                 rebuild:(BOOL)rebuild
                  render:(BOOL)render;
- (int)updateLabels:(const viva_text_label *)labels
              count:(NSUInteger)count;
- (BOOL)replaceCompositePlanForRectangles:(const viva_rectangle *)rectangles
                                    clips:(const viva_clip *)clips
                            rectangleCount:(NSUInteger)rectangleCount
                                 textLayers:(const uint16_t *)textLayers
                                  textCount:(NSUInteger)textCount;
- (int)updateAccessibility:(const viva_accessibility_node *)nodes
                     count:(NSUInteger)count;
- (int)updateText:(const unsigned char *)utf8
            length:(NSUInteger)length
     selectedRange:(NSRange)selectedRange
       markedRange:(NSRange)markedRange
     hasMarkedText:(BOOL)hasMarkedText;
- (int)updateTextInput:(viva_text_input_config)textInput;
- (void)setTitlebarHeight:(CGFloat)height;
- (int)moveTextInputToFrame:(NSRect)frame;
- (void)restartCaretBlink;
- (void)stopCaretBlink;
- (void)shutdownTextInput;
- (void)detachCallbacks;
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
        _commandQueue = [commandQueue retain];
        _rectanglePipeline = [rectanglePipeline retain];
        _rectangleBuffer = [rectangleBuffer retain];
        _rectangleCount = rectangleCount;
        if (rectangleCount > 0) {
            _rectangleClips = malloc(rectangleCount * sizeof(viva_clip));
            if (_rectangleClips == NULL) {
                [self release];
                return nil;
            }
            memcpy(_rectangleClips, rectangleClips, rectangleCount * sizeof(viva_clip));
        }
        _textPipeline = [textPipeline retain];
        _textInputEnabled = textInput.enabled != 0;
        _textInputMultiline = textInput.multiline != 0;
        _textPlaced = textInput.placed != 0;
        _textNavigationKeys = textInput.navigation_keys != 0;
        _textLayer = textInput.layer;
        _textCaretFrame = NSMakeRect(textInput.caret_x, textInput.caret_y, textInput.caret_width, textInput.caret_height);
        _textSecure = textInput.secure != 0;
        _textTerminal = textInput.terminal != 0;
        _textMonoFace = textInput.mono_face;
        _textAltSendsEscape = textInput.alt_sends_escape != 0;
        _textKeyReleases = textInput.key_releases != 0;
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
        _textColor = [[NSColor colorWithSRGBRed:textInput.red
                                          green:textInput.green
                                           blue:textInput.blue
                                          alpha:textInput.alpha] retain];
        _selectedRange = NSMakeRange(textInput.selected_start, textInput.selected_length);
        _markedRange = NSMakeRange(textInput.marked_start, textInput.marked_length);
        _hasMarkedText = textInput.has_marked_text != 0;
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
        _clearColor = clearColor;
        _lastReportedSize = frame.size;
        _reportedSize = YES;
        _profileEnabled = viva_profile_enabled();

        _metalLayer = [[CAMetalLayer alloc] init];
        _metalLayer.device = device;
        _metalLayer.pixelFormat = MTLPixelFormatBGRA8Unorm;
        _metalLayer.framebufferOnly = YES;
        _metalLayer.opaque = windowMode != VIVA_WINDOW_PANEL;
        _metalLayer.maximumDrawableCount = 2;
        _metalLayer.allowsNextDrawableTimeout = YES;

        self.wantsLayer = YES;
        self.layer = _metalLayer;
        if (windowMode == VIVA_WINDOW_PANEL) {
            _metalLayer.cornerRadius = VIVA_PANEL_CORNER_RADIUS;
            _metalLayer.masksToBounds = YES;
        }
        [self updateDrawableSize];
        /* A panel opens while the application is not the active one, and its
         * rows still react to the pointer. */
        NSTrackingAreaOptions activeness = windowMode == VIVA_WINDOW_PANEL ? NSTrackingActiveAlways : NSTrackingActiveInActiveApp;
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
        if (textLabelCount > 0) {
            _textLabelLayers = malloc(textLabelCount * sizeof(uint16_t));
            if (_textLabelLayers == NULL) {
                [self release];
                return nil;
            }
            for (NSUInteger index = 0; index < textLabelCount; index++) {
                _textLabelLayers[index] = textLabels[index].layer;
            }
        }
        _textLabelCount = textLabelCount;
        _textLabels = [[NSMutableArray alloc] initWithCapacity:textLabelCount];
        for (NSUInteger index = 0; index < textLabelCount; index++) {
            VivaTextLayer *layer = [[VivaTextLayer alloc] initWithConfig:textLabels[index]];
            if (layer == nil) {
                [self release];
                return nil;
            }
            [_textLabels addObject:layer];
            [layer release];
        }
        if (![self rebuildTextLabels]) {
            [self release];
            return nil;
        }
        if (![self replaceCompositePlanForRectangles:
                rectangleBuffer == nil ? NULL : rectangleBuffer.contents
                                               clips:_rectangleClips
                                      rectangleCount:rectangleCount
                                           textLayers:_textLabelLayers
                                            textCount:_textLabelCount]) {
            [self release];
            return nil;
        }
        viva_label_initial_count = textLabelCount;
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
    free(_compositeCommands);
    free(_rectangleClips);
    free(_textLabelLayers);
    [_cursor release];
    [_rectangleBuffer release];
    [_rectanglePipeline release];
    [_textTexture release];
    [_textLabels release];
    [_accessibilityElements release];
    [_textPipeline release];
    [_text release];
    [_textColor release];
    [_commandQueue release];
    [_metalLayer release];
    [super dealloc];
}

- (BOOL)replaceCompositePlanForRectangles:(const viva_rectangle *)rectangles
                                    clips:(const viva_clip *)clips
                            rectangleCount:(NSUInteger)rectangleCount
                                 textLayers:(const uint16_t *)textLayers
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

- (BOOL)isOpaque {
    return _windowMode != VIVA_WINDOW_PANEL;
}

- (BOOL)acceptsFirstResponder {
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
    if (_cursor != nil) {
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

- (void)mouseDown:(NSEvent *)event {
    int press = VIVA_PRESS_CLIENT;
    if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR && _callbackHandle != 0) {
        // The application declares where the window moves from (a move region).
        NSPoint local = [self convertPoint:event.locationInWindow fromView:nil];
        press = vivaPressAt(_callbackHandle, local.x, self.bounds.size.height - local.y);
    }
    if (press == VIVA_PRESS_MOVE) {
        if (event.clickCount >= 2) {
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
        [self.inputContext activate];
        if (_textPlaced) {
            // The application puts the caret where the press landed, by rows.
            _textSelectionDragging = NO;
            return;
        }
        NSUInteger index = [self textIndexForEvent:event];
        _textSelectionAnchor = index;
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
            // application is told the pointer left, which cancels its press.
            _titlebarDragPending = NO;
            [self emitPointerExit];
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
    [self emitPointerEvent:VIVA_POINTER_MOVE fromEvent:event];
}

- (void)rightMouseDown:(NSEvent *)event { [self emitPointerEvent:VIVA_POINTER_DOWN fromEvent:event]; }
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
    NSString *characters = event.charactersIgnoringModifiers;
	BOOL control = (event.modifierFlags & NSEventModifierFlagControl) != 0;
	BOOL command = (event.modifierFlags & NSEventModifierFlagCommand) != 0;
	if (_keyEventsEnabled && _callbackHandle != 0 && (control || command) && characters.length > 0) {
		int named = viva_terminal_key_code([characters characterAtIndex:0]);
		if (named == 0) {
			NSData *text = [characters dataUsingEncoding:NSUTF8StringEncoding];
			if (text.length > 0 && vivaShortcutEvent(
					_callbackHandle,
					self,
					viva_modifier_flags(event.modifierFlags),
					event.isARepeat ? 1 : 0,
					text.bytes,
					text.length)) {
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
            // Control+C); named keys carry no text.
            NSData *text = named != 0 || characters.length == 0 ? nil
                : [characters dataUsingEncoding:NSUTF8StringEncoding];
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
            _interpretingKey = YES;
            _interpretedConsumed = NO;
            _interpretedPlain = typed;
            [self interpretKeyEvents:@[event]];
            _interpretingKey = NO;
            _interpretedPlain = nil;
            if (!_interpretedConsumed && !_hasMarkedText) {
                NSData *base = [[characters lowercaseString] dataUsingEncoding:NSUTF8StringEncoding];
                NSData *typedText = [typed dataUsingEncoding:NSUTF8StringEncoding];
                viva_key_event_count++;
                vivaRawKeyEvent(_callbackHandle, self, 0, viva_modifier_flags(event.modifierFlags),
                    event.isARepeat ? 1 : 0, (unsigned int)event.keyCode + 1, base.bytes, base.length,
                    typedText.length > 0 ? typedText.bytes : NULL, typedText.length);
            }
            return;
        }
        [self interpretKeyEvents:@[event]];
        return;
    }
    int key = characters.length == 0 ? 0 : viva_key_code([characters characterAtIndex:0]);
    if (!viva_text_input_row_key_is_raw(_textInputEnabled && _textPlaced, _hasMarkedText, key, command) &&
        !viva_text_input_navigation_key_is_raw(_textInputEnabled && _textNavigationKeys, _hasMarkedText, key) &&
        viva_text_input_should_interpret_key(_textInputEnabled, _textInputMultiline, key)) {
        [self interpretKeyEvents:@[event]];
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
    [super flagsChanged:event];
}

- (BOOL)performKeyEquivalent:(NSEvent *)event {
    NSEventModifierFlags modifiers = event.modifierFlags &
        (NSEventModifierFlagCommand | NSEventModifierFlagControl |
         NSEventModifierFlagOption | NSEventModifierFlagShift);
    NSString *characters = event.charactersIgnoringModifiers;
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
}

- (void)endLiveResizeRelaxation {
    if (!_liveResizeRelaxed) {
        return;
    }
    _liveResizeRelaxed = NO;
    CGFloat scale = MAX(_metalLayer.contentsScale, 1.0);
    for (VivaTextLayer *layer in _textLabels) {
        [layer forceTextureRebuildWithDevice:_metalLayer.device scale:scale];
    }
    viva_label_texture_rebuild_count += _textLabels.count;
    // setNeedsDisplay is not honored without drawRect; render explicitly so
    // the refreshed textures actually reach the screen.
    [self render];
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
    CGFloat scale = MAX(_metalLayer.contentsScale, 1.0);
    for (VivaTextLayer *layer in _textLabels) {
        if (![layer rebuildWithDevice:_metalLayer.device scale:scale]) {
            return NO;
        }
    }
    return YES;
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
    [self syncSecureEventInput];
    // The keys that were down will be released to whichever window has the
    // keyboard now, so the application must let go of them here.
    if ([notification.name isEqualToString:NSWindowDidResignKeyNotification] &&
        notification.object == self.window && _callbackHandle != 0 && _keyEventsEnabled) {
        vivaFocusLost(_callbackHandle, self);
    }
}

- (viva_editor_line)newTextLine {
    return viva_new_editor_line([self displayedText], _textFontSize, _textColor, _textTerminal, _textMonoFace);
}

- (BOOL)rebuildTextTexture {
    if (!_textInputEnabled || _textPipeline == nil) {
        return !_textInputEnabled;
    }
    id<MTLTexture> texture = viva_text_input_texture(
        _metalLayer.device, MAX(_metalLayer.contentsScale, 1.0), [self displayedText],
        _textFontSize, _textColor, _textFrame.size, _selectedRange, _hasMarkedText,
        _markedRange, _caretVisible, _textTerminal, _textMonoFace);
    if (texture == nil) {
        return NO;
    }
    [_textTexture release];
    _textTexture = texture;
    return YES;
}

- (void)caretBlinkTimerFired:(NSTimer *)timer {
    if (timer != _caretTimer || !viva_text_input_caret_should_blink(
            _textInputEnabled, _selectedRange.length, _hasMarkedText, _textTerminal)) {
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
    _caretVisible = YES;
    if (!viva_text_input_caret_should_blink(
            _textInputEnabled, _selectedRange.length, _hasMarkedText, _textTerminal)) {
        return;
    }
    _caretTimer = [[NSTimer timerWithTimeInterval:0.5
                                           target:self
                                         selector:@selector(caretBlinkTimerFired:)
                                         userInfo:nil
                                          repeats:YES] retain];
    [[NSRunLoop mainRunLoop] addTimer:_caretTimer forMode:NSRunLoopCommonModes];
}

- (void)shutdownTextInput {
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

        MTLRenderPassDescriptor *pass = [MTLRenderPassDescriptor renderPassDescriptor];
        pass.colorAttachments[0].texture = drawable.texture;
        pass.colorAttachments[0].loadAction = MTLLoadActionClear;
        pass.colorAttachments[0].storeAction = MTLStoreActionStore;
        pass.colorAttachments[0].clearColor = _clearColor;

        id<MTLCommandBuffer> commandBuffer = [_commandQueue commandBuffer];
        id<MTLRenderCommandEncoder> encoder =
            [commandBuffer renderCommandEncoderWithDescriptor:pass];
        viva_encode_scene(encoder, _rectanglePipeline, _textPipeline, _rectangleBuffer,
                         _compositeCommands, _compositeCommandCount, _textLabels,
                         _textTexture, _textFrame, _textLayer, self.bounds.size,
                         drawable.texture.width, drawable.texture.height);
        [encoder endEncoding];
        [commandBuffer presentDrawable:drawable];
        [commandBuffer commit];
        viva_gpu_submission_count++;
        if (probe) {
            viva_probe_committed();
        }

        if (wait) {
            // Live resize must not return to AppKit before the new frame is
            // scheduled; otherwise the compositor stretches the previous
            // drawable across the resized layer for one or more frames.
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

- (int)updateRectangles:(const viva_rectangle *)rectangles
                  clips:(const viva_clip *)clips
              totalCount:(NSUInteger)totalCount
                   first:(NSUInteger)first
                   count:(NSUInteger)count
                 rebuild:(BOOL)rebuild
                  render:(BOOL)render {
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
            replacement = [_metalLayer.device
                newBufferWithBytes:rectangles
                            length:totalCount * sizeof(viva_rectangle)
                           options:MTLResourceStorageModeShared];
            if (replacement == nil) {
                free(replacementClips);
                return 3;
            }
            if (_callbackHandle != 0) {
                viva_rectangle *destination = replacement.contents;
                for (NSUInteger index = 0; index < totalCount; index++) {
                    destination[index].reserved0 =
                        (float)vivaRectangleLayer(_callbackHandle, index);
                }
                [replacement didModifyRange:NSMakeRange(
                    0, totalCount * sizeof(viva_rectangle))];
            }
        }
        if (![self replaceCompositePlanForRectangles:
                replacement == nil ? NULL : replacement.contents
                                               clips:replacementClips
                                      rectangleCount:totalCount
                                           textLayers:_textLabelLayers
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
        viva_gpu_uploaded_bytes += totalCount * sizeof(viva_rectangle);
        viva_gpu_full_rebuild_count++;
    } else {
        if (count == 0) {
            return 0;
        }
        if (_rectangleBuffer == nil || _rectangleClips == NULL || clips == NULL ||
            totalCount < _rectangleCount ||
            first > totalCount || count > totalCount - first) {
            return 2;
        }
        BOOL grew = totalCount > _rectangleCount;
        if (grew) {
            // The scene appended instances. Both buffers are retained at their
            // exact count, so grow them by copying the old contents and then
            // writing the new tail. Existing instance slots keep their values.
            if (first > _rectangleCount || first + count != totalCount) {
                return 2;
            }
            id<MTLBuffer> replacement = [_metalLayer.device
                newBufferWithLength:totalCount * sizeof(viva_rectangle)
                            options:MTLResourceStorageModeShared];
            if (replacement == nil) {
                return 3;
            }
            viva_clip *replacementClips = malloc(totalCount * sizeof(viva_clip));
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
            _rectangleCount = totalCount;
        }
        memcpy(_rectangleClips + first, clips, count * sizeof(viva_clip));
        memcpy(
            (uint8_t *)_rectangleBuffer.contents + first * sizeof(viva_rectangle),
            rectangles,
            count * sizeof(viva_rectangle)
        );
        if (_callbackHandle != 0) {
            viva_rectangle *destination = _rectangleBuffer.contents;
            for (NSUInteger index = first; index < first + count; index++) {
                destination[index].reserved0 =
                    (float)vivaRectangleLayer(_callbackHandle, index);
            }
        }
        [_rectangleBuffer didModifyRange:NSMakeRange(
            grew ? 0 : first * sizeof(viva_rectangle),
            grew ? totalCount * sizeof(viva_rectangle) : count * sizeof(viva_rectangle)
        )];
        if (![self replaceCompositePlanForRectangles:_rectangleBuffer.contents
                                               clips:_rectangleClips
                                      rectangleCount:_rectangleCount
                                           textLayers:_textLabelLayers
                                            textCount:_textLabelCount]) {
            return 3;
        }
        viva_gpu_uploaded_bytes += count * sizeof(viva_rectangle);
        viva_gpu_partial_update_count++;
    }

    if (render) {
        [self render];
    }
    return 0;
}

- (int)updateLabels:(const viva_text_label *)labels
              count:(NSUInteger)count {
    CGFloat scale = MAX(_metalLayer.contentsScale, 1.0);
    uint16_t *replacementLayers = NULL;
    if (count > 0) {
        replacementLayers = malloc(count * sizeof(uint16_t));
        if (replacementLayers == NULL) {
            return 3;
        }
        for (NSUInteger index = 0; index < count; index++) {
            replacementLayers[index] = labels[index].layer;
        }
    }
    NSMutableArray<VivaTextLayer *> *replacement =
        [[NSMutableArray alloc] initWithCapacity:count];
    NSUInteger oldCount = _textLabels.count;
    // Decide which old texture each new label can take over (see
    // viva_plan_label_reuse), then confirm every pick against the label's exact
    // content: the hash only proposes.
    uint64_t *oldHashes = malloc(MAX(oldCount, 1) * sizeof(uint64_t));
    uint64_t *newHashes = malloc(MAX(count, 1) * sizeof(uint64_t));
    int64_t *source = malloc(MAX(count, 1) * sizeof(int64_t));
    if (oldHashes == NULL || newHashes == NULL || source == NULL) {
        free(oldHashes);
        free(newHashes);
        free(source);
        [replacement release];
        free(replacementLayers);
        return 3;
    }
    for (NSUInteger index = 0; index < oldCount; index++) {
        oldHashes[index] = [_textLabels[index] contentHash];
    }
    for (NSUInteger index = 0; index < count; index++) {
        newHashes[index] = viva_label_content_hash(labels[index]);
    }
    viva_plan_label_reuse(oldHashes, oldCount, newHashes, count, source);
    free(oldHashes);
    free(newHashes);
    for (NSUInteger index = 0; index < count; index++) {
        VivaTextLayer *existing = nil;
        if (source[index] >= 0 && [_textLabels[(NSUInteger)source[index]] matchesConfig:labels[index]]) {
            existing = _textLabels[(NSUInteger)source[index]];
        }
        if (existing != nil) {
            // Reused layers must draw at the new position and size immediately.
            [existing updateFrame:NSMakeRect(
                labels[index].x, labels[index].y,
                labels[index].width, labels[index].height)];
            [existing updateClipFromConfig:labels[index]];
            [replacement addObject:existing];
            continue;
        }
        VivaTextLayer *layer = [[VivaTextLayer alloc] initWithConfig:labels[index]];
        if (layer == nil) {
            free(source);
            [replacement release];
            free(replacementLayers);
            return 2;
        }
        if (![layer rebuildWithDevice:_metalLayer.device scale:scale]) {
            free(source);
            [layer release];
            [replacement release];
            free(replacementLayers);
            return 3;
        }
        [layer markContentRebuild];
        [replacement addObject:layer];
        [layer release];
    }
    free(source);
    if (![self replaceCompositePlanForRectangles:
            _rectangleBuffer == nil ? NULL : _rectangleBuffer.contents
                                           clips:_rectangleClips
                                  rectangleCount:_rectangleCount
                                       textLayers:replacementLayers
                                        textCount:count]) {
        [replacement release];
        free(replacementLayers);
        return 3;
    }
    [_textLabels release];
    _textLabels = replacement;
    free(_textLabelLayers);
    _textLabelLayers = replacementLayers;
    _textLabelCount = count;
    viva_label_update_count++;
    return 0;
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
    _textInputEnabled = NO;
    [self syncSecureEventInput];
    _textSelectionDragging = NO;
    [self stopCaretBlink];
    if (wasEnabled) {
        [self.inputContext discardMarkedText];
        [self.inputContext deactivate];
    }
    _hasMarkedText = NO;

    if (!textInput.enabled) {
        _textSecure = NO;
        _textTerminal = NO;
        _textMonoFace = 0;
        _textAltSendsEscape = NO;
        _textKeyReleases = NO;
        [_text release];
        _text = [@"" retain];
        [_textColor release];
        _textColor = nil;
        [_textTexture release];
        _textTexture = nil;
        _textFrame = NSZeroRect;
        _textFontSize = 0;
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
    [_textColor release];
    _textColor = replacementColor;
    _textFrame = NSMakeRect(textInput.x, textInput.y, textInput.width, textInput.height);
    _textFontSize = textInput.font_size;
    _selectedRange = viva_clamp_range(
        NSMakeRange(textInput.selected_start, textInput.selected_length),
        _text.length
    );
    _markedRange = viva_clamp_range(
        NSMakeRange(textInput.marked_start, textInput.marked_length),
        _text.length
    );
    _hasMarkedText = textInput.has_marked_text != 0;
    _textInputMultiline = textInput.multiline != 0;
    _textPlaced = textInput.placed != 0;
    _textNavigationKeys = textInput.navigation_keys != 0;
    _textLayer = textInput.layer;
    _textCaretFrame = NSMakeRect(textInput.caret_x, textInput.caret_y, textInput.caret_width, textInput.caret_height);
    _textSecure = textInput.secure != 0;
    _textTerminal = textInput.terminal != 0;
    _textMonoFace = textInput.mono_face;
    _textAltSendsEscape = textInput.alt_sends_escape != 0;
    _textKeyReleases = textInput.key_releases != 0;
    _textInputEnabled = YES;
    [self syncSecureEventInput];
    _caretVisible = YES;
    if (![self rebuildTextTexture]) {
        return 3;
    }
    [self.window makeFirstResponder:self];
    [self.inputContext activate];
    [self.inputContext invalidateCharacterCoordinates];
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
     hasMarkedText:(BOOL)hasMarkedText {
    if (!_textInputEnabled || (utf8 == NULL && length > 0)) {
        return 1;
    }
    NSString *replacement = [[NSString alloc] initWithBytes:utf8
                                                        length:length
                                                      encoding:NSUTF8StringEncoding];
    if (replacement == nil) {
        return 2;
    }
    [_text release];
    _text = replacement;
    _selectedRange = viva_clamp_range(selectedRange, _text.length);
    _markedRange = viva_clamp_range(markedRange, _text.length);
    _hasMarkedText = hasMarkedText;
    _caretVisible = YES;
    if (![self rebuildTextTexture]) {
        return 3;
    }
    [self restartCaretBlink];
    return 0;
}

- (void)emitTextKind:(int)kind
                text:(NSString *)text
       selectedRange:(NSRange)selectedRange
     replacementRange:(NSRange)replacementRange
             command:(int)command {
    if (_callbackHandle == 0 || !_textInputEnabled) {
        return;
    }
    if (_interpretingKey) {
        // A key a terminal gets as a raw press: the text system answering with
        // exactly the typed character is the key itself, anything else (a dead
        // key, composition, a commit) belongs to the input method.
        if (kind == VIVA_TEXT_INSERT && !_hasMarkedText && _interpretedPlain != nil && [text isEqualToString:_interpretedPlain]) {
            return;
        }
        _interpretedConsumed = YES;
    }
    NSData *data = [text dataUsingEncoding:NSUTF8StringEncoding];
    int64_t replacementStart = -1;
    int64_t replacementLength = 0;
    if (replacementRange.location != NSNotFound) {
        replacementStart = (int64_t)replacementRange.location;
        replacementLength = (int64_t)replacementRange.length;
    }
    viva_text_event_count++;
    vivaTextInput(
        _callbackHandle,
        self,
        kind,
        data.length > 0 ? data.bytes : NULL,
        data.length,
        (int64_t)selectedRange.location,
        (int64_t)selectedRange.length,
        replacementStart,
        replacementLength,
        command
    );
}

- (void)insertText:(id)string replacementRange:(NSRange)replacementRange {
    NSString *plain = [string isKindOfClass:[NSAttributedString class]]
        ? [(NSAttributedString *)string string]
        : (NSString *)string;
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
    if (line.shaped == NULL && line.mono == NULL) {
        return _selectedRange.location;
    }
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
    NSUInteger start = MIN(_textSelectionAnchor, index);
    NSUInteger end = MAX(_textSelectionAnchor, index);
    [self selectTextRange:NSMakeRange(start, end - start)];
}

- (void)selectTextRange:(NSRange)range {
    range = viva_clamp_range(range, _text.length);
    if (!_textInputEnabled || (NSEqualRanges(range, _selectedRange) && !_hasMarkedText)) {
        return;
    }
    if (_hasMarkedText) {
        [self.inputContext discardMarkedText];
    }
    [self emitTextKind:VIVA_TEXT_SET_SELECTION
                  text:@""
         selectedRange:range
       replacementRange:NSMakeRange(NSNotFound, 0)
               command:0];
}

- (void)selectAll:(id)sender {
    (void)sender;
    _textSelectionAnchor = 0;
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
    if (line.shaped != NULL || line.mono != NULL) {
        offset = [self textOffsetForIndex:safe.location line:line];
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

// copy: puts the selected text on the pasteboard. A password field never
// exports what it holds, and an empty selection leaves the pasteboard alone.
- (void)copy:(id)sender {
    if (!_textInputEnabled || _textSecure) {
        return;
    }
    NSRange selected = viva_clamp_range(_selectedRange, _text.length);
    if (selected.length == 0) {
        return;
    }
    NSPasteboard *pasteboard = viva_pasteboard();
    [pasteboard clearContents];
    [pasteboard setString:[_text substringWithRange:selected] forType:NSPasteboardTypeString];
}

// cut: copies, then removes the selection exactly as Backspace would.
- (void)cut:(id)sender {
    if (!_textInputEnabled || _textSecure) {
        return;
    }
    if (viva_clamp_range(_selectedRange, _text.length).length == 0) {
        return;
    }
    [self copy:sender];
    [self emitTextKind:VIVA_TEXT_COMMAND_EVENT
                  text:@""
         selectedRange:NSMakeRange(0, 0)
       replacementRange:NSMakeRange(NSNotFound, 0)
               command:VIVA_TEXT_DELETE_BACKWARD];
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
    BOOL _keyEventsEnabled;
    BOOL _scrollEventsEnabled;
    int _windowMode;
    CGFloat _titlebarHeight;
    NSMutableArray<NSView *> *_titlebarWatched; /* views whose frame changes re-center the traffic lights */
    BOOL _closeInterceptEnabled;
    BOOL _fileDropEnabled;
    BOOL _closeConfirmed;
    uintptr_t _callbackHandle;
    BOOL _running;
    int _status;
}

- (void)updateMinimumSize;

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
- (void)confirmClose;
- (void)hideWindow;
- (void)showWindow;
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
        _keyEventsEnabled = keyEventsEnabled;
        _scrollEventsEnabled = scrollEventsEnabled;
        _windowMode = windowMode;
        _titlebarHeight = config.titlebar_height > 0 ? config.titlebar_height : VIVA_DEFAULT_TITLEBAR_HEIGHT;
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
    } else if (_windowMode == VIVA_WINDOW_PANEL) {
        style = NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel;
    } else if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
        style |= NSWindowStyleMaskFullSizeContentView;
    }

    Class windowClass = _windowMode == VIVA_WINDOW_PANEL ? [VivaPanel class] : [NSWindow class];
    _window = [[windowClass alloc] initWithContentRect:frame
                                             styleMask:style
                                               backing:NSBackingStoreBuffered
                                                 defer:NO];
    _window.title = _title;
    _window.delegate = self;
    [[NSNotificationCenter defaultCenter] addObserver:self selector:@selector(screenParametersChanged:)
                                                 name:NSApplicationDidChangeScreenParametersNotification object:nil];
    _window.releasedWhenClosed = NO;
    if (_windowMode == VIVA_WINDOW_PANEL) {
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
        _window.titlebarAppearsTransparent = YES;
        _window.titleVisibility = NSWindowTitleHidden;
        _window.movableByWindowBackground = NO;
    } else {
        _window.minSize = NSMakeSize(320, 200);
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
    [view setTitlebarHeight:_titlebarHeight];
    if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
        [self keepTrafficLightsCentered];
    }

    if (_windowMode == VIVA_WINDOW_PANEL) {
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
    [_window makeKeyAndOrderFront:nil];
    [_window makeFirstResponder:view];
    if (_windowMode != VIVA_WINDOW_PANEL) {
        // A panel appears without bringing the application forward.
        [NSApp activateIgnoringOtherApps:YES];
    }
    viva_running_window = _window;
    viva_running_view = view;
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
- (void)keepTrafficLightsCentered {
    viva_place_traffic_lights(_window, _titlebarHeight);
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
    viva_place_traffic_lights(_window, _titlebarHeight);
}

/* AppKit reports some of the buttons' moves a turn of the run loop late, so the
 * resize itself is also a moment to place them. */
- (void)windowDidResize:(NSNotification *)notification {
    (void)notification;
    if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
        viva_place_traffic_lights(_window, _titlebarHeight);
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
    if (!_frameEventsEnabled || !_running || _callbackHandle == 0 || _window == nil) {
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

- (void)windowDidExitFullScreen:(NSNotification *)notification {
    (void)notification;
    if (_windowMode == VIVA_WINDOW_CUSTOM_TITLEBAR) {
        [self keepTrafficLightsCentered];
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

- (BOOL)isPanel {
    return _windowMode == VIVA_WINDOW_PANEL;
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
    NSScreen *screen = nil;
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

static void viva_install_main_menu(NSString *applicationName) {
    NSMenu *menuBar = [[[NSMenu alloc] init] autorelease];
    NSMenuItem *applicationItem = [[[NSMenuItem alloc] init] autorelease];
    [menuBar addItem:applicationItem];
    NSApp.mainMenu = menuBar;

    NSMenu *applicationMenu = [[[NSMenu alloc] init] autorelease];
    NSString *quitTitle = [@"Quit " stringByAppendingString:applicationName];
    NSMenuItem *quitItem = [[[NSMenuItem alloc]
        initWithTitle:quitTitle
               action:@selector(terminate:)
        keyEquivalent:@"q"] autorelease];
    [applicationMenu addItem:quitItem];
    applicationItem.submenu = applicationMenu;
}

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
@end

static VivaApplicationDelegate *viva_application_delegate;

int viva_application_begin(void) {
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
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        if (device == nil) {
            viva_application_status = 1;
            return NULL;
        }

        id<MTLCommandQueue> commandQueue = [device newCommandQueue];
        if (commandQueue == nil) {
            [device release];
            viva_application_status = 2;
            return NULL;
        }

        id<MTLRenderPipelineState> rectanglePipeline = nil;
        id<MTLRenderPipelineState> textPipeline = nil;
        id<MTLBuffer> rectangleBuffer = nil;
        if (config.shader_library != NULL && config.shader_library_length > 0) {
            _Static_assert(sizeof(viva_rectangle) == 80, "rectangle ABI must be 80 bytes");
            _Static_assert(sizeof(viva_text_label) == 160, "text label ABI must be 160 bytes");
            _Static_assert(sizeof(viva_text_run) == 36, "text run ABI must be 36 bytes");
            _Static_assert(sizeof(viva_composite_command) == 32, "composite command ABI must be 32 bytes");
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
                viva_application_status = 3;
                return NULL;
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
                    viva_application_status = 4;
                    return NULL;
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
                    viva_application_status = 6;
                    return NULL;
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
                    viva_application_status = 5;
                    return NULL;
                }
                viva_gpu_uploaded_bytes +=
                    config.rectangle_count * sizeof(viva_rectangle);
                viva_gpu_full_rebuild_count++;
            }
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

void viva_application_hide_window(void *window) {
    viva_post_window_event(VIVA_HIDE_EVENT, window);
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
           hasMarkedText:has_marked_text != 0];
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

int viva_view_move_text_input(void *view, float x, float y, float width, float height) {
    if (view == NULL) {
        return 1;
    }
    @autoreleasepool {
        return [(VivaMetalView *)view moveTextInputToFrame:NSMakeRect(x, y, width, height)];
    }
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
        NSFont *nativeFont = [NSFont systemFontOfSize:font_size weight:font_weight];
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

int viva_measure_mono(double font_size, int mono_face, double *advance, double *height) {
    if (advance == NULL || height == NULL || font_size <= 0) {
        return 1;
    }
    @autoreleasepool {
        // The cell is the primary font's, whatever the face's fallbacks hold.
        NSFont *native = viva_mono_font_primary(font_size, 0, mono_face);
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
        *height = CTFontGetAscent(font) + CTFontGetDescent(font) + CTFontGetLeading(font);
        CFRelease(font);
        return *advance > 0 ? 0 : 4;
    }
}

#ifdef VIVA_OFFSCREEN  /* defined by offscreen.go, which builds only with -tags vivahead */

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
- (BOOL)renderInto:(unsigned char *)pixels capacity:(size_t)capacity
             width:(size_t *)width height:(size_t *)height;
@end

@implementation VivaOffscreen

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
    _device = MTLCreateSystemDefaultDevice();
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
        _labelLayers, _labelCount, replacement, capacity, &count);
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
    [_labels removeAllObjects];
    free(_labelLayers);
    _labelLayers = NULL;
    _labelCount = 0;
    if (count > 0) {
        _labelLayers = malloc(count * sizeof(uint16_t));
        if (_labelLayers == NULL) {
            return NO;
        }
    }
    for (NSUInteger index = 0; index < count; index++) {
        VivaTextLayer *layer = [[VivaTextLayer alloc] initWithConfig:labels[index]];
        if (layer == nil || ![layer rebuildWithDevice:_device scale:_scale]) {
            [layer release];
            return NO;
        }
        [_labels addObject:layer];
        [layer release];
        _labelLayers[index] = labels[index].layer;
    }
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
        _device, _scale, shown, config.font_size, color, _inputFrame.size,
        NSMakeRange(config.selected_start, config.selected_length),
        config.has_marked_text != 0,
        NSMakeRange(config.marked_start, config.marked_length), YES, config.terminal != 0, config.mono_face);
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
