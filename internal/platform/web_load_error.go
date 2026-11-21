package platform

import "strconv"

// WebLoadError is why a navigation did not load the page, in terms each engine can be held to: the
// engines name the same failures differently, and a page that fails to load is worth reporting the
// same way on every platform. Zero is a navigation that loaded.
type WebLoadError uint8

const (
	// WebLoadNoError is a navigation that loaded.
	WebLoadNoError WebLoadError = iota
	// WebLoadUnknownError is a navigation the engine reports as failed without a reason this
	// package classifies; Description carries whatever the engine said about it.
	WebLoadUnknownError
	// WebLoadCancelled is a navigation that was stopped: by the engine, or by a policy that
	// interrupted the load it was part of.
	WebLoadCancelled
	// WebLoadTimeout is a server that did not answer in time.
	WebLoadTimeout
	// WebLoadHostNotFound is a name that did not resolve.
	WebLoadHostNotFound
	// WebLoadServerUnreachable is a host that could not be reached: no route, or a connection that
	// was refused.
	WebLoadServerUnreachable
	// WebLoadConnectionLost is a connection that broke while the page was arriving, or a machine
	// with no network at all.
	WebLoadConnectionLost
	// WebLoadBadServerResponse is an answer the engine could not read as HTTP, or one that broke its
	// own framing.
	WebLoadBadServerResponse
	// WebLoadBadCertificate is a TLS certificate the engine refused, or a client certificate the
	// server refused.
	WebLoadBadCertificate
	// WebLoadRedirectFailed is a redirect the engine could not follow.
	WebLoadRedirectFailed
	// WebLoadAuthenticationRequired is a server or proxy that stopped the navigation for credentials
	// the engine did not have.
	WebLoadAuthenticationRequired
)

// String names the failure, for a status line or a log.
func (e WebLoadError) String() string {
	switch e {
	case WebLoadNoError:
		return "no error"
	case WebLoadUnknownError:
		return "unknown error"
	case WebLoadCancelled:
		return "cancelled"
	case WebLoadTimeout:
		return "timed out"
	case WebLoadHostNotFound:
		return "host name not found"
	case WebLoadServerUnreachable:
		return "server unreachable"
	case WebLoadConnectionLost:
		return "connection lost"
	case WebLoadBadServerResponse:
		return "bad server response"
	case WebLoadBadCertificate:
		return "certificate refused"
	case WebLoadRedirectFailed:
		return "redirect failed"
	case WebLoadAuthenticationRequired:
		return "authentication required"
	}
	return "web load error " + strconv.FormatUint(uint64(e), 10)
}

// COREWEBVIEW2_WEB_ERROR_STATUS values, which ICoreWebView2NavigationCompletedEventArgs.get_
// WebErrorStatus reports for a navigation that did not load.
const (
	webView2StatusCertificateCommonNameIncorrect  = 1
	webView2StatusCertificateExpired              = 2
	webView2StatusClientCertificateContainsErrors = 3
	webView2StatusCertificateRevoked              = 4
	webView2StatusCertificateIsInvalid            = 5
	webView2StatusServerUnreachable               = 6
	webView2StatusTimeout                         = 7
	webView2StatusErrorHTTPInvalidServerResponse  = 8
	webView2StatusConnectionAborted               = 9
	webView2StatusConnectionReset                 = 10
	webView2StatusDisconnected                    = 11
	webView2StatusCannotConnect                   = 12
	webView2StatusHostNameNotResolved             = 13
	webView2StatusOperationCanceled               = 14
	webView2StatusRedirectFailed                  = 15
	webView2StatusAuthenticationRequired          = 17
	webView2StatusProxyAuthenticationRequired     = 18
)

// WebLoadErrorFromWebView2Status maps the status of a completed WebView2 navigation.
func WebLoadErrorFromWebView2Status(status int32) WebLoadError {
	switch status {
	case webView2StatusCertificateCommonNameIncorrect, webView2StatusCertificateExpired,
		webView2StatusClientCertificateContainsErrors, webView2StatusCertificateRevoked,
		webView2StatusCertificateIsInvalid:
		return WebLoadBadCertificate
	case webView2StatusServerUnreachable, webView2StatusCannotConnect:
		return WebLoadServerUnreachable
	case webView2StatusTimeout:
		return WebLoadTimeout
	case webView2StatusErrorHTTPInvalidServerResponse:
		return WebLoadBadServerResponse
	case webView2StatusConnectionAborted, webView2StatusConnectionReset, webView2StatusDisconnected:
		return WebLoadConnectionLost
	case webView2StatusHostNameNotResolved:
		return WebLoadHostNotFound
	case webView2StatusOperationCanceled:
		return WebLoadCancelled
	case webView2StatusRedirectFailed:
		return WebLoadRedirectFailed
	case webView2StatusAuthenticationRequired, webView2StatusProxyAuthenticationRequired:
		return WebLoadAuthenticationRequired
	}
	// Unknown (0) and UnexpectedError (16) are the statuses the runtime reaches for a failure it has
	// no name for, so they are the same thing here; a status this package does not know is new to the
	// runtime, and is reported as such rather than guessed at.
	return WebLoadUnknownError
}

