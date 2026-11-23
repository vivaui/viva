package dev.viva.host;

import android.app.Activity;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Context;
import android.content.Intent;
import android.content.res.Configuration;
import android.net.Uri;
import android.os.LocaleList;
import android.provider.Settings;
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

    /** The program's accessibility tree, whole: its nodes and what a screen reader reads of them. */
    public static void tree(float[] bounds, int[] ints, float[] ranges, char[] chars, int[] offsets) {
        VivaView current = view;
        if (current != null) {
            current.accessibility().set(bounds, ints, ranges, chars, offsets);
        }
    }

    /**
     * Shows the handles and the menu over a selection, or takes them away: where its start and
     * its end are (the bottom of the line at each), how tall the line is, and the rectangle the
     * selection covers, in the surface's pixels.
     */
    public static void selection(final boolean show, final float startX, final float startY, final float endX,
            final float endY, final float lineHeight, final float left, final float top, final float right, final float bottom) {
        final VivaView current = view;
        if (current != null) {
            current.post(() -> current.selection(show, startX, startY, endX, endY, lineHeight, left, top, right, bottom));
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

    /** The text on the clipboard, or null when there is none or the system will not say. */
    public static String clipboardText() {
        Activity current = activity;
        if (current == null) {
            return null;
        }
        ClipboardManager clipboard = (ClipboardManager) current.getSystemService(Context.CLIPBOARD_SERVICE);
        if (clipboard == null || !clipboard.hasPrimaryClip() || clipboard.getPrimaryClip().getItemCount() == 0) {
            return null;
        }
        CharSequence text = clipboard.getPrimaryClip().getItemAt(0).coerceToText(current);
        return text == null ? null : text.toString();
    }

    public static boolean setClipboardText(String text) {
        Activity current = activity;
        if (current == null) {
            return false;
        }
        ClipboardManager clipboard = (ClipboardManager) current.getSystemService(Context.CLIPBOARD_SERVICE);
        if (clipboard == null) {
            return false;
        }
        clipboard.setPrimaryClip(ClipData.newPlainText(null, text));
        return true;
    }

    /** Opens a link in the app that takes it. */
    public static boolean openURL(String url) {
        Activity current = activity;
        if (current == null) {
            return false;
        }
        try {
            current.startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(url)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));
            return true;
        } catch (RuntimeException e) {
            return false;
        }
    }

    /** The user's languages, most preferred first, as BCP 47 tags separated by newlines. */
    public static String languages() {
        LocaleList list = LocaleList.getAdjustedDefault();
        StringBuilder tags = new StringBuilder();
        for (int i = 0; i < list.size(); i++) {
            if (i > 0) {
                tags.append('\n');
            }
            tags.append(list.get(i).toLanguageTag());
        }
        return tags.toString();
    }

    /** 1 when the system shows the dark theme, 0 for the light one. */
    public static int darkTheme() {
        Activity current = activity;
        if (current == null) {
            return -1;
        }
        int night = current.getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK;
        return night == Configuration.UI_MODE_NIGHT_YES ? 1 : 0;
    }

    /** True when the user turned animations off. */
    public static boolean animationsOff() {
        Activity current = activity;
        if (current == null) {
            return false;
        }
        return Settings.Global.getFloat(current.getContentResolver(), Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f;
    }
}
