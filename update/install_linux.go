//go:build linux

package update

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// linuxHelperTimeout is how many seconds the detached helper waits for the old
// process to exit before it gives up without changing anything.
const linuxHelperTimeout = 60

// lookPath resolves a command name to its path. It is a variable so tests can
// say whether dpkg-query exists on the machine running them.
var lookPath = exec.LookPath

// runCommand runs a command and returns its combined output. It is a variable
// so tests can inject dpkg-query output without a package manager.
var runCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// install replaces the running Linux executable with the tar.gz artifact and
// starts the new version through a detached helper. The helper waits for this
// process to exit before it changes anything, so the old and the new version
// never run at the same time (which a single-instance application would see as
// a second launch handing its arguments to the old copy).
func install(options InstallOptions, quit func()) error {
	if options.Asset.Format != FormatTarGz {
		return fmt.Errorf("%w: linux installs a %s archive, not %q", ErrManifest, FormatTarGz, options.Asset.Format)
	}
	if os.Getenv("APPIMAGE") != "" {
		return fmt.Errorf("%w: an AppImage updates itself", ErrUnsupported)
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("update: locating the running executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("update: resolving the running executable: %w", err)
	}
	if managedByDpkg(executable) {
		return fmt.Errorf("%w: %s is installed by dpkg", ErrManagedExternally, executable)
	}
	directory := filepath.Dir(executable)
	if err := checkWritable(directory); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrNotWritable, directory, err)
	}

	options.report("unpacking")
	staging, err := os.MkdirTemp(directory, ".viva-update-*")
	if err != nil {
		return fmt.Errorf("update: creating a staging directory in %s: %w", directory, err)
	}
	started := false
	defer func() {
		if !started {
			_ = os.RemoveAll(staging)
		}
	}()
	var replacement string
	if options.Payload != "" {
		options.report("copying the patched executable")
		info, err := os.Lstat(options.Payload)
		if err != nil {
			return fmt.Errorf("update: %w", err)
		}
		program := options.Payload
		if info.IsDir() {
			program, err = findProgram(options.Payload, filepath.Base(executable))
			if err != nil {
				return err
			}
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: the payload is not a regular file", ErrManifest)
		}
		replacement = filepath.Join(staging, filepath.Base(executable))
		if err := copyRegularFile(program, replacement, 0o755); err != nil {
			return fmt.Errorf("update: copying the patched executable: %w", err)
		}
	} else {
		options.report("unpacking")
		if err := extractTarGz(options.Artifact, staging); err != nil {
			return err
		}
		replacement, err = findProgram(staging, filepath.Base(executable))
		if err != nil {
			return err
		}
	}

	options.report("replacing")
	script, err := writeHelperScript(staging, helperScript(replacementPlan{
		ParentPID:   os.Getpid(),
		Executable:  executable,
		Replacement: replacement,
		Backup:      executable + ".old",
		Staging:     staging,
		Args:        options.Args,
	}))
	if err != nil {
		return err
	}
	command := exec.Command("/bin/sh", script)
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

// replacementPlan is what the helper does once the old process exits.
type replacementPlan struct {
	ParentPID   int
	Executable  string
	Replacement string
	Backup      string
	Staging     string
	Args        []string
}

