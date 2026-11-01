#include "../native.m"
#include <assert.h>
#include <objc/runtime.h>

// Records the click counts native button events report, and nothing else.
static int recordedKinds[16];
static int recordedClicks[16];
static int recordedCount;
void vivaPointerEvent(uintptr_t handle, void *view, int kind, int button, int modifiers, int clickCount, double x, double y, double screenX, double screenY) {
    if (recordedCount < 16) {
        recordedKinds[recordedCount] = kind;
        recordedClicks[recordedCount++] = clickCount;
    }
}
void vivaViewResized(uintptr_t handle, void *view, double width, double height) {}
void vivaRedrawRequested(uintptr_t handle, void *view) {}
// Raw key events a terminal view delivers.
static int rawKeys[16];
static int rawModifiers[16];
static char rawText[16][8];
static int rawCount;
void vivaKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) {
    if (rawCount < 16) {
        rawKeys[rawCount] = key;
        rawModifiers[rawCount] = modifiers;
        memset(rawText[rawCount], 0, sizeof(rawText[rawCount]));
        memcpy(rawText[rawCount], utf8, MIN(utf8Length, sizeof(rawText[rawCount]) - 1));
        rawCount++;
    }
}
void vivaRawKeyEvent(uintptr_t handle, void *view, int key, int modifiers, int flags, unsigned int code, const unsigned char *utf8, size_t utf8Length, const unsigned char *typed, size_t typedLength) {}
void vivaFocusLost(uintptr_t handle, void *view) {}
// The terminal's table, reduced to what these tests use: CJK ideographs take two cells,
// combining marks none.
int vivaTerminalCells(uint32_t character) {
    if (character >= 0x0300 && character <= 0x036F) {
        return 0;
    }
    return character >= 0x4E00 && character <= 0x9FFF ? 2 : 1;
}
int vivaAccessibilityAction(uintptr_t handle, void *view, uint64_t node, int action, double number, const unsigned char *utf8, size_t utf8Length, int start, int end) { return 0; }
// The engine's shaped line, uncached: the caller releases it.
viva_text_line *vivaShapedLine(const unsigned char *utf8, size_t length, double size, double weight) { return viva_text_line_create(utf8, length, size, weight); }
int vivaShortcutEvent(uintptr_t handle, void *view, int modifiers, int repeat, const unsigned char *utf8, size_t utf8Length) { return 0; }
void vivaOpenURL(const char *url) {}
int vivaApplicationQuitRequested(void) { return 0; }
int vivaFileDropEvent(uintptr_t handle, void *view, int phase, double x, double y, const char *paths, size_t length) { return 0; }
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

@interface SelectionView : VivaMetalView {
@public
    NSUInteger selectionEvents;
    NSUInteger textureRebuilds;
    NSUInteger renders;
    // Edit events (insert, command) are asserted away unless recordEdits is set,
    // so the selection tests keep proving they never emit one.
    BOOL recordEdits;
    NSUInteger editEvents;
    int lastKind;
    int lastCommand;
    NSString *lastText;
    // The modifiers the next Return key is taken to have.
    NSEventModifierFlags returnFlags;
}
- (void)resetText:(NSString *)value;
- (NSEvent *)mouseEvent:(NSEventType)type offset:(CGFloat)offset;
- (CGFloat)offsetForIndex:(NSUInteger)index;
- (BOOL)dragging;
- (void)enablePointerCallbacks;
- (void)disablePointerCallbacks;
- (NSEvent *)buttonEvent:(NSEventType)type clicks:(NSInteger)clicks;
- (void)setSecure:(BOOL)value;
- (void)setMultiline:(BOOL)value;
- (void)setEditing:(BOOL)value;
- (CGFloat)fontSize;
- (BOOL)hasCaretTimer;
- (void)beginComposition;
- (void)setTerminal:(BOOL)value;
- (void)setCellWidth:(CGFloat)value;
- (void)enableKeyCallbacks;
- (void)endComposition;
- (BOOL)composing;
- (CGFloat)frameX;
@end

