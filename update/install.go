package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

var (
	// ErrUnsupported reports that this build target has no way to replace a
	// running application.
	ErrUnsupported = errors.New("update: installing an update is unsupported here")
	// ErrManagedExternally reports an application a package manager owns: its
	// package manager updates it, not the program itself.
	ErrManagedExternally = errors.New("update: the application is updated by its package manager")
	// ErrNotWritable reports that the running application cannot be replaced:
	// its directory is read-only, or the bundle is not where it can be changed.
	ErrNotWritable = errors.New("update: the application cannot be replaced")
	// ErrUnsafeArchive reports an archive whose paths or entries could write
	// outside the directory it is unpacked into.
	ErrUnsafeArchive = errors.New("update: archive is not safe to unpack")
	// ErrUntrusted reports an update the platform refuses to trust: a macOS
	// bundle whose code signature does not verify, or a Windows installer whose
	// Authenticode signature exists but does not.
	ErrUntrusted = errors.New("update: the update's signature is not trusted")
)

// InstallOptions says what to install and how the new version starts. The
// Artifact and Asset come from Client.Check and Client.Download; an application
// must not build them from somewhere else.
type InstallOptions struct {
	// Artifact is the file Download returned. Exactly one of Artifact and
	// Payload is set.
	Artifact string
	// Payload is a new application the caller already made with PatchReader.Apply:
	// the staged .app on macOS, or the staged executable on Linux. Install
	// replaces the running application with it exactly as it would with an
	// unpacked archive, running the same checks. Windows has no patch support,
	// so a payload there reports ErrUnsupported.
	Payload string
	// Asset is the archive it came from. Its Format tells Install how to unpack
	// it, and the format must suit the running platform.
	Asset Asset
	// Version is the release being installed. It is checked against the version
	// recorded in the archive where the platform records one.
	Version string
	// App identifies the application: the macOS bundle identifier, or the
	// executable's name elsewhere. Empty skips the identity check.
	App string
	// Args are handed to the relaunched program, after its own path.
	Args []string
	// Progress, when set, receives short human-readable steps: unpacking,
	// checking the signature, replacing the application. It runs on the
	// goroutine calling Install.
	Progress func(step string)
}

// Install replaces the running application with the artifact the signed feed
// listed and starts the new version. quit is called once the replacement is
// under way, and the application should make it end the event loop; Install
// returns after that call so the helper it started can wait for this process to
// exit. When Install returns an error nothing was changed and quit was not
// called.
//
// The call is platform-specific: macOS replaces the .app bundle, Windows runs
// the signed NSIS installer, and Linux replaces the executable in a tar.gz.
// An application a package manager installed reports ErrManagedExternally.
func Install(options InstallOptions, quit func()) error {
	if quit == nil {
		return fmt.Errorf("update: install needs a quit function")
	}
	if err := options.validate(); err != nil {
		return err
	}
	return install(options, quit)
}

func (options InstallOptions) validate() error {
	if (options.Artifact == "") == (options.Payload == "") {
		return fmt.Errorf("update: install needs exactly one of the downloaded artifact and a staged payload")
	}
	if options.Payload != "" {
		info, err := os.Lstat(options.Payload)
		if err != nil {
			return fmt.Errorf("update: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s is a link", ErrManifest, options.Payload)
		}
	} else {
		info, err := os.Lstat(options.Artifact)
		if err != nil {
			return fmt.Errorf("update: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s is a link", ErrManifest, options.Artifact)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: %s is not a regular file", ErrManifest, options.Artifact)
		}
		// Download verified these bytes, but the file sits on disk between the two
		// calls. Hash it again at the moment it is about to be installed.
		if options.Asset.Size > 0 && info.Size() != options.Asset.Size {
			return fmt.Errorf("%w: %s is %d bytes, the feed declared %d", ErrSize, options.Artifact, info.Size(), options.Asset.Size)
		}
		if len(options.Asset.SHA256) == 64 {
			digest, err := fileSHA256(options.Artifact)
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}
			if !strings.EqualFold(digest, options.Asset.SHA256) {
				return fmt.Errorf("%w: %s", ErrChecksum, options.Artifact)
			}
		}
	}
	if options.Version == "" {
		return fmt.Errorf("update: install needs the version it installs")
	}
	if _, err := parseVersion(options.Version); err != nil {
		return err
	}
	switch options.Asset.Format {
	case FormatZIP, FormatNSIS, FormatTarGz:
	default:
		return fmt.Errorf("%w: format %q", ErrManifest, options.Asset.Format)
	}
	if options.Asset.OS != "" && options.Asset.OS != runtime.GOOS {
		return fmt.Errorf("%w: archive is for %s, not %s", ErrManifest, options.Asset.OS, runtime.GOOS)
	}
	if options.Asset.Arch != "" && options.Asset.Arch != runtime.GOARCH {
		return fmt.Errorf("%w: archive is for %s, not %s", ErrManifest, options.Asset.Arch, runtime.GOARCH)
	}
	return nil
}

func (options InstallOptions) report(step string) {
	if options.Progress != nil {
		options.Progress(step)
	}
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
