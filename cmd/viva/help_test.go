package main

import (
	"bytes"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestOverviewListsEveryCommand(t *testing.T) {
	var output bytes.Buffer
	if err := runCLI([]string{"help"}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, c := range commands() {
		if !strings.Contains(output.String(), "  "+c.name+" ") || !strings.Contains(output.String(), c.summary) {
			t.Errorf("the overview does not list %q:\n%s", c.name, output.String())
		}
	}
	for _, line := range strings.Split(output.String(), "\n") {
		if len(line) > 100 {
			t.Errorf("an overview line is %d characters long: %q", len(line), line)
		}
	}
}

func TestEveryCommandPrintsItsOwnHelp(t *testing.T) {
	for _, c := range commands() {
		for _, arguments := range [][]string{{"help", c.name}, {c.name, "-h"}, {c.name, "--help"}} {
			var output bytes.Buffer
			if err := runCLI(arguments, &output, io.Discard); err != nil {
				t.Errorf("viva %s: %v", strings.Join(arguments, " "), err)
				continue
			}
			if !strings.HasPrefix(output.String(), "usage: viva "+c.usage+"\n") || !strings.Contains(output.String(), c.summary) {
				t.Errorf("viva %s printed:\n%s", strings.Join(arguments, " "), output.String())
			}
		}
	}
}

func TestCommandsWithFlagsListThem(t *testing.T) {
	for name, flags := range map[string][]string{
		"build":   {"-C", "-o", "-target", "-arch", "-tags", "-debug"},
		"package": {"-C", "-o", "-zip", "-format", "-target", "-tags", "-file"},
		"dev":     {"-debounce", "-poll", "-build-timeout"},
		"add":     {"-out", "-package", "-force"},
	} {
		var output bytes.Buffer
		if err := runCLI([]string{"help", name}, &output, io.Discard); err != nil {
			t.Fatal(err)
		}
		for _, flag := range flags {
			if !strings.Contains(output.String(), "\n  "+flag+" ") && !strings.Contains(output.String(), "\n  "+flag+"\n") {
				t.Errorf("help for %s does not list %s:\n%s", name, flag, output.String())
			}
		}
	}
}

func TestNoCommandShowsTheOverviewAndFails(t *testing.T) {
	var output, errorOutput bytes.Buffer
	if err := runCLI(nil, &output, &errorOutput); err == nil {
		t.Fatal("running viva with nothing to do succeeded")
	}
	if output.Len() != 0 || !strings.Contains(errorOutput.String(), "commands:") {
		t.Errorf("stdout %q, stderr %q", output.String(), errorOutput.String())
	}
	var help bytes.Buffer
	for _, flag := range []string{"-h", "--help", "-help"} {
		help.Reset()
		if err := runCLI([]string{flag}, &help, io.Discard); err != nil || !strings.Contains(help.String(), "commands:") {
			t.Errorf("viva %s: %v\n%s", flag, err, help.String())
		}
	}
}

func TestUnknownThingsPointToHelp(t *testing.T) {
	for _, arguments := range [][]string{{"nonsense"}, {"help", "nonsense"}} {
		err := runCLI(arguments, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "viva help") {
			t.Errorf("viva %v: %v", arguments, err)
		}
	}
	if err := runCLI([]string{"help", "build", "extra"}, io.Discard, io.Discard); err == nil {
		t.Error("help with two arguments was accepted")
	}
	if err := runCLI([]string{"list", "extra"}, io.Discard, io.Discard); err == nil {
		t.Error("list with an argument was accepted")
	}
	err := runCLI([]string{"build", "--nope"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "viva help build") {
		t.Errorf("an unknown flag: %v", err)
	}
}

func TestParseFlagsAllowsFlagsAnywhere(t *testing.T) {
	c, _ := findCommand("build")
	for _, test := range []struct {
		name      string
		arguments []string
		target    string
		debug     bool
		rest      []string
	}{
		{"before", []string{"--target", "web", "./cmd/app"}, "web", false, []string{"./cmd/app"}},
		{"after", []string{"./cmd/app", "--target", "web"}, "web", false, []string{"./cmd/app"}},
		{"between", []string{"a", "--debug", "b"}, "darwin", true, []string{"a", "b"}},
		{"equals form", []string{"-target=windows", "x"}, "windows", false, []string{"x"}},
		{"after the terminator everything is positional", []string{"--target", "web", "--", "--debug", "-x"}, "web", false, []string{"--debug", "-x"}},
		{"nothing", nil, "darwin", false, nil},
	} {
		flags := flag.NewFlagSet("build", flag.ContinueOnError)
		target := flags.String("target", "darwin", "")
		debug := flags.Bool("debug", false, "")
		rest, helped, err := parseFlags(c, flags, test.arguments, io.Discard)
		if err != nil || helped || *target != test.target || *debug != test.debug || strings.Join(rest, "|") != strings.Join(test.rest, "|") {
			t.Errorf("%s: target %q debug %v rest %q helped %v err %v", test.name, *target, *debug, rest, helped, err)
		}
	}
}
