//go:build unix

package linux

import (
	"errors"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/dbus"
)

// variant is the bytes of a variant holding an unsigned number or a boolean, as the bus sends it.
func variant(signature string, value uint32, nested bool) []byte {
	var encoder dbus.Encoder
	if nested {
		encoder.Variant("v")
	}
	encoder.Variant(signature)
	encoder.Uint32(value)
	return encoder.Bytes()
}

func TestSettingsReadAnswersInEitherVariantShape(t *testing.T) {
	for _, test := range []struct {
		name string
		body []byte
		want portalNumber
	}{
		{"nested", variant("u", 1, true), portalNumber{'u', 1}},
		{"one variant", variant("u", 2, false), portalNumber{'u', 2}},
		{"boolean", variant("b", 0, true), portalNumber{'b', 0}},
		{"a string", []byte{1, 's', 0, 0, 0, 0, 0, 0}, portalNumber{}},
		{"truncated", []byte{1, 'v', 0, 1, 'u', 0}, portalNumber{}},
		{"empty", nil, portalNumber{}},
	} {
		if got := readPortalNumber(dbus.NewDecoder(test.body)); got != test.want {
			t.Errorf("%s: got %+v, want %+v", test.name, got, test.want)
		}
	}
}

func TestASettingChangedSignalNamesTheSettingAndItsValue(t *testing.T) {
	var body dbus.Encoder
	body.String(portalAppearance)
	body.String(portalColorScheme)
	body.Variant("u")
	body.Uint32(1)
	namespace, key, value, ok := parseSettingChanged(body.Bytes())
	if !ok || namespace != portalAppearance || key != portalColorScheme || value != (portalNumber{'u', 1}) {
		t.Fatalf("parsed %q %q %+v %v", namespace, key, value, ok)
	}
	if _, _, _, ok := parseSettingChanged(body.Bytes()[:10]); ok {
		t.Fatal("a truncated signal was taken")
	}
}

func TestTheFileChooserRequestCarriesTheOptionsTheCallerAskedFor(t *testing.T) {
	request := portalFileRequest{
		method: "OpenFile", parent: "x11:2a", title: "Pick", multiple: true,
		filters: []portalFilter{{name: "Images", patterns: []string{"*.png", "*.jpg"}}, {name: "All", patterns: []string{"*"}}},
	}
	decoder := dbus.NewDecoder(encodeFileRequest(request, "tok"))
	if parent, title := decoder.String(), decoder.String(); parent != "x11:2a" || title != "Pick" {
		t.Fatalf("parent %q title %q", parent, title)
	}
	seen := map[string]bool{}
	end := decoder.ArrayLength(8)
	for decoder.Pos() < end && decoder.Err() == nil {
		decoder.Align(8)
		key, signature := decoder.String(), decoder.Signature()
		seen[key] = true
		switch key {
		case "handle_token":
			if signature != "s" || decoder.String() != "tok" {
				t.Fatal("handle_token is wrong")
			}
		case "multiple", "modal":
			if signature != "b" || !decoder.Bool() {
				t.Fatalf("%s is wrong", key)
			}
		case "filters":
			if signature != "a(sa(us))" {
				t.Fatalf("filters signature %q", signature)
			}
			var names []string
			var patterns int
			listEnd := decoder.ArrayLength(8)
			for decoder.Pos() < listEnd && decoder.Err() == nil {
				decoder.Align(8)
				names = append(names, decoder.String())
				patternEnd := decoder.ArrayLength(8)
				for decoder.Pos() < patternEnd && decoder.Err() == nil {
					decoder.Align(8)
					if kind := decoder.Uint32(); kind != 0 {
						t.Fatalf("pattern kind %d", kind)
					}
					_ = decoder.String()
					patterns++
				}
			}
			if strings.Join(names, ",") != "Images,All" || patterns != 3 {
				t.Fatalf("filters %v with %d patterns", names, patterns)
			}
		default:
			decoder.Skip(signature)
		}
	}
	if decoder.Err() != nil || !decoder.Done() {
		t.Fatalf("decoder: %v, done %v", decoder.Err(), decoder.Done())
	}
	for _, key := range []string{"handle_token", "modal", "multiple", "filters"} {
		if !seen[key] {
			t.Errorf("option %s is missing", key)
		}
	}
	if seen["directory"] || seen["current_name"] {
		t.Error("an open request carries options of other requests")
	}

	save := encodeFileRequest(portalFileRequest{method: "SaveFile", title: "Save", currentName: "a.txt", multiple: true, directory: true}, "t")
	if !strings.Contains(string(save), "current_name") || strings.Contains(string(save), "multiple") || strings.Contains(string(save), "directory") {
		t.Error("a save request carries the wrong options")
	}
}

