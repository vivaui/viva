//go:build unix

package linux

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vivaui/viva/internal/dbus"
)

// The desktop portals (xdg-desktop-portal) are how a desktop answers an application
// over the session bus whatever the toolkit and whether or not it is sandboxed: the
// settings it keeps, the file chooser it shows. The notification service is the
// desktop's own. Everything here is a message on the bus, so nothing needs a helper
// program or a library, and nothing keeps a thread: a caller that waits for the user
// blocks on its own connection.

const (
	portalName      = "org.freedesktop.portal.Desktop"
	portalPath      = "/org/freedesktop/portal/desktop"
	portalSettings  = "org.freedesktop.portal.Settings"
	portalChooser   = "org.freedesktop.portal.FileChooser"
	portalRequest   = "org.freedesktop.portal.Request"
	portalRequests  = portalPath + "/request/"
	dbusName        = "org.freedesktop.DBus"
	dbusPath        = "/org/freedesktop/DBus"
	notifyName      = "org.freedesktop.Notifications"
	notifyPath      = "/org/freedesktop/Notifications"
	settingsMatch   = "type='signal',sender='" + portalName + "',interface='" + portalSettings + "',member='SettingChanged'"
	portalGoneMatch = "type='signal',sender='" + dbusName + "',interface='" + dbusName + "',member='NameOwnerChanged',arg0='" + portalName + "'"
)

// errServiceMissing wraps an error that says the service asked is not on the bus (or, for
// a portal, is too old to know the call): the caller falls back to what it used before
// the service existed.
var errServiceMissing = errors.New("linux: the desktop service is not available")

// busNameMissing reports whether a bus error means nobody owns the name asked: there is
// no such service, and none that the bus can start.
func busNameMissing(err error) bool {
	var failure *dbus.Error
	if !errors.As(err, &failure) {
		return false
	}
	switch failure.Name {
	case "org.freedesktop.DBus.Error.ServiceUnknown", "org.freedesktop.DBus.Error.NameHasNoOwner",
		"org.freedesktop.DBus.Error.NoReply", "org.freedesktop.DBus.Error.Spawn.ServiceNotFound",
		"org.freedesktop.DBus.Error.Spawn.ExecFailed":
		return true
	}
	return false
}

// busServiceMissing is busNameMissing, or a service so old that it does not know the call.
func busServiceMissing(err error) bool {
	if busNameMissing(err) {
		return true
	}
	var failure *dbus.Error
	if !errors.As(err, &failure) {
		return false
	}
	switch failure.Name {
	case "org.freedesktop.DBus.Error.UnknownMethod", "org.freedesktop.DBus.Error.UnknownObject",
		"org.freedesktop.DBus.Error.UnknownInterface":
		return true
	}
	return false
}

type portalCaller interface {
	Call(destination, path, iface, member, signature string, body []byte) (*dbus.Message, error)
}

func addMatch(conn portalCaller, rule string) error {
	var match dbus.Encoder
	match.String(rule)
	_, err := conn.Call(dbusName, dbusPath, dbusName, "AddMatch", "s", match.Bytes())
	return err
}

// readPortalNumber reads a setting's value from a body whose next value is a variant
// (the portal's Read wraps it in a second one).
func readPortalNumber(decoder *dbus.Decoder) portalNumber {
	signature := decoder.Signature()
	for signature == "v" && decoder.Err() == nil {
		signature = decoder.Signature()
	}
	if decoder.Err() != nil {
		return portalNumber{}
	}
	switch signature {
	case "u", "b":
		number := decoder.Uint32()
		if decoder.Err() == nil {
			return portalNumber{kind: signature[0], number: number}
		}
	}
	return portalNumber{}
}

// parseSettingChanged reads the portal's SettingChanged signal: the namespace, the key and
// the new value.
func parseSettingChanged(body []byte) (namespace, key string, value portalNumber, ok bool) {
	decoder := dbus.NewDecoder(body)
	namespace, key = decoder.String(), decoder.String()
	value = readPortalNumber(decoder)
	return namespace, key, value, decoder.Err() == nil
}

// readPortalSetting asks the portal for one setting.
func readPortalSetting(conn portalCaller, namespace, key string) (portalNumber, error) {
	var request dbus.Encoder
	request.String(namespace)
	request.String(key)
	reply, err := conn.Call(portalName, portalPath, portalSettings, "Read", "ss", request.Bytes())
	if err != nil {
		return portalNumber{}, err
	}
	decoder := reply.Args()
	value := readPortalNumber(decoder)
	if decoder.Err() != nil {
		return portalNumber{}, decoder.Err()
	}
	return value, nil
}

type portalReadResult uint8

const (
	portalReadOK portalReadResult = iota
	portalReadRetry
	portalReadUnsupported
)

