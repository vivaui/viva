#ifndef VIVA_INTERNAL_PLATFORM_DARWIN_NATIVE_H
#define VIVA_INTERNAL_PLATFORM_DARWIN_NATIVE_H

/* How far below the middle of a label's line box its baseline sits, in font sizes.
 * The same constant as platform.BaselineBelowCenter, which every backend places its
 * text by; internal/platform/baseline_test.go keeps the two equal. */
#define VIVA_BASELINE_BELOW_CENTER 0.378

#include <stddef.h>
#include <stdint.h>

typedef struct {
    float x;
    float y;
    float width;
    float height;
    float red;
    float green;
    float blue;
    float alpha;
    float corner_radius;
    float reserved0;
    float reserved1;
    float reserved2;
    // The colors of a triangle's second and third corner. A triangle's first
    // is red..alpha; a flat triangle repeats it here. Zero for a rectangle.
    float shade1_red;
    float shade1_green;
    float shade1_blue;
    float shade1_alpha;
    float shade2_red;
    float shade2_green;
    float shade2_blue;
    float shade2_alpha;
} viva_rectangle;

typedef struct {
    double width;
    double height;
    double ascent;
    double descent;
    double leading;
} viva_text_metrics;

// viva_text_line is one line of text the platform shaped once: the CoreText
// line, the string it was made from (for cluster boundaries) and its metrics. The
// engine's cache, the label drawing and the native editor all hold the same one. It is
// reference counted: the creator owns one reference.
typedef struct viva_text_line {
    void *line;   // CTLineRef
    void *string; // CFStringRef
    double width;
    double ascent;
    double descent;
    int references;
} viva_text_line;

typedef struct {
    const unsigned char *utf8;
    size_t utf8_length;
    float x;
    float y;
    float width;
    float height;
    float font_size;
    float red;
    float green;
    float blue;
    float alpha;
    size_t selected_start;
    size_t selected_length;
    size_t marked_start;
    size_t marked_length;
    int has_marked_text;
    int multiline;
    int enabled;
    int secure;
    // A character-cell terminal's preedit surface: fixed-pitch font, no caret
    // or selection, raw key delivery.
    int terminal;
	int mono_face;
    // Printable Option chords become raw key events carrying their base
    // character, so terminal encoders can send Escape-prefixed Meta input.
    int alt_sends_escape;
    // Presses and releases of every key, modifier keys included.
    int key_releases;
    // Navigation keys (Up, Down, PageUp, PageDown) reach the application raw.
    int navigation_keys;
    // The field lays its rows out itself: the pointer places its caret, row
    // movement keys reach the application raw, and input-method windows go to
    // the caret rectangle (top-left origin, like x and y).
    int placed;
    float caret_x;
    float caret_y;
    float caret_width;
    float caret_height;
    // The canvas layer the editor draws in: a surface on a higher layer (a
    // popup over the field) covers its caret and text.
    uint16_t layer;
} viva_text_input_config;

// viva_text_run colors a stretch of a fixed-pitch label: the next `bytes` bytes
// of its text, starting at cell `column` and spanning `cells` columns.
typedef struct {
    uint32_t column;
    uint32_t bytes;
    uint32_t cells;
    uint32_t flags;
    float weight;
    float red;
    float green;
    float blue;
    float alpha;
} viva_text_run;

enum {
    VIVA_RUN_ITALIC = 1,
    VIVA_RUN_UNDERLINE = 2,
    VIVA_RUN_STRIKE = 4,
    // Shape the run as one line and fit it to its cell span, so the font's
    // ligatures appear instead of one glyph per cell.
    VIVA_RUN_LIGATURES = 8,
};

enum {
    VIVA_LABEL_MONO = 1,
    VIVA_LABEL_BITMAP = 2,
    // The label may only paint inside clip_x/clip_y/clip_width/clip_height.
    VIVA_LABEL_CLIPPED = 8,
};

typedef struct {
    const unsigned char *utf8;
    size_t utf8_length;
    float x;
    float y;
    float width;
    float height;
    float font_size;
    float font_weight;
    float red;
    float green;
    float blue;
    float alpha;
    uint16_t layer;
    // mono_face is the ui.MonoFace of a fixed-pitch label: 0 system, 1 classic,
    // or a family registered with viva_register_mono_family.
    uint16_t mono_face;
    // Fixed-pitch grid lines: flags (VIVA_LABEL_MONO), the column pitch and the
    // per-run colors. run_count is 0 for an ordinary label.
    const viva_text_run *runs;
    size_t run_count;
    float cell_width;
    uint32_t flags;
    const unsigned char *bitmap;
    size_t bitmap_length;
    uint32_t bitmap_width;
    uint32_t bitmap_height;
    uint32_t bitmap_stride;
    uint64_t bitmap_id;
    uint32_t bitmap_version;
    // bitmap_upload reuses the reserved word to keep the 160-byte ABI stable.
    uint32_t reserved1;
    float clip_x;
    float clip_y;
    float clip_width;
    float clip_height;
    // Where layout put the text, less x and y, in logical pixels (platform.TextLabel.OriginX).
    float origin_x;
    float origin_y;
} viva_text_label;

