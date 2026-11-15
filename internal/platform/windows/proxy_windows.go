//go:build windows

package windows

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"syscall"
	"unsafe"
)

// ErrProxyUnavailable reports that WinHTTP refused or failed the automatic
// proxy request.
var ErrProxyUnavailable = errors.New("windows: the system proxy settings cannot be read")

const (
	winHTTPAccessTypeAutomaticProxy = 4
	winHTTPAutoproxyAutoDetect      = 1
	winHTTPAutoproxyConfigURL       = 2
	winHTTPAutoDetectDHCP           = 1
	winHTTPAutoDetectDNS            = 2
	winHTTPAccessTypeNoProxy        = 1
)

var (
	winhttp            = systemDLL("winhttp.dll")
	winHTTPOpen        = winhttp.NewProc("WinHttpOpen")
	winHTTPGetIEProxy  = winhttp.NewProc("WinHttpGetIEProxyConfigForCurrentUser")
	winHTTPGetProxyFor = winhttp.NewProc("WinHttpGetProxyForUrl")
	winHTTPClose       = winhttp.NewProc("WinHttpCloseHandle")
	winHTTPFree        = kernel.NewProc("GlobalFree")
)

// The next three types are WinHTTP's structures; the layout is checked by the
// package's Windows test, which reads what the system answers for a URL.
type winHTTPAutoproxyOptions struct {
	Flags                 uint32
	AutoDetectFlags       uint32
	AutoConfigURL         *uint16
	Reserved              unsafe.Pointer
	ReservedDWORD         uint32
	AutoLogonIfChallenged int32
}

type winHTTPProxyInfo struct {
	AccessType  uint32
	Proxy       *uint16
	ProxyBypass *uint16
}

type winHTTPCurrentUserIEProxyConfig struct {
	AutoDetect    int32
	AutoConfigURL *uint16
	Proxy         *uint16
	ProxyBypass   *uint16
}

// SystemProxy asks WinHTTP which proxy the system selects for target: the
// per-user static settings, auto-detection (WPAD), or a PAC script, whichever
// the machine uses.
func SystemProxy(target string) (*url.URL, error) {
	session, _, callErr := winHTTPOpen.Call(0, winHTTPAccessTypeAutomaticProxy, 0, 0, 0)
	if session == 0 {
		return nil, fmt.Errorf("%w: WinHttpOpen: %v", ErrProxyUnavailable, callErr)
	}
	defer winHTTPClose.Call(session)
	var config winHTTPCurrentUserIEProxyConfig
	if ok, _, callErr := winHTTPGetIEProxy.Call(uintptr(unsafe.Pointer(&config))); ok == 0 {
		return nil, fmt.Errorf("%w: WinHttpGetIEProxyConfigForCurrentUser: %v", ErrProxyUnavailable, callErr)
	}
	defer freeWinHTTPString(config.AutoConfigURL)
	defer freeWinHTTPString(config.Proxy)
	defer freeWinHTTPString(config.ProxyBypass)
	// A machine with neither auto-detection nor a PAC script has no policy to
	// evaluate: either the static proxy is the answer, or traffic goes direct.
	if config.AutoDetect == 0 && config.AutoConfigURL == nil {
		if config.Proxy == nil {
			return nil, nil
		}
		return parseWinHTTPProxy(utf16PtrToString(config.Proxy), target)
	}
	options := winHTTPAutoproxyOptions{
		Flags:                 winHTTPAutoproxyAutoDetect,
		AutoDetectFlags:       winHTTPAutoDetectDHCP | winHTTPAutoDetectDNS,
		AutoLogonIfChallenged: 1,
	}
	if config.AutoConfigURL != nil {
		options.Flags |= winHTTPAutoproxyConfigURL
		options.AutoConfigURL = config.AutoConfigURL
	}
	pointer, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProxyUnavailable, err)
	}
	var info winHTTPProxyInfo
	if ok, _, callErr := winHTTPGetProxyFor.Call(
		session, uintptr(unsafe.Pointer(pointer)),
		uintptr(unsafe.Pointer(&options)), uintptr(unsafe.Pointer(&info)),
	); ok == 0 {
		// Auto-detection that finds no WPAD server is the normal answer on a
		// machine without one: the static settings, or direct, are what the
		// system itself would use. A PAC script that fails is a policy that
		// could not be evaluated, and is reported instead of bypassed.
		if config.Proxy != nil {
			return parseWinHTTPProxy(utf16PtrToString(config.Proxy), target)
		}
		if config.AutoConfigURL != nil {
			return nil, fmt.Errorf("%w: WinHttpGetProxyForUrl: %v", ErrProxyUnavailable, callErr)
		}
		return nil, nil
	}
	defer freeWinHTTPString(info.Proxy)
	defer freeWinHTTPString(info.ProxyBypass)
	if info.AccessType == winHTTPAccessTypeNoProxy {
		return nil, nil
	}
	text := utf16PtrToString(info.Proxy)
	if text == "" && config.Proxy != nil {
		text = utf16PtrToString(config.Proxy)
	}
	return parseWinHTTPProxy(text, target)
}

// utf16PtrToString reads a NUL-terminated UTF-16 string the system allocated.
func utf16PtrToString(value *uint16) string {
	if value == nil {
		return ""
	}
	length := 0
	for pointer := unsafe.Pointer(value); *(*uint16)(pointer) != 0; pointer = unsafe.Add(pointer, 2) {
		length++
	}
	if length == 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice(value, length))
}

// parseWinHTTPProxy turns WinHTTP's "host:port" or "http=host:port;https=..." into
// a URL for target's scheme.
func parseWinHTTPProxy(text, target string) (*url.URL, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	hostPort := text
	if strings.Contains(text, "=") {
		wanted := "http"
		if strings.HasPrefix(target, "https:") {
			wanted = "https"
		}
		chosen := ""
		for _, part := range strings.Split(text, ";") {
			scheme, value, ok := strings.Cut(strings.TrimSpace(part), "=")
			if !ok {
				continue
			}
			if scheme == wanted {
				chosen = value
				break
			}
			if scheme == "http" && chosen == "" {
				chosen = value
			}
		}
		if chosen == "" {
			return nil, nil
		}
		hostPort = strings.TrimSpace(chosen)
	}
	proxy, err := url.Parse("http://" + hostPort)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProxyUnavailable, err)
	}
	return proxy, nil
}

func freeWinHTTPString(value *uint16) {
	if value != nil {
		winHTTPFree.Call(uintptr(unsafe.Pointer(value)))
	}
}
