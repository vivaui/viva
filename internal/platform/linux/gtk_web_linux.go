//go:build linux && vivagtk

package linux

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/diag"
	"github.com/vivaui/viva/internal/platform"
)

// Web pages inside the window (ADR 0064): the system's WebKitGTK, one widget per page, stacked over
// the window's drawing area in a GtkOverlay and placed where Viva leaves a hole. What Viva draws on
// a layer above the hole shows over the page (the Wayland subsurface lies above everything GTK
// draws). The engine is loaded when a window opens its first page; an application that opens none
// never loads it.

// webKit is the engine's entry points.
type webKit struct {
	managerNew        func() uintptr
	managerRegister   func(manager uintptr, name *byte) int32
	managerAddScript  func(manager, script uintptr)
	scriptNew         func(source *byte, frames, time int32, allow, block uintptr) uintptr
	viewNew           func(manager uintptr) uintptr
	loadURI           func(view uintptr, uri *byte)
	loadHTML          func(view uintptr, content, base *byte)
	reload            func(view uintptr)
	goBack, goForward func(view uintptr)
	getURI            func(view uintptr) uintptr
	// The page's own state: its document's title, and whether its history and its loading are
	// where they are.
	getTitle          func(view uintptr) uintptr
	canGoBack         func(view uintptr) int32
	canGoForward      func(view uintptr) int32
	isLoading         func(view uintptr) int32
	getContext        func(view uintptr) uintptr
	setZoom           func(view uintptr, level float64)
	registerScheme    func(context uintptr, scheme *byte, callback, data, destroy uintptr)
	securityManager   func(context uintptr) uintptr
	schemeSecure      func(manager uintptr, scheme *byte)
	schemeCORS        func(manager uintptr, scheme *byte)
	schemeURI         func(request uintptr) uintptr
	schemeFinish      func(request, stream uintptr, length int64, mime *byte)
	schemeFinishError func(request, err uintptr)
	jscString         func(value uintptr) uintptr
	resultValue       func(result uintptr) uintptr
	memoryStream      func(data uintptr, length int64, destroy uintptr) uintptr
	gMalloc           func(size uint64) uintptr
	quarkFrom         func(name *byte) uint32
	errorLiteral      func(domain uint32, code int32, message *byte) uintptr
	// Either way of running a script is there: the first from 2.40, the second before.
	evaluate func(view uintptr, script *byte, length int64, world, source, cancellable, callback, data uintptr)
	run      func(view uintptr, script *byte, cancellable, callback, data uintptr)
	free     uintptr
	// For a page held to a proxy (2.16, and 2.32 for the data manager's setter); nil without.
	contextEphemeral   func() uintptr
	viewWithContext    func(context uintptr) uintptr
	viewManager        func(view uintptr) uintptr
	proxySettingsNew   func(uri *byte, ignore uintptr) uintptr
	proxySettingsFree  func(settings uintptr)
	contextDataManager func(context uintptr) uintptr
	managerSetProxy    func(manager uintptr, mode int32, settings uintptr)
	contextSetProxy    func(context uintptr, mode int32, settings uintptr)
	// For a store of a page's own: a context over a website data manager of the partition's, the
	// manager itself, and the directories it took (WebKitGTK 2.10); nil without.
	contextWithDataManager func(manager uintptr) uintptr
	dataManagerNew         func(name1, value1, name2, value2, terminator uintptr) uintptr
	dataManagerBaseData    func(manager uintptr) uintptr
	dataManagerBaseCache   func(manager uintptr) uintptr
	// The 2.36 answer, which carries a status and headers; nil on an engine that has only the stream
	// and type form, where a scheme's status and headers are dropped.
	schemeResponse    bool
	responseNew       func(stream uintptr, length int64) uintptr
	responseStatus    func(response uintptr, status uint32, reason *byte)
	responseType      func(response uintptr, mime *byte)
	responseHeaders   func(response uintptr, headers uintptr)
	finishResponse    func(request, response uintptr)
	soupHeadersNew    func(kind int32) uintptr
	soupHeadersAppend func(headers uintptr, name, value *byte)
	// schemeWebView tells which page the request is for, so a late answer for a page that closed can
	// be dropped; objectRef keeps a request alive until its answer arrives.
	schemeWebView func(request uintptr) uintptr
	objectRef     func(object uintptr) uintptr
	// Navigation decisions, new windows and downloads.
	decisionUse, decisionIgnore func(decision uintptr)
	navigationAction            func(decision uintptr) uintptr
	actionRequest               func(action uintptr) uintptr
	actionUserGesture           func(action uintptr) int32
	actionRedirect              func(action uintptr) int32 // 2.20, nil before
	uriRequestURI               func(request uintptr) uintptr
	downloadRequest             func(download uintptr) uintptr
	downloadWebView             func(download uintptr) uintptr
	downloadSetDestination      func(download uintptr, destination *byte)
	downloadAllowOverwrite      func(download uintptr, allow int32)
	downloadCancel              func(download uintptr)
	// viewNetworkSession is 2.40; before it the context's download-started is used instead.
	viewNetworkSession func(view uintptr) uintptr
	// downloadDestinationPath says set_destination takes a path (2.40 and later) rather than a URI,
	// decided from the library's own major and minor version at load time.
	downloadDestinationPath bool
	// processTerminated tells whether the engine has the web-process-terminated signal, which names
	// a page whose web process is gone: WebKitGTK has it from 2.20, and an engine without it is not
	// handed a signal to connect.
	processTerminated bool
}

var (
	webOnce sync.Once
	webAPI  *webKit
	webErr  error
	// webProcessTerminatedUnsupported says once, rather than at every page, that this engine cannot
	// report a page whose process is gone.
	webProcessTerminatedUnsupported sync.Once
)

