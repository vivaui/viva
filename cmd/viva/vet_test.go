package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const vetPage = `package main

import "github.com/vivaui/viva/ui"

type item struct {
	ID   int
	Name string
	Done bool
}

type model struct {
	items []item
	open  bool
	kept  ui.C
}

func (m *model) page(c ui.C) {
	for _, it := range m.items {
		ui.Checkbox(c, &it.Done, it.Name) // a copy: works, not reported
		ui.Button(c, "Delete")            // the same label in every row, no Key
	}
	for i := range m.items {
		it := &m.items[i]
		ui.Row(c.KeyInt(it.ID)).Children(func() {
			ui.Button(c, "Delete") // keyed
		})
	}
	for _, label := range []string{"A", "B"} {
		ui.Button(c, label) // the labels differ
	}
	for range 2 {
		// viva:ignore unkeyed-button
		ui.Button(c, "Same")
	}
	ui.Modal(c, &m.open, "Title", nil, nil)
}

func main() { _ = (&model{}).page }
`

func TestVetFindsPageMistakes(t *testing.T) {
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
	want := []string{"kept-context@14", "unkeyed-button@20"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("findings = %v, want %v", got, want)
	}
}