// helperScript renders the /bin/sh program that replaces the executable after
// the old process exits. Every path and argument is quoted. The helper starts
// the new version in the background, keeps the old one for two seconds and
// restores it only when the new one exited with a failure in that time.
func helperScript(plan replacementPlan) string {
	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("# Viva update helper: replace the executable after the old process exits.\n")
	script.WriteString("set -u\n\n")
	fmt.Fprintf(&script, "parent=%d\n", plan.ParentPID)
	fmt.Fprintf(&script, "exe=%s\n", shellQuote(plan.Executable))
	fmt.Fprintf(&script, "staged=%s\n", shellQuote(plan.Replacement))
	fmt.Fprintf(&script, "backup=%s\n", shellQuote(plan.Backup))
	fmt.Fprintf(&script, "staging=%s\n\n", shellQuote(plan.Staging))
	script.WriteString("cleanup() {\n\trm -rf \"$staging\"\n}\n")
	script.WriteString("trap cleanup EXIT\n\n")
	script.WriteString("waited=0\n")
	script.WriteString("while kill -0 \"$parent\" 2>/dev/null; do\n")
	fmt.Fprintf(&script, "\tif [ \"$waited\" -ge %d ]; then\n", linuxHelperTimeout)
	fmt.Fprintf(&script, "\t\techo 'viva update: the old process did not exit within %d seconds' >&2\n", linuxHelperTimeout)
	script.WriteString("\t\texit 1\n\tfi\n\tsleep 1\n\twaited=$((waited + 1))\ndone\n\n")
	script.WriteString("rm -rf \"$backup\" || exit 1\n")
	script.WriteString("if ! mv \"$exe\" \"$backup\"; then\n")
	script.WriteString("\techo 'viva update: could not move the old executable aside' >&2\n\texit 1\nfi\n\n")
	script.WriteString("if ! mv \"$staged\" \"$exe\"; then\n")
	script.WriteString("\techo 'viva update: could not install the new executable; restoring the old one' >&2\n")
	script.WriteString("\tif ! mv \"$backup\" \"$exe\"; then\n\t\techo 'viva update: could not restore the old executable' >&2\n\tfi\n")
	script.WriteString("\texit 1\nfi\n\n")
	script.WriteString("nohup \"$exe\"")
	for _, argument := range plan.Args {
		script.WriteString(" ")
		script.WriteString(shellQuote(argument))
	}
	script.WriteString(" >/dev/null 2>&1 &\n")
	script.WriteString("child=$!\n")
	script.WriteString("sleep 2\n")
	script.WriteString("if kill -0 \"$child\" 2>/dev/null; then\n\trm -f \"$backup\"\n\texit 0\nfi\n")
	script.WriteString("wait \"$child\"\nstatus=$?\n")
	// A clean exit inside the two seconds is a launch that handed over to
	// another instance or finished at once; only a failure rolls back.
	script.WriteString("if [ \"$status\" -eq 0 ]; then\n\trm -f \"$backup\"\n\texit 0\nfi\n")
	script.WriteString("if [ -f \"$backup\" ]; then\n\tmv \"$backup\" \"$exe\"\nfi\n")
	script.WriteString("echo 'viva update: the new version did not start; the old one is back' >&2\n")
	script.WriteString("exit \"$status\"\n")
	return script.String()
}

// writeHelperScript writes script into dir with mode 0700 and returns its path.
func writeHelperScript(dir, script string) (string, error) {
	file, err := os.CreateTemp(dir, "viva-update-*.sh")
	if err != nil {
		return "", fmt.Errorf("update: writing the update helper: %w", err)
	}
	path := file.Name()
	if err := file.Chmod(0o700); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("update: writing the update helper: %w", err)
	}
	if _, err := file.WriteString(script); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("update: writing the update helper: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("update: writing the update helper: %w", err)
	}
	return path, nil
}

// shellQuote returns text as one single-quoted shell word.
func shellQuote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}

// managedByDpkg reports whether dpkg owns executable. dpkg-query exits zero and
// prints "package: path" when it does; a missing command, a failed query or
// output without the path all mean dpkg does not own it.
func managedByDpkg(executable string) bool {
	query, err := lookPath("dpkg-query")
	if err != nil {
		return false
	}
	output, err := runCommand(query, "-S", executable)
	if err != nil {
		return false
	}
	return strings.Contains(string(output), executable)
}

// checkWritable reports whether directory accepts a new file, which is the
// permission replacing the executable needs.
func checkWritable(directory string) error {
	probe, err := os.CreateTemp(directory, ".viva-update-probe-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

// findProgram returns the executable inside root that should replace the
// running one. A file named name wins; otherwise root must hold exactly one
// regular executable file.
func findProgram(root, name string) (string, error) {
	var named, executables []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			return nil
		}
		if entry.Name() == name {
			named = append(named, path)
		}
		executables = append(executables, path)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("update: scanning the archive: %w", err)
	}
	switch {
	case len(named) == 1:
		return named[0], nil
	case len(named) > 1:
		return "", fmt.Errorf("update: archive has %d executable files named %s", len(named), name)
	case len(executables) == 1:
		return executables[0], nil
	case len(executables) == 0:
		return "", fmt.Errorf("update: archive has no executable file, want one named %s", name)
	default:
		return "", fmt.Errorf("update: archive has %d executable files and none named %s", len(executables), name)
	}
}