@implementation SelectionView
- (void)resetText:(NSString *)value {
    [self stopCaretBlink];
    [_text release];
    _text = [value copy];
    [_textColor release];
    _textColor = [[NSColor blackColor] retain];
    _textFontSize = 16;
    _textFrame = NSMakeRect(40, 80, 260, 32);
    _selectedRange = NSMakeRange(_text.length, 0);
    _hasMarkedText = NO;
    _textInputEnabled = YES;
    _dynamicTextInput = YES;
    _textSelectionDragging = NO;
    selectionEvents = 0;
    textureRebuilds = 0;
    renders = 0;
    recordEdits = NO;
    editEvents = 0;
    lastKind = 0;
    lastCommand = 0;
    [lastText release];
    lastText = nil;
    _textSecure = NO;
    _textInputMultiline = NO;
}
- (NSTextInputContext *)inputContext { return nil; }
- (NSEventModifierFlags)returnKeyModifierFlags { return returnFlags; }
- (BOOL)rebuildTextTexture { textureRebuilds++; return YES; }
- (void)render { renders++; }
- (void)emitTextKind:(int)kind text:(NSString *)value selectedRange:(NSRange)selected
    replacementRange:(NSRange)replacement command:(int)command {
    if (recordEdits && kind != VIVA_TEXT_SET_SELECTION) {
        editEvents++;
        lastKind = kind;
        lastCommand = command;
        [lastText release];
        lastText = [value copy];
        return;
    }
    assert(kind == VIVA_TEXT_SET_SELECTION);
    assert(replacement.location == NSNotFound);
    assert(value.length == 0);
    selectionEvents++;
    _selectedRange = selected;
    _hasMarkedText = NO;
    [self restartCaretBlink];
}
- (NSEvent *)mouseEvent:(NSEventType)type offset:(CGFloat)offset {
    return [NSEvent mouseEventWithType:type
        location:NSMakePoint(_textFrame.origin.x + offset,
                             self.bounds.size.height - NSMidY(_textFrame))
        modifierFlags:0 timestamp:0 windowNumber:0 context:nil eventNumber:0 clickCount:1 pressure:1];
}
- (CGFloat)offsetForIndex:(NSUInteger)index {
    viva_editor_line line = [self newTextLine];
    CGFloat offset = [self textOffsetForIndex:index line:line];
    viva_editor_line_free(line);
    return offset;
}
- (BOOL)dragging { return _textSelectionDragging; }
- (void)enablePointerCallbacks { _callbackHandle = 1; _pointerEventsEnabled = YES; }
- (void)disablePointerCallbacks { _callbackHandle = 0; _pointerEventsEnabled = NO; }
- (NSEvent *)buttonEvent:(NSEventType)type clicks:(NSInteger)clicks {
    return [NSEvent mouseEventWithType:type location:NSMakePoint(5, 5) modifierFlags:0 timestamp:0
        windowNumber:0 context:nil eventNumber:0 clickCount:clicks pressure:1];
}
- (void)setSecure:(BOOL)value { _textSecure = value; }
- (void)setMultiline:(BOOL)value { _textInputMultiline = value; }
- (void)setEditing:(BOOL)value { _textInputEnabled = value; }
- (CGFloat)fontSize { return _textFontSize; }
- (BOOL)hasCaretTimer { return _caretTimer != nil; }
- (void)beginComposition { _hasMarkedText = YES; }
- (void)setTerminal:(BOOL)value { _textTerminal = value; }
- (void)setCellWidth:(CGFloat)value { _textCellWidth = value; }
- (void)enableKeyCallbacks { _callbackHandle = 1; _keyEventsEnabled = YES; }
- (void)endComposition { _hasMarkedText = NO; }
- (BOOL)composing { return _hasMarkedText; }
- (CGFloat)frameX { return _textFrame.origin.x; }
@end

static NSEvent *keyEvent(NSString *characters, NSEventModifierFlags modifiers) {
    return [NSEvent keyEventWithType:NSEventTypeKeyDown location:NSZeroPoint
        modifierFlags:modifiers timestamp:0 windowNumber:0 context:nil characters:characters
        charactersIgnoringModifiers:characters isARepeat:NO keyCode:0];
}

