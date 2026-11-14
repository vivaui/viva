//go:build darwin

package update

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// helperTimeoutSeconds is how many seconds the detached helper waits for the
// old process to exit before it gives up without changing anything.
const helperTimeoutSeconds = 60

// install replaces the running application bundle with the one options.Artifact
// holds. It checks the archive first, then starts a detached helper and calls
// quit, so the helper can wait for this process to exit, swap the bundles and
// start the new version. Nothing is changed and quit is not called when a check
// fails.
func install(options InstallOptions, quit func()) error {
	if options.Asset.Format != FormatZIP {
		return fmt.Errorf("%w: macOS installs a zip archive, not %q", ErrManifest, options.Asset.Format)
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("update: locating the running program: %w", err)
	}
	bundle, err := bundleRootFor(executable)
	if err != nil {
		return err
	}
	for _, path := range []string{executable, bundle} {
		if err := checkBundleLocation(path); err != nil {
			return err
		}
	}
	codesign, err := exec.LookPath("codesign")
	if err != nil {
		return fmt.Errorf("update: codesign is required to install an update: %w", err)
	}
	// The temporary directory must share a volume with the bundle: the helper
	// moves the new bundle into place with a rename.
	staging, err := os.MkdirTemp(filepath.Dir(bundle), ".viva-update-")
	if err != nil {
		return fmt.Errorf("%w: cannot create a temporary directory beside %s: %v", ErrNotWritable, filepath.Base(bundle), err)
	}
	started := false
	defer func() {
		if !started {
			_ = os.RemoveAll(staging)
		}
	}()

	var staged string
	if options.Payload != "" {
		options.report("copying the patched application")
		if err := copyTree(options.Payload, filepath.Join(staging, filepath.Base(options.Payload))); err != nil {
			return err
		}
		staged, err = stagedBundle(staging)
	} else {
		options.report("unpacking")
		if err := extractZip(options.Artifact, staging); err != nil {
			return err
		}
		staged, err = stagedBundle(staging)
	}
	if err != nil {
		return err
	}
	info, err := readBundleInfo(staged)
	if err != nil {
		return err
	}
	if options.App != "" && info.Identifier != options.App {
		return fmt.Errorf("%w: the archive holds %q, not %q", ErrManifest, info.Identifier, options.App)
	}
	if info.Version != "" && info.Version != options.Version {
		return fmt.Errorf("%w: the archive holds version %q, not %q", ErrManifest, info.Version, options.Version)
	}

	options.report("checking signature")
	if err := verifyBundleSignature(codesign, staged); err != nil {
		return fmt.Errorf("%w: %v", ErrUntrusted, err)
	}

	options.report("replacing")
	script, err := writeHelperScript(staging, helperScript(replacementPlan{
		ParentPID: os.Getpid(),
		Bundle:    bundle,
		Staged:    staged,
		Staging:   staging,
		Args:      options.Args,
	}))
	if err != nil {
		return err
	}
	command := exec.Command("/bin/sh", script)
	// The helper must not keep a working directory inside the bundle it
	// replaces, because that directory is renamed and deleted.
	command.Dir = "/"
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return fmt.Errorf("update: starting the replacement helper: %w", err)
	}
	started = true
	_ = command.Process.Release()
	quit()
	return nil
}

// bundleRootFor walks up from executable to the .app directory that contains
// it. A path ending in .app is a bundle only when it has a Contents/MacOS
// directory, which keeps a plain directory with that suffix from matching.
func bundleRootFor(executable string) (string, error) {
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("update: resolving %s: %w", executable, err)
	}
	for path := resolved; ; path = filepath.Dir(path) {
		if strings.HasSuffix(path, ".app") && isDirectory(filepath.Join(path, "Contents", "MacOS")) {
			return path, nil
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
	}
	return "", fmt.Errorf("%w: %s is not inside an application bundle", ErrNotWritable, filepath.Base(executable))
}

// checkBundleLocation refuses a translocated bundle. macOS runs such a copy
// from a read-only location, so replacing it would not change what the user
// starts the next time.
func checkBundleLocation(path string) error {
	if strings.Contains(path, "/AppTranslocation/") {
		return fmt.Errorf("%w: the application runs from a translocated copy; move it to /Applications and try again", ErrNotWritable)
	}
	return nil
}

