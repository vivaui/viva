#include "../core_text.m"
#include "../core_render.m"
#include "../native.m"
#include "../main_menu_darwin.m"
#include <assert.h>

// The callbacks native.m reports to Go. The file drop one records what it is
// told and answers as the test says.
static int drop_calls, drop_last_phase, drop_answer;
static double drop_last_x, drop_last_y;
static char drop_last_paths[512];
static size_t drop_last_length;

void vivaOpenURL(const char *url) {}
void vivaOpenFile(const char *path) {}
int vivaApplicationQuitRequested(void) { return 0; }
int vivaApplicationReopenRequested(void) { return 0; }
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
// The promised content callback records what it is asked for and answers as the
// test says.
static int promise_calls, promise_answer = 1, promise_item;
static char promise_path[512];
int vivaFilePromiseContent(intptr_t drag, int item, const char *path) {
    promise_calls++;
    promise_item = item;
    memset(promise_path, 0, sizeof(promise_path));
    if (path != NULL) {
        snprintf(promise_path, sizeof(promise_path), "%s", path);
    }
    return promise_answer;
}
static int finished_calls;
static intptr_t finished_drag;
void vivaFileDragFinished(intptr_t drag) {
    finished_calls++;
    finished_drag = drag;
}
void vivaViewResized(uintptr_t handle, void *view, double width, double height) {}
void vivaRedrawRequested(uintptr_t handle, void *view) {}
void vivaKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) {}
void vivaRawKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int flags, unsigned int code, const unsigned char *utf8, size_t utf8Length, const unsigned char *typed, size_t typedLength) {}
void vivaWindowActivated(uintptr_t handle) {}
void vivaWindowDeactivated(uintptr_t handle, void *view) {}
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
void vivaCompositionInput(uintptr_t handle, void *view, int kind, const unsigned char *utf8,
                  size_t length, int64_t start, int64_t selectedLength,
                  int64_t replacementStart, int64_t replacementLength, int command,
                  int phase, int64_t rawStart, int64_t rawLength, int hasRaw) {}
void vivaTextCompositionLost(uintptr_t handle, void *view, int reason) {}
unsigned char *vivaCopyText(uintptr_t handle, void *view, size_t *length) { *length = 0; return NULL; }

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

        // The platform's effect bits become what a dragging source reports.
        assert(viva_drag_operation(0) == NSDragOperationNone);
        assert(viva_drag_operation(1) == NSDragOperationCopy);
        assert(viva_drag_operation(2) == NSDragOperationMove);
        assert(viva_drag_operation(4) == NSDragOperationLink);
        assert(viva_drag_operation(1 | 4) == (NSDragOperationCopy | NSDragOperationLink));

        // A promised item names itself and asks the application for its content
        // at the destination, then completes without an error.
        VivaFileDragContext *context = [[VivaFileDragContext alloc] initWithDrag:42];
        VivaFilePromiseDelegate *delegate = [[VivaFilePromiseDelegate alloc] initWithName:@"报告.pdf" item:3 context:context];
        [context release];
        NSFilePromiseProvider *provider = [[NSFilePromiseProvider alloc] initWithFileType:@"public.data" delegate:delegate];
        assert([[delegate filePromiseProvider:provider fileNameForType:@"public.data"] isEqualToString:@"报告.pdf"]);
        __block int completed = 0;
        __block NSError *failure = nil;
        promise_answer = 0;
        [delegate filePromiseProvider:provider writePromiseToURL:[NSURL fileURLWithPath:@"/tmp/报告.pdf"] completionHandler:^(NSError *error) {
            completed = 1;
            failure = error;
        }];
        assert(completed == 1 && failure == nil);
        assert(promise_calls == 1 && promise_item == 3 && strcmp(promise_path, "/tmp/报告.pdf") == 0);

        // A failure reaches the target as an error rather than an empty file.
        completed = 0;
        failure = nil;
        promise_answer = 1;
        [delegate filePromiseProvider:provider writePromiseToURL:[NSURL fileURLWithPath:@"/tmp/failed.txt"] completionHandler:^(NSError *error) {
            completed = 1;
            failure = error;
        }];
        assert(completed == 1 && failure != nil);
        [provider release];
        // The provider and its target are gone, so the promises can be forgotten.
        [delegate release];
        assert(finished_calls == 1 && finished_drag == 42);
        printf("file drop: ok\n");
    }
    return 0;
}
