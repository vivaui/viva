package dev.viva.host;

import android.view.KeyEvent;
import android.view.View;
import android.view.inputmethod.BaseInputConnection;
import android.view.inputmethod.CompletionInfo;
import android.view.inputmethod.ExtractedText;
import android.view.inputmethod.ExtractedTextRequest;

/**
 * What an input method says to the program's focused text input. Each edit is sent to
 * the program and answered when it has been applied, with the editor's state (Editor)
 * as it is after, because a keyboard asks what is around the cursor as soon as it has
 * typed.
 */
final class VivaInputConnection extends BaseInputConnection {
    VivaInputConnection(View view) {
        super(view, false);
    }

    @Override
    public boolean commitText(CharSequence text, int newCursorPosition) {
        Native.commitText(text.toString(), newCursorPosition);
        return true;
    }

    @Override
    public boolean setComposingText(CharSequence text, int newCursorPosition) {
        Native.setComposingText(text.toString(), newCursorPosition);
        return true;
    }

    @Override
    public boolean finishComposingText() {
        Native.finishComposing();
        return true;
    }

    @Override
    public boolean deleteSurroundingText(int before, int after) {
        Native.deleteSurrounding(before, after, false);
        return true;
    }

    @Override
    public boolean deleteSurroundingTextInCodePoints(int before, int after) {
        Native.deleteSurrounding(before, after, true);
        return true;
    }

    @Override
    public boolean setSelection(int start, int end) {
        Native.setSelection(start, end);
        return true;
    }

    @Override
    public boolean setComposingRegion(int start, int end) {
        Native.setComposingRegion(start, end);
        return true;
    }

    @Override
    public boolean performEditorAction(int action) {
        Native.editorAction(action);
        return true;
    }

    @Override
    public boolean sendKeyEvent(KeyEvent event) {
        // A soft keyboard's Delete and Enter come as keys.
        if (event.getAction() == KeyEvent.ACTION_DOWN) {
            Native.key(event.getKeyCode(), event.getMetaState(), event.getUnicodeChar(event.getMetaState()), event.getRepeatCount() > 0);
        }
        return true;
    }

    @Override
    public CharSequence getTextBeforeCursor(int length, int flags) {
        return Editor.before(length);
    }

    @Override
    public CharSequence getTextAfterCursor(int length, int flags) {
        return Editor.after(length);
    }

    @Override
    public CharSequence getSelectedText(int flags) {
        return Editor.selected();
    }

    @Override
    public int getCursorCapsMode(int reqModes) {
        return 0;
    }

    @Override
    public ExtractedText getExtractedText(ExtractedTextRequest request, int flags) {
        ExtractedText extracted = new ExtractedText();
        extracted.text = Editor.text();
        extracted.selectionStart = Editor.selectionStart();
        extracted.selectionEnd = Editor.selectionEnd();
        return extracted;
    }

    @Override
    public boolean commitCompletion(CompletionInfo text) {
        return false;
    }

    @Override
    public boolean beginBatchEdit() {
        return true;
    }

    @Override
    public boolean endBatchEdit() {
        return true;
    }
}