// isDirectory reports whether path names a directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// stagedBundle returns the one application bundle at the top of staging. An
// archive that holds none, several, or a file with the .app suffix is refused.
func stagedBundle(staging string) (string, error) {
	entries, err := os.ReadDir(staging)
	if err != nil {
		return "", fmt.Errorf("update: reading the unpacked archive: %w", err)
	}
	found := ""
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".app") {
			continue
		}
		if !entry.IsDir() {
			return "", fmt.Errorf("%w: %s is not a directory", ErrManifest, entry.Name())
		}
		if found != "" {
			return "", fmt.Errorf("%w: the archive holds more than one application bundle", ErrManifest)
		}
		found = filepath.Join(staging, entry.Name())
	}
	if found == "" {
		return "", fmt.Errorf("%w: the archive holds no application bundle", ErrManifest)
	}
	return found, nil
}

// bundleInfo is the part of Info.plist an update is checked against.
type bundleInfo struct {
	Identifier string
	Version    string
}

// readBundleInfo reads the bundle identifier and short version from the
// Info.plist inside app. It parses the XML form itself and asks plutil only
// when the file uses another property-list format.
func readBundleInfo(app string) (bundleInfo, error) {
	path := filepath.Join(app, "Contents", "Info.plist")
	data, err := os.ReadFile(path)
	if err != nil {
		return bundleInfo{}, fmt.Errorf("%w: reading %s: %v", ErrManifest, path, err)
	}
	info, xmlErr := parseBundleInfoXML(data)
	if xmlErr == nil {
		return info, nil
	}
	converted, err := convertPropertyList(path)
	if err != nil {
		return bundleInfo{}, fmt.Errorf("%w: %s is not a readable property list: %v", ErrManifest, path, xmlErr)
	}
	info, err = parseBundleInfoXML(converted)
	if err != nil {
		return bundleInfo{}, fmt.Errorf("%w: %s is not a readable property list: %v", ErrManifest, path, err)
	}
	return info, nil
}

// convertPropertyList renders a property list in another format as XML, for the
// bundles whose Info.plist is binary.
var convertPropertyList = func(path string) ([]byte, error) {
	return exec.Command("plutil", "-convert", "xml1", "-o", "-", path).Output()
}

// parseBundleInfoXML reads the top-level CFBundleIdentifier and
// CFBundleShortVersionString entries of an XML property list. Entries nested in
// another dictionary are ignored.
func parseBundleInfoXML(data []byte) (bundleInfo, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var stack []string
	var key string
	var info bundleInfo
	dictionary := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return bundleInfo{}, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			if len(stack) == 1 && stack[0] == "plist" && element.Name.Local == "dict" {
				dictionary = true
			}
			top := len(stack) == 2 && stack[0] == "plist" && stack[1] == "dict"
			if top && element.Name.Local == "key" {
				if err := decoder.DecodeElement(&key, &element); err != nil {
					return bundleInfo{}, err
				}
				continue
			}
			if top && key != "" {
				if element.Name.Local != "string" {
					key = ""
					stack = append(stack, element.Name.Local)
					continue
				}
				var value string
				if err := decoder.DecodeElement(&value, &element); err != nil {
					return bundleInfo{}, err
				}
				switch key {
				case "CFBundleIdentifier":
					info.Identifier = value
				case "CFBundleShortVersionString":
					info.Version = value
				}
				key = ""
				continue
			}
			stack = append(stack, element.Name.Local)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if !dictionary {
		return bundleInfo{}, fmt.Errorf("no plist dictionary")
	}
	return info, nil
}

// verifyBundleSignature checks the code signature on app with codesign. It is a
// variable so tests can replace the system tool.
var verifyBundleSignature = func(codesign, app string) error {
	return runCommand(codesignVerifyCommand(codesign, app))
}

// codesignVerifyCommand builds the command that checks the signature of app.
func codesignVerifyCommand(codesign, app string) *exec.Cmd {
	return exec.Command(codesign, "--verify", "--deep", "--strict", app)
}

// runCommand runs command and reports its exit status together with anything it
// wrote, so a failed signature check says why it failed.
func runCommand(command *exec.Cmd) error {
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		return fmt.Errorf("update: %s: %w", command.Path, err)
	}
	return fmt.Errorf("update: %s: %w: %s", command.Path, err, message)
}

// replacementPlan describes the move the helper makes once the old process
// exits.
type replacementPlan struct {
	// ParentPID is the process the helper waits for.
	ParentPID int
	// Bundle is the running application path that the new bundle replaces.
	Bundle string
	// Staged is the new bundle inside Staging.
	Staged string
	// Staging is the temporary directory beside Bundle.
	Staging string
	// Args are passed to the new version after --args.
	Args []string
}