// The NSError codes a WKWebView reports for a navigation it could not load, in NSURLErrorDomain and
// in WebKitErrorDomain (the engine's own errors). Only the ones that name a kind of failure above are
// here: the rest are addresses the engine will not load at all, which stay WebLoadUnknownError.
const (
	nsURLErrorCancelled                       = -999
	nsURLErrorTimedOut                        = -1001
	nsURLErrorCannotFindHost                  = -1003
	nsURLErrorCannotConnectToHost             = -1004
	nsURLErrorNetworkConnectionLost           = -1005
	nsURLErrorDNSLookupFailed                 = -1006
	nsURLErrorHTTPTooManyRedirects            = -1007
	nsURLErrorNotConnectedToInternet          = -1009
	nsURLErrorRedirectToNonExistentLocation   = -1010
	nsURLErrorBadServerResponse               = -1011
	nsURLErrorUserCancelledAuthentication     = -1012
	nsURLErrorUserAuthenticationRequired      = -1013
	nsURLErrorZeroByteResource                = -1014
	nsURLErrorCannotDecodeRawData             = -1015
	nsURLErrorCannotDecodeContentData         = -1016
	nsURLErrorCannotParseResponse             = -1017
	nsURLErrorInternationalRoamingOff         = -1018
	nsURLErrorCallIsActive                    = -1019
	nsURLErrorDataNotAllowed                  = -1020
	nsURLErrorSecureConnectionFailed          = -1200
	nsURLErrorServerCertificateHasBadDate     = -1201
	nsURLErrorServerCertificateUntrusted      = -1202
	nsURLErrorServerCertificateHasUnknownRoot = -1203
	nsURLErrorServerCertificateNotYetValid    = -1204
	nsURLErrorClientCertificateRejected       = -1205
	nsURLErrorClientCertificateRequired       = -1206

	webKitErrorFrameLoadInterruptedByPolicyChange = 102
)

// WebLoadErrorFromDarwinError maps the NSError a WKWebView reports for a navigation it could not
// load: domain is NSError.domain ("NSURLErrorDomain" for failures of the URL loading system, and
// "WebKitErrorDomain" for the engine's own), and code is NSError.code.
func WebLoadErrorFromDarwinError(domain string, code int) WebLoadError {
	switch domain {
	case "NSURLErrorDomain":
		switch code {
		case nsURLErrorCancelled, nsURLErrorUserCancelledAuthentication:
			return WebLoadCancelled
		case nsURLErrorTimedOut:
			return WebLoadTimeout
		case nsURLErrorCannotFindHost, nsURLErrorDNSLookupFailed:
			return WebLoadHostNotFound
		case nsURLErrorCannotConnectToHost:
			return WebLoadServerUnreachable
		case nsURLErrorNetworkConnectionLost, nsURLErrorNotConnectedToInternet,
			nsURLErrorInternationalRoamingOff, nsURLErrorCallIsActive, nsURLErrorDataNotAllowed:
			return WebLoadConnectionLost
		case nsURLErrorHTTPTooManyRedirects, nsURLErrorRedirectToNonExistentLocation:
			return WebLoadRedirectFailed
		case nsURLErrorBadServerResponse, nsURLErrorZeroByteResource, nsURLErrorCannotDecodeRawData,
			nsURLErrorCannotDecodeContentData, nsURLErrorCannotParseResponse:
			return WebLoadBadServerResponse
		case nsURLErrorUserAuthenticationRequired, nsURLErrorClientCertificateRequired:
			return WebLoadAuthenticationRequired
		case nsURLErrorSecureConnectionFailed, nsURLErrorServerCertificateHasBadDate,
			nsURLErrorServerCertificateUntrusted, nsURLErrorServerCertificateHasUnknownRoot,
			nsURLErrorServerCertificateNotYetValid, nsURLErrorClientCertificateRejected:
			return WebLoadBadCertificate
		}
	case "WebKitErrorDomain":
		if code == webKitErrorFrameLoadInterruptedByPolicyChange {
			return WebLoadCancelled
		}
	}
	return WebLoadUnknownError
}

// The WebKitGTK error codes a load-failed signal carries: WebKitNetworkError counts from 1,
// WebKitPolicyError from 100, WebKitPluginError from 200 (WebKitError.h). Downloads, which start at
// 300, are not a load.
const (
	webKitNetworkErrorTransport = 2
	webKitNetworkErrorCancelled = 4

	webKitPolicyErrorFrameLoadInterruptedByPolicyChange = 102
	webKitPluginErrorConnectionCancelled                = 203
)

// WebLoadErrorFromWebKitError maps the code of the GError a WebKitGTK load-failed signal carries.
func WebLoadErrorFromWebKitError(code int32) WebLoadError {
	switch code {
	case webKitNetworkErrorTransport:
		return WebLoadConnectionLost
	case webKitNetworkErrorCancelled, webKitPolicyErrorFrameLoadInterruptedByPolicyChange,
		webKitPluginErrorConnectionCancelled:
		return WebLoadCancelled
	}
	// Network failures WebKitGTK has no name for, files that are not there, and the policy and
	// plugin errors: the engine's message says which it was. A certificate it refuses reaches the
	// load as a cancellation, because the TLS error signal is WebKitGTK's alone to answer and this
	// package leaves it to the engine.
	return WebLoadUnknownError
}
