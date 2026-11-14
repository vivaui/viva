package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/tools/appbundle"
	"github.com/vivaui/viva/tools/manifest"
)

// packageFormats lists, per target, the formats viva package can make; the first
// is what a run without --format makes.
var packageFormats = map[string][]string{
	"darwin":  {"app", "zip", "dmg"},
	"windows": {"nsis", "zip"},
	"linux":   {"tar.gz", "deb", "pkgbuild"},
}

// packageRequest is what one target's packager is given.
type packageRequest struct {
	plan buildPlan
	// formats are the formats asked for, validated against packageFormats.
	formats []string
	// out is -o, or empty: the .app for darwin, the output directory otherwise.
	out string
	// program is the executable to package: built into a scratch directory, or
	// the file --program named, which is used as it is.
	program  string
	prebuilt bool
	// sign is what the command line said about signing a macOS package.
	sign        signingOptions
	output      io.Writer
	errorOutput io.Writer
	// fileArgs are --file SRC=DEST values, resolved against the manifest when
	// the Windows packages are made.
	fileArgs []string
	// pkgbuildURL is --pkgbuild-url: where makepkg downloads the tar.gz from.
	// Empty means the archive is beside the PKGBUILD.
	pkgbuildURL string
}

// build makes the program, unless --program supplied one.
func (request packageRequest) build() error {
	if request.prebuilt {
		return nil
	}
	return request.plan.run(request.program, request.errorOutput)
}

func (request packageRequest) wants(format string) bool { return contains(request.formats, format) }

func runPackage(arguments []string, output, errorOutput io.Writer) error {
	c, _ := findCommand("package")
	flags := flag.NewFlagSet("package", flag.ContinueOnError)
	var shared buildFlags
	shared.register(flags, "darwin")
	var requested, formatList, program string
	var makeZip bool
	var sign signingOptions
	var extraFiles []string
	var pkgbuildURL string
	flags.StringVar(&requested, "o", "", "where to write: the .app for darwin, the output directory otherwise; default is [build] out")
	flags.StringVar(&requested, "out", "", "same as -o")
	flags.StringVar(&formatList, "format", "", "comma-separated formats: darwin app, zip, dmg; windows nsis, zip; linux tar.gz, deb, pkgbuild")
	flags.StringVar(&program, "program", "", "package this already built executable instead of building one (it must be for --target and --arch)")
	flags.StringVar(&pkgbuildURL, "pkgbuild-url", "", "linux pkgbuild: where makepkg downloads the tar.gz from (default: beside the PKGBUILD)")
	flags.BoolVar(&makeZip, "zip", false, "same as adding zip to --format")
	flags.Func("file", "windows: ship SRC beside the program as DEST (repeatable, SRC=DEST; DEST is relative)", func(value string) error {
		source, destination, ok := strings.Cut(value, "=")
		if !ok || source == "" || destination == "" {
			return errors.New("expected SRC=DEST")
		}
		extraFiles = append(extraFiles, value)
		return nil
	})
	flags.StringVar(&sign.identity, "sign-identity", "", "macOS: sign with this identity (default $APPLE_SIGNING_IDENTITY); - signs ad hoc, for this Mac only")
	flags.StringVar(&sign.entitlements, "entitlements", "", "macOS: entitlements property list to sign with (default [mac] entitlements)")
	flags.BoolVar(&sign.notarize, "notarize", false, "macOS: require notarization (it happens anyway when the credentials are in the environment)")
	flags.BoolVar(&sign.skip, "no-sign", false, "do not sign or notarize even if the environment is set up to")
	flags.StringVar(&sign.signCommand, "sign-command", "", "windows: a command that signs {file} in place, for a key in a token or a signing service (default $WINDOWS_SIGN_COMMAND)")
	positional, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if len(positional) != 0 {
		return errors.New("package takes no arguments; the application comes from viva.toml (run 'viva help package')")
	}
	available, ok := packageFormats[shared.target]
	if !ok {
		return fmt.Errorf("cannot package for %s; use viva build for the browser, or one of: darwin, windows, linux", shared.target)
	}
	formats, err := chooseFormats(shared.target, available, formatList, makeZip)
	if err != nil {
		return err
	}
	if len(extraFiles) > 0 && shared.target != "windows" {
		return errors.New("--file is for Windows packages only")
	}

	plan, err := newPlan(shared, "")
	if err != nil {
		return err
	}
	if err := plan.manifest.RequirePackaging(); err != nil {
		return err
	}

	scratch, err := os.MkdirTemp("", "viva-package-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	request := packageRequest{
		plan: plan, formats: formats, out: requested, sign: sign, fileArgs: extraFiles, pkgbuildURL: pkgbuildURL,
		program: filepath.Join(scratch, plan.executable), output: output, errorOutput: errorOutput,
	}
	if program != "" {
		if request.program, err = filepath.Abs(program); err != nil {
			return err
		}
		info, statErr := os.Stat(request.program)
		if statErr != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("--program %s is not a file", program)
		}
		request.prebuilt = true
	}
	switch shared.target {
	case "darwin":
		return packageDarwin(request)
	case "windows":
		return packageWindows(request)
	default:
		return packageLinux(request)
	}
}

// chooseFormats resolves --format and --zip against what the target offers.
func chooseFormats(target string, available []string, list string, zip bool) ([]string, error) {
	var formats []string
	add := func(format string) {
		if !contains(formats, format) {
			formats = append(formats, format)
		}
	}
	for _, format := range strings.Split(list, ",") {
		if format = strings.TrimSpace(format); format == "" {
			continue
		}
		if !contains(available, format) {
			return nil, fmt.Errorf("%s packages cannot be made as %q; the formats are: %s", target, format, strings.Join(available, ", "))
		}
		add(format)
	}
	if zip {
		if !contains(available, "zip") {
			return nil, fmt.Errorf("--zip does not apply to %s packages; the formats are: %s", target, strings.Join(available, ", "))
		}
		add("zip")
	}
	if len(formats) == 0 {
		add(available[0])
	}
	return formats, nil
}

// checkDeployment refuses a bundle that claims to run on a system older than the
// program it holds does: it would be offered to those Macs and then fail to
// start. required is what the program's linker recorded; declared is what
// Info.plist will say.
func checkDeployment(required, declared string) error {
	if manifest.CompareVersions(required, declared) > 0 {
		return fmt.Errorf("the program was built for macOS %s or later but [mac] min_system says %s; "+
			"a CGO_CFLAGS or CGO_LDFLAGS in the environment may be setting a newer deployment target", required, declared)
	}
	return nil
}

func infoEntries(entries []manifest.InfoEntry) []appbundle.KeyValue {
	converted := make([]appbundle.KeyValue, len(entries))
	for index, entry := range entries {
		converted[index] = appbundle.KeyValue{Key: entry.Key, Value: entry.Value}
	}
	return converted
}
