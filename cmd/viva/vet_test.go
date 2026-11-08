package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const vetPage = `package main

import "github.com/vivaui/viva/flex"

type item struct {
	ID   int
	Name string
	Done bool
}

type model struct {
	items []item
	open  bool
	kept  flex.Screen
}

func (m *model) page(s flex.Screen) {
	for _, it := range m.items {
		s.Checkbox(&it.Done, it.Name) // copy: a range variable
		s.Button("Delete")            // the same label in every row, no Key
	}
	for i := range m.items {
		it := &m.items[i]
		s.Checkbox(&it.Done, it.Name) // a pointer into the model
		r := s.KeyIndex(it.ID)
		r.Button("Delete")
		done := it.Done
		if r.Checkbox(&done, "").Changed() {
			it.Done = done
		}
		s.Key(it.Name).Button("Open")
	}
	var local bool
	s.Switch(&local, "Local") // copy: made in every paint
	s.Modal(&m.open, "Title", func(flex.ModalScreen) {})
	for _, label := range []string{"A", "B"} {
		s.Button(label)
	}
	for range 2 {
		// viva:ignore unkeyed-button
		s.Button("Same")
	}
}

func main() {
	shared := false
	_ = func(s flex.Screen) { s.Checkbox(&shared, "Kept between paints") }
	_ = (&model{}).page
}
`

func TestVetFindsScreenMistakes(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a module")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	write("go.mod", "module page\n\ngo 1.26\n\nrequire github.com/vivaui/viva v0.0.0\n\nreplace github.com/vivaui/viva => "+root+"\n")
	write("go.sum", string(sum))
	write("main.go", vetPage)
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOWORK", "off")

	findings, err := vetPackages(directory, []string{"./..."})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, finding := range findings {
		got = append(got, finding.check+"@"+strconv.Itoa(finding.pos.Line))
	}
	want := []string{"kept-screen@14", "bound-copy@19", "unkeyed-button@20", "bound-copy@34"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("findings = %v, want %v", got, want)
	}
}