// helperScript renders the /bin/sh program that swaps the bundle and starts the
// new version. Every path is quoted, and a failed swap or launch puts the old
// bundle back before the script exits non-zero.
func helperScript(plan replacementPlan) string {
	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("# Viva update helper: replace the application after the old process exits.\n")
	script.WriteString("set -u\n\n")
	fmt.Fprintf(&script, "parent=%d\n", plan.ParentPID)
	fmt.Fprintf(&script, "bundle=%s\n", shellQuote(plan.Bundle))
	fmt.Fprintf(&script, "staged=%s\n", shellQuote(plan.Staged))
	fmt.Fprintf(&script, "backup=%s\n", shellQuote(plan.Bundle+".old"))
	fmt.Fprintf(&script, "staging=%s\n\n", shellQuote(plan.Staging))
	script.WriteString("cleanup() {\n")
	script.WriteString("\trm -rf \"$staging\"\n")
	script.WriteString("}\n")
	script.WriteString("trap cleanup EXIT\n\n")
	script.WriteString("waited=0\n")
	script.WriteString("while kill -0 \"$parent\" 2>/dev/null; do\n")
	fmt.Fprintf(&script, "\tif [ \"$waited\" -ge %d ]; then\n", helperTimeoutSeconds)
	fmt.Fprintf(&script, "\t\techo 'viva update: the old process did not exit within %d seconds' >&2\n", helperTimeoutSeconds)
	script.WriteString("\t\texit 1\n")
	script.WriteString("\tfi\n")
	script.WriteString("\tsleep 1\n")
	script.WriteString("\twaited=$((waited + 1))\n")
	script.WriteString("done\n\n")
	script.WriteString("rm -rf \"$backup\" || exit 1\n")
	script.WriteString("if ! mv \"$bundle\" \"$backup\"; then\n")
	script.WriteString("\techo 'viva update: could not move the old application aside' >&2\n")
	script.WriteString("\texit 1\n")
	script.WriteString("fi\n\n")
	script.WriteString("if ! mv \"$staged\" \"$bundle\"; then\n")
	script.WriteString("\techo 'viva update: could not install the new application; restoring the old one' >&2\n")
	script.WriteString("\tif ! mv \"$backup\" \"$bundle\"; then\n")
	script.WriteString("\t\techo 'viva update: could not restore the old application' >&2\n")
	script.WriteString("\tfi\n")
	script.WriteString("\texit 1\n")
	script.WriteString("fi\n\n")
	launch := "open -n \"$bundle\""
	if len(plan.Args) > 0 {
		launch += " --args"
		for _, arg := range plan.Args {
			launch += " " + shellQuote(arg)
		}
	}
	fmt.Fprintf(&script, "if ! %s; then\n", launch)
	script.WriteString("\techo 'viva update: could not start the new application; restoring the old one' >&2\n")
	script.WriteString("\tif ! rm -rf \"$bundle\"; then\n")
	script.WriteString("\t\techo 'viva update: could not remove the new application' >&2\n")
	script.WriteString("\t\texit 1\n")
	script.WriteString("\tfi\n")
	script.WriteString("\tif ! mv \"$backup\" \"$bundle\"; then\n")
	script.WriteString("\t\techo 'viva update: could not restore the old application' >&2\n")
	script.WriteString("\tfi\n")
	script.WriteString("\texit 1\n")
	script.WriteString("fi\n\n")
	script.WriteString("# The update is running; a backup that cannot be removed is left behind.\n")
	script.WriteString("rm -rf \"$backup\" 2>/dev/null || true\n")
	script.WriteString("exit 0\n")
	return script.String()
}

// writeHelperScript writes script into dir with mode 0700 and returns its path.
// The helper holds no secrets, but only its owner needs to run it.
func writeHelperScript(dir, script string) (string, error) {
	file, err := os.CreateTemp(dir, "viva-update-*.sh")
	if err != nil {
		return "", fmt.Errorf("update: writing the update helper: %w", err)
	}
	path := file.Name()
	if err := file.Chmod(0o700); err != nil {
		file.Close()
		os.Remove(path)
		return "", fmt.Errorf("update: writing the update helper: %w", err)
	}
	if _, err := file.WriteString(script); err != nil {
		file.Close()
		os.Remove(path)
		return "", fmt.Errorf("update: writing the update helper: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("update: writing the update helper: %w", err)
	}
	return path, nil
}

// shellQuote returns text as one single-quoted shell word, with embedded single
// quotes escaped the way POSIX shells require.
func shellQuote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}
