package dmg

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// backgroundFile is the picture's name at the top of the image. The leading dot
// keeps the Finder from showing it.
const backgroundFile = ".background.png"

// Options describes one image to build.
type Options struct {
	// Bundle is the .app directory to put in the image.
	Bundle string
	// Volume is the name the mounted image shows.
	Volume string
	// Image is the .dmg file to write. An existing one is replaced only when the
	// new one is complete.
	Image  string
	Layout Layout
}

// Build writes a compressed read-only image holding the bundle, a shortcut to
// /Applications and the Finder settings of Options.Layout. It uses hdiutil,
// which every Mac has; the image is not byte-for-byte reproducible (HFS+
// records creation times).
func Build(options Options) error {
	work, err := os.MkdirTemp("", "viva-dmg-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	staging := filepath.Join(work, "image")
	if err := os.Mkdir(staging, 0o755); err != nil {
		return err
	}
	app := filepath.Base(options.Bundle)
	if out, err := exec.Command("cp", "-R", options.Bundle, filepath.Join(staging, app)).CombinedOutput(); err != nil {
		return fmt.Errorf("copying the bundle: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Symlink("/Applications", filepath.Join(staging, "Applications")); err != nil {
		return err
	}
	picture, err := options.Layout.picture()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, backgroundFile), picture, 0o644); err != nil {
		return err
	}

	// The Finder's record of the background names the picture by the volume's
	// own catalog IDs and dates, which exist only once the image is a volume. So
	// make it writable, mount it, write .DS_Store and compress the result.
	writable := filepath.Join(work, "writable.dmg")
	if err := hdiutil("create", "-volname", options.Volume, "-srcfolder", staging,
		"-fs", "HFS+", "-format", "UDRW", "-ov", writable); err != nil {
		return err
	}
	if err := settle(writable, options, app); err != nil {
		return err
	}

	// Write beside the target first so a failure leaves the previous image alone.
	temporary := strings.TrimSuffix(options.Image, ".dmg") + ".viva-tmp.dmg"
	_ = os.Remove(temporary)
	if err := convertImage(writable, temporary); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, options.Image)
}

func convertImage(writable, temporary string) error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = hdiutil("convert", writable, "-format", "UDZO", "-imagekey", "zlib-level=9", "-ov", "-o", temporary); err == nil {
			return nil
		}
		if attempt < 4 {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}
	}
	return err
}

// picture is the background's PNG: the project's, or the default.
func (l Layout) picture() ([]byte, error) {
	if l.Background != "" {
		return os.ReadFile(l.Background)
	}
	return l.defaultBackground()
}

// settle mounts the writable image and writes the Finder's settings into it.
func settle(writable string, options Options, app string) (err error) {
	mount := filepath.Join(filepath.Dir(writable), "mount")
	if err := os.Mkdir(mount, 0o755); err != nil {
		return err
	}
	if err := hdiutil("attach", "-nobrowse", "-noautoopen", "-noverify", "-readwrite", "-mountpoint", mount, writable); err != nil {
		return err
	}
	defer func() {
		if detachErr := detach(mount); err == nil {
			err = detachErr
		}
	}()

	file, err := Describe(mount, options.Volume, backgroundFile)
	if err != nil {
		return err
	}
	alias, err := file.Alias()
	if err != nil {
		return err
	}
	store, err := options.Layout.settings(app, alias)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(mount, ".DS_Store"), store, 0o644); err != nil {
		return err
	}
	// Mounting leaves bookkeeping behind that has no use on the next Mac.
	for _, name := range []string{".fseventsd", ".Trashes"} {
		_ = os.RemoveAll(filepath.Join(mount, name))
	}
	return nil
}

// detach unmounts the image, retrying while the system still holds it, and
// forcing it as the last resort.
func detach(mount string) error {
	if mounted, err := volumeMounted(mount); err == nil && !mounted {
		return nil
	}
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = hdiutil("detach", mount); err == nil {
			return nil
		}
		if mounted, checkErr := volumeMounted(mount); checkErr == nil && !mounted {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	if forced := hdiutil("detach", "-force", mount); forced == nil {
		return nil
	}
	if mounted, checkErr := volumeMounted(mount); checkErr == nil && !mounted {
		return nil
	}
	return err
}

func hdiutil(arguments ...string) error {
	out, err := exec.Command("hdiutil", arguments...).CombinedOutput()
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return errors.New("a .dmg needs hdiutil, which ships with macOS")
	}
	return fmt.Errorf("hdiutil %s failed: %w: %s", arguments[0], err, strings.TrimSpace(string(out)))
}
