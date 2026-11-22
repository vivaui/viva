package dev.viva.host;

/**
 * The text input the program has focused, as the input method reads it: the text and
 * where the selection and the composing text are, in UTF-16 units. The program is the
 * owner of it and sends it whole after every change; the input method only reads.
 */
final class Editor {
    private Editor() {}

    private static String text = "";
    private static int selectionStart;
    private static int selectionEnd;
    private static int compositionStart = -1;
    private static int compositionEnd = -1;

    static synchronized void set(String value, int start, int end, int compStart, int compEnd) {
        text = value;
        selectionStart = start;
        selectionEnd = end;
        compositionStart = compStart;
        compositionEnd = compEnd;
    }

    static synchronized String text() {
        return text;
    }

    static synchronized int selectionStart() {
        return Math.min(selectionStart, selectionEnd);
    }

    static synchronized int selectionEnd() {
        return Math.max(selectionStart, selectionEnd);
    }

    static synchronized int compositionStart() {
        return compositionStart;
    }

    static synchronized int compositionEnd() {
        return compositionEnd;
    }

    static synchronized CharSequence before(int length) {
        int end = selectionStart();
        return text.substring(Math.max(0, end - length), end);
    }

    static synchronized CharSequence after(int length) {
        int start = selectionEnd();
        return text.substring(start, Math.min(text.length(), start + length));
    }

    static synchronized CharSequence selected() {
        int start = selectionStart();
        int end = selectionEnd();
        return start == end ? null : text.substring(start, end);
    }
}
