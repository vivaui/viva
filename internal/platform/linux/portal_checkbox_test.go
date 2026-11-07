//go:build unix

package linux

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/dbus"
)

func TestPortalSaveCheckboxChoiceEncodingAndResult(t *testing.T) {
	request := portalFileRequest{method: "SaveFile", title: "Save", checkboxLabel: "Save Folder as Workspace", checkboxChecked: true}
	decoder := dbus.NewDecoder(encodeFileRequest(request, "token"))
	_ = decoder.String()
	_ = decoder.String()
	end := decoder.ArrayLength(8)
	found := false
	for decoder.Pos() < end && decoder.Err() == nil {
		decoder.Align(8)
		key, signature := decoder.String(), decoder.Signature()
		if key != "choices" {
			decoder.Skip(signature)
			continue
		}
		if signature != "a(ssa(ss)s)" {
			t.Fatalf("choices signature = %q", signature)
		}
		choiceEnd := decoder.ArrayLength(8)
		decoder.Align(8)
		identifier, label := decoder.String(), decoder.String()
		if optionsEnd := decoder.ArrayLength(8); decoder.Pos() != optionsEnd {
			t.Fatal("checkbox has nonempty options")
		}
		selected := decoder.String()
		if identifier != "viva_save_checkbox" || label != request.checkboxLabel || selected != "true" || decoder.Pos() != choiceEnd {
			t.Fatalf("choice = %q, %q, %q", identifier, label, selected)
		}
		found = true
	}
	if !found || decoder.Err() != nil {
		t.Fatalf("portal checkbox missing or invalid: %v", decoder.Err())
	}
	var encoder dbus.Encoder
	encoder.Uint32(0)
	results := encoder.BeginArray(8)
	encoder.Align(8)
	encoder.String("choices")
	encoder.Variant("a(ss)")
	choices := encoder.BeginArray(8)
	encoder.Align(8)
	encoder.String("viva_save_checkbox")
	encoder.String("false")
	encoder.EndArray(choices)
	encoder.EndArray(results)
	code, _, selected, err := parseFileResponseWithChoices(encoder.Bytes())
	if code != 0 || err != nil || selected["viva_save_checkbox"] != "false" {
		t.Fatalf("portal selected = %d, %+v, %v", code, selected, err)
	}
}

func TestPortalSaveCheckboxRejectsMissingOrInvalidChoice(t *testing.T) {
	for _, choices := range []map[string]string{
		nil,
		{"viva_save_checkbox": ""},
		{"viva_save_checkbox": "yes"},
	} {
		path, checked, canceled, err := portalSaveCheckboxResult([]string{"/tmp/session.xml"}, choices, false, nil)
		if path != "" || checked || canceled || !errors.Is(err, errServiceMissing) {
			t.Fatalf("choice %v returned %q, %v, %v, %v", choices, path, checked, canceled, err)
		}
	}
	if path, checked, canceled, err := portalSaveCheckboxResult([]string{"/tmp/session.xml"}, nil, true, nil); path != "" || checked || !canceled || err != nil {
		t.Fatalf("canceled choice returned %q, %v, %v, %v", path, checked, canceled, err)
	}
	if path, checked, canceled, err := portalSaveCheckboxResult(nil, nil, false, errServiceMissing); path != "" || checked || canceled || !errors.Is(err, errServiceMissing) {
		t.Fatalf("missing portal returned %q, %v, %v, %v", path, checked, canceled, err)
	}
	if path, checked, canceled, err := portalSaveCheckboxResult([]string{"/tmp/session.xml"}, map[string]string{"viva_save_checkbox": "false"}, false, nil); path != "/tmp/session.xml" || checked || canceled || err != nil {
		t.Fatalf("unchecked choice returned %q, %v, %v, %v", path, checked, canceled, err)
	}
	if path, checked, canceled, err := portalSaveCheckboxResult([]string{"/tmp/session.xml"}, map[string]string{"viva_save_checkbox": "true"}, false, nil); path != "/tmp/session.xml" || !checked || canceled || err != nil {
		t.Fatalf("checked choice returned %q, %v, %v, %v", path, checked, canceled, err)
	}
}
