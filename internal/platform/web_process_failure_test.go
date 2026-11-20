package platform

import "testing"

func TestEachWebView2ProcessFailureNamesItsReason(t *testing.T) {
	for _, c := range []struct {
		kind, reason int32
		want         WebProcessFailure
	}{
		{0, 0, WebProcessCrashed},       // BROWSER_PROCESS_EXITED, UNEXPECTED
		{1, 3, WebProcessCrashed},       // RENDER_PROCESS_EXITED, CRASHED
		{1, 0, WebProcessCrashed},       // RENDER_PROCESS_EXITED, UNEXPECTED
		{1, 2, WebProcessKilled},        // RENDER_PROCESS_EXITED, TERMINATED
		{1, 5, WebProcessKilled},        // RENDER_PROCESS_EXITED, OUT_OF_MEMORY
		{1, 4, WebProcessLaunchFailed},  // RENDER_PROCESS_EXITED, LAUNCH_FAILED
		{1, 1, WebProcessUnresponsive},  // RENDER_PROCESS_EXITED, UNRESPONSIVE
		{1, 6, WebProcessUnknown},       // 6 is the deprecated PROFILE_DELETED, never reported now
		{1, 7, WebProcessUnknown},       // a reason newer than this package
		{1, -1, WebProcessUnknown},      // a runtime too old to say why
		{2, 1, WebProcessUnresponsive},  // RENDER_PROCESS_UNRESPONSIVE, UNRESPONSIVE
		{2, 3, WebProcessUnresponsive},  // the kind decides: an unresponsive process is not crashed
		{2, -1, WebProcessUnresponsive}, // a runtime too old to say why
		{0, -1, WebProcessUnknown},      // a browser process gone with no reason given
	} {
		got, ok := WebProcessFailureFromWebView2(c.kind, c.reason)
		if !ok || got != c.want {
			t.Errorf("kind %d reason %d = %v, %v, want %v, true", c.kind, c.reason, got, ok, c.want)
		}
	}
}

func TestAWebView2ProcessThePageSurvivesIsNotReported(t *testing.T) {
	for _, kind := range []int32{
		3,  // FRAME_RENDER_PROCESS_EXITED: a frame's own renderer, replaced by an error page
		4,  // UTILITY_PROCESS_EXITED
		5,  // SANDBOX_HELPER_PROCESS_EXITED
		6,  // GPU_PROCESS_EXITED
		7,  // PPAPI_PLUGIN_PROCESS_EXITED
		8,  // PPAPI_BROKER_PROCESS_EXITED
		9,  // UNKNOWN_PROCESS_EXITED
		10, // a kind newer than this package
		-1, // never reported, but not a page's process either
	} {
		if got, ok := WebProcessFailureFromWebView2(kind, 3); ok || got != WebProcessNoFailure {
			t.Errorf("kind %d = %v, %v, want the page not to hear of it", kind, got, ok)
		}
	}
}

func TestEachWebKitTerminationReasonNamesItsFailure(t *testing.T) {
	for _, c := range []struct {
		reason int32
		want   WebProcessFailure
	}{
		{0, WebProcessCrashed}, // WEBKIT_WEB_PROCESS_CRASHED
		{1, WebProcessKilled},  // WEBKIT_WEB_PROCESS_EXCEEDED_MEMORY_LIMIT
		{2, WebProcessKilled},  // WEBKIT_WEB_PROCESS_TERMINATED_BY_API
		{3, WebProcessUnknown}, // a reason newer than this package
		{-1, WebProcessUnknown},
	} {
		if got := WebProcessFailureFromWebKitTerminationReason(c.reason); got != c.want {
			t.Errorf("reason %d = %v, want %v", c.reason, got, c.want)
		}
	}
}

func TestDarwinTerminationNamesNoReason(t *testing.T) {
	if WebProcessFailureDarwinTermination != WebProcessUnknown {
		t.Fatalf("a WKWebView's termination = %v, want the reason it does not give", WebProcessFailureDarwinTermination)
	}
}

func TestEachProcessFailureNamesItself(t *testing.T) {
	for _, c := range []struct {
		failure WebProcessFailure
		want    string
	}{
		{WebProcessNoFailure, "no failure"},
		{WebProcessCrashed, "crashed"},
		{WebProcessKilled, "killed"},
		{WebProcessUnresponsive, "unresponsive"},
		{WebProcessLaunchFailed, "launch failed"},
		{WebProcessUnknown, "unknown failure"},
		{WebProcessFailure(200), "web process failure 200"},
	} {
		if got := c.failure.String(); got != c.want {
			t.Errorf("%d = %q, want %q", c.failure, got, c.want)
		}
	}
}

func TestAPageWhoseProcessIsThereNamesNoFailure(t *testing.T) {
	var failure WebProcessFailure
	if failure != WebProcessNoFailure {
		t.Fatalf("a zero WebProcessFailure = %v, want a page that has its process", failure)
	}
}
