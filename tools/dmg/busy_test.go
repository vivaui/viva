package dmg

import (
	"errors"
	"os/exec"
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
func TestReleasedWaitsUntilTheImageIsNoLongerAttached(t *testing.T) {
	const image = "/tmp/viva-release-test/writable.dmg"
	held := reply{"<plist><array><dict><key>image-path</key><string>" + image + "</string></dict></array></plist>", nil}
	free := reply{"<plist><array></array></plist>", nil}
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

// A volume's node is a slice of a disk, and detaching the slice leaves the image
// attached: detach is given the disk.
func TestDetachTargetsTheWholeDisk(t *testing.T) {
	for node, want := range map[string]string{"/dev/disk7s1": "/dev/disk7", "/dev/disk12s3": "/dev/disk12", "/dev/disk7": "/dev/disk7", "/dev/diskimage": "/dev/diskimage"} {
		if got := wholeDisk(node); got != want {
			t.Errorf("wholeDisk(%q) = %q, want %q", node, got, want)
		}
	}
}
