package dev.viva.host;

import android.content.Context;
import android.text.InputType;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.SurfaceView;
import android.view.ViewConfiguration;
import android.view.accessibility.AccessibilityManager;
import android.view.accessibility.AccessibilityNodeProvider;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputMethodManager;

/**
 * The surface the program draws on, which is also what the keyboard types into when
 * the program has a text input focused.
 */
final class VivaView extends SurfaceView {
    private boolean editing;
    private boolean multiline;
    private boolean secure;

    private final Accessibility accessibility = new Accessibility(this);

    VivaView(Context context) {
        super(context);
        setFocusable(true);
        setFocusableInTouchMode(true);
        setImportantForAccessibility(IMPORTANT_FOR_ACCESSIBILITY_YES);
    }

    Accessibility accessibility() {
        return accessibility;
    }

    @Override
    public AccessibilityNodeProvider getAccessibilityNodeProvider() {
        return accessibility;
    }

    @Override
    protected void onAttachedToWindow() {
        super.onAttachedToWindow();
        AccessibilityManager manager = (AccessibilityManager) getContext().getSystemService(Context.ACCESSIBILITY_SERVICE);
        if (manager != null) {
            Native.accessibility(manager.isEnabled());
            manager.addAccessibilityStateChangeListener(Native::accessibility);
        }
    }

    /** Makes the view a text editor, or not, and restarts the input method on it. */
    void configure(boolean editing, boolean multiline, boolean secure) {
        boolean restart = this.editing != editing || this.multiline != multiline || this.secure != secure;
        this.editing = editing;
        this.multiline = multiline;
        this.secure = secure;
        InputMethodManager imm = (InputMethodManager) getContext().getSystemService(Context.INPUT_METHOD_SERVICE);
        if (imm == null) {
            return;
        }
        if (!editing) {
            imm.hideSoftInputFromWindow(getWindowToken(), 0);
            if (restart) {
                imm.restartInput(this);
            }
            return;
        }
        requestFocus();
        if (restart) {
            imm.restartInput(this);
        }
        imm.showSoftInput(this, 0);
    }

    /** The program changed the text under the input method. */
    void editorChanged() {
        InputMethodManager imm = (InputMethodManager) getContext().getSystemService(Context.INPUT_METHOD_SERVICE);
        if (imm != null && editing) {
            imm.updateSelection(this, Editor.selectionStart(), Editor.selectionEnd(), Editor.compositionStart(), Editor.compositionEnd());
        }
    }

    @Override
    public boolean onCheckIsTextEditor() {
        return editing;
    }

    @Override
    public InputConnection onCreateInputConnection(EditorInfo info) {
        if (!editing) {
            return null;
        }
        int type = InputType.TYPE_CLASS_TEXT;
        if (multiline) {
            type |= InputType.TYPE_TEXT_FLAG_MULTI_LINE;
        }
        if (secure) {
            type |= InputType.TYPE_TEXT_VARIATION_PASSWORD;
        }
        info.inputType = type;
        info.imeOptions = EditorInfo.IME_FLAG_NO_FULLSCREEN | EditorInfo.IME_FLAG_NO_EXTRACT_UI
                | (multiline ? EditorInfo.IME_ACTION_NONE : EditorInfo.IME_ACTION_DONE);
        info.initialSelStart = Editor.selectionStart();
        info.initialSelEnd = Editor.selectionEnd();
        return new VivaInputConnection(this);
    }

    @Override
    public boolean onKeyDown(int keyCode, KeyEvent event) {
        if (keyCode == KeyEvent.KEYCODE_BACK) {
            return super.onKeyDown(keyCode, event);
        }
        Native.key(keyCode, event.getMetaState(), event.getUnicodeChar(event.getMetaState()), event.getRepeatCount() > 0);
        return true;
    }

    @Override
    public boolean onGenericMotionEvent(MotionEvent event) {
        long time = event.getEventTime();
        switch (event.getActionMasked()) {
            case MotionEvent.ACTION_HOVER_MOVE:
            case MotionEvent.ACTION_HOVER_ENTER:
                Native.hover(event.getX(), event.getY(), event.getMetaState(), time);
                return true;
            case MotionEvent.ACTION_HOVER_EXIT:
                Native.hoverExit();
                return true;
            case MotionEvent.ACTION_SCROLL: {
                ViewConfiguration configuration = ViewConfiguration.get(getContext());
                float horizontal = event.getAxisValue(MotionEvent.AXIS_HSCROLL) * configuration.getScaledHorizontalScrollFactor();
                float vertical = event.getAxisValue(MotionEvent.AXIS_VSCROLL) * configuration.getScaledVerticalScrollFactor();
                Native.scroll(event.getX(), event.getY(), horizontal, vertical, event.getMetaState());
                return true;
            }
            default:
                return super.onGenericMotionEvent(event);
        }
    }
}