// portalServiceTransient reports errors that a restarting bus or portal can answer
// while its name is changing owners. They must not become a process-wide lockout.
func portalServiceTransient(err error) bool {
	var failure *dbus.Error
	if !errors.As(err, &failure) {
		return false
	}
	switch failure.Name {
	case "org.freedesktop.DBus.Error.NameHasNoOwner", "org.freedesktop.DBus.Error.NoReply",
		"org.freedesktop.DBus.Error.Disconnected", "org.freedesktop.DBus.Error.Timeout",
		"org.freedesktop.DBus.Error.TimedOut":
		return true
	}
	return false
}

// readPortalValues asks the portal for the settings Viva follows. A setting the
// portal does not have is left unknown; transport failure is distinguished from
// a service that is permanently unavailable.
func readPortalValues(conn portalCaller) (values portalValues, result portalReadResult) {
	for _, setting := range [...][2]string{
		{portalAppearance, portalColorScheme},
		{portalAppearance, portalReducedMo},
		{portalGnomeUI, portalAnimationsOn},
	} {
		value, err := readPortalSetting(conn, setting[0], setting[1])
		if err != nil {
			if portalServiceTransient(err) {
				return values, portalReadRetry
			}
			if busNameMissing(err) {
				return values, portalReadUnsupported
			}
			var failure *dbus.Error
			if !errors.As(err, &failure) {
				return values, portalReadRetry
			}
			continue // a portal that does not have this setting
		}
		values.apply(setting[0], setting[1], value)
	}
	return values, portalReadOK
}

// portalFilter is one entry of a file chooser's filter list.
type portalFilter struct {
	name     string
	patterns []string
}

// portalFileRequest is a call to the portal's file chooser.
type portalFileRequest struct {
	// method is OpenFile or SaveFile.
	method string
	// parent is the window the dialog belongs to, as the portal names it ("x11:<id>"), or "".
	parent              string
	title               string
	multiple, directory bool
	filters             []portalFilter
	currentName         string
}

// encodeFileRequest is the arguments of OpenFile or SaveFile: the parent window, the
// title and the options.
func encodeFileRequest(request portalFileRequest, token string) []byte {
	var encoder dbus.Encoder
	encoder.String(request.parent)
	encoder.String(request.title)
	options := encoder.BeginArray(8)
	option := func(key, signature string, write func()) {
		encoder.Align(8)
		encoder.String(key)
		encoder.Variant(signature)
		write()
	}
	option("handle_token", "s", func() { encoder.String(token) })
	option("modal", "b", func() { encoder.Bool(true) })
	if request.method == "OpenFile" {
		if request.multiple {
			option("multiple", "b", func() { encoder.Bool(true) })
		}
		if request.directory {
			option("directory", "b", func() { encoder.Bool(true) })
		}
	}
	if len(request.filters) > 0 {
		option("filters", "a(sa(us))", func() {
			list := encoder.BeginArray(8)
			for _, filter := range request.filters {
				encoder.Align(8)
				encoder.String(filter.name)
				patterns := encoder.BeginArray(8)
				for _, pattern := range filter.patterns {
					encoder.Align(8)
					encoder.Uint32(0) // a glob pattern
					encoder.String(pattern)
				}
				encoder.EndArray(patterns)
			}
			encoder.EndArray(list)
		})
	}
	if request.currentName != "" {
		option("current_name", "s", func() { encoder.String(request.currentName) })
	}
	encoder.EndArray(options)
	return encoder.Bytes()
}

// parseFileResponse reads the Response signal of a portal request: the code (0 the user
// chose, anything else not) and the URIs chosen.
func parseFileResponse(body []byte) (code uint32, uris []string, err error) {
	decoder := dbus.NewDecoder(body)
	code = decoder.Uint32()
	end := decoder.ArrayLength(8)
	for decoder.Pos() < end && decoder.Err() == nil {
		decoder.Align(8)
		key := decoder.String()
		signature := decoder.Signature()
		if key == "uris" && signature == "as" {
			listEnd := decoder.ArrayLength(4)
			for decoder.Pos() < listEnd && decoder.Err() == nil {
				uris = append(uris, decoder.String())
			}
			continue
		}
		decoder.Skip(signature)
	}
	return code, uris, decoder.Err()
}

// requestHandle is the object path the portal gives the request a connection makes with a
// token, which a caller can know before the call and so not miss the answer.
func requestHandle(uniqueName, token string) string {
	sender := strings.ReplaceAll(strings.TrimPrefix(uniqueName, ":"), ".", "_")
	return portalRequests + sender + "/" + token
}

var requestCounter atomic.Uint32

