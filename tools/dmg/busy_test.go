package dmg

import (
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// stub replaces the tool with replies, one per run, and records the runs and the waits.
func stub(t *testing.T, replies []struct {
	out string
	err error
}) (runs *[][]string, waits *[]time.Duration) {
	t.Helper()
	runs, waits = new([][]string), new([]time.Duration)
	savedRun, savedPause := runHdiutil, pause
	t.Cleanup(func() { runHdiutil, pause = savedRun, savedPause })
	runHdiutil = func(arguments ...string) ([]byte, error) {
		reply := replies[min(len(*runs), len(replies)-1)]
		*runs = append(*runs, arguments)
		return []byte(reply.out), reply.err
	}
	pause = func(d time.Duration) { *waits = append(*waits, d) }
	return runs, waits
}

type reply = struct {
	out string
	err error
}

var errExit = errors.New("exit status 1")

// A busy system is waited for, with a longer wait each time, and the command is run
// again until it takes.
func TestHdiutilWaitsOutABusySystem(t *testing.T) {
	for _, command := range []string{"create", "convert", "attach"} {
		for _, message := range []string{"Resource busy", "Resource temporarily unavailable"} {
			t.Run(command+"/"+message, func(t *testing.T) {
				busy := reply{"hdiutil: " + command + " failed - " + message, errExit}
				runs, waits := stub(t, []reply{busy, busy, {"ready", nil}})
				out, err := hdiutilOutput(command, "image")
				if err != nil || out != "ready" {
					t.Fatalf("output %q, error %v", out, err)
				}
				if len(*runs) != 3 || len(*waits) != 2 || (*waits)[0] >= (*waits)[1] {
					t.Fatalf("%d runs, waits %v", len(*runs), *waits)
				}
				for _, arguments := range *runs {
					if len(arguments) != 2 || arguments[0] != command || arguments[1] != "image" {
						t.Fatalf("retried arguments %v", arguments)
					}
				}
			})
		}
	}
}

// create answers "No such file or directory" while the disk image service starts under
// load, with the folder it was given there: that is waited out too, but only for create.
func TestHdiutilCreateRetriesALostFolder(t *testing.T) {
	lost := reply{"hdiutil: create failed - No such file or directory", errExit}
	runs, _ := stub(t, []reply{lost, {"ready", nil}})
	if out, err := hdiutilOutput("create", "image"); err != nil || out != "ready" || len(*runs) != 2 {
		t.Fatalf("output %q, error %v, %d runs", out, err, len(*runs))
	}
	runs, _ = stub(t, []reply{{"hdiutil: attach failed - No such file or directory", errExit}})
	if _, err := hdiutilOutput("attach", "image"); err == nil || len(*runs) != 1 {
		t.Fatalf("a missing image was retried: %v, %d runs", err, len(*runs))
	}
}

// It does not wait for ever, and the error says what the tool said.
func TestHdiutilGivesUpOnAStaysBusySystem(t *testing.T) {
	for _, message := range []string{"Resource busy", "Resource temporarily unavailable"} {
		t.Run(message, func(t *testing.T) {
			runs, waits := stub(t, []reply{{"hdiutil: convert failed - " + message, errExit}})
			_, err := hdiutilOutput("convert", "image")
			if err == nil || !strings.Contains(err.Error(), message) || !errors.Is(err, errExit) {
				t.Fatalf("error %v", err)
			}
			if len(*runs) != busyAttempts || len(*waits) != busyAttempts-1 {
				t.Fatalf("%d runs, %d waits, want %d runs", len(*runs), len(*waits), busyAttempts)
			}
		})
	}
}

// Any other failure is reported at once, and so is a missing tool; detach waits in
// its own way.
func TestHdiutilReportsOtherFailuresAtOnce(t *testing.T) {
	for name, c := range map[string]struct {
		command string
		reply   reply
	}{
		"other":                          {"create", reply{"hdiutil: create failed - No space left on device", errExit}},
		"detach":                         {"detach", reply{"hdiutil: couldn't unmount - Resource busy", errExit}},
		"detach temporarily unavailable": {"detach", reply{"hdiutil: couldn't unmount - Resource temporarily unavailable", errExit}},
		"missing":                        {"create", reply{"", exec.ErrNotFound}},
	} {
		t.Run(name, func(t *testing.T) {
			runs, waits := stub(t, []reply{c.reply})
			if _, err := hdiutilOutput(c.command, "x"); err == nil {
				t.Fatal("no error")
			}
			if len(*runs) != 1 || len(*waits) != 0 {
				t.Fatalf("%d runs, waits %v", len(*runs), *waits)
			}
		})
	}
}

// The image is converted only once the disk image service has let go of it:
// released waits while hdiutil info still lists it, and says so if it never
// does.
func TestReleasedDetachesTheDiskAgainWhileTheImageStaysAttached(t *testing.T) {
	const image = "/tmp/viva-release-test/writable.dmg"
	held := reply{imageInfoFixture(imageEntryFixture(image, "/dev/disk7")), nil}
	free := reply{imageInfoFixture(""), nil}
	replies := []reply{}
	for i := 0; i < 7; i++ { // attempts 0 to 5 find it held, and the force detach of attempt 5 is one more run
		replies = append(replies, held)
	}
	replies = append(replies, free)
	runs, _ := stub(t, replies)
	if err := released(image); err != nil {
		t.Fatal(err)
	}
	forced := false
	for _, arguments := range *runs {
		if len(arguments) == 3 && arguments[0] == "detach" && arguments[1] == "-force" && arguments[2] == "/dev/disk7" {
			forced = true
		}
	}
	if !forced {
		t.Fatalf("the disk was never detached again: %v", *runs)
	}
}

func TestReleasedWaitsUntilTheImageIsNoLongerAttached(t *testing.T) {
	const image = "/tmp/viva-release-test/writable.dmg"
	held := reply{imageInfoFixture(imageEntryFixture(image, "")), nil}
	free := reply{imageInfoFixture(""), nil}
	runs, waits := stub(t, []reply{held, held, free})
	if err := released(image); err != nil {
		t.Fatal(err)
	}
	if len(*runs) != 3 || len(*waits) != 2 {
		t.Fatalf("%d checks and %d waits, want 3 and 2", len(*runs), len(*waits))
	}
	stub(t, []reply{held})
	if err := released(image); err == nil || !strings.Contains(err.Error(), image) {
		t.Fatalf("an image that stays attached gave %v", err)
	}
}

func imageInfoFixture(entries string) string {
	return `<plist><dict><key>images</key><array>` + entries + `</array></dict></plist>`
}

func imageEntryFixture(path, device string) string {
	var escaped strings.Builder
	_ = xml.EscapeText(&escaped, []byte(path))
	entry := `<dict><key>image-path</key><string>` + escaped.String() + `</string>`
	if device != "" {
		entry += `<key>system-entities</key><array><dict><key>dev-entry</key><string>` + device + `</string></dict></array>`
	}
	return entry + `</dict>`
}

func TestReleasedNeverForcesTheNextImageWhenTheOwnedEntryHasNoDisk(t *testing.T) {
	const owned = "/tmp/owned.dmg"
	other := imageEntryFixture("/tmp/other.dmg", "/dev/disk11")
	held := reply{imageInfoFixture(imageEntryFixture(owned, "") + other), nil}
	replies := slices.Repeat([]reply{held}, 7)
	replies = append(replies, reply{imageInfoFixture(other), nil})
	runs, _ := stub(t, replies)
	if err := released(owned); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range *runs {
		if arguments[0] != "info" {
			t.Fatalf("unexpected command while the owned entry has no disk: %v", arguments)
		}
	}
}

func TestImageRecordDecodesTheOwnedPathAndUsesItsPartition(t *testing.T) {
	const owned = "/tmp/one & two.dmg"
	info := imageInfoFixture(imageEntryFixture(owned, "/dev/disk7s1") + imageEntryFixture("/tmp/other.dmg", "/dev/disk11"))
	held, device, err := imageRecord(info, owned, owned)
	if !held || device != "/dev/disk7" || err != nil {
		t.Fatalf("held=%v, device=%q, error=%v", held, device, err)
	}
	if _, _, err := imageRecord(`<plist><dict/></plist>`, owned, owned); err == nil {
		t.Fatal("missing image information was treated as released")
	}
}

func TestBuildWaitsForOwnedImageAfterBusyDetach(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the mocked build uses Darwin file metadata")
	}
	dir := t.TempDir()
	bundle := filepath.Join(dir, "Sample.app")
	if err := os.Mkdir(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	layout, err := Resolve(Config{})
	if err != nil {
		t.Fatal(err)
	}
	stub(t, nil) // Restore the command and disable real delays.
	var staging, writable string
	forced, converted := 0, false
	runHdiutil = func(arguments ...string) ([]byte, error) {
		switch arguments[0] {
		case "create":
			staging = arguments[slices.Index(arguments, "-srcfolder")+1]
			writable = arguments[len(arguments)-1]
			return nil, os.WriteFile(writable, nil, 0o644)
		case "attach":
			mount := arguments[slices.Index(arguments, "-mountpoint")+1]
			for _, name := range []string{backgroundFile, stagedBundle} {
				if err := os.Rename(filepath.Join(staging, name), filepath.Join(mount, name)); err != nil {
					t.Fatal(err)
				}
			}
			mount, err := filepath.EvalSymlinks(mount)
			if err != nil {
				t.Fatal(err)
			}
			return []byte(`<plist><dict><key>system-entities</key><array><dict><key>dev-entry</key><string>/dev/disk7s1</string><key>mount-point</key><string>` + mount + `</string></dict></array></dict></plist>`), nil
		case "detach":
			if arguments[len(arguments)-1] != "/dev/disk7" {
				t.Fatalf("attempted to detach an unrelated disk: %v", arguments)
			}
			if slices.Contains(arguments, "-force") {
				forced++
				if forced == 2 {
					return nil, nil
				}
			}
			return []byte(`hdiutil: couldn't eject "disk7" - Resource busy`), errExit
		case "info":
			entries := imageEntryFixture("/tmp/other.dmg", "/dev/disk11")
			if forced < 2 {
				entries += imageEntryFixture(writable, "/dev/disk7")
			}
			return []byte(imageInfoFixture(entries)), nil
		case "convert":
			if forced != 2 {
				t.Fatal("conversion began while the image was still attached")
			}
			converted = true
			return nil, os.WriteFile(arguments[len(arguments)-1], []byte("converted"), 0o644)
		default:
			t.Fatalf("unexpected hdiutil call: %v", arguments)
			return nil, errExit
		}
	}
	image := filepath.Join(dir, "sample.dmg")
	if err := Build(Options{Bundle: bundle, Volume: "Sample", Image: image, Layout: layout}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(image); err != nil || !converted || string(got) != "converted" {
		t.Fatalf("final image = %q, converted=%v, err=%v", got, converted, err)
	}
}

// A volume's node is a slice of a disk, and detaching the slice leaves the image
// attached: detach is given the disk.
func TestDetachTargetsTheWholeDisk(t *testing.T) {
	for node, want := range map[string]string{"/dev/disk7s1": "/dev/disk7", "/dev/disk12s3": "/dev/disk12", "/dev/disk7": "/dev/disk7", "/dev/diskimage": "/dev/diskimage"} {
		if got := wholeDisk(node); got != want {
			t.Errorf("wholeDisk(%q) = %q, want %q", node, got, want)
		}
	}
}
