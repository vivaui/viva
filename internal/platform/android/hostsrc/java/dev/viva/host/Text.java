package dev.viva.host;

import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.RectF;
import android.graphics.Typeface;
import android.os.Build;
import android.text.Layout;
import android.text.StaticLayout;
import android.text.TextPaint;

import java.nio.ByteBuffer;

/**
 * What the Go program asks of the system's text drawing: the width of a line, the
 * extent of each of its grapheme clusters, and its pixels. The system shapes with
 * Minikin and HarfBuzz and draws with Skia, so what the program measures is what is
 * drawn. The program asks on one thread and caches what it learns; none of this is
 * called for a line it has seen.
 */
public final class Text {
    private Text() {}

    private static final TextPaint paint = new TextPaint(Paint.ANTI_ALIAS_FLAG | Paint.SUBPIXEL_TEXT_FLAG);
    private static final Path path = new Path();
    private static final RectF bounds = new RectF();
    private static final float[] advances = new float[256];

    /** The family the interface's proportional text is shaped in; null for the system's. */
    private static volatile String family;

    /** Moves on every change after which the same line shapes differently. */
    public static void setFamily(String name) {
        family = (name == null || name.isEmpty()) ? null : name;
    }

    private static Typeface face(int weight, boolean italic, boolean mono) {
        Typeface base = mono ? Typeface.MONOSPACE : null;
        if (base == null) {
            String name = family;
            base = name == null ? Typeface.DEFAULT : Typeface.create(name, Typeface.NORMAL);
        }
        if (Build.VERSION.SDK_INT >= 28) {
            return Typeface.create(base, weight, italic);
        }
        int style = (weight >= 600 ? Typeface.BOLD : Typeface.NORMAL) | (italic ? Typeface.ITALIC : 0);
        return Typeface.create(base, style);
    }

    private static TextPaint set(float size, int weight, boolean italic, boolean mono) {
        paint.setTypeface(face(weight, italic, mono));
        paint.setTextSize(size);
        return paint;
    }

    /** The advance of the whole line. */
    public static float width(String value, float size, int weight, boolean italic, boolean mono) {
        return set(size, weight, italic, mono).measureText(value);
    }

    /** The ascent and descent of the font at size, and the gap it asks between lines, in out. */
    public static void metrics(float size, int weight, boolean italic, boolean mono, float[] out) {
        Paint.FontMetrics fm = set(size, weight, italic, mono).getFontMetrics();
        out[0] = -fm.ascent;
        out[1] = fm.descent;
        out[2] = fm.leading;
    }

    /**
     * The horizontal extent, from the left end of the line, of each range
     * [bounds[i], bounds[i+1]) of value, in UTF-16 units: left to right in lefts and
     * rights. A line that is all left-to-right adds up the advances the font gives;
     * one with right-to-left text asks the layout, which orders the runs.
     */
    public static void rects(String value, float size, int weight, boolean italic, boolean mono, boolean rtl,
            int[] boundaries, float[] lefts, float[] rights) {
        TextPaint p = set(size, weight, italic, mono);
        int n = value.length();
        int count = boundaries.length - 1;
        if (!rtl && !hasRightToLeft(value)) {
            char[] chars = value.toCharArray();
            float[] advance = n <= advances.length ? advances : new float[n];
            p.getTextRunAdvances(chars, 0, n, 0, n, false, advance, 0);
            float x = 0;
            int unit = 0;
            for (int i = 0; i < count; i++) {
                float start = x;
                // Ligatures give the first unit the whole advance and the rest none,
                // so summing the units of a cluster is the cluster's width.
                for (; unit < boundaries[i + 1]; unit++) {
                    x += advance[unit];
                }
                lefts[i] = start;
                rights[i] = x;
            }
            return;
        }
        StaticLayout layout = StaticLayout.Builder.obtain(value, 0, n, p, 1 << 24)
                .setIncludePad(false)
                .setAlignment(Layout.Alignment.ALIGN_NORMAL)
                .setTextDirection(rtl ? android.text.TextDirectionHeuristics.RTL : android.text.TextDirectionHeuristics.LTR)
                .build();
        for (int i = 0; i < count; i++) {
            path.rewind();
            layout.getSelectionPath(boundaries[i], boundaries[i + 1], path);
            path.computeBounds(bounds, true);
            lefts[i] = bounds.left;
            rights[i] = bounds.right;
        }
    }

    private static boolean hasRightToLeft(String value) {
        for (int i = 0; i < value.length(); i++) {
            byte d = Character.getDirectionality(value.charAt(i));
            if (d == Character.DIRECTIONALITY_RIGHT_TO_LEFT || d == Character.DIRECTIONALITY_RIGHT_TO_LEFT_ARABIC
                    || d == Character.DIRECTIONALITY_RIGHT_TO_LEFT_EMBEDDING || d == Character.DIRECTIONALITY_RIGHT_TO_LEFT_OVERRIDE
                    || d == Character.DIRECTIONALITY_ARABIC_NUMBER) {
                return true;
            }
        }
        return false;
    }

    private static Bitmap bitmap;
    private static Canvas canvas;

    /** Starts a transparent bitmap of width x height pixels that draw adds runs to. */
    public static void begin(int width, int height) {
        if (bitmap == null || bitmap.getWidth() != width || bitmap.getHeight() != height) {
            if (bitmap != null) {
                bitmap.recycle();
            }
            bitmap = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888);
            canvas = new Canvas(bitmap);
        }
        bitmap.eraseColor(Color.TRANSPARENT);
    }

    /** Draws a run of text with its left end at x and its baseline at baseline, in pixels. */
    public static void draw(String value, float x, float baseline, float size, int weight, boolean italic, boolean mono,
            int argb, boolean underline, boolean rtl) {
        TextPaint p = set(size, weight, italic, mono);
        p.setColor(argb);
        p.setUnderlineText(underline);
        if (rtl) {
            // Draw from the right end: the run's own direction lays its glyphs out.
            canvas.drawText(value, 0, value.length(), x, baseline, p);
        } else {
            canvas.drawText(value, 0, value.length(), x, baseline, p);
        }
        p.setUnderlineText(false);
    }

    /** Copies the pixels, premultiplied, in the order of Bitmap's ARGB_8888 (RGBA bytes), to out. */
    public static void end(ByteBuffer out) {
        out.clear();
        bitmap.copyPixelsToBuffer(out);
    }
}