// viva_clip is the region, in logical pixels, that one rectangle or triangle
// instance may paint. clipped is 0 for an instance no clip cuts; its other fields
// are then ignored.
typedef struct {
    float x;
    float y;
    float width;
    float height;
    uint32_t clipped;
} viva_clip;

// A rectangle command draws the instances first..first+count-1, which share a
// layer and a clip; clipped, clip_x.. say which (see viva_clip).
typedef struct {
    uint16_t layer;
    uint16_t kind;
    uint32_t first;
    uint32_t count;
    uint32_t clipped;
    float clip_x;
    float clip_y;
    float clip_width;
    float clip_height;
} viva_composite_command;

enum {
    VIVA_COMPOSITE_RECTANGLES = 0,
    VIVA_COMPOSITE_TEXT = 1,
};

typedef struct {
    uint8_t role;
    float x;
    float y;
    float width;
    float height;
    const char *label;
    size_t label_length;
    const char *value;
    size_t value_length;
    const char *hint;
    size_t hint_length;
    int focused;
    int disabled;
    int checked;
    int mixed;
    int selected;
    int expanded;
    int multiline;
    int read_only;
    int secure;
    // The control's identity (ui.ID.Bits), echoed back by an accessibility action.
    uint64_t id;
    int focusable;
    int expandable;
    const char *description;
    size_t description_length;
    int level;
    int pos_in_set;
    int set_size;
    // 0 off, 1 polite, 2 assertive.
    int live;
    int range_set;
    double range_now;
    double range_min;
    double range_max;
    double range_step;
    // The selected span of a text field in UTF-16 units, as AppKit counts.
    int selection_valid;
    int selection_start;
    int selection_end;
    int text_length;
} viva_accessibility_node;

// What an accessibility client asks of a node (vivaAccessibilityAction).
enum {
    VIVA_AX_PRESS = 1,
    VIVA_AX_FOCUS = 2,
    VIVA_AX_INCREMENT = 3,
    VIVA_AX_DECREMENT = 4,
    VIVA_AX_SET_NUMBER = 5,
    VIVA_AX_SET_TEXT = 6,
    VIVA_AX_SET_SELECTION = 7,
};

typedef struct {
    const char *title;
    int width;
    int height;
    int min_width;
    int min_height;
    double clear_red;
    double clear_green;
    double clear_blue;
    double clear_alpha;
    const unsigned char *shader_library;
    size_t shader_library_length;
    const viva_rectangle *rectangles;
    // One clip per rectangle, or NULL when there are no rectangles.
    const viva_clip *rectangle_clips;
    size_t rectangle_count;
    const viva_text_label *text_labels;
    size_t text_label_count;
    const viva_accessibility_node *accessibility;
    size_t accessibility_count;
    viva_text_input_config text_input;
    int pointer_events_enabled;
    int resize_events_enabled;
    // Nonzero reports every move and size change of the content area through
    // vivaFrameChanged.
    int frame_events_enabled;
    // Nonzero reports the window being minimized and restored through
    // vivaWindowMinimized.
    int minimize_events_enabled;
    int key_events_enabled;
    int scroll_events_enabled;
    // Nonzero makes the window a target for files dragged from Finder.
    int file_drop_enabled;
    int redraw_events_enabled;
    int dynamic_text_input;
    int window_mode;
    float titlebar_height;
    int close_intercept_enabled;
    uintptr_t callback_handle;
    // Where a panel opens: the screen rectangle of its status item, in the
    // bottom-left-origin coordinates of Cocoa.
    int has_anchor;
    float anchor_x;
    float anchor_y;
    float anchor_width;
    float anchor_height;
    // Where the content area's top-left opens, in screen coordinates (y down
    // from the primary display's top edge).
    int has_origin;
    float origin_x;
    float origin_y;
} viva_window_config;

int viva_fragment_blends_premultiplied(const char *fragment_name);
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
);
int viva_reference_text(const unsigned char *utf8, size_t length, double size, double weight, double scale,
                       size_t width, size_t height, unsigned char *pixels);
void viva_override_default_integer(const char *key, int value, int remove);
void viva_probe_type(const char *utf8);
void viva_probe_front(void);
void viva_probe_close(void);
int viva_offscreen_set_rectangles(void *offscreen, const viva_rectangle *rectangles, const viva_clip *clips, size_t count);
int viva_offscreen_set_labels(void *offscreen, const viva_text_label *labels, size_t count);
int viva_offscreen_set_text_input(void *offscreen, viva_text_input_config config);
int viva_offscreen_render(
    void *offscreen,
    unsigned char *pixels,
    size_t capacity,
    size_t *width,
    size_t *height
);
void viva_offscreen_release(void *offscreen);
int viva_run(viva_window_config config);
int viva_application_begin(void);
int viva_application_last_status(void);
void *viva_application_open_window(viva_window_config config);
/* Writes x, y, width, height of the window's content area in top-left screen
 * coordinates and returns 1, or returns 0 when there is no such window. Safe from
 * any thread; off the main thread it waits for it. */
