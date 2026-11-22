package dmg

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	// Detaching returns before the disk image service lets go of the file;
	// converting it while the service still holds it fails with "Resource
	// temporarily unavailable", which hosted runners show for over a minute.
	if err := released(writable); err != nil {
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
		if detachErr := detach(device); err == nil {
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

// detach unmounts the image and detaches its disk, retrying while the system still
// holds it, and forcing it as the last resort. The volume being unmounted already
// (by the system, or by an earlier try) is not the end of it: the image stays
// attached, and the disk image service keeps holding the file, until its disk is
// detached. What ends it is the disk no longer being attached.
func detach(device string) error {
	// Detaching a partition's node unmounts that volume and leaves the image attached,
	// which the disk image service then holds: the whole disk is what goes.
	device = wholeDisk(device)
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = hdiutil("detach", device); err == nil {
			return nil
		}
		if gone, checkErr := diskGone(device); checkErr != nil {
			return fmt.Errorf("%w; checking the disk: %v", err, checkErr)
		} else if gone {
			return nil
		}
		pause(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	if forced := hdiutil("detach", "-force", device); forced == nil {
		return nil
	}
	if gone, checkErr := diskGone(device); checkErr == nil && gone {
		return nil
	} else if checkErr != nil {
		return fmt.Errorf("%w; checking the disk: %v", err, checkErr)
	}
	return err
}

// diskGone says no attached image has this disk, as hdiutil info lists them.
func diskGone(device string) (bool, error) {
	out, err := hdiutilOutput("info", "-plist")
	if err != nil {
		return false, err
	}
	return !strings.Contains(out, "<string>"+device+"</string>"), nil
}

var partitionNode = regexp.MustCompile(`^(/dev/disk\d+)s\d+$`)

// wholeDisk is the device node of the disk a node is on: /dev/disk7 for /dev/disk7s1.
func wholeDisk(device string) string {
	if match := partitionNode.FindStringSubmatch(device); match != nil {
		return match[1]
	}
	return device
}

// releaseAttempts and releaseWait bound how long released waits for the disk
// image service: up to two minutes.
const (
	releaseAttempts = 120
	releaseWait     = time.Second
)

// released waits until no attached image is the file at image, as hdiutil info
// lists them. It reports the list when the image is still attached at the end.
func released(image string) error {
	path := image
	if resolved, err := filepath.EvalSymlinks(image); err == nil {
		path = resolved
	}
	var attached string
	for attempt := 0; attempt < releaseAttempts; attempt++ {
		out, err := hdiutilOutput("info", "-plist")
		if err != nil {
			return err
		}
		if !strings.Contains(out, "<string>"+path+"</string>") && !strings.Contains(out, "<string>"+image+"</string>") {
			return nil
		}
		attached = out
		if redetachAt[attempt] {
			// The service can keep an image attached, and hold the file, after a detach
			// that reported success: take its disk away again.
			if device := diskOf(out, path, image); device != "" {
				_ = hdiutil("detach", "-force", device)
			}
		}
		pause(releaseWait)
	}
	return fmt.Errorf("the disk image service still holds %s after detaching it:\n%s", image, attached)
}

// redetachAt are the attempts of released at which an image still attached has its disk
// detached again.
var redetachAt = map[int]bool{5: true, 20: true, 50: true, 90: true}

var diskNode = regexp.MustCompile(`<string>(/dev/disk\d+)</string>`)

// diskOf is the disk of the attached image at one of the paths, from hdiutil info's
// list: the first disk its entry names, since an entry lists its image's path before
// its system entities.
func diskOf(info string, paths ...string) string {
	for _, path := range paths {
		at := strings.Index(info, "<string>"+path+"</string>")
		if at < 0 {
			continue
		}
		if match := diskNode.FindStringSubmatch(info[at:]); match != nil {
			return match[1]
		}
	}
	return ""
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
// mounting images, and refuses with "Resource busy" while it is. The increasing
// waits total at most 45 seconds: hosted macOS runners can retain an image's
// resources beyond the previous ten-second window after detaching its volume.
const busyAttempts = 10

// transient says a failure of hdiutil is the disk image service being unready rather
// than a mistake of ours: it is busy or unavailable, or, for create, it lost the staged
// folder it was just given ("No such file or directory" with the folder there), which
// is how it answers while the service starts under load. Another run takes.
func transient(command, output string) bool {
	return strings.Contains(output, "Resource busy") || strings.Contains(output, "Resource temporarily unavailable") ||
		command == "create" && strings.Contains(output, "create failed - No such file or directory")
}

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
		if arguments[0] != "detach" && attempt < busyAttempts && transient(arguments[0], string(out)) {
			pause(time.Duration(attempt) * time.Second)
			continue
		}
		return "", fmt.Errorf("hdiutil %s failed: %w: %s", arguments[0], err, strings.TrimSpace(string(out)))
	}
}