func loadWebKit(g *gtkLibrary) (*webKit, error) {
	webOnce.Do(func() {
		var err error
		found := false
		for _, name := range []string{"libwebkit2gtk-4.1.so.0", "libwebkit2gtk-4.0.so.37"} {
			var library uintptr
			if library, err = purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
				g.libraries = append(g.libraries, library)
				found = true
				break
			}
		}
		if !found {
			webErr = fmt.Errorf("%w: WebKitGTK is not installed: install libwebkit2gtk-4.1-0 (Debian, Ubuntu) or webkit2gtk4.1 (Fedora)", platform.ErrWebViewUnsupported)
			return
		}
		for _, name := range []string{"libjavascriptcoregtk-4.1.so.0", "libjavascriptcoregtk-4.0.so.18"} {
			if library, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
				g.libraries = append(g.libraries, library)
				break
			}
		}
		k := &webKit{}
		// webkit_download_set_destination takes a path from WebKitGTK 2.40 and a URI before it. The
		// 4.1 soname has existed since 2.36, so it does not say which form this library expects;
		// ask the library's own version instead, and keep the URI form when it cannot be read.
		var major, minor func() uint32
		if g.lookup(&major, "webkit_get_major_version") && g.lookup(&minor, "webkit_get_minor_version") {
			if m, n := major(), minor(); m > 2 || (m == 2 && n >= 40) {
				k.downloadDestinationPath = true
			}
		}
		for _, r := range []struct {
			function any
			symbol   string
		}{
			{&k.resultValue, "webkit_javascript_result_get_js_value"},
			{&k.managerNew, "webkit_user_content_manager_new"}, {&k.managerRegister, "webkit_user_content_manager_register_script_message_handler"},
			{&k.managerAddScript, "webkit_user_content_manager_add_script"}, {&k.scriptNew, "webkit_user_script_new"},
			{&k.viewNew, "webkit_web_view_new_with_user_content_manager"}, {&k.loadURI, "webkit_web_view_load_uri"},
			{&k.loadHTML, "webkit_web_view_load_html"}, {&k.reload, "webkit_web_view_reload"}, {&k.goBack, "webkit_web_view_go_back"},
			{&k.goForward, "webkit_web_view_go_forward"}, {&k.getURI, "webkit_web_view_get_uri"},
			{&k.getTitle, "webkit_web_view_get_title"}, {&k.canGoBack, "webkit_web_view_can_go_back"},
			{&k.canGoForward, "webkit_web_view_can_go_forward"}, {&k.isLoading, "webkit_web_view_is_loading"},
			{&k.getContext, "webkit_web_view_get_context"}, {&k.setZoom, "webkit_web_view_set_zoom_level"},
			{&k.registerScheme, "webkit_web_context_register_uri_scheme"}, {&k.schemeURI, "webkit_uri_scheme_request_get_uri"},
			{&k.schemeFinish, "webkit_uri_scheme_request_finish"}, {&k.schemeFinishError, "webkit_uri_scheme_request_finish_error"},
			{&k.jscString, "jsc_value_to_string"},
			{&k.memoryStream, "g_memory_input_stream_new_from_data"}, {&k.gMalloc, "g_malloc"},
			{&k.quarkFrom, "g_quark_from_string"}, {&k.errorLiteral, "g_error_new_literal"},
			{&k.objectRef, "g_object_ref"},
			{&k.decisionUse, "webkit_policy_decision_use"}, {&k.decisionIgnore, "webkit_policy_decision_ignore"},
			{&k.navigationAction, "webkit_navigation_policy_decision_get_navigation_action"},
			{&k.actionRequest, "webkit_navigation_action_get_request"},
			{&k.actionUserGesture, "webkit_navigation_action_is_user_gesture"},
			{&k.uriRequestURI, "webkit_uri_request_get_uri"},
			{&k.downloadRequest, "webkit_download_get_request"},
			{&k.downloadWebView, "webkit_download_get_web_view"},
			{&k.downloadSetDestination, "webkit_download_set_destination"},
			{&k.downloadAllowOverwrite, "webkit_download_set_allow_overwrite"},
			{&k.downloadCancel, "webkit_download_cancel"},
		} {
			if !g.lookup(r.function, r.symbol) {
				webErr = fmt.Errorf("%w: WebKitGTK lacks %s", platform.ErrWebViewUnsupported, r.symbol)
				return
			}
		}
		if !g.lookup(&k.evaluate, "webkit_web_view_evaluate_javascript") {
			k.evaluate = nil
		}
		if !g.lookup(&k.run, "webkit_web_view_run_javascript") {
			k.run = nil
		}
		if k.evaluate == nil && k.run == nil {
			webErr = fmt.Errorf("%w: WebKitGTK cannot run scripts", platform.ErrWebViewUnsupported)
			return
		}
		g.lookup(&k.securityManager, "webkit_web_context_get_security_manager")
		g.lookup(&k.schemeSecure, "webkit_security_manager_register_uri_scheme_as_secure")
		g.lookup(&k.schemeCORS, "webkit_security_manager_register_uri_scheme_as_cors_enabled")
		// A redirect is not a user gesture even when the request that led to it was one.
		g.lookup(&k.actionRedirect, "webkit_navigation_action_is_redirect")
		g.lookup(&k.viewNetworkSession, "webkit_web_view_get_network_session")
		k.free = g.symbol("g_free")
		if !g.lookup(&k.contextEphemeral, "webkit_web_context_new_ephemeral") ||
			!g.lookup(&k.viewWithContext, "webkit_web_view_new_with_context") ||
			!g.lookup(&k.viewManager, "webkit_web_view_get_user_content_manager") ||
			!g.lookup(&k.proxySettingsNew, "webkit_network_proxy_settings_new") ||
			!g.lookup(&k.proxySettingsFree, "webkit_network_proxy_settings_free") {
			k.contextEphemeral = nil
		}
		if !g.lookup(&k.contextWithDataManager, "webkit_web_context_new_with_website_data_manager") ||
			!g.lookup(&k.dataManagerNew, "webkit_website_data_manager_new") ||
			!g.lookup(&k.dataManagerBaseData, "webkit_website_data_manager_get_base_data_directory") ||
			!g.lookup(&k.dataManagerBaseCache, "webkit_website_data_manager_get_base_cache_directory") {
			k.contextWithDataManager = nil
		}
		if !g.lookup(&k.contextDataManager, "webkit_web_context_get_website_data_manager") ||
			!g.lookup(&k.managerSetProxy, "webkit_website_data_manager_set_network_proxy_settings") {
			k.managerSetProxy = nil
		}
		g.lookup(&k.contextSetProxy, "webkit_web_context_set_network_proxy_settings")
		// The answer object and libsoup's headers exist from WebKitGTK 2.36; on an older engine the
		// shell falls back to webkit_uri_scheme_request_finish, which has no status or headers.
		if g.lookup(&k.responseNew, "webkit_uri_scheme_response_new") &&
			g.lookup(&k.responseStatus, "webkit_uri_scheme_response_set_status") &&
			g.lookup(&k.responseType, "webkit_uri_scheme_response_set_content_type") &&
			g.lookup(&k.responseHeaders, "webkit_uri_scheme_response_set_http_headers") &&
			g.lookup(&k.finishResponse, "webkit_uri_scheme_request_finish_with_response") &&
			g.lookup(&k.soupHeadersNew, "soup_message_headers_new") &&
			g.lookup(&k.soupHeadersAppend, "soup_message_headers_append") {
			k.schemeResponse = true
		}
		g.lookup(&k.schemeWebView, "webkit_uri_scheme_request_get_web_view")
		k.processTerminated = webProcessTerminatedSignal(g)
		webAPI = k
	})
	return webAPI, webErr
}

