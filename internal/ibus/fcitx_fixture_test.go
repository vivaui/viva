//go:build darwin || linux

package ibus

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestFcitxFixtureWaitsForTheExportedInputContextInterface(t *testing.T) {
	runFcitxFixture(t, false)
}

func TestFcitxFixtureCancellationKillsOwnedProcesses(t *testing.T) {
	runFcitxFixture(t, true)
}

func runFcitxFixture(t *testing.T, block bool) {
	t.Helper()
	if _, err := os.Stat("/usr/bin/python3"); err != nil {
		t.Skip("fixture requires Python")
	}
	directory := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		file := filepath.Join(directory, name)
		if err := os.WriteFile(file, []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
		return file
	}
	daemon, err := os.ReadFile("testdata/native-fcitx-daemon.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := write("native-fcitx-daemon.sh", string(daemon))
	write("fcitx5", "#!/bin/sh\nif [ \"$1\" = --version ]; then echo fixture; else echo $$ >\"$VIVA_FIXTURE_DAEMON_PID\"; exec sleep 30; fi\n")
	write("dconf", "#!/bin/sh\nexit 0\n")
	write("reference-client.py", "import sys\nprint('{}' if '--keycodes' in sys.argv else 'activation fixture')\n")
	write("gdbus", `#!/bin/sh
if [ "$1" = call ]; then
    echo '(true,)'
elif [ "$1" = introspect ]; then
    if [ ! -f "$VIVA_FIXTURE_EXPORTED" ]; then
        touch "$VIVA_FIXTURE_EXPORTED"
        echo '<node/>'
    else
        touch "$VIVA_FIXTURE_READY"
        echo '<node><interface name="org.freedesktop.IBus"><method name="CreateInputContext"/></interface></node>'
    fi
else
    exit 2
fi
`)
	client := write("client", "#!/bin/sh\nif [ ! -f \"$VIVA_FIXTURE_READY\" ]; then echo 'client ran before the input context object was exported' >&2; exit 42; fi\necho $$ >\"$VIVA_FIXTURE_CLIENT_PID\"\nif [ \"$VIVA_FIXTURE_BLOCK\" = true ]; then exec sleep 30; fi\n")
	timeout := 5 * time.Second
	if block {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, "sh", script)
	// Every fixture child stays in this test's process group, including a
	// client or daemon that is blocked when the deadline cancels the shell.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGTERM) }
	command.WaitDelay = time.Second
	readPID := func(name string) int {
		data, _ := os.ReadFile(filepath.Join(directory, name))
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		return pid
	}
	t.Cleanup(func() {
		if command.Process != nil {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
		for _, name := range []string{"daemon.pid", "client.pid"} {
			if pid := readPID(name); pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	command.Env = append(os.Environ(), "PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"), "XDG_RUNTIME_DIR="+filepath.Join(directory, "runtime"), "DBUS_SESSION_BUS_ADDRESS=fixture", "VIVA_FCITX_BACKEND=x11", "VIVA_FCITX_REFERENCE=0", "VIVA_FCITX_GTK_REFERENCE=0", "VIVA_FCITX_TEST_BINARY="+client, "VIVA_FIXTURE_EXPORTED="+filepath.Join(directory, "exported"), "VIVA_FIXTURE_READY="+filepath.Join(directory, "ready"), "VIVA_FIXTURE_DAEMON_PID="+filepath.Join(directory, "daemon.pid"), "VIVA_FIXTURE_CLIENT_PID="+filepath.Join(directory, "client.pid"), "VIVA_FIXTURE_BLOCK="+strconv.FormatBool(block))
	output, err := command.CombinedOutput()
	if !block && err != nil {
		t.Fatalf("fixture did not wait for the context interface: %v\n%s", err, output)
	}
	if block && (err == nil || ctx.Err() == nil) {
		t.Fatalf("fixture did not exercise deadline cancellation: %v\n%s", err, output)
	}
	deadline := time.Now().Add(time.Second)
	for syscall.Kill(-command.Process.Pid, 0) != syscall.ESRCH {
		if time.Now().After(deadline) {
			t.Fatal("fixture left an owned process group running")
		}
		time.Sleep(5 * time.Millisecond)
	}
	for _, name := range []string{"daemon.pid", "client.pid"} {
		pid := readPID(name)
		if pid <= 0 {
			t.Fatalf("fixture did not start its owned %s", name)
		}
		if syscall.Kill(pid, 0) != syscall.ESRCH {
			t.Fatalf("fixture left its owned %s running", name)
		}
	}
}
