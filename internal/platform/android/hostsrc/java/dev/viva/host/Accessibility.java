package dev.viva.host;

import android.graphics.Rect;
import android.os.Bundle;
import android.view.View;
import android.view.accessibility.AccessibilityEvent;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.accessibility.AccessibilityNodeProvider;

import java.util.ArrayList;
import java.util.List;

/**
 * The program's accessibility tree, shown to TalkBack and the other services as virtual
 * views of the surface. The program sends the tree whole whenever it changes (Host.tree),
 * and a service's requests go back to it as what to do to which node (Native.accessible).
 */
final class Accessibility extends AccessibilityNodeProvider {
    // The roles, in the program's numbering (platform.A11yRole*).
    private static final int BUTTON = 3, TEXT = 4, TEXT_FIELD = 5, CHECKBOX = 6, SLIDER = 7, IMAGE = 8, LINK = 9, RADIO = 10,
            TAB = 11, SWITCH = 14, LIST_BOX = 17, OPTION = 18, HEADING = 21, STATUS = 22, PROGRESS = 24;
    // The flags of a node.
    private static final int FOCUSED = 1, DISABLED = 2, CHECKED = 4, MIXED = 8, SELECTED = 16, EXPANDED = 32, MULTILINE = 64,
            READ_ONLY = 128, INVALID = 256, SECURE = 512, FOCUSABLE = 1024, EXPANDABLE = 2048, HAS_RANGE = 4096;
    // What a service asks of a node (platform.Accessibility*).
    static final int PRESS = 1, FOCUS = 2, INCREMENT = 3, DECREMENT = 4, SET_NUMBER = 5, SET_TEXT = 6, SET_SELECTION = 7;

    private static final class Node {
        int role, flags, level, position, size;
        float x, y, width, height;
        float min, max, value;
        String label, text, hint, description;
    }

    private final View view;
    private volatile Node[] nodes = new Node[0];
    private volatile boolean enabled;

    Accessibility(View view) {
        this.view = view;
    }

    boolean enabled() {
        return enabled;
    }

    void setEnabled(boolean on) {
        enabled = on;
    }

    /** The tree the program has now. */
    void set(float[] bounds, int[] ints, float[] ranges, char[] chars, int[] offsets) {
        int count = bounds.length / 4;
        Node[] next = new Node[count];
        for (int i = 0; i < count; i++) {
            Node node = new Node();
            node.x = bounds[4 * i];
            node.y = bounds[4 * i + 1];
            node.width = bounds[4 * i + 2];
            node.height = bounds[4 * i + 3];
            node.role = ints[5 * i];
            node.flags = ints[5 * i + 1];
            node.level = ints[5 * i + 2];
            node.position = ints[5 * i + 3];
            node.size = ints[5 * i + 4];
            node.min = ranges[3 * i];
            node.max = ranges[3 * i + 1];
            node.value = ranges[3 * i + 2];
            node.label = text(chars, offsets, 4 * i);
            node.text = text(chars, offsets, 4 * i + 1);
            node.hint = text(chars, offsets, 4 * i + 2);
            node.description = text(chars, offsets, 4 * i + 3);
            next[i] = node;
        }
        nodes = next;
        view.post(() -> view.sendAccessibilityEvent(AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED));
    }

    private static String text(char[] chars, int[] offsets, int index) {
        return new String(chars, offsets[index], offsets[index + 1] - offsets[index]);
    }

    private static String className(Node node) {
        switch (node.role) {
            case BUTTON:
            case LINK:
                return "android.widget.Button";
            case TEXT_FIELD:
                return "android.widget.EditText";
            case CHECKBOX:
                return "android.widget.CheckBox";
            case SWITCH:
                return "android.widget.Switch";
            case RADIO:
                return "android.widget.RadioButton";
            case SLIDER:
                return "android.widget.SeekBar";
            case PROGRESS:
                return "android.widget.ProgressBar";
            case IMAGE:
                return "android.widget.ImageView";
            case LIST_BOX:
                return "android.widget.ListView";
            case TEXT:
            case HEADING:
            case STATUS:
                return "android.widget.TextView";
            default:
                return "android.view.View";
        }
    }