// webProcessTerminatedSignal reports whether the engine has WebKitWebView's web-process-terminated
// signal, which says the page's web process is gone: WebKitGTK added it in 2.20, and an older engine
// would only warn about a signal it does not have.
func webProcessTerminatedSignal(g *gtkLibrary) bool {
	var viewType func() uintptr
	var signalLookup func(name *byte, itype uintptr) uint32
	var classRef func(itype uintptr) uintptr
	var classUnref func(class uintptr)
	if !g.lookup(&viewType, "webkit_web_view_get_type") || !g.lookup(&signalLookup, "g_signal_lookup") ||
		!g.lookup(&classRef, "g_type_class_ref") || !g.lookup(&classUnref, "g_type_class_unref") {
		return false
	}
	// Signals are installed in the type's class_init, so a type that no instance has
	// initialized yet has none to look up: take the class first, then give it back.
	class := classRef(viewType())
	if class == 0 {
		return false
	}
	defer classUnref(class)
	name := cString("web-process-terminated")
	return signalLookup(&name[0], viewType()) != 0
}

// webQuirks keeps a virtual machine or a container from breaking the engine: without a GPU render
// node its buffer sharing between processes fails and pages stay blank.
func webQuirks() {
	if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		if _, err := os.Stat("/dev/dri/renderD128"); err != nil {
			os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
		}
	}
}

// WebKitLoadEvent.
const (
	webLoadStarted   = 0
	webLoadCommitted = 2
	webLoadFinished  = 3
)

// WebKitPolicyDecisionType: a navigation in this view. A new window is asked about through the
// create signal, and a response is left to the engine's own policy.
const webDecisionNavigation = 0

// gtkWebView is one page.
type gtkWebView struct {
	w       *gtkWindow
	k       *webKit
	id      uint32
	handle  uintptr // the key of its callbacks
	widget  uintptr
	manager uintptr
	private *webContext // the page's ephemeral context, when it has one of its own
	config  platform.WebViewConfig
	closed  bool
	shown   bool
	// downloadOwner announces this page's downloads: the default context or network session,
	// which outlives the page, or a proxy's, which goes with its context. downloadHandler is the
	// connection to it; only a shared owner's connection is removed by hand.
	downloadOwner   uintptr
	downloadHandler uint64
	// at is where the widget is, in widget units; zoom is the page's zoom.
	at   viewPlace
	zoom float64
	// failed is a navigation the load-failed signal already reported: the load-changed that follows
	// it ends the same load, and must not be reported twice. refused is the address of a navigation
	// the application's Navigate cancelled, whose interruption the application already answered. It
	// is an address rather than a flag because the engine reports navigation actions for subframes
	// too: only the interruption of that same request is the answer the application gave.
	failed  bool
	refused string
	// failure is why the page's web process is gone, WebProcessNoFailure while it is there: the
	// engine reports the termination and why, and a navigation that starts again clears it.
	failure platform.WebProcessFailure
}

// viewPlace is a rectangle in the overlay's units.
type viewPlace struct{ x, y, width, height int32 }

var (
	webViews      = map[uintptr]*gtkWebView{}
	webNext       uintptr
	webSchemes    = map[string]*platform.WebScheme{}
	webSchemeDone = map[webSchemeKey]bool{}
	webProxies    = map[string]*webContext{}
	webPartitions = map[string]*webContext{}
)

// webSchemeKey is a scheme registered with one context: the default, or a context of a page's own.
type webSchemeKey struct {
	context uintptr
	name    string
}

// webContext is a WebKit context a page runs in besides the default one. The pages held to one proxy
// share one; a page that asked to keep nothing, or both to keep nothing and to use a proxy, has one
// of its own. The pages of a named storage partition share one too, which keeps what they save
// together and apart from every other partition's. What an ephemeral one keeps does not outlive it,
// and it goes with its last page.
type webContext struct {
	proxy     string
	partition string
	ephemeral bool
	object    uintptr
	pages     int
}

// WebKitNetworkProxyMode.
const (
	webProxyNone   = 1
	webProxyCustom = 2
)

// proxyContext is the context for pages held to proxy, shared by the pages behind that proxy and
// made the first time.
func proxyContext(g *gtkLibrary, k *webKit, proxy string) (*webContext, error) {
	if context := webProxies[proxy]; context != nil {
		context.pages++
		return context, nil
	}
	context, err := newProxyContext(k, proxy)
	if err != nil {
		return nil, err
	}
	webProxies[proxy] = context
	return context, nil
}

// newProxyContext makes an ephemeral context with the requested network policy. It is not
// shared: a page that asked for a proxy and to keep nothing gets one of these, so it shares
// nothing with the pages behind the same proxy.
func newProxyContext(k *webKit, proxy string) (*webContext, error) {
	if k.contextEphemeral == nil || (k.managerSetProxy == nil && k.contextSetProxy == nil) {
		return nil, fmt.Errorf("%w: WebKitGTK cannot hold a page to a proxy (2.16 or newer is required)", platform.ErrWebViewUnsupported)
	}
	context, err := newEphemeralContext(k)
	if err != nil {
		return nil, err
	}
	mode := int32(webProxyCustom)
	var settings uintptr
	if proxy == platform.WebProxyDirect {
		mode = webProxyNone
	} else {
		// No hosts are left out: loopback goes through the proxy too, and names are the proxy's to resolve.
		uri := cString("socks5://" + proxy)
		settings = k.proxySettingsNew(&uri[0], 0)
	}
	if k.managerSetProxy != nil {
		k.managerSetProxy(k.contextDataManager(context.object), mode, settings)
	} else {
		k.contextSetProxy(context.object, mode, settings)
	}
	if settings != 0 {
		k.proxySettingsFree(settings)
	}
	context.proxy = proxy
	return context, nil
}

