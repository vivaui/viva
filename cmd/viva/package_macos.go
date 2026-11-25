package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/tools/appbundle"
	"github.com/vivaui/viva/tools/dmg"
	"github.com/vivaui/viva/tools/manifest"
)

// packageDarwin builds the program and assembles <name>.app, then the archives
// asked for from it.
func packageDarwin(request packageRequest) error {
	plan, loaded, output := request.plan, request.plan.manifest, request.output
	dest := request.out
	if dest == "" {
		dest = loaded.Path(filepath.Join(loaded.Build.Out, loaded.App.Name+".app"))
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

	signing, cleanup, err := prepareSigning(request, loaded)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := request.build(); err != nil {
		return err
	}
	required, err := appbundle.RequiredMacOS(request.program)
	if err != nil {
		return err
	}
	if err := checkDeployment(required, loaded.Mac.MinSystem); err != nil {
		return err
	}

	result, err := appbundle.Assemble(appbundle.Options{
		Dest: dest, Executable: request.program, Icon: icon,
		Info: appbundle.Info{
			Name: loaded.App.Name, ID: loaded.App.ID, Version: loaded.App.Version, Executable: plan.executable,
			MinSystem: loaded.Mac.MinSystem, Category: loaded.Mac.Category, Copyright: loaded.Mac.Copyright,
			URLSchemes: loaded.App.URLSchemes, FileTypes: macFileTypes(loaded.App.FileTypes), Extra: infoEntries(loaded.Mac.Info),
		},
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(output, "packaged %s\n", displayPath(result.Path))
	fmt.Fprintf(output, "  app      %s %s (%s)\n", loaded.App.Name, loaded.App.Version, loaded.App.ID)
	target := "darwin/" + plan.arch + " (macOS " + loaded.Mac.MinSystem + " or later)"
	fmt.Fprintf(output, "  target   %s\n", target)
	if len(plan.tags) > 0 {
		fmt.Fprintf(output, "  tags     %s\n", strings.Join(plan.tags, ","))
	}
	fmt.Fprintf(output, "  program  %s%s\n", formatSize(result.ExecutableBytes), sizeNote(plan, result.ExecutableBytes))
	fmt.Fprintf(output, "  bundle   %s\n", formatSize(result.BundleBytes))
	switch {
	case icon == "":
		fmt.Fprintf(output, "  icon     none (set [app] icon in %s)\n", manifest.FileName)
	case result.IconPixels == 0:
		fmt.Fprintf(output, "  icon     %s\n", displayPath(icon))
	case result.IconPixels < appbundle.RecommendedIconPixels:
		fmt.Fprintf(output, "  icon     %s (largest %d px; %d px is recommended)\n", displayPath(icon), result.IconPixels, appbundle.RecommendedIconPixels)
	default:
		fmt.Fprintf(output, "  icon     %s\n", displayPath(icon))
	}
	reportOpening(output, loaded.App, false)

	if signing.signs() {
		if err := signing.signPath(runSigningTool, result.Path, true); err != nil {
			return err
		}
		if signing.adHoc() {
			reportSigning(output, "signing", "ad hoc: only this Mac accepts the signature")
		} else {
			reportSigning(output, "signing", "signed with the hardened runtime and a secure timestamp")
		}
	}
	// A bundle that goes into a zip is notarized by itself, and the ticket stapled
	// to it before it is zipped: whoever unpacks that zip gets a bundle the system
	// accepts without a network lookup, which is what the in-app updater needs. A
	// disk image is notarized and stapled separately below, since it is that image
	// the system judges when it is opened.
	if signing.notarize && (request.wants("zip") || !request.wants("dmg")) {
		if err := notarizeBundle(signing, result.Path, output); err != nil {
			return err
		}
	}

	stem := fmt.Sprintf("%s-%s-darwin-%s", plan.executable, loaded.App.Version, plan.arch)
	if request.wants("zip") {
		archive := filepath.Join(filepath.Dir(result.Path), stem+".zip")
		if err := appbundle.Zip(result.Path, archive); err != nil {
			return err
		}
		if err := reportArchive(output, "zip", archive); err != nil {
			return err
		}
	}
	if request.wants("dmg") {
		image := filepath.Join(filepath.Dir(result.Path), stem+".dmg")
		if err := makeDMG(loaded, result.Path, image); err != nil {
			return err
		}
		if signing.signs() {
			if err := signing.signPath(runSigningTool, image, false); err != nil {
				return err
			}
		}
		if signing.notarize {
			id, err := signing.submit(runSigningTool, image)
			if err != nil {
				return err
			}
			if err := staple(runSigningTool, image); err != nil {
				return err
			}
			reportSigning(output, "notarized", fmt.Sprintf("accepted by Apple (submission %s), ticket stapled to %s", id, filepath.Base(image)))
		}
		if err := reportArchive(output, "dmg", image); err != nil {
			return err
		}
	}
	if !signing.signs() {
		fmt.Fprintln(output, "  signing  none: the bundle runs on this Mac; other Macs block it until it is signed and notarized (see viva help package)")
	}
	return nil
}

// runSigningTool runs codesign, notarytool and the rest; tests replace it.
var runSigningTool toolRunner = execTool

// prepareSigning decides what to sign from the flags, the environment and the
// manifest, and imports the certificate APPLE_CERTIFICATE holds, if any. cleanup
// removes what that left behind.
func prepareSigning(request packageRequest, loaded *manifest.Manifest) (macSigning, func(), error) {
	cleanup := func() {}
	fromManifest := ""
	if loaded.Mac.Entitlements != "" {
		fromManifest = loaded.Path(loaded.Mac.Entitlements)
	}
	signing, err := resolveSigning(request.sign, os.Getenv, fromManifest)
	if err != nil {
		return signing, cleanup, err
	}
	if !signing.signs() {
		return signing, cleanup, nil
	}
	if signing.entitlements != "" {
		if signing.entitlements, err = filepath.Abs(signing.entitlements); err != nil {
			return signing, cleanup, err
		}
		if info, statErr := os.Stat(signing.entitlements); statErr != nil || !info.Mode().IsRegular() {
			return signing, cleanup, fmt.Errorf("entitlements %s is not a file", displayPath(signing.entitlements))
		}
	}
	if certificate := os.Getenv("APPLE_CERTIFICATE"); certificate != "" {
		keychain, identity, remove, err := importCertificate(runSigningTool, certificate, os.Getenv("APPLE_CERTIFICATE_PASSWORD"))
		if err != nil {
			return signing, cleanup, err
		}
		cleanup = remove
		signing.keychain = keychain
		if os.Getenv("APPLE_SIGNING_IDENTITY") == "" && request.sign.identity == "" {
			signing.identity = identity
		}
	}
	return signing, cleanup, nil
}

// notarizeBundle notarizes a bundle that is not going into a disk image: it is
// zipped for the submission, then the ticket is stapled to the bundle itself.
func notarizeBundle(signing macSigning, bundle string, output io.Writer) error {
	scratch, err := os.MkdirTemp("", "viva-notarize-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	archive := filepath.Join(scratch, filepath.Base(bundle)+".zip")
	if err := appbundle.Zip(bundle, archive); err != nil {
		return err
	}
	id, err := signing.submit(runSigningTool, archive)
	if err != nil {
		return err
	}
	if err := staple(runSigningTool, bundle); err != nil {
		return err
	}
	reportSigning(output, "notarized", fmt.Sprintf("accepted by Apple (submission %s), ticket stapled to %s", id, filepath.Base(bundle)))
	return nil
}

// makeDMG writes a compressed disk image holding the bundle and a shortcut to
// /Applications, in the window [mac.dmg] describes, so opening it shows the
// usual drag-to-install content.
func makeDMG(loaded *manifest.Manifest, bundle, image string) error {
	config := loaded.Mac.DMG
	settings := dmg.Config{IconSize: config.IconSize}
	if config.Background != "" {
		settings.Background = loaded.Path(config.Background)
	}
	if config.App != nil {
		settings.App = &dmg.Point{X: config.App.X, Y: config.App.Y}
	}
	if config.Applications != nil {
		settings.Applications = &dmg.Point{X: config.Applications.X, Y: config.Applications.Y}
	}
	layout, err := dmg.Resolve(settings)
	if err != nil {
		return err
	}
	return dmg.Build(dmg.Options{Bundle: bundle, Volume: loaded.App.Name, Image: image, Layout: layout})
}

// reportArchive prints one archive line of the summary.
func reportArchive(output interface{ Write([]byte) (int, error) }, label, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "  %-8s %s  %s\n", label, displayPath(path), formatSize(info.Size()))
	return nil
}
