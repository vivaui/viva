package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/tools/debpkg"
	"github.com/vivaui/viva/tools/linuxicon"
	"github.com/vivaui/viva/tools/manifest"
	"github.com/vivaui/viva/tools/pkgbuild"
	"github.com/vivaui/viva/tools/tarball"
)

// linuxRuntimeDepends are the libraries a Viva program opens at run time with
// dlopen (Vulkan, XCB and Cairo). Nothing in the executable's headers names
// them, so a .deb has to say so itself.
var linuxRuntimeDepends = []string{"libvulkan1", "libxcb1", "libcairo2"}

// linuxArchRuntimeDepends are the same libraries under Arch's names.
var linuxArchRuntimeDepends = []string{"vulkan-icd-loader", "libxcb", "cairo"}

// packageLinux builds the program and writes the Linux packages asked for: a
// .tar.gz with an install script, a .deb, and a PKGBUILD with its .SRCINFO for
// an Arch binary package. Each is written beside its final name and renamed
// once every one of them is complete, so a failure leaves earlier output alone.
func packageLinux(request packageRequest) error {
	plan, loaded, output := request.plan, request.plan.manifest, request.output
	dest := request.out
	if dest == "" {
		dest = loaded.Path(loaded.Build.Out)
	}
	dest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	if info, err := os.Stat(dest); err == nil && !info.IsDir() {
		return fmt.Errorf("%s is a file; for Linux packages -o names the output directory", displayPath(dest))
	}

	// The icon is checked before the long build, as the macOS bundle does.
	var icons []linuxicon.Rendition
	iconPath, iconPixels := "", 0
	if loaded.App.Icon != "" {
		if iconPath, err = filepath.Abs(loaded.Path(loaded.App.Icon)); err != nil {
			return err
		}
		if icons, iconPixels, err = loadLinuxIcon(iconPath); err != nil {
			return err
		}
	}
	// Name checks that do not need the build.
	debName := ""
	if request.wants("deb") {
		if _, err := debpkg.PackageName(plan.executable); err != nil {
			return err
		}
	}

	if err := request.build(); err != nil {
		return err
	}
	program, err := os.ReadFile(request.program)
	if err != nil {
		return err
	}

	category := loaded.Linux.LinuxCategory()
	type artifact struct {
		label, temporary, final string
	}
	var artifacts []artifact
	cleanup := true
	defer func() {
		if cleanup {
			for _, item := range artifacts {
				_ = os.Remove(item.temporary)
			}
		}
	}()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	// write writes one artifact and returns its SHA-256, which the PKGBUILD
	// needs for the archive it installs.
	write := func(label, name string, build func(io.Writer) error) (string, error) {
		final := filepath.Join(dest, name)
		file, err := os.CreateTemp(dest, "."+name+".viva-tmp-*")
		if err != nil {
			return "", err
		}
		item := artifact{label: label, temporary: file.Name(), final: final}
		artifacts = append(artifacts, item)
		digest := sha256.New()
		if err := build(io.MultiWriter(file, digest)); err != nil {
			file.Close()
			return "", err
		}
		if err := file.Chmod(0o644); err != nil {
			file.Close()
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
		return hex.EncodeToString(digest.Sum(nil)), nil
	}

	var largest linuxicon.Rendition
	if len(icons) > 0 {
		largest = linuxicon.Largest(icons)
	}
	tarballName, tarballHash := "", ""
	if request.wants("tar.gz") || request.wants("pkgbuild") {
		spec := tarball.Linux{
			Executable: plan.executable, Name: loaded.App.Name, Version: loaded.App.Version, Arch: plan.arch,
			Comment: loaded.Linux.Comment, Category: category, Keywords: loaded.Linux.Keywords, URLSchemes: loaded.App.URLSchemes, FileTypes: linuxFileTypes(loaded.App.FileTypes), Program: program,
		}
		if largest.Pixels > 0 {
			spec.Icon, spec.IconPixels = largest.PNG, largest.Pixels
		}
		tarballName = spec.Directory() + ".tar.gz"
		tarballHash, err = write("tar.gz", tarballName, spec.Write)
		if err != nil {
			return err
		}
	}
	if request.wants("deb") {
		maintainer := loaded.Linux.Maintainer
		if maintainer == "" {
			maintainer = loaded.App.Name
		}
		spec := debpkg.Package{
			Executable: plan.executable, ID: loaded.App.ID, Name: loaded.App.Name, Version: loaded.App.Version, Arch: plan.arch,
			Maintainer: maintainer, Homepage: loaded.Linux.Homepage, Comment: loaded.Linux.Comment, Category: category,
			Keywords: loaded.Linux.Keywords, URLSchemes: loaded.App.URLSchemes, FileTypes: linuxFileTypes(loaded.App.FileTypes), Depends: debDepends(loaded.Linux.Depends), Program: program, Icons: icons,
		}
		if debName, err = spec.FileName(); err != nil {
			return err
		}
		if _, err := write("deb", debName, spec.Write); err != nil {
			return err
		}
	}
	if request.wants("pkgbuild") {
		maintainer := loaded.Linux.Maintainer
		if maintainer == "" {
			maintainer = loaded.App.Name
		}
		spec := pkgbuild.Package{
			Executable: plan.executable, ID: loaded.App.ID, Name: loaded.App.Name, Version: loaded.App.Version, Arch: plan.arch,
			Maintainer: maintainer, Homepage: loaded.Linux.Homepage, Comment: loaded.Linux.Comment,
			Depends: archDepends(loaded.Linux.ArchDepends), Archive: tarballName, SHA256: tarballHash,
			SourceURL: request.pkgbuildURL, IconPixels: largest.Pixels,
		}
		text, err := spec.PKGBUILD()
		if err != nil {
			return err
		}
		if _, err := write("pkgbuild", "PKGBUILD", func(w io.Writer) error {
			_, err := w.Write(text)
			return err
		}); err != nil {
			return err
		}
		srcinfo, err := spec.SRCINFO()
		if err != nil {
			return err
		}
		if _, err := write("srcinfo", ".SRCINFO", func(w io.Writer) error {
			_, err := w.Write(srcinfo)
			return err
		}); err != nil {
			return err
		}
	}

	for _, item := range artifacts {
		if err := os.Rename(item.temporary, item.final); err != nil {
			return err
		}
	}
	cleanup = false

	fmt.Fprintf(output, "packaged %s\n", displayPath(dest))
	fmt.Fprintf(output, "  app      %s %s (%s)\n", loaded.App.Name, loaded.App.Version, loaded.App.ID)
	fmt.Fprintf(output, "  target   linux/%s\n", plan.arch)
	if len(plan.tags) > 0 {
		fmt.Fprintf(output, "  tags     %s\n", strings.Join(plan.tags, ","))
	}
	fmt.Fprintf(output, "  program  %s%s\n", formatSize(int64(len(program))), sizeNote(plan, int64(len(program))))
	switch {
	case iconPath == "":
		fmt.Fprintf(output, "  icon     none (set [app] icon in %s)\n", manifest.FileName)
	case iconPixels < linuxicon.Recommended:
		fmt.Fprintf(output, "  icon     %s (%d px; %d px is recommended)\n", displayPath(iconPath), iconPixels, linuxicon.Recommended)
	default:
		fmt.Fprintf(output, "  icon     %s\n", displayPath(iconPath))
	}
	reportOpening(output, loaded.App, true)
	for _, item := range artifacts {
		if err := reportArchive(output, item.label, item.final); err != nil {
			return err
		}
	}
	fmt.Fprintln(output, "  signing  none: the packages are not signed")
	return nil
}

// debDepends puts the libraries the program opens at run time before the
// application's own dependencies, skipping any the manifest names itself.
func debDepends(own []string) []string {
	depends := make([]string, 0, len(linuxRuntimeDepends)+len(own))
	for _, library := range linuxRuntimeDepends {
		named := false
		for _, dependency := range own {
			if strings.Fields(dependency)[0] == library {
				named = true
			}
		}
		if !named {
			depends = append(depends, library)
		}
	}
	return append(depends, own...)
}

// archDepends is debDepends for Arch, whose package names differ and whose
// relations may be written without spaces (libxcb>=1.16).
func archDepends(own []string) []string {
	depends := make([]string, 0, len(linuxArchRuntimeDepends)+len(own))
	for _, library := range linuxArchRuntimeDepends {
		named := false
		for _, dependency := range own {
			if archDependencyName(dependency) == library {
				named = true
			}
		}
		if !named {
			depends = append(depends, library)
		}
	}
	seen := make(map[string]bool, len(depends)+len(own))
	for _, dependency := range depends {
		seen[archDependencyName(dependency)] = true
	}
	for _, dependency := range own {
		if name := archDependencyName(dependency); !seen[name] {
			seen[name] = true
			depends = append(depends, dependency)
		}
	}
	return depends
}

// archDependencyName is the package name of an Arch dependency without its
// version relation.
func archDependencyName(dependency string) string {
	for _, relation := range []string{">=", "<=", ">", "<", "="} {
		if before, _, ok := strings.Cut(dependency, relation); ok {
			return strings.TrimSpace(before)
		}
	}
	return strings.TrimSpace(dependency)
}

// loadLinuxIcon reads the [app] icon, which must be a PNG, and renders its
// hicolor sizes.
func loadLinuxIcon(path string) ([]linuxicon.Rendition, int, error) {
	if !strings.EqualFold(filepath.Ext(path), ".png") {
		return nil, 0, fmt.Errorf("the icon %s must be a .png file for Linux packages", path)
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("reading the icon: %w", err)
	}
	renditions, pixels, err := linuxicon.Render(source)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", path, err)
	}
	return renditions, pixels, nil
}