// newEphemeralContext makes an ephemeral context: a page in it keeps nothing once it closes.
func newEphemeralContext(k *webKit) (*webContext, error) {
	if k.contextEphemeral == nil {
		return nil, fmt.Errorf("%w: WebKitGTK cannot open a page without a persistent store (2.16 or newer is required)", platform.ErrWebViewUnsupported)
	}
	object := k.contextEphemeral()
	if object == 0 {
		return nil, fmt.Errorf("%w: WebKitGTK made no context", platform.ErrWebViewUnsupported)
	}
	return &webContext{object: object, pages: 1}, nil
}

// partitionContext is the context of a named storage partition, shared by the pages that name it and
// made the first time one opens. A name is one store: a page that asks for it keeping nothing when
// the open one persists, or the other way round, is refused rather than given a store of its own
// under the same name.
func partitionContext(g *gtkLibrary, k *webKit, class, name string, ephemeral bool) (*webContext, error) {
	if context := webPartitions[name]; context != nil {
		if context.ephemeral != ephemeral {
			return nil, fmt.Errorf("linux: partition %q is open as a %s store, and a page cannot ask for the other kind under the same name", name, partitionKind(context.ephemeral))
		}
		context.pages++
		return context, nil
	}
	var context *webContext
	var err error
	if ephemeral {
		context, err = newEphemeralContext(k)
	} else {
		context, err = newPartitionContext(g, k, class, name)
	}
	if err != nil {
		return nil, err
	}
	context.partition, context.ephemeral = name, ephemeral
	webPartitions[name] = context
	return context, nil
}

// partitionKind names a partition's store in an error.
func partitionKind(ephemeral bool) string {
	if ephemeral {
		return "memory-only"
	}
	return "persistent"
}

// newPartitionContext makes a context over a website data manager of the partition's own: the manager
// keeps the partition's cookies, storage and caches in a directory of its name, so the pages of
// another partition, and the pages of none, do not see them.
func newPartitionContext(g *gtkLibrary, k *webKit, class, name string) (*webContext, error) {
	if k.contextWithDataManager == nil || k.dataManagerNew == nil || k.dataManagerBaseData == nil || k.dataManagerBaseCache == nil {
		return nil, fmt.Errorf("%w: WebKitGTK cannot give a page a store of its own (2.10 or newer is required)", platform.ErrWebViewUnsupported)
	}
	data, cache, err := partitionDirectories(class, name)
	if err != nil {
		return nil, err
	}
	manager := makeDataManager(k, data, cache)
	if manager == 0 {
		return nil, fmt.Errorf("%w: WebKitGTK made no store for partition %q", platform.ErrWebViewUnsupported, name)
	}
	// A manager given no directories keeps the engine's defaults, which are the default store under
	// another name: a partition whose directory the engine did not take is refused rather than opened
	// over the pages it must not share with.
	if got := goString(k.dataManagerBaseData(manager)); got != data {
		g.objectUnref(manager)
		return nil, fmt.Errorf("%w: WebKitGTK kept %q as the data directory of partition %q, not %q", platform.ErrWebViewUnsupported, got, name, data)
	}
	if got := goString(k.dataManagerBaseCache(manager)); got != cache {
		g.objectUnref(manager)
		return nil, fmt.Errorf("%w: WebKitGTK kept %q as the cache directory of partition %q, not %q", platform.ErrWebViewUnsupported, got, name, cache)
	}
	object := k.contextWithDataManager(manager)
	g.objectUnref(manager) // the context holds the reference it was given
	if object == 0 {
		return nil, fmt.Errorf("%w: WebKitGTK made no context for partition %q", platform.ErrWebViewUnsupported, name)
	}
	return &webContext{object: object, pages: 1}, nil
}

// makeDataManager makes a website data manager that keeps its data in data and its caches in cache.
// webkit_website_data_manager_new has no non-variadic form and the two properties that carry the
// directories are construct-only, so the names and values go as its varargs, NUL-terminated:
// pointers only, which is what the C ABI's variadic call takes on every architecture Viva builds for.
// What the manager took is read back before a page is put in it.
func makeDataManager(k *webKit, data, cache string) uintptr {
	dataName, cacheName := cString("base-data-directory"), cString("base-cache-directory")
	dataValue, cacheValue := cString(data), cString(cache)
	manager := k.dataManagerNew(
		uintptr(unsafe.Pointer(&dataName[0])), uintptr(unsafe.Pointer(&dataValue[0])),
		uintptr(unsafe.Pointer(&cacheName[0])), uintptr(unsafe.Pointer(&cacheValue[0])), 0)
	runtime.KeepAlive(dataName)
	runtime.KeepAlive(dataValue)
	runtime.KeepAlive(cacheName)
	runtime.KeepAlive(cacheValue)
	return manager
}

// partitionDirectories is where a named partition keeps its data and its caches: in the user's data
// and cache directories, under a folder of the application's own name and then of the partition's, so
// that two applications that name a partition alike do not share one store.
func partitionDirectories(class, name string) (data, cache string, err error) {
	if class == "" {
		if executable, executableErr := os.Executable(); executableErr == nil {
			class = strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
		}
	}
	if class == "" {
		class = "viva"
	}
	data, err = userDirectory("XDG_DATA_HOME", ".local/share", class, name)
	if err != nil {
		return "", "", err
	}
	cache, err = userDirectory("XDG_CACHE_HOME", ".cache", class, name)
	if err != nil {
		return "", "", err
	}
	return data, cache, nil
}

// userDirectory is environment's directory when it names one, else the fallback under the user's home
// directory, with webkit and parts added; it is made, since the engine writes into it before the page
// loads.
func userDirectory(environment, fallback, class, name string) (string, error) {
	base := os.Getenv(environment)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%w: a storage partition has nowhere to keep its data: %v", platform.ErrWebViewUnsupported, err)
		}
		base = filepath.Join(home, filepath.FromSlash(fallback))
	}
	directory := filepath.Join(base, class, "webkit", name)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("%w: a storage partition has nowhere to keep its data: %v", platform.ErrWebViewUnsupported, err)
	}
	return directory, nil
}

// leave lets go of the context for one page; the last page ends it.
func (context *webContext) leave(g *gtkLibrary) {
	context.pages--
	if context.pages > 0 {
		return
	}
	// Only a shared context is in webProxies or webPartitions. A page that asked for a proxy and to
	// keep nothing has a context of its own with the proxy's settings, and must not drop the shared
	// one behind the same proxy.
	if context.proxy != "" && webProxies[context.proxy] == context {
		delete(webProxies, context.proxy)
	}
	if context.partition != "" && webPartitions[context.partition] == context {
		delete(webPartitions, context.partition)
	}
	for key := range webSchemeDone {
		if key.context == context.object {
			delete(webSchemeDone, key)
		}
	}
	g.objectUnref(context.object)
}