int viva_application_window_frame(void *window, double *out);
int viva_application_run(void);
void viva_application_close_all(void);
void viva_application_close_window(void *window);
void viva_application_hide_window(void *window);
void viva_application_show_window(void *window);
void viva_application_quit(void);
void viva_application_post_status_remove(void *item);
void viva_request_redraw(uintptr_t callback_handle);
int viva_view_update_rectangles(
    void *view,
    const viva_rectangle *rectangles,
    const viva_clip *clips,
    size_t total_count,
    size_t first,
    size_t count,
    int rebuild,
    int render
);
int viva_view_update_labels(
    void *view,
    const viva_text_label *labels,
    size_t count
);

int viva_plan_composite(
    const viva_rectangle *rectangles,
    const viva_clip *clips,
    size_t rectangle_count,
    const uint16_t *text_layers,
    size_t text_count,
    viva_composite_command *commands,
    size_t command_capacity,
    size_t *command_count
);
int viva_plan_label_reuse(
    const uint64_t *old_hashes,
    size_t old_count,
    const uint64_t *new_hashes,
    size_t new_count,
    int64_t *source
);
int viva_view_update_accessibility(
    void *view,
    const viva_accessibility_node *nodes,
    size_t count
);
void viva_view_render(void *view);
void viva_view_render_sync(void *view);
void viva_view_close(void *view);
void viva_view_close_confirmed(void *view);
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
);
int viva_view_update_text_input(void *view, viva_text_input_config text_input);
int viva_text_input_caret_should_blink(
    int enabled,
    size_t selected_length,
    int has_marked_text,
    int terminal
);
int viva_text_input_should_interpret_key(int enabled, int multiline, int key);
// viva_text_input_row_key_is_raw says whether a key typed into a field that lays
// its rows out itself goes to the application as a raw key event (1) rather than
// to the text system, which knows one line: the keys that move between rows, and
// Command with Left and Right, which move to the ends of the row.
int viva_text_input_row_key_is_raw(int placed, int composing, int key, int command);
// viva_text_input_navigation_key_is_raw says whether a key typed into a field
// that asked for navigation keys reaches the application as a raw key event
// (1): Up, Down, PageUp and PageDown.
int viva_text_input_navigation_key_is_raw(int navigation_keys, int composing, int key);
// viva_terminal_key_routes_raw says whether a key typed into a terminal goes to
// the application as a raw key event (1) or to the text system (0).
int viva_terminal_key_routes_raw(
    int composing,
    int named_key,
    int control,
    int command,
    int alt,
    int alt_sends_escape
);
// viva_terminal_key_code names the key a character stands for, or 0 for text.
int viva_terminal_key_code(unsigned short character);
// viva_view_move_text_input moves the editor surface without disturbing a
// composition in progress.
int viva_view_move_text_input(void *view, float x, float y, float width, float height);
int viva_measure_text(
    const unsigned char *utf8,
    size_t utf8_length,
    double font_size,
    double font_weight,
    viva_text_metrics *metrics
);
// viva_text_line_create shapes UTF-8 text in the system font at a size and weight, with
// the font fallback CoreText picks per character; NULL when the text is not valid UTF-8.
viva_text_line *viva_text_line_create(const unsigned char *utf8, size_t length, double font_size, double font_weight);
void viva_text_line_retain(viva_text_line *line);
void viva_text_line_release(viva_text_line *line);
// Offsets are UTF-16 units of the string, as CoreText counts them; an offset inside a
// cluster stands for the start of that cluster. viva_text_line_caret is the x of the
// caret there, and viva_text_line_index_at the cluster boundary nearest x.
double viva_text_line_caret(const viva_text_line *line, size_t offset);
size_t viva_text_line_index_at(const viva_text_line *line, double x);
// viva_text_line_selection writes the extents of [start, end) as (x, width) pairs, left
// to right, and returns how many it wrote (at most capacity): one, or several where the
// selection crosses a change of direction.
size_t viva_text_line_selection(const viva_text_line *line, size_t start, size_t end, double *spans, size_t capacity);

// viva_measure_mono reports the system fixed-pitch font's cell advance and line
// height at a size.
int viva_measure_mono(double font_size, int mono_face, double *advance, double *height);
// viva_register_mono_family teaches what a mono_face number (2 and up) stands for: the
// installed family it draws in first (empty for a stack over a built-in face), whether
// that built-in face is the classic one, and the families that draw the characters the
// primary lacks, one name per line, in order. viva_list_mono_families returns every
// installed fixed-pitch family, one per line, in memory the caller frees.
void viva_register_mono_family(int face, const char *utf8, int classic, const char *fallbacks);
char *viva_list_mono_families(void);

#endif
