package dev.viva.host;

import android.app.Activity;
import android.view.Surface;

/** What the Go program asks of the host, on any thread. */
public final class Host {
    private Host() {}

    private static volatile Activity activity;

    static void attach(Activity current) {
        activity = current;
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