// openWebView is the window's WebHost.Open.
func (w *gtkWindow) openWebView(c platform.WebViewConfig) (platform.WebView, error) {
	if c.TopCornerRadius != 0 {
		return nil, fmt.Errorf("%w: native rounded web views are unavailable on GTK", platform.ErrWebViewUnsupported)
	}
	if w.closed || w.overlay == 0 {
		return nil, platform.ErrWebViewUnsupported
	}
	for _, v := range w.views {
		if v.id == c.ID {
			return nil, fmt.Errorf("linux: a web view with id %d is already open", c.ID)
		}
	}
	g := w.g
	webQuirks()
	k, err := loadWebKit(g)
	if err != nil {
		return nil, err
	}
	if c.Partition != "" && c.Proxy != "" {
		// The proxy is the store's own setting here, and one store cannot carry two: two pages of one
		// partition could not disagree about it, so the page is refused rather than opened with a
		// route another page of its partition chose.
		return nil, fmt.Errorf("%w: WebKitGTK cannot combine a named storage partition with a proxy: the proxy belongs to the store the partition names", platform.ErrWebViewUnsupported)
	}
	var private *webContext
	switch {
	case c.Partition != "":
		if private, err = partitionContext(g, k, w.config.Class, c.Partition, c.Ephemeral); err != nil {
			return nil, err
		}
	case c.Proxy != "" && c.Ephemeral:
		// A page that asks for both needs its own ephemeral context with the proxy's settings. The
		// shared context behind the same proxy would do the proxy but would also share its cookies
		// and localStorage with every other page behind that proxy.
		if private, err = newProxyContext(k, c.Proxy); err != nil {
			return nil, err
		}
	case c.Proxy != "":
		if private, err = proxyContext(g, k, c.Proxy); err != nil {
			return nil, err
		}
	case c.Ephemeral:
		if private, err = newEphemeralContext(k); err != nil {
			return nil, err
		}
	}
	cb := callbacks()
	webNext++
	v := &gtkWebView{w: w, k: k, id: c.ID, handle: webNext, config: c, private: private}
	if private != nil {
		v.widget = k.viewWithContext(private.object)
		v.manager = k.viewManager(v.widget)
	} else {
		v.manager = k.managerNew()
	}
	name := cString("viva")
	k.managerRegister(v.manager, &name[0])
	source, frames := cString(platform.WebBridgeScript), int32(0)
	if c.BridgeMainFrameOnly {
		source, frames = cString(platform.WebBridgeMainFrameOnlyScript), 1 // WEBKIT_USER_CONTENT_INJECT_TOP_FRAME
	}
	k.managerAddScript(v.manager, k.scriptNew(&source[0], frames, 0, 0, 0))
	g.connect(v.manager, "script-message-received::viva", cb.webMessage, v.handle)
	if private == nil {
		v.widget = k.viewNew(v.manager)
	}
	// Placed by its margins from the overlay's top left; hidden until a frame says where.
	g.widgetSetHAlign(v.widget, 1)
	g.widgetSetVAlign(v.widget, 1)
	g.overlayAdd(w.overlay, v.widget)
	g.connect(v.widget, "load-changed", cb.webLoad, v.handle)
	g.connect(v.widget, "load-failed", cb.webFailed, v.handle)
	if k.processTerminated {
		g.connect(v.widget, "web-process-terminated", cb.webProcessTerminated, v.handle)
	} else if c.OnProcessFailed != nil {
		// The page is opened: only the report of a process it loses is missing, and that is said
		// once rather than left as a callback that never runs.
		webProcessTerminatedUnsupported.Do(func() {
			if diag.Enabled(diag.Warn) {
				diag.Report(diag.Warn, "web", "process-failed",
					"this WebKitGTK predates 2.20 and cannot report a page whose process is gone; OnProcessFailed is never called")
			}
		})
	}
	g.connect(v.widget, "focus-in-event", cb.event, w.id)
	g.connect(v.widget, "focus-out-event", cb.event, w.id)
	g.connect(v.widget, "decide-policy", cb.webDecide, v.handle)
	g.connect(v.widget, "create", cb.webCreate, v.handle)
	// Downloads are announced by the network session the page belongs to (2.40), which is also the
	// page's own when it is held to a proxy; before 2.40 the context announces them.
	downloadOwner := uintptr(0)
	if k.viewNetworkSession != nil {
		downloadOwner = k.viewNetworkSession(v.widget)
	}
	if downloadOwner == 0 {
		downloadOwner = k.getContext(v.widget)
	}
	v.downloadOwner = downloadOwner
	v.downloadHandler = g.connect(downloadOwner, "download-started", cb.webDownload, v.handle)
	if c.Scheme != nil && c.Scheme.Name != "" {
		v.registerScheme(c.Scheme)
	}
	webViews[v.handle] = v
	w.views = append(w.views, v)
	switch {
	case c.HTML != "":
		v.LoadHTML(c.HTML, c.BaseURL)
	case c.URL != "":
		v.Load(c.URL)
	}
	if len(w.viewRects) > 0 {
		w.layoutViews() // the application may have declared its place before it opened
	}
	return v, nil
}

// registerScheme makes the engine ask the application for a custom scheme's resources. The engine's
// default context is shared by every page without a proxy, so the scheme is registered once with
// each context and answered by the latest page that declared it.
func (v *gtkWebView) registerScheme(s *platform.WebScheme) {
	webSchemes[s.Name] = s
	k := v.k
	context := k.getContext(v.widget)
	key := webSchemeKey{context, s.Name}
	if webSchemeDone[key] {
		return
	}
	webSchemeDone[key] = true
	name := cString(s.Name)
	k.registerScheme(context, &name[0], callbacks().webScheme, 0, 0)
	if k.securityManager != nil {
		manager := k.securityManager(context)
		if k.schemeSecure != nil {
			k.schemeSecure(manager, &name[0])
		}
		if k.schemeCORS != nil {
			k.schemeCORS(manager, &name[0])
		}
	}
}

func (v *gtkWebView) Load(address string) {
	if v.closed {
		return
	}
	text := cString(address)
	v.k.loadURI(v.widget, &text[0])
}