// portalChoose shows the portal's file chooser and waits for the user. canceled is true
// when the user dismissed it. An error wrapping errServiceMissing means no dialog was
// shown and the caller may use another way.
func portalChoose(request portalFileRequest) (paths []string, canceled bool, err error) {
	conn, err := dbus.DialSession()
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v", errServiceMissing, err)
	}
	defer conn.Close()
	token := fmt.Sprintf("viva%d_%d", os.Getpid(), requestCounter.Add(1))
	handle := requestHandle(conn.UniqueName(), token)
	if err := addMatch(conn, "type='signal',interface='"+portalRequest+"',member='Response',path='"+handle+"'"); err != nil {
		return nil, false, fmt.Errorf("%w: %v", errServiceMissing, err)
	}
	if err := addMatch(conn, portalGoneMatch); err != nil {
		return nil, false, fmt.Errorf("%w: %v", errServiceMissing, err)
	}
	reply, err := conn.Call(portalName, portalPath, portalChooser, request.method, "ssa{sv}", encodeFileRequest(request, token))
	if err != nil {
		if busServiceMissing(err) {
			return nil, false, fmt.Errorf("%w: %v", errServiceMissing, err)
		}
		// The chooser's backend (the desktop's own portal implementation) is what is
		// missing when the portal itself answers this way.
		var failure *dbus.Error
		if errors.As(err, &failure) && strings.HasPrefix(failure.Name, "org.freedesktop.portal.Error") {
			return nil, false, fmt.Errorf("%w: %v", errServiceMissing, err)
		}
		return nil, false, fmt.Errorf("linux: file chooser portal: %w", err)
	}
	if actual := reply.Args().String(); actual != "" && actual != handle {
		// A portal older than the token convention picks the path itself.
		handle = actual
		if err := addMatch(conn, "type='signal',interface='"+portalRequest+"',member='Response',path='"+handle+"'"); err != nil {
			return nil, false, err
		}
	}
	for {
		messages, err := conn.Wait(time.Minute)
		if err != nil {
			return nil, false, fmt.Errorf("linux: file chooser portal: %w", err)
		}
		for _, message := range messages {
			if message.Type != dbus.TypeSignal {
				continue
			}
			switch {
			case message.Member == "Response" && message.Path == handle:
				code, uris, err := parseFileResponse(message.Body)
				if err != nil {
					return nil, false, fmt.Errorf("linux: file chooser portal answered badly: %w", err)
				}
				if code != 0 {
					// 1 is the user dismissing the dialog, and 2 "ended in some other
					// way", which is what a GTK backend answers to Escape; no toolkit
					// tells the two apart for the user.
					return nil, true, nil
				}
				for _, uri := range uris {
					path, ok := fileURIPath(uri)
					if !ok {
						return nil, false, fmt.Errorf("linux: file chooser portal returned %q, which is not a local file", uri)
					}
					paths = append(paths, path)
				}
				return paths, false, nil
			case message.Member == "NameOwnerChanged" && message.Signature == "sss":
				arguments := message.Args()
				if name, _, owner := arguments.String(), arguments.String(), arguments.String(); name == portalName && owner == "" {
					return nil, false, fmt.Errorf("linux: the desktop portal went away while its file chooser was open")
				}
			}
		}
	}
}

// escapeMarkup writes text as the notification body's markup subset has it.
func escapeMarkup(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}

// encodeNotify is the arguments of Notifications.Notify: no icon of its own, no actions,
// no hints, and the server's default expiry.
func encodeNotify(appName, summary, body string) []byte {
	var encoder dbus.Encoder
	encoder.String(appName)
	encoder.Uint32(0) // a new notification, replacing none
	encoder.String("")
	encoder.String(summary)
	encoder.String(body)
	actions := encoder.BeginArray(4)
	encoder.EndArray(actions)
	hints := encoder.BeginArray(8)
	encoder.EndArray(hints)
	encoder.Int32(-1)
	return encoder.Bytes()
}

// notifyOverBus posts a notification with the desktop's notification service. An error
// wrapping errServiceMissing means there is none to ask.
func notifyOverBus(appName, summary, body string) error {
	conn, err := dbus.DialSession()
	if err != nil {
		return fmt.Errorf("%w: %v", errServiceMissing, err)
	}
	defer conn.Close()
	// A server that renders markup would take a "<" of the text for a tag. The question is
	// also the cheapest way to learn that nobody serves: a name the bus can start but whose
	// program never comes up is not answered within the call's timeout.
	capabilities, err := conn.Call(notifyName, notifyPath, notifyName, "GetCapabilities", "", nil)
	if err != nil {
		return fmt.Errorf("%w: %v", errServiceMissing, err)
	}
	decoder := capabilities.Args()
	end := decoder.ArrayLength(4)
	for decoder.Pos() < end && decoder.Err() == nil {
		if decoder.String() == "body-markup" {
			body = escapeMarkup(body)
			break
		}
	}
	if _, err := conn.Call(notifyName, notifyPath, notifyName, "Notify", "susssasa{sv}i", encodeNotify(appName, summary, body)); err != nil {
		if busServiceMissing(err) {
			return fmt.Errorf("%w: %v", errServiceMissing, err)
		}
		return fmt.Errorf("linux: send notification: %w", err)
	}
	return nil
}
