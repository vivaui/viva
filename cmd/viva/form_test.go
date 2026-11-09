package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const formSource = `package settings

import "github.com/vivaui/viva/ui"

type Theme int

const (
	Light Theme = iota
	Dark
)

func (t Theme) String() string { return [...]string{"Light", "Dark"}[t] }

var (
	themes = []Theme{Light, Dark}
	sizes  = []string{"Small", "Large"}
)

type Settings struct {
	Name     ui.Field       ` + "`form:\"Name\" placeholder:\"Your name\"`" + `
	Sync     bool           ` + "`form:\"Sync over SSH\"`" + `
	Beta     bool           ` + "`form:\"Beta builds,checkbox\"`" + `
	Theme    Theme          ` + "`form:\"Theme\" choices:\"themes,Theme.String\"`" + `
	Size     string         ` + "`form:\"Size,select\" choices:\"sizes\"`" + `
	Volume   ui.SliderState ` + "`form:\"Volume\"`" + `
	Internal int
	Skipped  bool ` + "`form:\"-\"`" + `
}

type Unrelated struct{ N int }
`

func writePackage(t *testing.T, files map[string]string) string {
	t.Helper()
	directory := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func TestFormDeclaresAControlPerTaggedField(t *testing.T) {
	directory := writePackage(t, map[string]string{"settings.go": formSource})
	source, made, err := generateForms(directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(made, ",") != "Settings" {
		t.Fatalf("made Form for %v", made)
	}
	text := string(source)
	for _, want := range []string{
		formHeader,
		`func (v *Settings) Form(c ui.C) ui.Box {`,
		`ui.TextField(c, &v.Name, "Name", "Your name")`,
		`ui.Switch(c, &v.Sync, "Sync over SSH")`,
		`ui.Checkbox(c, &v.Beta, "Beta builds")`,
		`ui.Choice(c, &v.Theme, themes, Theme.String)`,
		`ui.Select(c, &v.Size, sizes, nil)`,
		`ui.Slider(c, &v.Volume)`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated code lacks %s:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Internal", "Skipped", "Unrelated"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("generated code declares %s", unwanted)
		}
	}
	// The generated file is not read back as a source of structs.
	if err := os.WriteFile(filepath.Join(directory, formFile), source, 0o644); err != nil {
		t.Fatal(err)
	}
	again, _, err := generateForms(directory, []string{"Settings"})
	if err != nil || string(again) != text {
		t.Fatalf("a second run differs or fails: %v", err)
	}
}

func TestFormRejectsWhatItCannotDeclare(t *testing.T) {
	for _, test := range []struct{ field, want string }{
		{"N int `form:\"Count\"`", "no control for its type"},
		{"N bool `form:\"\"`", "no label"},
		{"N bool `form:\"On,select\"`", `option "select" does not apply to a switch`},
		{"N Kind `form:\"Kind\" choices:\"kinds\"`", "need a function naming them"},
		{"N string `form:\"Kind\" choices:\"\"`", "choices tag"},
		{"N bool `form:\"On\" placeholder:\"x\"`", "placeholder tag applies to a ui.Field only"},
	} {
		directory := writePackage(t, map[string]string{"a.go": "package a\n\ntype Kind int\n\ntype S struct {\n\t" + test.field + "\n}\n"})
		_, _, err := generateForms(directory, nil)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: error %v, want one saying %q", test.field, err, test.want)
		}
		if err != nil && !strings.Contains(err.Error(), "a.go:6") {
			t.Errorf("%s: error %v does not say where", test.field, err)
		}
	}
	directory := writePackage(t, map[string]string{"a.go": "package a\n\ntype S struct{ N int }\n"})
	if _, _, err := generateForms(directory, nil); err == nil || !strings.Contains(err.Error(), "no struct") {
		t.Errorf("a package with no tags: %v", err)
	}
	if _, _, err := generateForms(directory, []string{"T"}); err == nil || !strings.Contains(err.Error(), "declares no struct type T") {
		t.Errorf("a missing type: %v", err)
	}
}

// The generated Form compiles against package ui and its controls write into the struct.
func TestFormDrivesTheStruct(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a module")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	directory := writePackage(t, map[string]string{
		"go.mod":      "module settings\n\ngo 1.26\n\nrequire github.com/vivaui/viva v0.0.0\n\nreplace github.com/vivaui/viva => " + root + "\n",
		"go.sum":      string(sum),
		"settings.go": formSource,
		"settings_test.go": `package settings

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestForm(t *testing.T) {
	s := &Settings{Size: "Small"}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) { s.Form(c).Pad(16) }})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 480, Height: 480}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	click := func(name string) {
		for _, hit := range driver.Snapshot().Hits {
			if strings.HasPrefix(hit.Label, name) {
				b := hit.Bounds
				if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
					t.Fatal(err)
				}
				return
			}
		}
		t.Fatalf("no control named %q", name)
	}
	click("Sync over SSH")
	click("Beta builds")
	click("Dark")
	if !s.Sync || !s.Beta || s.Theme != Dark {
		t.Fatalf("the form did not write the struct: %+v", s)
	}
}
`,
	})
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOWORK", "off")
	source, _, err := generateForms(directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, formFile), source, 0o644); err != nil {
		t.Fatal(err)
	}
	test := exec.Command("go", "test", "-count=1", "./...")
	test.Dir = directory
	if output, err := test.CombinedOutput(); err != nil {
		t.Fatalf("the generated form does not work: %v\n%s\n%s", err, output, source)
	}
}
