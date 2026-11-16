package windows

import (
	"syscall"
	"testing"
	"unsafe"
)

// familyExists asks DirectWrite whether name is a family of the system collection,
// the way every text format here resolves one.
func familyExists(services *textServices, name string) (bool, error) {
	var collection uintptr
	if err := hresult(comCall(services.write, writeFactoryGetSystemFontCollection, uintptr(unsafe.Pointer(&collection)), 0)); err != nil || collection == 0 {
		return false, err
	}
	defer release(&collection)
	wide, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false, err
	}
	var index, exists uint32
	if err := hresult(comCall(collection, writeCollectionFindFamilyName, uintptr(unsafe.Pointer(wide)), uintptr(unsafe.Pointer(&index)), uintptr(unsafe.Pointer(&exists)))); err != nil {
		return false, err
	}
	return exists != 0, nil
}

// Every listed name must be a family DirectWrite resolves: a settings page applies
// a name from the list through CreateTextFormat, and the typeface names GDI reports
// ("Maple Mono Light", "Cascadia Mono SemiBold") are not families, so choosing one
// silently kept the fallback font and previewed it instead.
func TestEveryListedFamilyResolves(t *testing.T) {
	services := &textServices{}
	if err := services.initializeWrite(); err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	defer services.close()
	lists := []struct {
		name     string
		families []string
	}{
		{"FontFamilies", FontFamilies()},
		{"MonospaceFamilies", MonospaceFamilies()},
	}
	for _, list := range lists {
		if len(list.families) == 0 {
			t.Fatalf("%s listed no family", list.name)
		}
		resolved := map[string]bool{}
		for _, family := range list.families {
			if resolved[family] {
				t.Errorf("%s lists %q twice", list.name, family)
				continue
			}
			exists, err := familyExists(services, family)
			if err != nil {
				t.Fatalf("%s: %q: %v", list.name, family, err)
			}
			if !exists {
				t.Errorf("%s lists %q, which DirectWrite does not resolve", list.name, family)
			}
			resolved[family] = true
		}
	}
}
