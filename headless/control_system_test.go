//go:build darwin || windows || linux

package headless

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
)

func TestControlReportsPublicSystemIntegrationRequests(t *testing.T) {
	session, err := Open(app.Config{}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if err := app.SetClipboardText("remote text"); err != nil {
		t.Fatal(err)
	}
	if reply, _ := execute(session, "clip"); reply != "ok \"remote text\"\n" {
		t.Fatalf("clip reply = %q", reply)
	}
	if err := app.OpenURL("https://example.test/docs"); err != nil {
		t.Fatal(err)
	}
	if reply, _ := execute(session, "openedurl"); reply != "ok \"https://example.test/docs\"\n" {
		t.Fatalf("openedurl reply = %q", reply)
	}
}

func TestControlDeclaresWhichURLSchemesHaveAnApplication(t *testing.T) {
	session, err := Open(app.Config{}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if available, _ := app.SchemeHandlerAvailable("zed"); available {
		t.Fatal("a scheme was registered before any was declared")
	}
	if reply, _ := execute(session, "schemehandlers zed vscode"); reply != "ok\n" {
		t.Fatalf("schemehandlers reply = %q", reply)
	}
	if available, _ := app.SchemeHandlerAvailable("zed"); !available {
		t.Fatal("zed was declared but reads as unregistered")
	}
	if err := app.OpenSchemeURL("zed", "zed://file/tmp"); err != nil {
		t.Fatal(err)
	}
	if reply, _ := execute(session, "openedurl"); reply != "ok \"zed://file/tmp\"\n" {
		t.Fatalf("openedurl reply = %q", reply)
	}
	execute(session, "schemehandlers")
	if available, _ := app.SchemeHandlerAvailable("zed"); available {
		t.Fatal("an empty declaration did not clear the schemes")
	}
}

func TestControlSetsTheSystemAppearanceTheRunReports(t *testing.T) {
	session, err := Open(app.Config{}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if reply, _ := execute(session, "appearance"); reply != "ok unknown\n" {
		t.Fatalf("a fresh run reports %q", reply)
	}
	heard := 0
	stop := app.WatchSystemAppearance(func() { heard++ })
	defer stop()
	if reply, _ := execute(session, "appearance dark"); reply != "ok dark\n" || app.SystemAppearance() != app.AppearanceDark || heard != 1 {
		t.Fatalf("dark: reply %q, system %v, heard %d", reply, app.SystemAppearance(), heard)
	}
	if reply, _ := execute(session, "appearance light"); reply != "ok light\n" || heard != 2 {
		t.Fatalf("light: reply %q, heard %d", reply, heard)
	}
	if reply, _ := execute(session, "appearance mauve"); reply != "error appearance is light, dark or unknown\n" {
		t.Fatalf("a bad name: %q", reply)
	}
}

func TestControlReportsTheLastOpenedPath(t *testing.T) {
	session, err := Open(app.Config{}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if reply, _ := execute(session, "openedpath"); reply != "ok none\n" {
		t.Fatalf("before any request: %q", reply)
	}
	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := app.OpenPath(file); err != nil {
		t.Fatal(err)
	}
	if reply, _ := execute(session, "openedpath"); reply != "ok "+strconv.Quote(file)+"\n" {
		t.Fatalf("openedpath reply = %q", reply)
	}
}

func TestControlClipimageFeedsTheAppsClipboardImageReads(t *testing.T) {
	session, err := Open(app.Config{}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if app.HasClipboardImage() {
		t.Fatal("a fresh run has an image on the clipboard")
	}
	if _, err := app.ClipboardImagePNG(); err != app.ErrNoImage {
		t.Fatalf("empty clipboard error = %v", err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(path, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if reply, _ := execute(session, "clipimage "+path); reply != "ok\n" {
		t.Fatalf("clipimage reply = %q", reply)
	}
	if !app.HasClipboardImage() {
		t.Fatal("HasClipboardImage = false after clipimage")
	}
	got, err := app.ClipboardImagePNG()
	if err != nil || !bytes.Equal(got, encoded.Bytes()) {
		t.Fatalf("ClipboardImagePNG = %d bytes, %v", len(got), err)
	}
	var asked bool
	app.RequestClipboardImage(func(png []byte, err error) { asked = err == nil && bytes.Equal(png, encoded.Bytes()) })
	if !asked {
		t.Fatal("RequestClipboardImage did not deliver the image")
	}
	if reply, _ := execute(session, "clipimage"); reply != "ok image "+strconv.Itoa(encoded.Len())+" bytes\n" {
		t.Fatalf("clipimage query = %q", reply)
	}
	notPNG := filepath.Join(t.TempDir(), "b.png")
	os.WriteFile(notPNG, []byte("nope"), 0o600)
	if reply, _ := execute(session, "clipimage "+notPNG); !strings.HasPrefix(reply, "error ") {
		t.Fatalf("non-PNG accepted: %q", reply)
	}
	if reply, _ := execute(session, "clip text"); reply != "ok \"text\"\n" || app.HasClipboardImage() {
		t.Fatalf("copying text leaves the image: %q", reply)
	}
}