func (v *gtkWebView) LoadHTML(html, base string) {
	if v.closed {
		return
	}
	content := cString(html)
	var baseAddress *byte
	if base != "" {
		text := cString(base)
		baseAddress = &text[0]
	}
	v.k.loadHTML(v.widget, &content[0], baseAddress)
}

func (v *gtkWebView) Reload() {
	if !v.closed {
		v.k.reload(v.widget)
	}
}

func (v *gtkWebView) Back() {
	if !v.closed {
		v.k.goBack(v.widget)
	}
}

func (v *gtkWebView) Forward() {
	if !v.closed {
		v.k.goForward(v.widget)
	}
}

// Title is the title of the page's current document, which the engine keeps for the widget: empty
// before it has one, and once the page has closed.
func (v *gtkWebView) Title() string {
	if v.closed {
		return ""
	}
	return goString(v.k.getTitle(v.widget))
}

// CanGoBack and CanGoForward ask the engine about the page's history.
func (v *gtkWebView) CanGoBack() bool {
	return !v.closed && v.k.canGoBack(v.widget) != 0
}

func (v *gtkWebView) CanGoForward() bool {
	return !v.closed && v.k.canGoForward(v.widget) != 0
}

// IsLoading is the engine's own answer for the load it is in the middle of.
func (v *gtkWebView) IsLoading() bool {
	return !v.closed && v.k.isLoading(v.widget) != 0
}

// ProcessFailure is why the page's process is gone, WebProcessNoFailure while it is there.
func (v *gtkWebView) ProcessFailure() platform.WebProcessFailure { return v.failure }

func (v *gtkWebView) Eval(script string) {
	if v.closed {
		return
	}
	text := cString(script)
	if v.k.evaluate != nil {
		v.k.evaluate(v.widget, &text[0], -1, 0, 0, 0, 0, 0)
		return
	}
	v.k.run(v.widget, &text[0], 0, 0, 0)
}

func (v *gtkWebView) PostMessage(text string) {
	encoded, err := json.Marshal(text)
	if err != nil {
		return
	}
	v.Eval("window.viva.deliver(" + string(encoded) + ");")
}

func (v *gtkWebView) Focus() {
	if !v.closed && v.shown {
		v.w.g.widgetGrabFocus(v.widget)
	}
}

func (v *gtkWebView) Close() {
	if v.closed {
		return
	}
	w := v.w
	v.release()
	views := w.views[:0]
	for _, other := range w.views {
		if other != v {
			views = append(views, other)
		}
	}
	clear(w.views[len(views):])
	w.views = views
	w.g.widgetDestroy(v.widget)
	w.layoutViews()
}

// release forgets the page; its widget goes with the window or with Close.
func (v *gtkWebView) release() {
	v.closed = true
	delete(webViews, v.handle)
	// A handler on the shared default context or network session outlives the page, so it is
	// removed here; one on the page's own context goes with that context.
	if v.private == nil && v.downloadHandler != 0 {
		v.w.g.signalDisc(v.downloadOwner, v.downloadHandler)
	}
	v.downloadHandler, v.downloadOwner = 0, 0
	if v.private != nil {
		v.private.leave(v.w.g)
		v.private = nil
	}
	if v.w.webFocus == v.widget {
		v.w.webFocus = 0
		v.w.setNativeKeys(false)
	}
}

// place puts the page where Viva left its hole; a page with no place is hidden.
func (v *gtkWebView) place(at viewPlace, found bool) {
	g := v.w.g
	// A widget unit is not a logical pixel when GTK draws at a scale of its own (X11): the page
	// is zoomed so that its pixels are.
	if zoom := float64(v.w.ratio); found && zoom != v.zoom {
		v.zoom = zoom
		v.k.setZoom(v.widget, zoom)
	}
	if !found {
		if v.shown {
			g.widgetHide(v.widget)
			v.shown = false
		}
		return
	}
	if !v.shown || v.at != at {
		v.at = at
		g.widgetSetMarginX(v.widget, at.x)
		g.widgetSetMarginY(v.widget, at.y)
		g.widgetSetSize(v.widget, max(at.width, 1), max(at.height, 1))
	}
	if !v.shown {
		g.widgetShow(v.widget)
		v.shown = true
	}
}

// message is the page posting text to the application; result is a WebKitJavascriptResult.
func (v *gtkWebView) message(result uintptr) {
	if v.closed || v.config.OnMessage == nil || result == 0 {
		return
	}
	k := v.k
	value := k.resultValue(result)
	text := k.jscString(value)
	if text == 0 {
		return
	}
	defer v.w.g.gFree(text)
	v.config.OnMessage(goString(text))
}

// report tells the application one step of a navigation, with the title the page has at that step
// and, when the step ends a load that failed, why it failed.
func (v *gtkWebView) report(phase platform.WebLoadPhase, address string, code platform.WebLoadError, description string) {
	if v.closed || v.config.OnLoad == nil {
		return
	}
	v.config.OnLoad(platform.WebLoad{
		Phase: phase, URL: address, Title: v.Title(), Code: code, Description: description,
	})
}

// loadChanged follows the page's navigation.
func (v *gtkWebView) loadChanged(event int32) {
	if v.closed {
		return
	}
	address := goString(v.k.getURI(v.widget))
	switch event {
	case webLoadStarted:
		v.failed, v.refused = false, ""
		// A page that loads again has a web process again, so the one it lost is no longer what
		// it is.
		v.failure = platform.WebProcessNoFailure
		v.report(platform.WebLoadStarted, address, platform.WebLoadNoError, "")
	case webLoadCommitted:
		v.report(platform.WebLoadCommitted, address, platform.WebLoadNoError, "")
	case webLoadFinished:
		// A load-failed already reported this load's end, and the interruption of a navigation the
		// application refused is the answer the application gave.
		failed, refused := v.failed, v.refused
		v.failed, v.refused = false, ""
		if failed || (refused != "" && refused == address) {
			return
		}
		v.report(platform.WebLoadFinished, address, platform.WebLoadNoError, "")
	}
}

// gerror is the part of a GError this package reads: the code names the failure and the message is
// the engine's own words for it.
type gerror struct {
	domain  uint32
	code    int32
	message uintptr
}

