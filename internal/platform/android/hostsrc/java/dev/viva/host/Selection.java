package dev.viva.host;

import android.content.Context;
import android.content.res.TypedArray;
import android.graphics.Canvas;
import android.graphics.Rect;
import android.graphics.drawable.Drawable;
import android.os.Build;
import android.view.ActionMode;
import android.view.Gravity;
import android.view.Menu;
import android.view.MenuItem;
import android.view.MotionEvent;
import android.view.View;
import android.widget.Magnifier;
import android.widget.PopupWindow;

/**
 * What the system shows over a selected text: a handle at each end, a magnifier while one
 * is dragged, and the floating menu of Cut, Copy, Paste and Select all. They are the
 * system's own (the theme's handle drawables, the Magnifier, a floating ActionMode), so a
 * selection looks and behaves as in any other app. The program owns the selection: the
 * host says where the ends are, and tells the program where a finger drags a handle and
 * which menu item was chosen.
 */
final class Selection {
    private final VivaView view;
    private final Handle start;
    private final Handle end;
    private final int[] origin = new int[2];
    private Magnifier magnifier;
    private ActionMode menu;
    private float lineHeight;
    private final Rect covered = new Rect();
    private boolean dragging;

    Selection(VivaView view) {
        this.view = view;
        Context context = view.getContext();
        // The handles' drawables and hotspots are the theme's: the left one's tip is at three
        // quarters of its width, the right one's at a quarter of it.
        start = new Handle(context, android.R.attr.textSelectHandleLeft, 0, 0.75f);
        end = new Handle(context, android.R.attr.textSelectHandleRight, 1, 0.25f);
    }

    /** Puts the handles at the ends of the selection (the bottom of the line at each) and the menu over it. */
    void show(float startX, float startY, float endX, float endY, float lineHeight, float left, float top, float right, float bottom) {
        this.lineHeight = lineHeight;
        covered.set((int) left, (int) top, (int) right, (int) bottom);
        view.getLocationOnScreen(origin);
        start.place(startX, startY);
        end.place(endX, endY);
        if (menu == null) {
            menu = view.startActionMode(new MenuCallback(), ActionMode.TYPE_FLOATING);
        } else if (!dragging) {
            menu.invalidateContentRect();
        }
    }

    /** Takes them away: the selection is gone, or the text input is. */
    void hide() {
        start.dismiss();
        end.dismiss();
        dismissMagnifier();
        if (menu != null) {
            ActionMode mode = menu;
            menu = null;
            mode.finish();
        }
    }

    private void showMagnifier(float x, float y) {
        if (Build.VERSION.SDK_INT < 28) {
            return;
        }
        if (magnifier == null) {
            magnifier = new Magnifier(view);
        }
        magnifier.show(x, y);
    }

    private void dismissMagnifier() {
        if (magnifier != null) {
            magnifier.dismiss();
            magnifier = null;
        }
    }

    /** A handle: the theme's drawable in a popup of its own, which follows a finger. */
    private final class Handle extends View {
        private final PopupWindow popup;
        private final Drawable drawable;
        private final int which;
        private final float hotspot;
        private float downX;
        private float downY;
        private float tipX;
        private float tipY;

        Handle(Context context, int attribute, int which, float hotspot) {
            super(context);
            this.which = which;
            this.hotspot = hotspot;
            TypedArray array = context.getTheme().obtainStyledAttributes(new int[] {attribute});
            drawable = array.getDrawable(0);
            array.recycle();
            popup = new PopupWindow(this, drawable.getIntrinsicWidth(), drawable.getIntrinsicHeight());
            popup.setClippingEnabled(false);
            popup.setTouchable(true);
            popup.setFocusable(false);
            setMeasuredDimension(drawable.getIntrinsicWidth(), drawable.getIntrinsicHeight());
        }

        @Override
        protected void onMeasure(int widthSpec, int heightSpec) {
            setMeasuredDimension(drawable.getIntrinsicWidth(), drawable.getIntrinsicHeight());
        }

        @Override
        protected void onDraw(Canvas canvas) {
            drawable.setBounds(0, 0, getWidth(), getHeight());
            drawable.draw(canvas);
        }

        /** Puts the handle's tip at a point of the surface. */
        void place(float x, float y) {
            if (dragging) {
                return; // the finger is placing it
            }
            move(x, y);
        }

        private void move(float x, float y) {
            tipX = x;
            tipY = y;
            int left = (int) (origin[0] + x - drawable.getIntrinsicWidth() * hotspot);
            int top = (int) (origin[1] + y);
            if (popup.isShowing()) {
                popup.update(left, top, -1, -1);
            } else {
                popup.showAtLocation(view, Gravity.NO_GRAVITY, left, top);
            }
        }

        void dismiss() {
            popup.dismiss();
        }

        @Override
        public boolean onTouchEvent(MotionEvent event) {
            switch (event.getActionMasked()) {
                case MotionEvent.ACTION_DOWN:
                    dragging = true;
                    downX = event.getRawX() - (origin[0] + tipX);
                    downY = event.getRawY() - (origin[1] + tipY);
                    Native.selectionHandle(which, 0, 0, 0);
                    return true;
                case MotionEvent.ACTION_MOVE: {
                    // The handle keeps its place under the finger; what it points at is the line
                    // above its tip, half a line up.
                    float x = event.getRawX() - downX - origin[0];
                    float y = event.getRawY() - downY - origin[1];
                    move(x, y);
                    float pointY = y - lineHeight / 2;
                    Native.selectionHandle(which, 1, x, pointY);
                    showMagnifier(x, pointY);
                    return true;
                }
                case MotionEvent.ACTION_UP:
                case MotionEvent.ACTION_CANCEL:
                    dragging = false;
                    dismissMagnifier();
                    Native.selectionHandle(which, 2, 0, 0);
                    if (menu != null) {
                        menu.invalidateContentRect();
                    }
                    return true;
                default:
                    return true;
            }
        }
    }

    private static final int CUT = 1;
    private static final int COPY = 2;
    private static final int PASTE = 3;
    private static final int SELECT_ALL = 4;

    /** The floating menu over the selection; its items are the program's actions. */
    private final class MenuCallback extends ActionMode.Callback2 {
        @Override
        public boolean onCreateActionMode(ActionMode mode, Menu menu) {
            menu.add(Menu.NONE, CUT, 0, android.R.string.cut).setShowAsAction(MenuItem.SHOW_AS_ACTION_ALWAYS);
            menu.add(Menu.NONE, COPY, 1, android.R.string.copy).setShowAsAction(MenuItem.SHOW_AS_ACTION_ALWAYS);
            menu.add(Menu.NONE, PASTE, 2, android.R.string.paste).setShowAsAction(MenuItem.SHOW_AS_ACTION_ALWAYS);
            menu.add(Menu.NONE, SELECT_ALL, 3, android.R.string.selectAll).setShowAsAction(MenuItem.SHOW_AS_ACTION_ALWAYS);
            return true;
        }

        @Override
        public boolean onPrepareActionMode(ActionMode mode, Menu menu) {
            return false;
        }

        @Override
        public boolean onActionItemClicked(ActionMode mode, MenuItem item) {
            Native.selectionAction(item.getItemId());
            return true;
        }

        @Override
        public void onDestroyActionMode(ActionMode mode) {
            // The system took the menu away (a tap elsewhere, back): the selection stays, as
            // in a text field, and its handles with it.
            if (menu == mode) {
                menu = null;
            }
        }

        @Override
        public void onGetContentRect(ActionMode mode, View view, Rect outRect) {
            outRect.set(covered);
        }
    }
}
