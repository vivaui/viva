package dev.viva.host;

import android.view.Surface;

/**
 * The calls the host makes into the Go program, which runs in this process as a
 * shared library. They are made on the UI thread and only queue what happened: the
 * program works on a thread of its own.
 */
public final class Native {
    private Native() {}

    static {
        System.loadLibrary("viva");
    }

    /** Starts the program once for the process; later calls change nothing. */
    public static native void start(String filesDir, String cacheDir);

    /** The surface of the window has a size (and the display a density), or a new one. */
    public static native void surfaceChanged(Surface surface, int width, int height, float density);

    /**
     * The system shows the surface once it has a frame: the program draws the window on it
     * and this returns when the frame is presented.
     */
    public static native void redrawNeeded();

    /** The surface is going away; the program must stop drawing to it before this returns. */
    public static native void surfaceDestroyed();

    /** One finger or pen: its action, and where, in the surface's pixels. */
    public static native void touch(int action, int pointer, float x, float y, int toolType, int buttons, int modifiers, long uptimeMillis);

    /** The activity is in front (true) or has gone behind another (false). */
    public static native void visible(boolean visible);

    /** The theme, the language or the animation setting may have changed. */
    public static native void appearanceChanged();

    /** The activity went behind another (1) or came back (2): the program's Background and Foreground. */
    public static native void lifecycle(int kind);

    /** The system is short of memory. */
    public static native void lowMemory();

    /**
     * How far the system covers each edge of the surface, in its pixels: the bars, the display
     * cutout, and the navigation bar unless the keyboard is up. Sent before the surface is
     * known and whenever it changes.
     */
    public static native void insets(float left, float top, float right, float bottom);

    /** The user asked to close the window (the back gesture at the root). */
    public static native void closeRequested();

    // The input method's edits of the program's focused text input, in UTF-16 units. Each
    // returns when the program has applied it and sent the editor's state (Host.editor).

    public static native void commitText(String text, int newCursorPosition);

    public static native void setComposingText(String text, int newCursorPosition);

    public static native void finishComposing();

    public static native void deleteSurrounding(int before, int after, boolean codePoints);

    public static native void setSelection(int start, int end);

    public static native void setComposingRegion(int start, int end);

    public static native void editorAction(int action);

    /** A key of a keyboard: its Android key code, the meta state and the character it types. */
    public static native void key(int keyCode, int meta, int unicode, boolean repeat);

    /** A screen reader is reading the window, or has stopped. */
    public static native void accessibility(boolean on);

    /** A screen reader's request of a node: its place in the tree last sent, what to do, and the value of it. */
    public static native void accessible(int node, int action, float number, String text, int start, int end);

    /** A mouse over the surface. */
    public static native void hover(float x, float y, int meta, long uptimeMillis);

    public static native void hoverExit();

    /** A wheel, in pixels. */
    public static native void scroll(float x, float y, float dx, float dy, int meta);
}
