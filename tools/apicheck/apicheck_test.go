package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writePackage(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pkg.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckFindsDynamicTypesInExportedAPI(t *testing.T) {
	dir := writePackage(t, `package sample

import "reflect"

func Exported(value any) {}

func Variadic(values ...any) {}

func Results() map[string]interface{} { return nil }

func Fine(value string, values []int) *Widget { return nil }

func hidden(value any) {}

func Generic[T any](value T) {}

type Widget struct {
	Name    string
	Values  map[string]any
	private any
}

type Bag = map[string]any

type Handler interface {
	Handle(value any)
}

var Anything any
`)
	findings, err := check([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	found := make(map[string]bool)
	for _, finding := range findings {
		found[finding.symbol] = true
	}
	for _, want := range []string{
		"sample/reflect",
		"sample.Exported",
		"sample.Variadic",
		"sample.Results",
		"sample.Widget.Values",
		"sample.Bag",
		"sample.Handler.Handle",
		"sample.Anything",
	} {
		if !found[want] {
			t.Errorf("no finding for %s in %+v", want, findings)
		}
	}
	for _, unwanted := range []string{"sample.Fine", "sample.hidden", "sample.Generic", "sample.Widget.private"} {
		if found[unwanted] {
			t.Errorf("unexpected finding for %s", unwanted)
		}
	}
}

func TestAllowListExemptsASymbol(t *testing.T) {
	dir := writePackage(t, `package sample

func Kept(value any) {}
`)
	allowed := map[string]bool{"sample.Kept": true}
	findings, err := check([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	kept := findings[:0]
	for _, finding := range findings {
		if !allowed[finding.symbol] {
			kept = append(kept, finding)
		}
	}
	if len(kept) != 0 {
		t.Fatalf("kept = %+v, want none", kept)
	}
}
