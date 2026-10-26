#include "../native.m"
#include <assert.h>

// The callbacks native.m reports to Go. The file drop one records what it is
// told and answers as the test says.
static int drop_calls, drop_last_phase, drop_answer;
static double drop_last_x, drop_last_y;
static char drop_last_paths[512];
static size_t drop_last_length;

void vivaOpenURL(const char *url) {}
int vivaFileDropEvent(uintptr_t handle, void *view, int phase, double x, double y, const char *paths, size_t length) {
    drop_calls++;
    drop_last_phase = phase;
    drop_last_x = x;
    drop_last_y = y;
    drop_last_length = length;
    memset(drop_last_paths, 0, sizeof(drop_last_paths));
    if (paths != NULL && length < sizeof(drop_last_paths)) {
        memcpy(drop_last_paths, paths, length);
    }
    return drop_answer;
}
void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {}
void vivaViewResized(uintptr_t handle, void *view, double width, double height) {}
void vivaRedrawRequested(uintptr_t handle, void *view) {}
void vivaKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) {}
void vivaRawKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int flags, unsigned int code, const unsigned char *utf8, size_t utf8Length, const unsigned char *typed, size_t typedLength) {}
void vivaFocusLost(uintptr_t handle, void *view) {}
int vivaAccessibilityAction(uintptr_t handle, void *view, uint64_t node, int action, double number, const unsigned char *utf8, size_t utf8Length, int start, int end) { return 0; }
viva_text_line *vivaShapedLine(const unsigned char *utf8, size_t length, double size, double weight) { return viva_text_line_create(utf8, length, size, weight); }
int vivaShortcutEvent(uintptr_t handle, void *view, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) { return 0; }
void vivaScrollEvent(uintptr_t handle, void *view, double deltaX, double deltaY, double x, double y, int modifiers, int precise) {}
void vivaCloseRequested(uintptr_t handle, void *view) {}
void vivaWindowClosed(uintptr_t handle) {}
void vivaFrameChanged(uintptr_t handle, void *view, double x, double y, double width, double height) {}
void vivaWindowMinimized(uintptr_t handle, void *view, int minimized) {}
int vivaWindowReopenRequested(uintptr_t handle, void *view) { return 0; }
uint16_t vivaRectangleLayer(uintptr_t handle, size_t index) { return 0; }
int vivaPressAt(uintptr_t handle, double x, double y) { return 0; }
void vivaTextInput(uintptr_t handle, void *view, int kind, const unsigned char *utf8,
                  size_t length, int64_t start, int64_t selectedLength,
                  int64_t replacementStart, int64_t replacementLength, int command) {}

// What AppKit hands a drop target: where the pointer is and what is dragged.
@interface FakeDrag : NSObject
@property NSPoint location;
@property(retain) NSPasteboard *pasteboard;
@end
@implementation FakeDrag
- (NSPoint)draggingLocation { return self.location; }
- (NSPasteboard *)draggingPasteboard { return self.pasteboard; }
@end

int main(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        // A view that is never put in a window: nothing appears on screen.
        VivaMetalView *view = [[VivaMetalView alloc] initWithFrame:NSMakeRect(0, 0, 400, 300)];
        [view setValue:[NSNumber numberWithUnsignedLong:7] forKey:@"_callbackHandle"];

        // Without being enabled the view takes nothing.
        FakeDrag *drag = [[FakeDrag alloc] init];
        drag.location = NSMakePoint(40, 250);
        drop_answer = 1;
        assert([view draggingEntered:(id)drag] == NSDragOperationNone);
        assert(drop_calls == 0);

        [view enableFileDrops];
        assert([[view registeredDraggedTypes] containsObject:NSPasteboardTypeFileURL]);

        // Entering reports the point with the y axis from the top, and the answer
        // decides which cursor the user sees.
        assert([view draggingEntered:(id)drag] == NSDragOperationCopy);
        assert(drop_calls == 1 && drop_last_phase == 0 && drop_last_length == 0);
        assert(fabs(drop_last_x - 40) < 0.01 && fabs(drop_last_y - 50) < 0.01); // 300 - 250
        drop_answer = 0;
        assert([view draggingUpdated:(id)drag] == NSDragOperationNone);
        assert(drop_calls == 2 && drop_last_phase == 0);

        [view draggingExited:(id)drag];
        assert(drop_calls == 3 && drop_last_phase == 1);

        // A drop carries every file path, each ended by a NUL.
        NSPasteboard *board = [NSPasteboard pasteboardWithUniqueName];
        [board clearContents];
        NSURL *first = [NSURL fileURLWithPath:@"/tmp/first file.txt"];
        NSURL *second = [NSURL fileURLWithPath:@"/tmp/folder" isDirectory:YES];
        [board writeObjects:@[ first, second ]];
        drag.pasteboard = board;
        drop_answer = 1;
        assert([view prepareForDragOperation:(id)drag]);
        assert([view performDragOperation:(id)drag]);
        assert(drop_calls == 4 && drop_last_phase == 2);
        const char *expected = "/tmp/first file.txt\0/tmp/folder\0";
        assert(drop_last_length == strlen("/tmp/first file.txt") + 1 + strlen("/tmp/folder") + 1);
        assert(memcmp(drop_last_paths, expected, drop_last_length) == 0);

        // The application refusing the drop is reported to the system.
        drop_answer = 0;
        assert(![view performDragOperation:(id)drag]);

        // A pasteboard with no files is no drop at all.
        NSPasteboard *empty = [NSPasteboard pasteboardWithUniqueName];
        [empty clearContents];
        [empty setString:@"just text" forType:NSPasteboardTypeString];
        drag.pasteboard = empty;
        int before = drop_calls;
        assert(![view performDragOperation:(id)drag]);
        assert(drop_calls == before);
        printf("file drop: ok\n");
    }
    return 0;
}