// loadFailed reports a navigation the engine could not load, with the reason it gave. The load ends
// here: the load-changed that follows it ends the same load, and is not reported again.
func (v *gtkWebView) loadFailed(uri, failure uintptr) {
	if v.closed {
		return
	}
	failedAddress := goString(uri)
	if v.refused != "" && v.refused == failedAddress {
		v.refused = ""
		return
	}
	v.failed = true
	code, description := platform.WebLoadUnknownError, ""
	if failure != 0 {
		err := (*gerror)(unsafePointer(failure))
		code, description = platform.WebLoadErrorFromWebKitError(err.code), goString(err.message)
	}
	v.report(platform.WebLoadFinished, goString(uri), code, description)
}

// processTerminated reports the page's web process gone, with the engine's reason for it: the page
// cannot draw again until it is loaded again, and the reason decides what an application says, so it
// is reported in the terms every engine can be held to rather than the engine's own.
func (v *gtkWebView) processTerminated(reason int32) {
	if v.closed {
		return
	}
	v.failure = platform.WebProcessFailureFromWebKitTerminationReason(reason)
	if v.config.OnProcessFailed != nil {
		v.config.OnProcessFailed(v.failure)
	}
}

// navigationAllowed asks the application whether the view may make a navigation. An application
// that set no Navigate allows every one.
func (v *gtkWebView) navigationAllowed(address string, mainFrame, userGesture bool) bool {
	if v.config.Navigate == nil {
		return true
	}
	return v.config.Navigate(platform.WebNavigation{URL: address, MainFrame: mainFrame, UserGesture: userGesture})
}

// decidePolicy makes the engine's navigation decision from the application's answer, before the
// request leaves for the network. New windows and responses are left to the engine, which asks
// about the first through create and keeps its own policy for the second.
func (v *gtkWebView) decidePolicy(decision uintptr, kind int32) uintptr {
	k := v.k
	if v.closed || k == nil || kind != webDecisionNavigation {
		return 0
	}
	action := k.navigationAction(decision)
	if action == 0 {
		return 0
	}
	// WebKitGTK's navigation action names no target frame (it has no WebKitFrame and the frame API
	// cannot be reached from it), so a subframe navigation cannot be told apart from the main
	// frame's: everything is reported as the main frame. The decision still cancels the right
	// request; only the MainFrame bit is coarse on this engine.
	//
	// The redirect flag needs WebKitGTK 2.20; on an older engine a redirect keeps the gesture of
	// the request that led to it.
	gesture := k.actionUserGesture(action) != 0
	if k.actionRedirect != nil && k.actionRedirect(action) != 0 {
		gesture = false
	}
	address := goString(k.uriRequestURI(k.actionRequest(action)))
	if v.navigationAllowed(address, true, gesture) {
		k.decisionUse(decision)
	} else {
		// A refused navigation that was already loading is interrupted by the engine, which reports
		// the interruption as a failed load: the application answered it, so it is not reported.
		v.refused = address
		k.decisionIgnore(decision)
	}
	return 1
}

// createWindow answers a window.open or target=_blank. The page never opens in the window: an
// allowed address goes to the system's browser, and a refused or nil NewWindow opens nothing.
func (v *gtkWebView) createWindow(action uintptr) {
	if v.closed || v.config.NewWindow == nil || action == 0 {
		return
	}
	address := goString(v.k.uriRequestURI(v.k.actionRequest(action)))
	if v.config.NewWindow(address) && platform.OpenableURL(address) {
		_ = OpenURL(address)
	}
}

// downloadStarted is a download the engine is about to make. The default session is shared by
// every page, and each connected its own handler, so only the page whose view started it answers.
func (v *gtkWebView) downloadStarted(download uintptr) {
	if v.closed || download == 0 || v.k.downloadWebView(download) != v.widget {
		return
	}
	v.w.g.connect(download, "decide-destination", callbacks().webDestination, v.handle)
}

// downloadDestination names the file and asks the application before the body is written. A
// refused download is cancelled; an allowed one is never saved over a file that exists.
func (v *gtkWebView) downloadDestination(download uintptr, suggested string) {
	if v.closed {
		v.k.downloadCancel(download)
		return
	}
	address := ""
	if request := v.k.downloadRequest(download); request != 0 {
		address = goString(v.k.uriRequestURI(request))
	}
	if v.config.Download == nil || !v.config.Download(address) {
		v.k.downloadCancel(download)
		return
	}
	folder := downloadsFolder(v.w.g)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		v.k.downloadCancel(download)
		return
	}
	destination := platform.FreeDownloadPath(folder, platform.DownloadFileName(suggested, address))
	text := cString(destination)
	if v.k.downloadDestinationPath {
		v.k.downloadSetDestination(download, &text[0])
	} else {
		uri := v.w.g.filenameToURI(&text[0], nil, nil)
		if uri == 0 {
			v.k.downloadCancel(download)
			return
		}
		defer v.w.g.gFree(uri)
		v.k.downloadSetDestination(download, (*byte)(unsafePointer(uri)))
	}
	v.k.downloadAllowOverwrite(download, 0)
}

// downloadsFolder is where the desktop says the user keeps downloads (the XDG special directory),
// or the fallback beside the home folder when it does not say.
func downloadsFolder(g *gtkLibrary) string {
	if g.userSpecialDir != nil {
		if directory := goString(g.userSpecialDir(2)); directory != "" {
			return directory
		}
	}
	return platform.DownloadsFolder()
}

// serveScheme answers a request to a custom scheme from the application's resources. An asynchronous
// scheme keeps the request alive and finishes it on the loop, from the answer the application brings
// back.
func serveScheme(g *gtkLibrary, request uintptr) {
	k := webAPI
	if k == nil {
		return
	}
	address := goString(k.schemeURI(request))
	parsed, err := url.Parse(address)
	if err != nil {
		parsed = &url.URL{}
	}
	path := strings.TrimPrefix(parsed.Path, "/")
	scheme := webSchemes[parsed.Scheme]
	// The page the request is for, so an answer that arrives after it closed is dropped. Only
	// WebKitGTK 2.40 and newer can name it: an older engine has no
	// webkit_uri_scheme_request_get_web_view, so the page is unknowable and its answer is finished as
	// the engine asks, as before.
	var page *gtkWebView
	if k.schemeWebView != nil {
		view := k.schemeWebView(request)
		for _, candidate := range webViews {
			if candidate.widget == view && !candidate.closed {
				page = candidate
				break
			}
		}
	}
	if scheme != nil && scheme.ServeAsync != nil {
		// The engine waits for the answer: hold the request until the application gives one, from
		// whatever goroutine did the work, and finish it on the loop.
		k.objectRef(request)
		var answered sync.Once
		scheme.ServeAsync(platform.WebRequest{Path: path}, func(response platform.WebResponse) {
			// A request has one answer: a second call would make the engine finish it twice.
			answered.Do(func() {
				answer := platform.AnswerWebResponse(response, true)
				app := gtkRunning.Load()
				if app == nil {
					// There is no loop left to finish on, and GTK may only be touched from there.
					g.objectUnref(request)
					return
				}
				app.enqueue(func() {
					defer g.objectUnref(request) // the reference held for the wait
					finishScheme(g, k, page, request, answer)
				})
			})
		})
		return
	}
	response, found := platform.ServeWebScheme(scheme, path)
	finishScheme(g, k, page, request, platform.AnswerWebResponse(response, found))
}

