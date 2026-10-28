package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// command is one viva subcommand.
type command struct {
	name  string
	usage string // what follows "viva " in the usage line
	// summary is the one line the overview shows; detail follows it in the
	// command's own help.
	summary string
	detail  string
	run     func(arguments []string, output, errorOutput io.Writer) error
}

// commands lists every subcommand in the order help shows them. It is a
// function so help can refer to the table it is part of.
func commands() []command {
	return []command{
		{name: "build", usage: "build [flags] [package]",
			summary: "Build a release program for macOS, Windows, Linux or the browser.",
			detail: "Builds with -trimpath and a stripped, reproducible link, and reports the size. Settings come from\n" +
				"viva.toml when there is one; the package argument overrides [app] package (or [web] package for\n" +
				"--target web, which also writes the loader and an index.html). macOS builds are compiled for\n" +
				"[mac] min_system, not for the macOS of the machine building them. Windows release builds use\n" +
				"the GUI subsystem; with --debug they keep the console.",
			run: func(arguments []string, output, errorOutput io.Writer) error {
				return runBuild(arguments, output, errorOutput)
			}},
		{name: "sign", usage: "sign --target windows --program FILE",
			summary: "Sign a Windows program that was built elsewhere, in place.",
			detail: "Uses the same Windows signing options as package: a custom command with {file} " +
				"($WINDOWS_SIGN_COMMAND or --sign-command), or a certificate from $WINDOWS_CERTIFICATE_FILE/ " +
				"$WINDOWS_CERTIFICATE with $WINDOWS_CERTIFICATE_PASSWORD and $WINDOWS_TIMESTAMP_URL. With no " +
				"options it leaves the program as it is, as package does. A release that verifies build " +
				"information and then packages with --program needs this: package treats a supplied program " +
				"as finished.",
			run: func(arguments []string, output, errorOutput io.Writer) error {
				return runSign(arguments, output, errorOutput)
			}},
		{name: "package", usage: "package [flags]",
			summary: "Package the application for macOS, Windows or Linux (macOS signing is optional).",
			detail: "Builds the program and packages it, all from viva.toml. --target darwin makes <name>.app (--format\n" +
				"app, zip, dmg), --target windows an NSIS installer or a zip (nsis, zip; the installer needs makensis,\n" +
				"and the Homebrew build on Apple Silicon crashes, so build it on Linux or Windows), --target linux a\n" +
				"tar.gz with an install script or a .deb (tar.gz, deb). --program packages an executable you built\n" +
				"(or signed) yourself instead of building one. [app] url_schemes and [[app.file_types]] (name, extensions,\n" +
				"optional mime) register the links and documents the program opens with every format; on Windows and\n" +
				"Linux they arrive as arguments of a new process (app/singleinstance hands them to the running copy).\n" +
				"macOS signing and notarization are optional and off unless an identity is given: --sign-identity or\n" +
				"$APPLE_SIGNING_IDENTITY (\"-\" signs ad hoc, for this Mac only), $APPLE_CERTIFICATE and\n" +
				"$APPLE_CERTIFICATE_PASSWORD (a base64 .p12 for a machine with no keychain), --entitlements or [mac]\n" +
				"entitlements. With $APPLE_NOTARY_PROFILE, or $APPLE_API_KEY/_KEY_ID/_ISSUER, or $APPLE_ID/$APPLE_PASSWORD/\n" +
				"$APPLE_TEAM_ID as well, a Developer ID signature is also notarized and the ticket stapled.\n" +
				"Windows signing is optional too: $WINDOWS_CERTIFICATE_FILE (a .pfx) or $WINDOWS_CERTIFICATE (base64) with\n" +
				"$WINDOWS_CERTIFICATE_PASSWORD signs the program and the installer with osslsigncode (or signtool on\n" +
				"Windows), timestamped by $WINDOWS_TIMESTAMP_URL; --sign-command or $WINDOWS_SIGN_COMMAND names a command\n" +
				"that signs {file} in place, for keys in a token or a signing service. A program given with --program\n" +
				"is used as it is. --no-sign skips all signing. Unsigned, macOS blocks a downloaded bundle and Windows\n" +
				"SmartScreen warns about the installer; Linux packages are not signed.",
			run: func(arguments []string, output, errorOutput io.Writer) error {
				return runPackage(arguments, output, errorOutput)
			}},
		{name: "init", usage: "init [flags] <directory>",
			summary: "Create a runnable project with a test and a guide for writing its UI.",
			detail: "Writes main.go (a window laid out with github.com/vivaui/viva/flex), main_test.go (drives it without a window), viva.toml,\n" +
				"go.mod and AGENTS.md, the short guide to the layout, identities, clicks and tests, which a coding\n" +
				"assistant reads before it changes the UI. The project builds against the Viva source it finds above the\n" +
				"working directory, or the one named by -viva.",
			run: func(arguments []string, output, _ io.Writer) error { return runInit(arguments, output) }},
		{name: "dev", usage: "dev [flags] <package> [-- app arguments]",
			summary: "Run an application with hot reload.",
			detail:  "Rebuilds on change and swaps the worker process while the window and its state stay.",
			run: func(arguments []string, output, errorOutput io.Writer) error {
				return runDev(arguments, output, errorOutput)
			}},
		{name: "add", usage: "add [flags] <component>...",
			summary: "Copy component source into your project.",
			detail: "The files are yours to change; nothing links back to viva. See viva list for the names. The recipes come\n" +
				"with the viva command, so they match the Viva version it was built from; when the project's go.mod\n" +
				"requires a different version, add says so (go tool viva runs the version the project pins).",
			run: func(arguments []string, output, errorOutput io.Writer) error {
				return addComponents(arguments, output, errorOutput)
			}},
		{name: "list", usage: "list",
			summary: "List the components viva add can install.",
			run:     func(arguments []string, output, _ io.Writer) error { return runList(arguments, output) }},
		{name: "icons", usage: "icons [-o file] [-pkg name] [-license file] <svg files or directories>",
			summary: "Compile SVG icons into Go source.",
			detail:  "Produces ui.Icon values, so no SVG parser ships in the application.",
			run:     func(arguments []string, output, _ io.Writer) error { return runIcons(arguments, output) }},
		{name: "doctor", usage: "doctor [-C dir]",
			summary: "Check that this machine and project can build with viva.",
			detail: "Reports the Go toolchain, the platform libraries Viva loads (Xcode tools on macOS, XCB and Vulkan on\n" +
				"Linux), the packaging tools only some targets need, viva.toml, and whether the viva command matches the\n" +
				"Viva version the project in -C builds against. Each problem comes with the command that fixes it; the\n" +
				"exit status is non-zero only when something viva cannot work without is missing.",
			run: func(arguments []string, output, _ io.Writer) error { return runDoctor(arguments, output) }},
		{name: "version", usage: "version",
			summary: "Print the viva version.",
			run:     func(arguments []string, output, _ io.Writer) error { return runVersion(arguments, output) }},
		{name: "help", usage: "help [command]",
			summary: "Show this list, or the usage and flags of one command.",
			run:     func(arguments []string, output, _ io.Writer) error { return runHelp(arguments, output) }},
	}
}

