package app

import (
	"fmt"
	"strings"
)

// maxSchemeURL bounds a link handed to another application; Windows' shell
// refuses anything near this long anyway.
const maxSchemeURL = 8192

// reservedSchemes are the schemes OpenSchemeURL refuses: the ones OpenURL
// serves, and the ones whose handler runs or renders what the link carries.
var reservedSchemes = [...]string{
	"http", "https", "mailto", "ftp", "ws", "wss",
	"file", "javascript", "data", "vbscript", "blob", "about",
}

// OpenSchemeURL hands target to the application the system has registered for a
// custom URL scheme, as an editor registers vscode:// or a chat client its own,
// so the application can ask it to open something. The scheme must be a constant
// the application chose: never one taken from a file, a server or a user, because
// the system routes the link to whatever claims it. The scheme is lower case, as
// RFC 3986 writes it; web, mail and file schemes are refused here (OpenURL serves
// the first two, and a link that names a local file is OpenPath's). The target
// must start with scheme and a colon and be printable ASCII, so a caller
// percent-encodes what it embeds. Driver-backed runs record the request, which
// Driver.LastOpenedURL returns, and open nothing.
//
// A nil error means the system accepted the link, not that the application did
// anything with it. SchemeHandlerAvailable says beforehand whether anything has
// claimed the scheme.
func OpenSchemeURL(scheme, target string) error {
	if err := checkSchemeURL(scheme, target); err != nil {
		return err
	}
	if err, handled := driverOpenURL(target); handled {
		return err
	}
	return nativeOpenURL(target)
}

// SchemeHandlerAvailable reports whether an application has registered for a
// custom URL scheme, so a menu can offer only what would open. It reads the
// system's own registry and launches nothing. Linux asks xdg-mime, which runs a
// short process, so call it when a choice is shown and keep the answer; a system
// that cannot tell (Linux without xdg-mime, the web) returns ErrUnsupported, and
// the caller should then offer the choice rather than hide it. An application
// that registers its scheme only when first started (VS Code on Windows) reads
// as unavailable until then. The scheme follows OpenSchemeURL's rules.
func SchemeHandlerAvailable(scheme string) (bool, error) {
	if err := checkScheme(scheme); err != nil {
		return false, err
	}
	if available, handled := driverSchemeHandler(scheme); handled {
		return available, nil
	}
	return nativeSchemeHandlerAvailable(scheme)
}

// checkScheme accepts a lower-case RFC 3986 scheme that is not reserved.
func checkScheme(scheme string) error {
	if scheme == "" || len(scheme) > 32 {
		return fmt.Errorf("app: URL scheme %q must be 1 to 32 characters", scheme)
	}
	for index := 0; index < len(scheme); index++ {
		character := scheme[index]
		letter := character >= 'a' && character <= 'z'
		if !letter && (index == 0 || !(character >= '0' && character <= '9' || character == '+' || character == '-' || character == '.')) {
			return fmt.Errorf("app: URL scheme %q must be lower-case letters, digits, +, - and ., starting with a letter", scheme)
		}
	}
	for _, reserved := range reservedSchemes {
		if scheme == reserved {
			return fmt.Errorf("app: URL scheme %q is not for OpenSchemeURL", scheme)
		}
	}
	return nil
}

func checkSchemeURL(scheme, target string) error {
	if err := checkScheme(scheme); err != nil {
		return err
	}
	if len(target) > maxSchemeURL {
		return fmt.Errorf("app: %s URL is longer than %d bytes", scheme, maxSchemeURL)
	}
	if len(target) <= len(scheme) || !strings.EqualFold(target[:len(scheme)], scheme) || target[len(scheme)] != ':' {
		return fmt.Errorf("app: URL does not start with %s:", scheme)
	}
	for index := 0; index < len(target); index++ {
		if character := target[index]; character <= ' ' || character >= 0x7f {
			return fmt.Errorf("app: %s URL holds a byte (0x%02x) that must be percent-encoded", scheme, character)
		}
	}
	return nil
}
