package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSignRunsTheCustomCommandOnTheProgram(t *testing.T) {
	program := filepath.Join(t.TempDir(), "motty.exe")
	if err := os.WriteFile(program, []byte("program"), 0o755); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	run := func(name string, arguments ...string) (string, error) {
		calls = append(calls, append([]string{name}, arguments...))
		return "", nil
	}
	environment := func(key string) string {
		if key == "WINDOWS_SIGN_COMMAND" {
			return `my-signer --file {file} --timestamp`
		}
		return ""
	}
	var output strings.Builder
	if err := signProgram([]string{"--target", "windows", "--program", program}, environment, run, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0][0] != "my-signer" {
		t.Fatalf("signer calls = %v", calls)
	}
	found := false
	for _, argument := range calls[0] {
		if argument == "{file}" {
			t.Fatalf("the signer saw an unexpanded {file}: %v", calls[0])
		}
		if argument == program {
			found = true
		}
	}
	if !found {
		t.Fatalf("the signer was not told %s: %v", program, calls[0])
	}
	if !strings.Contains(output.String(), "signed") {
		t.Fatalf("sign said %q", output.String())
	}
}

func TestSignWithoutOptionsLeavesTheProgramAlone(t *testing.T) {
	program := filepath.Join(t.TempDir(), "motty.exe")
	if err := os.WriteFile(program, []byte("program"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(string, ...string) (string, error) { t.Fatal("ran a signer with no options"); return "", nil }
	var output strings.Builder
	if err := signProgram([]string{"--target", "windows", "--program", program}, func(string) string { return "" }, run, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "none") {
		t.Fatalf("sign said %q", output.String())
	}
}

func TestSignRejectsOtherTargetsAndMissingPrograms(t *testing.T) {
	run := func(string, ...string) (string, error) { return "", nil }
	for _, arguments := range [][]string{
		{"--target", "darwin", "--program", "x"},
		{"--target", "windows"},
		{"--target", "windows", "--program", filepath.Join(t.TempDir(), "missing.exe")},
	} {
		if err := signProgram(arguments, func(string) string { return "" }, run, io.Discard, io.Discard); err == nil {
			t.Fatalf("signed %v", arguments)
		}
	}
}