// finishScheme sends one answer to the engine. The synchronous form arrives here inside the callback,
// the asynchronous form later on the loop; either way the page may be gone by then. An engine before
// 2.40 cannot name the request's page, so page is nil for a reason other than the page being gone: only
// a request whose page the engine can name is checked, and one it cannot name is finished as before.
func finishScheme(g *gtkLibrary, k *webKit, page *gtkWebView, request uintptr, answer platform.WebAnswer) {
	if k.schemeWebView != nil && (page == nil || page.closed) {
		// The page closed while the answer was being made: the engine has nothing left to show.
		return
	}
	if !k.schemeResponse {
		// An engine without the answer object takes a stream and a type only: the status and the
		// headers are dropped. A 404 is still the failed load the engine shows for a missing resource.
		if answer.Status == 404 {
			domain, message := cString("viva"), cString("not found")
			failure := k.errorLiteral(k.quarkFrom(&domain[0]), 404, &message[0])
			k.schemeFinishError(request, failure)
			g.errorFree(failure)
			return
		}
		stream := schemeStream(k, answer.Body)
		mime := cString(answer.MIME)
		k.schemeFinish(request, stream, int64(len(answer.Body)), &mime[0])
		g.objectUnref(stream)
		return
	}
	stream := schemeStream(k, answer.Body)
	response := k.responseNew(stream, int64(len(answer.Body)))
	reason := cString(answer.Reason)
	k.responseStatus(response, uint32(answer.Status), &reason[0])
	mime := cString(answer.MIME)
	k.responseType(response, &mime[0])
	// The type and the stream's length are fields of the response of their own, so the header set
	// carries everything else. The response takes ownership of it.
	headers := k.soupHeadersNew(1) // SOUP_MESSAGE_HEADERS_RESPONSE
	for _, header := range answer.Headers {
		if header.Name == "Content-Type" || header.Name == "Content-Length" {
			continue
		}
		name, value := cString(header.Name), cString(header.Value)
		k.soupHeadersAppend(headers, &name[0], &value[0])
	}
	k.responseHeaders(response, headers)
	k.finishResponse(request, response)
	g.objectUnref(response)
	g.objectUnref(stream)
}

// schemeStream is the body as a GInputStream the engine reads; the stream frees its memory.
func schemeStream(k *webKit, body []byte) uintptr {
	memory := k.gMalloc(uint64(max(len(body), 1)))
	copy(unsafeSlice(memory, len(body)), body)
	return k.memoryStream(memory, int64(len(body)), k.free)
}

// setNativeViews takes the places the application keeps for native views.
func (w *gtkWindow) setNativeViews(views []platform.NativeView) {
	w.viewRects = append(w.viewRects[:0], views...)
	w.layoutViews()
}

// layoutViews puts the pages where the hole for each is. The hole is on the grid the widgets are
// placed on, so the bounds are brought to whole widget units first.
func (w *gtkWindow) layoutViews() {
	if w.scene == nil {
		return
	}
	ratio := float64(w.ratio)
	w.snapped = w.snapped[:0]
	w.places = w.places[:0]
	for _, view := range w.viewRects {
		b := view.Bounds
		left, top := math.Round(float64(b.X)*ratio), math.Round(float64(b.Y)*ratio)
		right, bottom := math.Round(float64(b.X+b.Width)*ratio), math.Round(float64(b.Y+b.Height)*ratio)
		if right <= left || bottom <= top {
			continue
		}
		w.snapped = append(w.snapped, platform.NativeView{ID: view.ID, Layer: view.Layer, Covers: view.Covers, Bounds: gfx.Rect{
			X: float32(left / ratio), Y: float32(top / ratio), Width: float32((right - left) / ratio), Height: float32((bottom - top) / ratio),
		}})
		w.places = append(w.places, viewPlace{int32(left), int32(top), int32(right - left), int32(bottom - top)})
	}
	w.scene.setViews(w.snapped)
	for _, v := range w.views {
		at, found := viewPlace{}, false
		for i := range w.snapped {
			if w.snapped[i].ID == v.id {
				at, found = w.places[i], true
				break
			}
		}
		v.place(at, found)
	}
	w.updateOverlayRegions()
	w.queuePaint()
}

// updateOverlayRegions makes the pointer, and on X11 the picture, the page's where it shows and
// Viva's where Viva draws over it.
func (w *gtkWindow) updateOverlayRegions() {
	if w.wl != nil {
		w.wl.updateRegions(w, int32(w.width+0.5), int32(w.height+0.5))
		return
	}
	w.layoutPieces()
}

// webFocusChanged is a page gaining or losing the keyboard.
func (w *gtkWindow) webFocusChanged(widget uintptr, in bool) {
	switch {
	case in:
		w.webFocus = widget
		w.setNativeKeys(true)
	case w.webFocus == widget:
		w.webFocus = 0
		w.setNativeKeys(false)
	}
}

// setNativeKeys says who has the keyboard: a page (true) or Viva. While a page has it the keys go
// to GTK's focus widget and the input method is the page's.
func (w *gtkWindow) setNativeKeys(on bool) {
	if w.nativeKeys == on {
		return
	}
	w.nativeKeys = on
	if on {
		clear(w.keys.down)
		if w.config.OnFocusLost != nil {
			frame, err := w.config.OnFocusLost()
			w.present(frame, err)
		}
	}
	if w.ime != nil {
		w.ime.sync()
	}
}

// takeKeys gives the keyboard back to Viva, as a press in Viva's content does.
func (w *gtkWindow) takeKeys() {
	if w.webFocus != 0 {
		w.g.windowSetFocus(w.toplevel, 0)
	}
}
