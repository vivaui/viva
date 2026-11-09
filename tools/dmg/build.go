package dmg

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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
	if err := hdiutil("convert", writable, "-format", "UDZO", "-imagekey", "zlib-level=9", "-ov", "-o", temporary); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, options.Image)
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
	output, err := hdiutilOutput("attach", "-plist", "-nobrowse", "-noautoopen", "-noverify", "-readwrite", "-mountpoint", mount, writable)
	if err != nil {
		return err
	}
	device, err := attachedDevice(output, mount)
	if err != nil {
		_ = hdiutil("detach", "-force", mount)
		return err
	}
	defer func() {
		if detachErr := detach(mount, device); err == nil {
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

func attachedDevice(output, mount string) (string, error) {
	resolved, err := filepath.EvalSymlinks(mount)
	if err != nil {
		return "", err
	}
	decoder := xml.NewDecoder(strings.NewReader(output))
	var key, device, point string
	inEntities, inEntity := false, false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("dmg: reading attached image: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "key":
				if err := decoder.DecodeElement(&key, &element); err != nil {
					return "", err
				}
			case "array":
				inEntities = key == "system-entities"
			case "dict":
				if inEntities {
					inEntity, device, point = true, "", ""
				}
			case "string":
				if inEntity {
					var value string
					if err := decoder.DecodeElement(&value, &element); err != nil {
						return "", err
					}
					switch key {
					case "dev-entry":
						device = value
					case "mount-point":
						point = value
					}
				}
			}
		case xml.EndElement:
			if element.Name.Local == "dict" && inEntity {
				if filepath.Clean(point) == resolved && strings.HasPrefix(device, "/dev/disk") {
					return device, nil
				}
				inEntity = false
			} else if element.Name.Local == "array" {
				inEntities = false
			}
		}
	}
	return "", fmt.Errorf("dmg: attached image has no device at %s", mount)
}

// detach unmounts the image, retrying while the system still holds it, and
// forcing it as the last resort.
func detach(mount, device string) error {
	mounted, err := volumeMounted(mount)
	if err != nil {
		return err
	}
	if !mounted {
		return nil
	}
	for attempt := 0; attempt < 5; attempt++ {
		if err = hdiutil("detach", device); err == nil {
			return nil
		}
		if mounted, checkErr := volumeMounted(mount); checkErr == nil && !mounted {
			return nil
		} else if checkErr != nil {
			return fmt.Errorf("%w; checking mount: %v", err, checkErr)
		}
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	if forced := hdiutil("detach", "-force", device); forced == nil {
		return nil
	}
	if mounted, checkErr := volumeMounted(mount); checkErr == nil && !mounted {
		return nil
	} else if checkErr != nil {
		return fmt.Errorf("%w; checking mount: %v", err, checkErr)
	}
	return err
}

func hdiutil(arguments ...string) error {
	_, err := hdiutilOutput(arguments...)
	return err
}

// runHdiutil runs the tool; a variable so that a test stands in for it, and pause waits
// between attempts for the same reason.
var (
	runHdiutil = func(arguments ...string) ([]byte, error) {
		return exec.Command("hdiutil", arguments...).CombinedOutput()
	}
	pause = time.Sleep
)

// busyAttempts is how many times a command that finds the system busy is run: the
// disk image service is shared with whatever else on the machine is making or
// mounting images, and refuses with "Resource busy" while it is.
const busyAttempts = 5

func hdiutilOutput(arguments ...string) (string, error) {
	for attempt := 1; ; attempt++ {
		out, err := runHdiutil(arguments...)
		if err == nil {
			return string(out), nil
		}
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("a .dmg needs hdiutil, which ships with macOS")
		}
		// detach has its own way of waiting, which checks whether the volume is gone.
		if arguments[0] != "detach" && attempt < busyAttempts && strings.Contains(string(out), "Resource busy") {
			pause(time.Duration(attempt) * time.Second)
			continue
		}
		return "", fmt.Errorf("hdiutil %s failed: %w: %s", arguments[0], err, strings.TrimSpace(string(out)))
	}
}
