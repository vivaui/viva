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

    /** The surface is going away; the program must stop drawing to it before this returns. */
    public static native void surfaceDestroyed();

    /** One finger or pen: its action, and where, in the surface's pixels. */
    public static native void touch(int action, int pointer, float x, float y, int toolType, int buttons, int modifiers, long uptimeMillis);

    /** The activity is in front (true) or has gone behind another (false). */
    public static native void visible(boolean visible);

    /** The system is short of memory. */
    public static native void lowMemory();

    /** The user asked to close the window (the back gesture at the root). */
    public static native void closeRequested();
}
