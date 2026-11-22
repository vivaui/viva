package dev.viva.host;

import android.app.Activity;
import android.view.Surface;

/** What the Go program asks of the host, on any thread. */
public final class Host {
    private Host() {}

    private static volatile Activity activity;
    private static volatile VivaView view;

    static void attach(Activity current, VivaView surface) {
        activity = current;
        view = surface;
    }

    /**
     * The program's focused text input has this text: the input method reads it from
     * here, and is told when it changes under it.
     */
    public static void editor(String text, int selectionStart, int selectionEnd, int compositionStart, int compositionEnd) {
        Editor.set(text, selectionStart, selectionEnd, compositionStart, compositionEnd);
        final VivaView current = view;
        if (current != null) {
            current.post(current::editorChanged);
        }
    }

    /** Shows the keyboard for a text input, or takes it away when editing is false. */
    public static void keyboard(final boolean editing, final boolean multiline, final boolean secure) {
        final VivaView current = view;
        if (current != null) {
            current.post(() -> current.configure(editing, multiline, secure));
        }
    }

    /** Ends the activity; the program's window is gone with it. */
    public static void finish() {
        final Activity current = activity;
        if (current != null) {
            current.runOnUiThread(current::finishAndRemoveTask);
        }
    }

    /** Names the task in the system's list of recent apps. */
    public static void setTitle(final String title) {
        final Activity current = activity;
        if (current != null) {
            current.runOnUiThread(() -> current.setTitle(title));
        }
    }

    /** Asks the display for the rate the program's frames take, the highest it has. */
    public static void preferHighRefresh(Surface surface) {
        if (surface != null && surface.isValid() && android.os.Build.VERSION.SDK_INT >= 30) {
            surface.setFrameRate(120f, Surface.FRAME_RATE_COMPATIBILITY_DEFAULT);
        }
    }
}