func TestTheFileChooserResponseGivesTheURIsOrTheCancellation(t *testing.T) {
	response := func(code uint32, uris ...string) []byte {
		var encoder dbus.Encoder
		encoder.Uint32(code)
		results := encoder.BeginArray(8)
		encoder.Align(8)
		encoder.String("current_filter")
		encoder.Variant("(sa(us))")
		encoder.Align(8)
		encoder.String("All")
		patterns := encoder.BeginArray(8)
		encoder.EndArray(patterns)
		if len(uris) > 0 {
			encoder.Align(8)
			encoder.String("uris")
			encoder.Variant("as")
			list := encoder.BeginArray(4)
			for _, uri := range uris {
				encoder.String(uri)
			}
			encoder.EndArray(list)
		}
		encoder.EndArray(results)
		return encoder.Bytes()
	}
	code, uris, err := parseFileResponse(response(0, "file:///a", "file:///b%20c"))
	if err != nil || code != 0 || len(uris) != 2 || uris[1] != "file:///b%20c" {
		t.Fatalf("code %d uris %v err %v", code, uris, err)
	}
	if code, uris, err := parseFileResponse(response(1)); err != nil || code != 1 || len(uris) != 0 {
		t.Fatalf("cancel: code %d uris %v err %v", code, uris, err)
	}
	if _, _, err := parseFileResponse(response(0, "file:///a")[:12]); err == nil {
		t.Fatal("a truncated response was taken")
	}
}

func TestARequestHandleIsPredictableFromTheConnectionName(t *testing.T) {
	if got, want := requestHandle(":1.42", "viva7_1"), "/org/freedesktop/portal/desktop/request/1_42/viva7_1"; got != want {
		t.Fatalf("handle = %q, want %q", got, want)
	}
}

func TestANotificationIsTheSevenArgumentsTheServiceExpects(t *testing.T) {
	decoder := dbus.NewDecoder(encodeNotify("app", "Title", "Body"))
	if name, replaces, icon, summary, body := decoder.String(), decoder.Uint32(), decoder.String(), decoder.String(), decoder.String(); name != "app" || replaces != 0 || icon != "" || summary != "Title" || body != "Body" {
		t.Fatalf("arguments %q %d %q %q %q", name, replaces, icon, summary, body)
	}
	if actions := decoder.ArrayLength(4); actions != decoder.Pos() {
		t.Fatalf("actions are not empty")
	}
	if hints := decoder.ArrayLength(8); hints != decoder.Pos() {
		t.Fatalf("hints are not empty")
	}
	if timeout := decoder.Int32(); timeout != -1 || decoder.Err() != nil || !decoder.Done() {
		t.Fatalf("timeout %d, err %v, done %v", timeout, decoder.Err(), decoder.Done())
	}
	if got := escapeMarkup("a < b & c > d"); got != "a &lt; b &amp; c &gt; d" {
		t.Fatalf("escaped %q", got)
	}
}

func TestOnlyAMissingNameMeansThereIsNoPortal(t *testing.T) {
	missing := &dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown"}
	oldPortal := &dbus.Error{Name: "org.freedesktop.DBus.Error.UnknownMethod"}
	noKey := &dbus.Error{Name: "org.freedesktop.portal.Error.NotFound"}
	if !busNameMissing(missing) || busNameMissing(oldPortal) || busNameMissing(noKey) || busNameMissing(errors.New("x")) {
		t.Error("busNameMissing is wrong")
	}
	if !busServiceMissing(missing) || !busServiceMissing(oldPortal) || busServiceMissing(noKey) {
		t.Error("busServiceMissing is wrong")
	}
}
