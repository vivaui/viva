//go:build darwin || windows || linux

package app

import "testing"

func TestDriverRecordsASchemeURLAndAnswersFromDeclaredHandlers(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if available, err := SchemeHandlerAvailable("vscode"); available || err != nil {
		t.Fatalf("before any declaration: available = %t, err = %v", available, err)
	}
	driver.SetSchemeHandlers("zed", "vscode")
	for scheme, want := range map[string]bool{"vscode": true, "zed": true, "cursor": false} {
		if available, err := SchemeHandlerAvailable(scheme); available != want || err != nil {
			t.Fatalf("%s: available = %t, err = %v; want %t", scheme, available, err, want)
		}
	}
	driver.SetSchemeHandlers("zed")
	if available, _ := SchemeHandlerAvailable("vscode"); available {
		t.Fatal("a later declaration did not replace the earlier one")
	}

	const link = "zed://ssh/prod/srv/app"
	if err := OpenSchemeURL("zed", link); err != nil {
		t.Fatal(err)
	}
	if opened, ok := driver.LastOpenedURL(); !ok || opened != link {
		t.Fatalf("recorded = %q, %t; want %q", opened, ok, link)
	}
	if err := OpenSchemeURL("vscode", "zed://x"); err == nil {
		t.Fatal("a link for another scheme was accepted")
	}
}
