package platform

import "testing"

func TestEachWebView2StatusNamesItsFailure(t *testing.T) {
	for _, c := range []struct {
		status int32
		want   WebLoadError
	}{
		{0, WebLoadUnknownError},            // UNKNOWN
		{1, WebLoadBadCertificate},          // CERTIFICATE_COMMON_NAME_INCORRECT
		{2, WebLoadBadCertificate},          // CERTIFICATE_EXPIRED
		{3, WebLoadBadCertificate},          // CLIENT_CERTIFICATE_CONTAINS_ERRORS
		{4, WebLoadBadCertificate},          // CERTIFICATE_REVOKED
		{5, WebLoadBadCertificate},          // CERTIFICATE_IS_INVALID
		{6, WebLoadServerUnreachable},       // SERVER_UNREACHABLE
		{7, WebLoadTimeout},                 // TIMEOUT
		{8, WebLoadBadServerResponse},       // ERROR_HTTP_INVALID_SERVER_RESPONSE
		{9, WebLoadConnectionLost},          // CONNECTION_ABORTED
		{10, WebLoadConnectionLost},         // CONNECTION_RESET
		{11, WebLoadConnectionLost},         // DISCONNECTED
		{12, WebLoadServerUnreachable},      // CANNOT_CONNECT
		{13, WebLoadHostNotFound},           // HOST_NAME_NOT_RESOLVED
		{14, WebLoadCancelled},              // OPERATION_CANCELED
		{15, WebLoadRedirectFailed},         // REDIRECT_FAILED
		{16, WebLoadUnknownError},           // UNEXPECTED_ERROR
		{17, WebLoadAuthenticationRequired}, // VALID_AUTHENTICATION_CREDENTIALS_REQUIRED
		{18, WebLoadAuthenticationRequired}, // VALID_PROXY_AUTHENTICATION_REQUIRED
		{19, WebLoadUnknownError},           // a status newer than this package
		{-1, WebLoadUnknownError},           // never reported, but not a crash
	} {
		if got := WebLoadErrorFromWebView2Status(c.status); got != c.want {
			t.Errorf("status %d = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestEachDarwinErrorNamesItsFailure(t *testing.T) {
	for _, c := range []struct {
		domain string
		code   int
		want   WebLoadError
	}{
		{"NSURLErrorDomain", -999, WebLoadCancelled},
		{"NSURLErrorDomain", -1001, WebLoadTimeout},
		{"NSURLErrorDomain", -1003, WebLoadHostNotFound},
		{"NSURLErrorDomain", -1006, WebLoadHostNotFound},
		{"NSURLErrorDomain", -1004, WebLoadServerUnreachable},
		{"NSURLErrorDomain", -1005, WebLoadConnectionLost},
		{"NSURLErrorDomain", -1009, WebLoadConnectionLost},
		{"NSURLErrorDomain", -1020, WebLoadConnectionLost},
		{"NSURLErrorDomain", -1007, WebLoadRedirectFailed},
		{"NSURLErrorDomain", -1010, WebLoadRedirectFailed},
		{"NSURLErrorDomain", -1011, WebLoadBadServerResponse},
		{"NSURLErrorDomain", -1017, WebLoadBadServerResponse},
		{"NSURLErrorDomain", -1013, WebLoadAuthenticationRequired},
		{"NSURLErrorDomain", -1206, WebLoadAuthenticationRequired},
		{"NSURLErrorDomain", -1200, WebLoadBadCertificate},
		{"NSURLErrorDomain", -1202, WebLoadBadCertificate},
		{"NSURLErrorDomain", -1205, WebLoadBadCertificate},
		{"NSURLErrorDomain", -1000, WebLoadUnknownError}, // a bad URL: the engine's message tells it apart
		{"NSURLErrorDomain", -1100, WebLoadUnknownError}, // a file that is not there
		{"WebKitErrorDomain", 102, WebLoadCancelled},     // the load was interrupted by a policy change
		{"WebKitErrorDomain", 101, WebLoadUnknownError},
		{"WebKitErrorDomain", -1001, WebLoadUnknownError}, // the domain decides, not the number
		{"NSCocoaErrorDomain", 260, WebLoadUnknownError},
		{"", 0, WebLoadUnknownError},
	} {
		if got := WebLoadErrorFromDarwinError(c.domain, c.code); got != c.want {
			t.Errorf("%s %d = %v, want %v", c.domain, c.code, got, c.want)
		}
	}
}

func TestEachWebKitErrorNamesItsFailure(t *testing.T) {
	for _, c := range []struct {
		code int32
		want WebLoadError
	}{
		{0, WebLoadUnknownError},
		{1, WebLoadUnknownError}, // NETWORK_ERROR_FAILED
		{2, WebLoadConnectionLost},
		{3, WebLoadUnknownError}, // UNKNOWN_PROTOCOL
		{4, WebLoadCancelled},
		{5, WebLoadUnknownError}, // FILE_DOES_NOT_EXIST
		{100, WebLoadUnknownError},
		{102, WebLoadUnknownError}, // CANNOT_SHOW_URI
		{199, WebLoadCancelled},
		{200, WebLoadUnknownError},
		{299, WebLoadCancelled},
		{300, WebLoadUnknownError}, // a download's error, not a load's
		{-1, WebLoadUnknownError},
	} {
		if got := WebLoadErrorFromWebKitError(c.code); got != c.want {
			t.Errorf("code %d = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestEachLoadErrorNamesItself(t *testing.T) {
	for _, c := range []struct {
		code WebLoadError
		want string
	}{
		{WebLoadNoError, "no error"},
		{WebLoadUnknownError, "unknown error"},
		{WebLoadCancelled, "cancelled"},
		{WebLoadTimeout, "timed out"},
		{WebLoadHostNotFound, "host name not found"},
		{WebLoadServerUnreachable, "server unreachable"},
		{WebLoadConnectionLost, "connection lost"},
		{WebLoadBadServerResponse, "bad server response"},
		{WebLoadBadCertificate, "certificate refused"},
		{WebLoadRedirectFailed, "redirect failed"},
		{WebLoadAuthenticationRequired, "authentication required"},
		{WebLoadError(200), "web load error 200"},
	} {
		if got := c.code.String(); got != c.want {
			t.Errorf("%d = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestALoadedNavigationCarriesNoError(t *testing.T) {
	var load WebLoad
	if load.Phase != WebLoadStarted || load.Code != WebLoadNoError || load.Title != "" || load.Description != "" || load.URL != "" {
		t.Fatalf("a zero WebLoad = %+v, want no navigation and no error", load)
	}
}