    @Override
    public AccessibilityNodeInfo createAccessibilityNodeInfo(int virtualViewId) {
        Node[] tree = nodes;
        if (virtualViewId == View.NO_ID) {
            AccessibilityNodeInfo info = AccessibilityNodeInfo.obtain(view);
            view.onInitializeAccessibilityNodeInfo(info);
            for (int i = 0; i < tree.length; i++) {
                info.addChild(view, i);
            }
            return info;
        }
        if (virtualViewId < 0 || virtualViewId >= tree.length) {
            return null;
        }
        Node node = tree[virtualViewId];
        AccessibilityNodeInfo info = AccessibilityNodeInfo.obtain(view, virtualViewId);
        info.setPackageName(view.getContext().getPackageName());
        info.setClassName(className(node));
        info.setParent(view);
        info.setEnabled((node.flags & DISABLED) == 0);
        info.setFocusable((node.flags & FOCUSABLE) != 0);
        info.setVisibleToUser(true);
        info.setContentDescription(node.label.isEmpty() ? null : node.label);
        if (!node.text.isEmpty()) {
            info.setText(node.text);
        } else if (node.role == TEXT || node.role == HEADING || node.role == STATUS) {
            info.setText(node.label);
        }
        if (!node.hint.isEmpty() && android.os.Build.VERSION.SDK_INT >= 26) {
            info.setHintText(node.hint);
        }
        if (!node.description.isEmpty() && android.os.Build.VERSION.SDK_INT >= 30) {
            info.setStateDescription(node.description);
        }
        if (node.role == HEADING) {
            info.setHeading(true);
        }
        if (node.role == CHECKBOX || node.role == SWITCH || node.role == RADIO) {
            info.setCheckable(true);
            info.setChecked((node.flags & CHECKED) != 0);
        }
        if ((node.flags & SELECTED) != 0) {
            info.setSelected(true);
        }
        if ((node.flags & INVALID) != 0) {
            info.setContentInvalid(true);
        }
        if (node.role == TEXT_FIELD) {
            info.setEditable((node.flags & READ_ONLY) == 0);
            info.setMultiLine((node.flags & MULTILINE) != 0);
            info.setPassword((node.flags & SECURE) != 0);
            if ((node.flags & READ_ONLY) == 0) {
                info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SET_TEXT);
            }
        }
        if ((node.flags & HAS_RANGE) != 0) {
            info.setRangeInfo(AccessibilityNodeInfo.RangeInfo.obtain(AccessibilityNodeInfo.RangeInfo.RANGE_TYPE_FLOAT, node.min, node.max, node.value));
            if (node.role == SLIDER) {
                info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_FORWARD);
                info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_BACKWARD);
                if (android.os.Build.VERSION.SDK_INT >= 24) {
                    info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SET_PROGRESS);
                }
            }
        }
        if (node.size > 0 && (node.role == TAB || node.role == OPTION || node.role == RADIO)) {
            info.setCollectionItemInfo(AccessibilityNodeInfo.CollectionItemInfo.obtain(Math.max(node.position - 1, 0), 1, 0, 1, false, (node.flags & SELECTED) != 0));
        }
        boolean operable = (node.flags & DISABLED) == 0;
        if (operable && (node.role == BUTTON || node.role == LINK || node.role == CHECKBOX || node.role == SWITCH || node.role == RADIO
                || node.role == TAB || node.role == OPTION || node.role == 12 || node.role == 20)) {
            info.setClickable(true);
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_CLICK);
        }
        if ((node.flags & EXPANDABLE) != 0 && operable) {
            info.addAction((node.flags & EXPANDED) != 0 ? AccessibilityNodeInfo.AccessibilityAction.ACTION_COLLAPSE : AccessibilityNodeInfo.AccessibilityAction.ACTION_EXPAND);
        }
        if ((node.flags & FOCUSABLE) != 0) {
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_FOCUS);
        }
        info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_ACCESSIBILITY_FOCUS);
        int[] origin = new int[2];
        view.getLocationOnScreen(origin);
        Rect screen = new Rect(origin[0] + Math.round(node.x), origin[1] + Math.round(node.y),
                origin[0] + Math.round(node.x + node.width), origin[1] + Math.round(node.y + node.height));
        info.setBoundsInScreen(screen);
        Rect parent = new Rect(Math.round(node.x), Math.round(node.y), Math.round(node.x + node.width), Math.round(node.y + node.height));
        info.setBoundsInParent(parent);
        return info;
    }

    @Override
    public boolean performAction(int virtualViewId, int action, Bundle arguments) {
        Node[] tree = nodes;
        if (virtualViewId == View.NO_ID) {
            return view.performAccessibilityAction(action, arguments);
        }
        if (virtualViewId < 0 || virtualViewId >= tree.length) {
            return false;
        }
        Node node = tree[virtualViewId];
        switch (action) {
            case AccessibilityNodeInfo.ACTION_CLICK:
            case AccessibilityNodeInfo.ACTION_EXPAND:
            case AccessibilityNodeInfo.ACTION_COLLAPSE:
                Native.accessible(virtualViewId, PRESS, 0, null, 0, 0);
                return true;
            case AccessibilityNodeInfo.ACTION_FOCUS:
                Native.accessible(virtualViewId, FOCUS, 0, null, 0, 0);
                return true;
            case AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS: {
                AccessibilityEvent event = AccessibilityEvent.obtain(AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUSED);
                event.setSource(view, virtualViewId);
                view.getParent().requestSendAccessibilityEvent(view, event);
                return true;
            }
            case AccessibilityNodeInfo.ACTION_SCROLL_FORWARD:
                Native.accessible(virtualViewId, INCREMENT, 0, null, 0, 0);
                return true;
            case AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD:
                Native.accessible(virtualViewId, DECREMENT, 0, null, 0, 0);
                return true;
            case AccessibilityNodeInfo.ACTION_SET_TEXT: {
                CharSequence text = arguments == null ? null : arguments.getCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE);
                Native.accessible(virtualViewId, SET_TEXT, 0, text == null ? "" : text.toString(), 0, 0);
                return true;
            }
            default:
                if (android.os.Build.VERSION.SDK_INT >= 24 && action == android.R.id.accessibilityActionSetProgress && arguments != null) {
                    Native.accessible(virtualViewId, SET_NUMBER, arguments.getFloat(AccessibilityNodeInfo.ACTION_ARGUMENT_PROGRESS_VALUE), null, 0, 0);
                    return true;
                }
                return false;
        }
    }
}
