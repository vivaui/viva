package dev.viva.host;

import android.app.Activity;
import android.content.res.Configuration;
import android.os.Bundle;
import android.view.MotionEvent;
import android.view.SurfaceHolder;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.WindowManager;
import android.graphics.Color;
import android.os.Build;
import android.widget.FrameLayout;

/**
 * The window of a Viva program: one SurfaceView that fills the screen, which the
 * program draws on with Vulkan. Everything the user does reaches the program as
 * what happened; nothing here lays anything out.
 */
public class VivaActivity extends Activity implements SurfaceHolder.Callback2 {
    private VivaView view;

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        view = new VivaView(this);
        Host.attach(this, view);
        view.getHolder().addCallback(this);
        view.setOnTouchListener(this::onTouch);
        // The window runs under the system bars and the display cutout, as an Android 15 window
        // must; the program is told how far they cover each edge and keeps what matters inside
        // them. The keyboard still shrinks the surface from the bottom.
        edgeToEdge();
        FrameLayout root = new FrameLayout(this);
        root.addView(view, new FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT));
        root.setOnApplyWindowInsetsListener(VivaActivity::applyInsets);
        setContentView(root);
        barIcons(); // needs the decor view, which setContentView installs
        if (Build.VERSION.SDK_INT >= 31) {
            // The window is drawn on the surface, which the system's reveal of the splash screen
            // does not follow: left to it, the reveal stops part way and leaves the window faded
            // until the next layout. The splash screen goes as soon as the window is drawn.
            getSplashScreen().setOnExitAnimationListener(splash -> splash.remove());
        }
        Native.start(getFilesDir().getAbsolutePath(), getCacheDir().getAbsolutePath());
    }

    private void edgeToEdge() {
        Window window = getWindow();
        if (Build.VERSION.SDK_INT >= 30) {
            window.setDecorFitsSystemWindows(false);
        } else {
            window.getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                    | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN);
        }
        window.setStatusBarColor(Color.TRANSPARENT);
        window.setNavigationBarColor(Color.TRANSPARENT);
        if (Build.VERSION.SDK_INT >= 29) {
            window.setNavigationBarContrastEnforced(false);
            window.setStatusBarContrastEnforced(false);
        }
        if (Build.VERSION.SDK_INT >= 28) {
            WindowManager.LayoutParams params = window.getAttributes();
            params.layoutInDisplayCutoutMode = Build.VERSION.SDK_INT >= 30
                    ? WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_ALWAYS
                    : WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
            window.setAttributes(params);
        }
    }

    /** The bars' icons are dark on the light theme and light on the dark one, as the system's own are. */
    private void barIcons() {
        Window window = getWindow();
        boolean night = (getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK)
                == Configuration.UI_MODE_NIGHT_YES;
        if (Build.VERSION.SDK_INT >= 30) {
            WindowInsetsController controller = window.getInsetsController();
            if (controller != null) {
                int mask = WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS
                        | WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS;
                controller.setSystemBarsAppearance(night ? 0 : mask, mask);
            }
        } else if (Build.VERSION.SDK_INT >= 23) {
            View decor = window.getDecorView();
            int light = View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR
                    | (Build.VERSION.SDK_INT >= 26 ? View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR : 0);
            int flags = decor.getSystemUiVisibility();
            decor.setSystemUiVisibility(night ? flags & ~light : flags | light);
        }
    }

    private boolean stopped;

    @Override
    protected void onStart() {
        super.onStart();
        Native.visible(true);
        if (stopped) {
            Native.lifecycle(2); // Foreground: it came back, it is not the first start
        }
    }

    @Override
    protected void onStop() {
        Native.visible(false);
        stopped = true;
        Native.lifecycle(1); // Background: the time to save what the user would miss
        super.onStop();
    }

    @Override
    public void onTrimMemory(int level) {
        super.onTrimMemory(level);
        // From the point the system runs short (TRIM_MEMORY_RUNNING_LOW) on; the level that
        // only says the user interface is hidden (TRIM_MEMORY_UI_HIDDEN) is Background's.
        if (level >= TRIM_MEMORY_RUNNING_LOW && level != TRIM_MEMORY_UI_HIDDEN) {
            Native.lowMemory();
        }
    }

    @Override
    public void onLowMemory() {
        super.onLowMemory();
        Native.lowMemory();
    }

    @Override
    public void onConfigurationChanged(Configuration config) {
        super.onConfigurationChanged(config);
        barIcons();
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

    /** The system waits for this before it shows the surface, so the program draws on it first. */
    @Override
    public void surfaceRedrawNeeded(SurfaceHolder holder) {
        Native.redrawNeeded();
    }

    @Override
    public void surfaceDestroyed(SurfaceHolder holder) {
        Native.surfaceDestroyed();
    }

    /**
     * The surface is the whole window, less the keyboard from the bottom; the program is told how
     * far the bars and the cutout cover each edge (the bottom one is not covered with the
     * keyboard up, which covers it instead).
     */
    private static WindowInsets applyInsets(View root, WindowInsets insets) {
        if (Build.VERSION.SDK_INT >= 30) {
            android.graphics.Insets bars = insets.getInsets(
                    WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout());
            boolean keyboard = insets.isVisible(WindowInsets.Type.ime());
            root.setPadding(0, 0, 0, keyboard ? insets.getInsets(WindowInsets.Type.ime()).bottom : 0);
            Native.insets(bars.left, bars.top, bars.right, keyboard ? 0 : bars.bottom);
            return WindowInsets.CONSUMED;
        }
        int bottom = insets.getSystemWindowInsetBottom();
        boolean keyboard = bottom > insets.getStableInsetBottom();
        root.setPadding(0, 0, 0, keyboard ? bottom : 0);
        Native.insets(insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetTop(),
                insets.getSystemWindowInsetRight(), keyboard ? 0 : insets.getStableInsetBottom());
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
