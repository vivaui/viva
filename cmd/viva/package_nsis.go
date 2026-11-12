package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/tools/manifest"
	"github.com/vivaui/viva/tools/winpkg"
)

// packageWindows builds the program and the Windows packages asked for: an NSIS
// installer, a zip holding the program, or both. Every file is written under a
// temporary name and renamed, so a failure leaves the previous package alone.
func packageWindows(request packageRequest) error {
	plan, loaded, output := request.plan, request.plan.manifest, request.output
	dest := request.out
	if dest == "" {
		dest = loaded.Path(loaded.Build.Out)
	}
	var err error
	if dest, err = filepath.Abs(dest); err != nil {
		return err
	}
	icon := ""
	if loaded.App.Icon != "" {
		if icon, err = filepath.Abs(loaded.Path(loaded.App.Icon)); err != nil {
			return err
		}
	}
	extras, err := extraFiles(request.fileArgs, loaded)
	if err != nil {
		return err
	}
	if err := winpkg.CheckExtraFiles(extras); err != nil {
		return err
	}

	// The icon is read before the build: a bad one should not cost a build.
	var iconData []byte
	var iconPixels int
	if icon != "" && request.wants("nsis") {
		if iconData, iconPixels, err = winpkg.LoadIcon(icon); err != nil {
			return err
		}
	}
	signing, err := resolveWindowsSigning(request.sign, os.Getenv)
	if err != nil {
		return err
	}
	defer signing.cleanup()
	if err := request.build(); err != nil {
		return err
	}
	var signed []string
	// A program built here is signed before it goes into the installer and the
	// zip; one that --program supplied is used as it is, signed or not, as before.
	if signing.signs() && !request.prebuilt {
		if err := signing.signFile(runSigningTool, request.program); err != nil {
			return err
		}
		signed = append(signed, plan.executable+".exe")
	}
	programInfo, err := os.Stat(request.program)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	stem := fmt.Sprintf("%s-%s-windows-%s", plan.executable, loaded.App.Version, plan.arch)
	exe := plan.executable + ".exe"
	var warnings []string
	var written []string // label and path pairs, reported after the summary header
	if request.wants("nsis") {
		scope := loaded.Windows.InstallScope()
		script := winpkg.Script{
			Name: loaded.App.Name, Version: loaded.App.Version, Publisher: loaded.Windows.Publisher,
			URL: loaded.Windows.URL, ID: loaded.App.ID, Scope: scope, Executable: exe,
			Program: request.program, Arch: plan.arch, URLSchemes: loaded.App.URLSchemes,
			FileTypes: windowsFileTypes(loaded.App.FileTypes), Files: extras,
		}
		if iconData != nil {
			script.Icon = filepath.Join(filepath.Dir(request.program), "icon.ico")
			if err := os.WriteFile(script.Icon, iconData, 0o644); err != nil {
				return err
			}
		}
		installer := filepath.Join(dest, stem+"-setup.exe")
		script.Output = installer + ".viva-tmp"
		result, err := winpkg.Build(script)
		if err != nil {
			os.Remove(script.Output)
			return err
		}
		if err := os.Rename(script.Output, installer); err != nil {
			os.Remove(script.Output)
			return err
		}
		if signing.signs() {
			if err := signing.signFile(runSigningTool, installer); err != nil {
				return err
			}
			signed = append(signed, filepath.Base(installer))
		}
		warnings = result.Warnings
		written = append(written, "nsis", installer)
	}
	if request.wants("zip") {
		archive := filepath.Join(dest, stem+".zip")
		if err := winpkg.ZipProgram(request.program, exe, archive+".viva-tmp", extras...); err != nil {
			return err
		}
		if err := os.Rename(archive+".viva-tmp", archive); err != nil {
			os.Remove(archive + ".viva-tmp")
			return err
		}
		written = append(written, "zip", archive)
	}

	fmt.Fprintf(output, "packaged %s\n", displayPath(written[1]))
	fmt.Fprintf(output, "  app      %s %s (%s)\n", loaded.App.Name, loaded.App.Version, loaded.App.ID)
	fmt.Fprintf(output, "  target   windows/%s\n", plan.arch)
	if len(plan.tags) > 0 {
		fmt.Fprintf(output, "  tags     %s\n", strings.Join(plan.tags, ","))
	}
	fmt.Fprintf(output, "  program  %s%s\n", formatSize(programInfo.Size()), sizeNote(plan, programInfo.Size()))
	if len(extras) > 0 {
		fmt.Fprintf(output, "  files    %d beside the program\n", len(extras))
	}
	if request.wants("nsis") {
		scope := loaded.Windows.InstallScope()
		where := "under the user's profile, no administrator rights"
		if scope == manifest.ScopeMachine {
			where = "in Program Files, asks for administrator rights"
		}
		fmt.Fprintf(output, "  scope    %s: installs %s\n", scope, where)
		switch {
		case icon == "":
			fmt.Fprintf(output, "  icon     none (set [app] icon in %s)\n", manifest.FileName)
		case iconPixels == 0 || iconPixels >= winpkg.FullIconPixels:
			fmt.Fprintf(output, "  icon     %s\n", displayPath(icon))
		default:
			fmt.Fprintf(output, "  icon     %s (largest %d px; %d px is recommended)\n", displayPath(icon), iconPixels, winpkg.FullIconPixels)
		}
	}
	if request.wants("nsis") {
		reportOpening(output, loaded.App, true)
	}
	for index := 0; index < len(written); index += 2 {
		if err := reportArchive(output, written[index], written[index+1]); err != nil {
			return err
		}
	}
	for _, warning := range warnings {
		fmt.Fprintf(output, "  warning  makensis: %s\n", warning)
	}
	switch {
	case len(signed) > 0:
		reportWindowsSigning(output, signing, signed...)
	case signing.signs():
		fmt.Fprintln(output, "  signing  the program was supplied with --program and is used as it is; nothing else needs a signature")
	default:
		fmt.Fprintln(output, "  signing  none: Windows SmartScreen warns on an unsigned installer (see viva help package)")
	}
	return nil
}

// extraFiles turns --file SRC=DEST values into files resolved against the
// manifest, so a package run from any directory names the application's files.
func extraFiles(args []string, loaded *manifest.Manifest) ([]winpkg.ExtraFile, error) {
	files := make([]winpkg.ExtraFile, 0, len(args))
	for _, argument := range args {
		source, destination, ok := strings.Cut(argument, "=")
		if !ok || source == "" || destination == "" {
			return nil, fmt.Errorf("--file %q: expected SRC=DEST", argument)
		}
		path, err := filepath.Abs(loaded.Path(source))
		if err != nil {
			return nil, fmt.Errorf("--file %q: %w", argument, err)
		}
		files = append(files, winpkg.ExtraFile{Source: path, Destination: destination})
	}
	return files, nil
}