int main(void) {
    @autoreleasepool {
        SelectionView *view = [[SelectionView alloc] initWithFrame:NSMakeRect(0, 0, 480, 320)];
        // An empty field must still get a caret of real height: a line with no
        // glyphs reports zero ascent and descent.
        [view resetText:@"x"];
        viva_editor_line filled = [view newTextLine];
        CGFloat filledHeight = filled.shaped->ascent + filled.shaped->descent;
        viva_editor_line_free(filled);
        [view resetText:@""];
        viva_editor_line blank = [view newTextLine];
        assert(blank.shaped != NULL);
        CGFloat rawAscent = 0, rawDescent = 0, rawLeading = 0;
        CTLineGetTypographicBounds((CTLineRef)blank.shaped->line, &rawAscent, &rawDescent, &rawLeading);
        assert(rawAscent + rawDescent == 0);
        CGFloat blankHeight = blank.shaped->ascent + blank.shaped->descent;
        viva_editor_line_free(blank);
        assert(blankHeight >= 10.0);
        assert(fabs(blankHeight - filledHeight) < 1.0);

        // A scale change (window dragged to another display) arrives with no
        // resize or input event, so the frame must be redrawn right there.
        [view resetText:@"A中😀B"];
        [view viewDidChangeBackingProperties];
        assert(view->renders == 1);

        [view resetText:@"A中😀B"];
        NSEvent *start = [view mouseEvent:NSEventTypeLeftMouseDown offset:[view offsetForIndex:2]];
        assert([view eventIsInsideTextInput:start]);
        [view mouseDown:start];
        assert(NSEqualRanges(view.selectedRange, NSMakeRange(2, 0)));
        assert([view hasCaretTimer]);
        [view mouseDragged:[view mouseEvent:NSEventTypeLeftMouseDragged offset:1000]];
        assert(NSEqualRanges(view.selectedRange, NSMakeRange(2, 3)));
        assert(![view hasCaretTimer]);
        [view mouseDragged:[view mouseEvent:NSEventTypeLeftMouseDragged offset:-100]];
        assert(NSEqualRanges(view.selectedRange, NSMakeRange(0, 2)));
        NSUInteger events = view->selectionEvents;
        [view mouseUp:[view mouseEvent:NSEventTypeLeftMouseUp offset:-100]];
        assert(![view dragging]);
        assert(view->selectionEvents == events);
        for (NSUInteger iteration = 0; iteration < 1000; iteration++) {
            [view selectTextRange:NSMakeRange(0, 2)];
        }
        assert(view->selectionEvents == events);
        assert(view->textureRebuilds == 0);

        [view resetText:@"A中😀B"];
        assert([view performKeyEquivalent:keyEvent(@"a", NSEventModifierFlagCommand)]);
        assert(NSEqualRanges(view.selectedRange, NSMakeRange(0, 5)));
        assert(view->selectionEvents == 1);
        [view doCommandBySelector:@selector(selectAll:)];
        [view selectAll:nil];
        assert(view->selectionEvents == 1);
        [view resetText:@"A中😀B"];
        assert(![view performKeyEquivalent:keyEvent(@"a", NSEventModifierFlagCommand | NSEventModifierFlagShift)]);
        assert(![view performKeyEquivalent:keyEvent(@"a", NSEventModifierFlagControl)]);
        assert(![view performKeyEquivalent:keyEvent(@"q", NSEventModifierFlagCommand)]);
        assert(view->selectionEvents == 0);
        assert([view performKeyEquivalent:keyEvent(@"A", NSEventModifierFlagCommand | NSEventModifierFlagCapsLock)]);
        assert(view->selectionEvents == 1);

        for (NSString *value in @[@"", @"A中😀B", @"Ae\u0301👩‍💻🇨🇳Z"]) {
            [view resetText:value];
            for (CGFloat offset = -10; offset < 200; offset += 0.25) {
                NSUInteger index = [view textIndexForEvent:[view mouseEvent:NSEventTypeLeftMouseDown offset:offset]];
                assert(index <= value.length);
                assert(index == value.length || index == [value rangeOfComposedCharacterSequenceAtIndex:index].location);
            }
        }

        // The editor places its caret, selection and clicks at the boundaries of the line it
        // draws: they never go backwards, and a click at a boundary lands on it.
        [view resetText:@"Ab cd  ef"];
        CGFloat previous = -1;
        for (NSUInteger index = 0; index <= 9; index++) {
            CGFloat boundary = [view offsetForIndex:index];
            assert(boundary >= previous);
            previous = boundary;
            assert([view textIndexForEvent:[view mouseEvent:NSEventTypeLeftMouseDown offset:boundary + 0.01]] == index);
        }
        assert([view offsetForIndex:0] == 0);
        [view resetText:@"A中😀B"];
        for (NSUInteger index = 0; index <= 5; index++) {
            if (index == 3) {
                continue; // inside the emoji
            }
            CGFloat boundary = [view offsetForIndex:index];
            assert([view textIndexForEvent:[view mouseEvent:NSEventTypeLeftMouseDown offset:boundary + 0.01]] == index);
        }

        [view resetText:@"marked"];
        [view beginComposition];
        [view selectTextRange:view.selectedRange];
        assert(view->selectionEvents == 1);
        assert(![view hasMarkedText]);
        [view mouseDown:[view mouseEvent:NSEventTypeLeftMouseDown offset:0]];
        assert([view dragging]);
        assert([view hasCaretTimer]);
        assert([view updateTextInput:(viva_text_input_config){0}] == 0);
        assert(![view dragging]);
        assert(![view hasCaretTimer]);
        events = view->selectionEvents;
        [view mouseDragged:[view mouseEvent:NSEventTypeLeftMouseDragged offset:80]];
        [view selectAll:nil];
        assert(![view performKeyEquivalent:keyEvent(@"a", NSEventModifierFlagCommand)]);
        assert(view->selectionEvents == events);


        // Button events carry the system's click count; moves and drags carry 0.
        [view resetText:@""];
        [view setEditing:NO];
        [view enablePointerCallbacks];
        recordedCount = 0;
        [view mouseDown:[view buttonEvent:NSEventTypeLeftMouseDown clicks:1]];
        [view mouseUp:[view buttonEvent:NSEventTypeLeftMouseUp clicks:1]];
        [view mouseDown:[view buttonEvent:NSEventTypeLeftMouseDown clicks:2]];
        [view mouseDragged:[view buttonEvent:NSEventTypeLeftMouseDragged clicks:2]];
        [view mouseUp:[view buttonEvent:NSEventTypeLeftMouseUp clicks:2]];
        [view mouseDown:[view buttonEvent:NSEventTypeLeftMouseDown clicks:0]];
        const int wantKinds[] = {VIVA_POINTER_DOWN, VIVA_POINTER_UP, VIVA_POINTER_DOWN, VIVA_POINTER_MOVE, VIVA_POINTER_UP, VIVA_POINTER_DOWN};
        const int wantClicks[] = {1, 1, 2, 0, 2, 1};
        assert(recordedCount == 6);
        for (int index = 0; index < 6; index++) {
            assert(recordedKinds[index] == wantKinds[index]);
            assert(recordedClicks[index] == wantClicks[index]);
        }
        [view disablePointerCallbacks];
        [view setEditing:YES];

        // Return asks to submit; Shift, Option or Control with it ask for a line
        // break, and so do the selectors AppKit sends for a line break.
        [view resetText:@""];
        view->recordEdits = YES;
        const struct { NSEventModifierFlags flags; int command; } returns[] = {
            {0, VIVA_TEXT_INSERT_NEWLINE},
            {NSEventModifierFlagCapsLock, VIVA_TEXT_INSERT_NEWLINE},
            {NSEventModifierFlagShift, VIVA_TEXT_INSERT_LINE_BREAK},
            {NSEventModifierFlagOption, VIVA_TEXT_INSERT_LINE_BREAK},
            {NSEventModifierFlagControl, VIVA_TEXT_INSERT_LINE_BREAK},
            {NSEventModifierFlagShift | NSEventModifierFlagCommand, VIVA_TEXT_INSERT_LINE_BREAK},
        };
        for (size_t index = 0; index < sizeof(returns) / sizeof(returns[0]); index++) {
            view->returnFlags = returns[index].flags;
            NSUInteger before = view->editEvents;
            [view doCommandBySelector:@selector(insertNewline:)];
            assert(view->editEvents == before + 1);
            assert(view->lastKind == VIVA_TEXT_COMMAND_EVENT);
            assert(view->lastCommand == returns[index].command);
        }
        view->returnFlags = 0;
        [view doCommandBySelector:@selector(insertLineBreak:)];
        assert(view->lastCommand == VIVA_TEXT_INSERT_LINE_BREAK);
        [view doCommandBySelector:@selector(insertNewlineIgnoringFieldEditor:)];
        assert(view->lastCommand == VIVA_TEXT_INSERT_LINE_BREAK);
        [view doCommandBySelector:@selector(undo:)];
        assert(view->lastCommand == VIVA_TEXT_UNDO);
        [view doCommandBySelector:@selector(redo:)];
        assert(view->lastCommand == VIVA_TEXT_REDO);
        assert([view performKeyEquivalent:keyEvent(@"z", NSEventModifierFlagCommand)]);
        assert(view->lastCommand == VIVA_TEXT_UNDO);
        assert([view performKeyEquivalent:keyEvent(@"Z", NSEventModifierFlagCommand | NSEventModifierFlagShift)]);
        assert(view->lastCommand == VIVA_TEXT_REDO);

        // The navigation chords AppKit's key bindings turn into selectors: Option
        // moves and deletes by word, Command goes to the ends of the line, and
        // Command with Up and Down, like Home and End, to the ends of the text.
        // The numbers are platform.TextCommand's, which the Go test passes in.
        _Static_assert(VIVA_TEXT_MOVE_LINE_START == GO_TEXT_MOVE_LINE_START, "line start");
        _Static_assert(VIVA_TEXT_MOVE_LINE_END == GO_TEXT_MOVE_LINE_END, "line end");
        _Static_assert(VIVA_TEXT_MOVE_LINE_START_AND_MODIFY_SELECTION == GO_TEXT_MOVE_LINE_START_AND_MODIFY_SELECTION, "line start selection");
        _Static_assert(VIVA_TEXT_MOVE_LINE_END_AND_MODIFY_SELECTION == GO_TEXT_MOVE_LINE_END_AND_MODIFY_SELECTION, "line end selection");
        _Static_assert(VIVA_TEXT_MOVE_DOCUMENT_START == GO_TEXT_MOVE_DOCUMENT_START, "document start");
        _Static_assert(VIVA_TEXT_MOVE_DOCUMENT_END == GO_TEXT_MOVE_DOCUMENT_END, "document end");
        _Static_assert(VIVA_TEXT_MOVE_DOCUMENT_START_AND_MODIFY_SELECTION == GO_TEXT_MOVE_DOCUMENT_START_AND_MODIFY_SELECTION, "document start selection");
        _Static_assert(VIVA_TEXT_MOVE_DOCUMENT_END_AND_MODIFY_SELECTION == GO_TEXT_MOVE_DOCUMENT_END_AND_MODIFY_SELECTION, "document end selection");
        _Static_assert(VIVA_TEXT_MOVE_WORD_LEFT == GO_TEXT_MOVE_WORD_LEFT, "word left");
        _Static_assert(VIVA_TEXT_MOVE_WORD_RIGHT == GO_TEXT_MOVE_WORD_RIGHT, "word right");
        _Static_assert(VIVA_TEXT_MOVE_WORD_LEFT_AND_MODIFY_SELECTION == GO_TEXT_MOVE_WORD_LEFT_AND_MODIFY_SELECTION, "word left selection");
        _Static_assert(VIVA_TEXT_MOVE_WORD_RIGHT_AND_MODIFY_SELECTION == GO_TEXT_MOVE_WORD_RIGHT_AND_MODIFY_SELECTION, "word right selection");
        _Static_assert(VIVA_TEXT_DELETE_WORD_BACKWARD == GO_TEXT_DELETE_WORD_BACKWARD, "delete word backward");
        _Static_assert(VIVA_TEXT_DELETE_WORD_FORWARD == GO_TEXT_DELETE_WORD_FORWARD, "delete word forward");
        _Static_assert(VIVA_TEXT_DELETE_TO_LINE_START == GO_TEXT_DELETE_TO_LINE_START, "delete to line start");
        _Static_assert(VIVA_TEXT_DELETE_TO_LINE_END == GO_TEXT_DELETE_TO_LINE_END, "delete to line end");
        const struct { const char *selector; int command; } navigation[] = {
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
        for (size_t index = 0; index < sizeof(navigation) / sizeof(navigation[0]); index++) {
            NSUInteger before = view->editEvents;
            [view doCommandBySelector:NSSelectorFromString(@(navigation[index].selector))];
            assert(view->editEvents == before + 1);
            assert(view->lastKind == VIVA_TEXT_COMMAND_EVENT);
            assert(view->lastCommand == navigation[index].command);
        }
        // The chords themselves, through AppKit's own key bindings: what each
        // key sends is the system's, and these are the commands it must become.
        // Command with Forward Delete is bound to nothing (noop:), so it stays so.
        {
            const struct { unichar character; unsigned short code; NSEventModifierFlags flags; int command; } chords[] = {
                {0x7f, 51, NSEventModifierFlagCommand, VIVA_TEXT_DELETE_TO_LINE_START},
                {0x7f, 51, NSEventModifierFlagOption, VIVA_TEXT_DELETE_WORD_BACKWARD},
                {NSDeleteFunctionKey, 117, NSEventModifierFlagOption, VIVA_TEXT_DELETE_WORD_FORWARD},
                {NSLeftArrowFunctionKey, 123, NSEventModifierFlagOption, VIVA_TEXT_MOVE_WORD_LEFT},
                {NSRightArrowFunctionKey, 124, NSEventModifierFlagOption | NSEventModifierFlagShift, VIVA_TEXT_MOVE_WORD_RIGHT_AND_MODIFY_SELECTION},
                {NSLeftArrowFunctionKey, 123, NSEventModifierFlagCommand, VIVA_TEXT_MOVE_LINE_START},
                {NSRightArrowFunctionKey, 124, NSEventModifierFlagCommand | NSEventModifierFlagShift, VIVA_TEXT_MOVE_LINE_END_AND_MODIFY_SELECTION},
                {NSUpArrowFunctionKey, 126, NSEventModifierFlagCommand, VIVA_TEXT_MOVE_DOCUMENT_START},
                {NSHomeFunctionKey, 115, 0, VIVA_TEXT_MOVE_DOCUMENT_START},
                {NSEndFunctionKey, 119, 0, VIVA_TEXT_MOVE_DOCUMENT_END},
                {NSDeleteFunctionKey, 117, NSEventModifierFlagCommand, -1},
            };
            for (size_t index = 0; index < sizeof(chords) / sizeof(chords[0]); index++) {
                NSString *characters = [NSString stringWithCharacters:&chords[index].character length:1];
                NSEvent *event = [NSEvent keyEventWithType:NSEventTypeKeyDown location:NSZeroPoint
                    modifierFlags:chords[index].flags timestamp:0 windowNumber:0 context:nil
                    characters:characters charactersIgnoringModifiers:characters isARepeat:NO
                    keyCode:chords[index].code];
                view->lastCommand = -1;
                [view interpretKeyEvents:@[event]];
                assert(view->lastCommand == chords[index].command);
            }
        }
        // A selector no key binding of ours stands for does nothing.
        NSUInteger unmapped = view->editEvents;
        [view doCommandBySelector:@selector(moveUp:)];
        assert(view->editEvents == unmapped);
        view->recordEdits = NO;

        // Clipboard shortcuts. A private pasteboard keeps the test away from the
        // user's clipboard.
        viva_test_pasteboard = [NSPasteboard pasteboardWithUniqueName];
        NSPasteboard *board = viva_test_pasteboard;
        NSEventModifierFlags command = NSEventModifierFlagCommand;

        // Copy: the selected UTF-16 range, emoji included, lands on the pasteboard.
        [view resetText:@"A中😀B"];
        view->recordEdits = YES;
        [view selectTextRange:NSMakeRange(1, 3)];
        assert([view performKeyEquivalent:keyEvent(@"c", command)]);
        assert([[board stringForType:NSPasteboardTypeString] isEqualToString:@"中😀"]);
        assert(view->editEvents == 0);

        // An empty selection leaves the pasteboard as it was.
        [board clearContents];
        [board setString:@"keep" forType:NSPasteboardTypeString];
        [view selectTextRange:NSMakeRange(2, 0)];
        assert([view performKeyEquivalent:keyEvent(@"c", command)]);
        assert([[board stringForType:NSPasteboardTypeString] isEqualToString:@"keep"]);

        // Cut copies, then deletes the selection the way Backspace does.
        [view selectTextRange:NSMakeRange(0, 1)];
        assert([view performKeyEquivalent:keyEvent(@"x", command)]);
        assert([[board stringForType:NSPasteboardTypeString] isEqualToString:@"A"]);
        assert(view->editEvents == 1 && view->lastKind == VIVA_TEXT_COMMAND_EVENT &&
               view->lastCommand == VIVA_TEXT_DELETE_BACKWARD);
        [view selectTextRange:NSMakeRange(2, 0)];
        NSUInteger edits = view->editEvents;
        assert([view performKeyEquivalent:keyEvent(@"x", command)]);
        assert(view->editEvents == edits);

        // Paste inserts the pasteboard text; the case of the key does not matter.
        [board clearContents];
        [board setString:@"粘贴 text" forType:NSPasteboardTypeString];
        assert([view performKeyEquivalent:keyEvent(@"V", command)]);
        assert(view->lastKind == VIVA_TEXT_PASTE && [view->lastText isEqualToString:@"粘贴 text"]);

        // Single-line fields flatten line breaks and drop trailing ones;
        // multiline fields keep what was copied.
        [board clearContents];
        [board setString:@"one\r\ntwo\nthree\n" forType:NSPasteboardTypeString];
        [view paste:nil];
        assert([view->lastText isEqualToString:@"one two three"]);
        [view setMultiline:YES];
        [view paste:nil];
        assert([view->lastText isEqualToString:@"one\r\ntwo\nthree\n"]);
        [view setMultiline:NO];

        // An empty or non-text pasteboard inserts nothing, and says so: a field
        // that takes pictures then looks at the pasteboard's other formats.
        [board clearContents];
        edits = view->editEvents;
        [view paste:nil];
        assert(view->editEvents == edits + 1 && view->lastKind == VIVA_TEXT_PASTE_NO_TEXT);
        assert([view->lastText length] == 0);
        // Text that is only line breaks flattens to nothing and says nothing.
        [board setString:@"\n\r\n" forType:NSPasteboardTypeString];
        edits = view->editEvents;
        [view paste:nil];
        assert(view->editEvents == edits);
        [board clearContents];

        // A password field never exports its text but accepts a paste.
        [view setSecure:YES];
        [board clearContents];
        [board setString:@"clip" forType:NSPasteboardTypeString];
        [view selectTextRange:NSMakeRange(0, 2)];
        edits = view->editEvents;
        assert([view performKeyEquivalent:keyEvent(@"c", command)]);
        assert([[board stringForType:NSPasteboardTypeString] isEqualToString:@"clip"]);
        assert([view performKeyEquivalent:keyEvent(@"x", command)]);
        assert([[board stringForType:NSPasteboardTypeString] isEqualToString:@"clip"]);
        assert(view->editEvents == edits);
        assert([view performKeyEquivalent:keyEvent(@"v", command)]);
        assert(view->editEvents == edits + 1 && [view->lastText isEqualToString:@"clip"]);
        [view setSecure:NO];

        // Only a bare Command chord counts, and only while a field is editing.
        edits = view->editEvents;
        assert(![view performKeyEquivalent:keyEvent(@"v", command | NSEventModifierFlagShift)]);
        assert(![view performKeyEquivalent:keyEvent(@"v", NSEventModifierFlagControl)]);
        assert(![view performKeyEquivalent:keyEvent(@"v", command | NSEventModifierFlagOption)]);
        [view setEditing:NO];
        assert(![view performKeyEquivalent:keyEvent(@"v", command)]);
        assert(![view performKeyEquivalent:keyEvent(@"c", command)]);
        assert(view->editEvents == edits);
        [view setEditing:YES];
        view->recordEdits = NO;
        viva_test_pasteboard = nil;

        // A terminal view: every key that is not text goes to the application
        // raw, text and composition go to the text system, the cursor is steady
        // and pasting says that it is a paste.
        [view resetText:@""];
        [view setTerminal:YES];
        [view enableKeyCallbacks];
        view->recordEdits = YES;
        rawCount = 0;
        NSEvent *controlC = [NSEvent keyEventWithType:NSEventTypeKeyDown location:NSZeroPoint
            modifierFlags:NSEventModifierFlagControl timestamp:0 windowNumber:0 context:nil characters:@"\x03"
            charactersIgnoringModifiers:@"c" isARepeat:NO keyCode:8];
        [view keyDown:controlC];
        // Control+C: the characters carry the control code, the base letter is "c".
        assert(rawCount == 1);
        assert(rawKeys[0] == 0 && rawModifiers[0] == VIVA_MODIFIER_CONTROL && strcmp(rawText[0], "c") == 0);
        rawCount = 0;
        unichar up = NSUpArrowFunctionKey;
        NSString *upText = [NSString stringWithCharacters:&up length:1];
        [view keyDown:keyEvent(upText, NSEventModifierFlagOption)];
        assert(rawCount == 1 && rawKeys[0] == VIVA_KEY_UP && rawModifiers[0] == VIVA_MODIFIER_ALT && rawText[0][0] == 0);
        unichar macDelete = 0x7f;
        [view keyDown:keyEvent([NSString stringWithCharacters:&macDelete length:1], 0)];
        assert(rawCount == 2 && rawKeys[1] == VIVA_KEY_BACKSPACE);
        unichar f5 = NSF5FunctionKey;
        [view keyDown:keyEvent([NSString stringWithCharacters:&f5 length:1], 0)];
        assert(rawCount == 3 && rawKeys[2] == VIVA_KEY_F1 + 4);
        // AppKit's window keeps Control+Tab for itself, so the view claims it as a
        // key equivalent and hands it to the application; the other Tab chords
        // are left to the normal key path.
        rawCount = 0;
        assert([view performKeyEquivalent:keyEvent(@"\t", NSEventModifierFlagControl)]);
        assert(rawCount == 1 && rawKeys[0] == VIVA_KEY_TAB && rawModifiers[0] == VIVA_MODIFIER_CONTROL && rawText[0][0] == 0);
        unichar backTab = NSBackTabCharacter;
        assert([view performKeyEquivalent:keyEvent([NSString stringWithCharacters:&backTab length:1],
            NSEventModifierFlagControl | NSEventModifierFlagShift)]);
        assert(rawCount == 2 && rawKeys[1] == VIVA_KEY_TAB && rawModifiers[1] == (VIVA_MODIFIER_CONTROL | VIVA_MODIFIER_SHIFT));
        assert(![view performKeyEquivalent:keyEvent(@"\t", 0)]);
        assert(![view performKeyEquivalent:keyEvent(@"\t", NSEventModifierFlagShift)]);
        assert(![view performKeyEquivalent:keyEvent(@"\t", NSEventModifierFlagCommand)]);
        assert(![view performKeyEquivalent:keyEvent(@"\t", NSEventModifierFlagControl | NSEventModifierFlagCommand)]);
        assert(![view performKeyEquivalent:keyEvent(@"a", NSEventModifierFlagControl)]);
        assert(rawCount == 2);
        rawCount = 0;
        [view beginComposition];
        assert(![view performKeyEquivalent:keyEvent(@"\t", NSEventModifierFlagControl)] && rawCount == 0);
        [view endComposition];
        // Plain text and composition never come out raw.
        rawCount = 0;
        [view keyDown:keyEvent(@"a", 0)];
        [view keyDown:keyEvent(@"A", NSEventModifierFlagShift)];
        [view keyDown:keyEvent(@" ", 0)];
        assert(rawCount == 0);
        [view beginComposition];
        [view keyDown:keyEvent(upText, 0)];
        [view keyDown:keyEvent(@"\r", 0)];
        assert(rawCount == 0);
        [view endComposition];
        // The cursor does not blink and a click does not start a selection.
        [view restartCaretBlink];
        assert(![view hasCaretTimer]);
        [view mouseDown:[view mouseEvent:NSEventTypeLeftMouseDown offset:2]];
        assert(![view dragging] && view->selectionEvents == 0);
        // Command+V pastes untouched, line breaks and all; the other editing
        // chords are not claimed.
        NSPasteboard *terminalBoard = [NSPasteboard pasteboardWithName:@"viva.test.terminal"];
        viva_test_pasteboard = terminalBoard;
        [terminalBoard clearContents];
        [terminalBoard setString:@"ls\nrm\r\n" forType:NSPasteboardTypeString];
        NSUInteger pastes = view->editEvents;
        assert([view performKeyEquivalent:keyEvent(@"v", NSEventModifierFlagCommand)]);
        assert(view->editEvents == pastes + 1 && view->lastKind == VIVA_TEXT_PASTE);
        assert([view->lastText isEqualToString:@"ls\nrm\r\n"]);
        assert(![view performKeyEquivalent:keyEvent(@"a", NSEventModifierFlagCommand)]);
        assert(![view performKeyEquivalent:keyEvent(@"x", NSEventModifierFlagCommand)]);
        // A terminal is never told of a paste with no text.
        [terminalBoard clearContents];
        pastes = view->editEvents;
        [view paste:nil];
        assert(view->editEvents == pastes);
        // Moving the editor does not cancel a composition and checks its size.
        [view beginComposition];
        assert([view moveTextInputToFrame:NSMakeRect(100, 50, 8, 16)] == 0);
        assert([view composing] && [view frameX] == 100);
        assert([view moveTextInputToFrame:NSMakeRect(100, 50, 0, 16)] == 1);
        [view setTerminal:NO];
        assert([view moveTextInputToFrame:NSMakeRect(100, 50, 8, 16)] == 1);
        [view endComposition];
        view->recordEdits = NO;
        viva_test_pasteboard = nil;

        // A terminal's preedit is laid on the terminal's cells, as its grid is: a wide
        // character takes two, a combining mark none, and the caret, the hit test and the
        // candidate window follow them, not the advances of the font.
        [view resetText:@"A中e\u0301好b"];
        [view setTerminal:YES];
        [view setCellWidth:10];
        {
            // UTF-16 units: A 0, 中 1, e 2, the mark 3, 好 4, b 5; the cell column of the
            // boundary before each, and the end.
            const int columns[] = {0, 1, 3, 4, 4, 6, 7};
            for (NSUInteger index = 0; index <= 6; index++) {
                assert([view offsetForIndex:index] == columns[index] * 10.0);
            }
            NSPoint origin = NSMakePoint([view frameX], 0);
            assert([view textIndexForLocalPoint:NSMakePoint(origin.x - 5, 0)] == 0);
            assert([view textIndexForLocalPoint:NSMakePoint(origin.x + 14, 0)] == 1);
            assert([view textIndexForLocalPoint:NSMakePoint(origin.x + 33, 0)] == 2);
            // Between the base and its mark is no boundary: the nearer edge of the pair is.
            assert([view textIndexForLocalPoint:NSMakePoint(origin.x + 36, 0)] == 4);
            assert([view textIndexForLocalPoint:NSMakePoint(origin.x + 500, 0)] == 6);
        }
        [view setTerminal:NO];

        [view resetText:@"close"];
        [view mouseDown:[view mouseEvent:NSEventTypeLeftMouseDown offset:0]];
        [view shutdownTextInput];
        assert(![view dragging]);
        assert(![view hasCaretTimer]);
        printf("native selection passed; view instance bytes=%zu; unchanged selections=1000, events=0, texture rebuilds=0\n",
               class_getInstanceSize([VivaMetalView class]));
        [view release];
    }
    return 0;
}
