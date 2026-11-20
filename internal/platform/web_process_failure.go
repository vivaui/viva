package platform

import "strconv"

// WebProcessFailure is why a page's process is gone, in terms each engine can be held to: the
// engines name the same failures differently, and what an application should say about a page that
// lost its process depends on which it was. A page whose renderer crashed may have lost input the
// user typed into it and is worth warning about before it is loaded again; a page whose renderer
// the system reclaimed under memory pressure did nothing wrong, and a reload is all it needs.
// Zero is a page whose process is there.
type WebProcessFailure uint8

const (
	// WebProcessNoFailure is a page whose process is there: the page can draw, and nothing is
	// wrong with it that this value would name.
	WebProcessNoFailure WebProcessFailure = iota
	// WebProcessCrashed is a process that stopped without being asked to: a fault in the page or
	// in the engine, which loses whatever the page held that was not saved.
	WebProcessCrashed
	// WebProcessKilled is a process that the system or the engine stopped: memory pressure, an
	// out-of-memory kill, or the engine ending it itself. The page is gone with it, but nothing
	// the page did made it happen, so a reload usually brings it back.
	WebProcessKilled
	// WebProcessUnresponsive is a process that stopped answering, which the engine ended.
	WebProcessUnresponsive
	// WebProcessLaunchFailed is a process that never started, so the page never ran at all.
	WebProcessLaunchFailed
	// WebProcessUnknown is a process the engine reports as gone without a reason this package
	// classifies, including an engine that names no reason at all.
	WebProcessUnknown
)

// String names the failure, for a status line or a log.
func (f WebProcessFailure) String() string {
	switch f {
	case WebProcessNoFailure:
		return "no failure"
	case WebProcessCrashed:
		return "crashed"
	case WebProcessKilled:
		return "killed"
	case WebProcessUnresponsive:
		return "unresponsive"
	case WebProcessLaunchFailed:
		return "launch failed"
	case WebProcessUnknown:
		return "unknown failure"
	}
	return "web process failure " + strconv.FormatUint(uint64(f), 10)
}

// COREWEBVIEW2_PROCESS_FAILED_KIND values, which ICoreWebView2ProcessFailedEventArgs.get_
// ProcessFailedKind reports for the process the runtime lost. Only the three that end the page are
// here: a frame's own renderer (3), a utility (4), a sandbox helper (5), a GPU (6) and a plugin (7,
// 8) process are the engine's to restart, and the page draws again without the application being
// told; the value this package does not know (9) is treated the same way, because a failure the
// runtime cannot name is not one it believes ended the page.
const (
	webView2FailedBrowserProcessExited      = 0
	webView2FailedRenderProcessExited       = 1
	webView2FailedRenderProcessUnresponsive = 2
)

// COREWEBVIEW2_PROCESS_FAILED_REASON values, which ICoreWebView2ProcessFailedEventArgs2.get_Reason
// reports for why the process exited. -1 is this package's own value for a runtime too old to say
// (see WebProcessFailureFromWebView2), not one the runtime reports.
const (
	webView2ReasonUnexpected    = 0
	webView2ReasonUnresponsive  = 1
	webView2ReasonTerminated    = 2
	webView2ReasonCrashed       = 3
	webView2ReasonLaunchFailed  = 4
	webView2ReasonOutOfMemory   = 5
	webView2ReasonProfileDelete = 6

	webView2ReasonUnsaid = -1
)

// WebProcessFailureFromWebView2 maps what the runtime reports about a process that failed: kind is
// COREWEBVIEW2_PROCESS_FAILED_KIND and reason is COREWEBVIEW2_PROCESS_FAILED_REASON, which is
// webView2ReasonUnsaid when the runtime predates ICoreWebView2ProcessFailedEventArgs2 (WebView2
// SDK 1.0.818.41) and says only the kind.
//
// ok is false for a process whose loss the page survives: the engine starts the others again by
// itself, and an application is told about the one that draws the page it hosts.
func WebProcessFailureFromWebView2(kind, reason int32) (WebProcessFailure, bool) {
	switch kind {
	case webView2FailedBrowserProcessExited, webView2FailedRenderProcessExited:
		// The browser process draws no page of its own, but every renderer is its child: when it
		// goes, the page goes with it, and the reason is the same kind of reason.
		return webProcessFailureFromWebView2Reason(reason), true
	case webView2FailedRenderProcessUnresponsive:
		return WebProcessUnresponsive, true
	}
	return WebProcessNoFailure, false
}

func webProcessFailureFromWebView2Reason(reason int32) WebProcessFailure {
	switch reason {
	case webView2ReasonUnresponsive:
		return WebProcessUnresponsive
	case webView2ReasonTerminated, webView2ReasonOutOfMemory:
		// The runtime calls a process it ended itself terminated, and one the system reclaimed
		// out of memory: both are a page that did nothing wrong, the second by the system rather
		// than by the engine.
		return WebProcessKilled
	case webView2ReasonCrashed, webView2ReasonUnexpected:
		// Unexpected is the exit the runtime has no name for, which is a process that stopped
		// without being asked in the terms an application acts on: the page lost what it held.
		return WebProcessCrashed
	case webView2ReasonLaunchFailed:
		return WebProcessLaunchFailed
	}
	// A reason this package does not know, the deprecated one, and a runtime that says only the
	// kind.
	return WebProcessUnknown
}

// WebKitWebProcessTerminationReason values, which the web-process-terminated signal carries from
// WebKitGTK 2.20.
const (
	webKitProcessCrashed             = 0
	webKitProcessExceededMemoryLimit = 1
	webKitProcessTerminatedByAPI     = 2
)

// WebProcessFailureFromWebKitTerminationReason maps the reason of WebKitGTK's web-process-terminated
// signal: the engine ended the web process, and says why. The reason a reason this package does not
// know is reported as such rather than guessed at.
func WebProcessFailureFromWebKitTerminationReason(reason int32) WebProcessFailure {
	switch reason {
	case webKitProcessCrashed:
		return WebProcessCrashed
	case webKitProcessExceededMemoryLimit, webKitProcessTerminatedByAPI:
		// Above the limit the system or the engine stopped it; by API the application asked
		// WebKitGTK to, which is the engine's own termination as well.
		return WebProcessKilled
	}
	return WebProcessUnknown
}

// WebProcessFailureDarwinTermination is what a WKWebView reports when its content process is gone:
// webViewWebContentProcessDidTerminate: carries no reason at all, so a page that lost its process on
// macOS is WebProcessUnknown in this package's terms rather than a guess at a crash.
const WebProcessFailureDarwinTermination = WebProcessUnknown
