package dev.viva.host;

import android.app.Activity;
import android.content.res.Configuration;
import android.os.Bundle;
import android.view.MotionEvent;
import android.view.SurfaceHolder;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.widget.FrameLayout;

/**
 * The window of a Viva program: one SurfaceView that fills the screen, which the
 * program draws on with Vulkan. Everything the user does reaches the program as
 * what happened; nothing here lays anything out.
 */
public class VivaActivity extends Activity implements SurfaceHolder.Callback {
    private VivaView view;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        view = new VivaView(this);
        Host.attach(this, view);
        view.getHolder().addCallback(this);
        view.setOnTouchListener(this::onTouch);
        // The surface lies inside what the system covers: its bars, the display cutout
        // and the keyboard. A program that wants the whole screen asks for it.
        FrameLayout root = new FrameLayout(this);
        root.addView(view, new FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT));
        root.setOnApplyWindowInsetsListener(VivaActivity::applyInsets);
        setContentView(root);
        Native.start(getFilesDir().getAbsolutePath(), getCacheDir().getAbsolutePath());
    }

    @Override
    protected void onStart() {
        super.onStart();
        Native.visible(true);
    }

    @Override
    protected void onStop() {
        Native.visible(false);
        super.onStop();
    }

    @Override
    public void onLowMemory() {
        super.onLowMemory();
        Native.lowMemory();
    }

    @Override
    public void onConfigurationChanged(Configuration config) {
        super.onConfigurationChanged(config);
        Native.appearanceChanged();
    }

    @Override
    public void onBackPressed() {
        Native.closeRequested();
    }

    @Override
    public void surfaceCreated(SurfaceHolder holder) {}

    @Override
    public void surfaceChanged(SurfaceHolder holder, int format, int width, int height) {
        Host.preferHighRefresh(holder.getSurface());
        Native.surfaceChanged(holder.getSurface(), width, height, getResources().getDisplayMetrics().density);
    }

    @Override
    public void surfaceDestroyed(SurfaceHolder holder) {
        Native.surfaceDestroyed();
    }

    private static WindowInsets applyInsets(View root, WindowInsets insets) {
        if (android.os.Build.VERSION.SDK_INT >= 30) {
            android.graphics.Insets bars = insets.getInsets(
                    WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout() | WindowInsets.Type.ime());
            root.setPadding(bars.left, bars.top, bars.right, bars.bottom);
            return WindowInsets.CONSUMED;
        }
        root.setPadding(insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetTop(),
                insets.getSystemWindowInsetRight(), insets.getSystemWindowInsetBottom());
        return insets.consumeSystemWindowInsets();
    }

    private boolean onTouch(View v, MotionEvent event) {
        int action = event.getActionMasked();
        long time = event.getEventTime();
        int buttons = event.getButtonState();
        int modifiers = event.getMetaState();
        switch (action) {
            case MotionEvent.ACTION_MOVE:
                for (int i = 0; i < event.getPointerCount(); i++) {
                    Native.touch(action, event.getPointerId(i), event.getX(i), event.getY(i), event.getToolType(i), buttons, modifiers, time);
                }
                break;
            case MotionEvent.ACTION_DOWN:
            case MotionEvent.ACTION_POINTER_DOWN:
            case MotionEvent.ACTION_UP:
            case MotionEvent.ACTION_POINTER_UP: {
                int i = event.getActionIndex();
                Native.touch(action, event.getPointerId(i), event.getX(i), event.getY(i), event.getToolType(i), buttons, modifiers, time);
                break;
            }
            case MotionEvent.ACTION_CANCEL:
                for (int i = 0; i < event.getPointerCount(); i++) {
                    Native.touch(action, event.getPointerId(i), event.getX(i), event.getY(i), event.getToolType(i), buttons, modifiers, time);
                }
                break;
            default:
                return false;
        }
        return true;
    }
}