func findCommand(name string) (command, bool) {
	for _, candidate := range commands() {
		if candidate.name == name {
			return candidate, true
		}
	}
	return command{}, false
}

func printOverview(output io.Writer) {
	fmt.Fprintln(output, "viva builds, packages and develops Viva applications.")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "usage: viva <command> [arguments]")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "commands:")
	for _, c := range commands() {
		fmt.Fprintf(output, "  %-8s %s\n", c.name, c.summary)
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Run 'viva help <command>' for a command's usage and flags.")
}

// printCommandHelp prints one command's usage, what it does and its flags.
func printCommandHelp(output io.Writer, c command, flags *flag.FlagSet) {
	fmt.Fprintf(output, "usage: viva %s\n\n%s\n", c.usage, c.summary)
	if c.detail != "" {
		fmt.Fprintf(output, "\n%s\n", c.detail)
	}
	if flags != nil {
		fmt.Fprintln(output, "\nflags:")
		flags.SetOutput(output)
		flags.PrintDefaults()
		flags.SetOutput(io.Discard)
	}
}

func runHelp(arguments []string, output io.Writer) error {
	switch len(arguments) {
	case 0:
		printOverview(output)
		return nil
	case 1:
		if isHelpFlag(arguments[0]) {
			arguments = []string{"help"}
		}
		c, ok := findCommand(arguments[0])
		if !ok {
			return fmt.Errorf("unknown command %q; run 'viva help' for the list", arguments[0])
		}
		if c.name == "help" {
			printCommandHelp(output, c, nil)
			return nil
		}
		// Each command prints its own help, flags included, for -h.
		return c.run([]string{"-h"}, output, io.Discard)
	}
	return errors.New("usage: viva help [command]")
}

func runList(arguments []string, output io.Writer) error {
	if len(arguments) == 1 && isHelpFlag(arguments[0]) {
		c, _ := findCommand("list")
		printCommandHelp(output, c, nil)
		return nil
	}
	if len(arguments) != 0 {
		return errors.New("list takes no arguments; run 'viva help list'")
	}
	for _, name := range componentNames() {
		fmt.Fprintln(output, name)
	}
	return nil
}

func isHelpFlag(argument string) bool {
	return argument == "-h" || argument == "-help" || argument == "--help"
}

// parseFlags parses flags that may appear before, between or after the
// positional arguments (viva build ./cmd/app --target web), which the flag
// package alone does not allow. Everything after a bare -- is positional. When
// help was asked for it prints the command's help and reports helped.
func parseFlags(c command, flags *flag.FlagSet, arguments []string, output io.Writer) (positional []string, helped bool, err error) {
	flags.SetOutput(io.Discard)
	var tail []string
	for index, argument := range arguments {
		if argument == "--" {
			tail = arguments[index+1:]
			arguments = arguments[:index]
			break
		}
	}
	for {
		if err := flags.Parse(arguments); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				printCommandHelp(output, c, flags)
				return nil, true, nil
			}
			return nil, false, fmt.Errorf("%s: %w; run 'viva help %s'", c.name, err, c.name)
		}
		rest := flags.Args()
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		arguments = rest[1:]
	}
	return append(positional, tail...), false, nil
}
