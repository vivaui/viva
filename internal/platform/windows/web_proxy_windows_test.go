package windows

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestProxyBrowserArgumentsKeepThePageOnTheProxy(t *testing.T) {
	got := proxyBrowserArguments("", "127.0.0.1:1080")
	for _, want := range []string{
		"--proxy-server=socks5://127.0.0.1:1080",
		"--proxy-bypass-list=<-loopback>",
		`"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1"`,
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("arguments %q lack %q", got, want)
		}
	}
	if got := proxyBrowserArguments(" --lang=de ", "127.0.0.1:1080"); !strings.HasPrefix(got, "--lang=de --proxy-server=") {
		t.Fatalf("the arguments already set must come first, so the proxy's win: %q", got)
	}
}

func TestAProxyFolderHeldByABrowserIsNotReused(t *testing.T) {
	userData := t.TempDir()
	if slot := freeProxySlot(userData); slot != 1 {
		t.Fatalf("first slot = %d", slot)
	}
	lock := filepath.Join(proxyFolder(userData, 1), "EBWebView", "lockfile")
	if err := os.MkdirAll(filepath.Dir(lock), 0o700); err != nil {
		t.Fatal(err)
	}
	name, _ := syscall.UTF16PtrFromString(lock)
	// A browser keeps its lock file open with no sharing.
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.CREATE_ALWAYS, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if slot := freeProxySlot(userData); slot != 2 {
		t.Fatalf("slot with folder 1 held = %d", slot)
	}
	syscall.CloseHandle(handle)
	if slot := freeProxySlot(userData); slot != 1 {
		t.Fatalf("slot with folder 1 let go = %d", slot)
	}
}

func TestTheBrowserArgumentsAreSetForTheRuntimeAndTakenBack(t *testing.T) {
	const name = "VIVA_TEST_BROWSER_ARGUMENTS"
	if err := setProcessVariable(name, "--a", true); err != nil {
		t.Fatal(err)
	}
	if value, ok := syscall.Getenv(name); !ok || value != "--a" {
		t.Fatalf("set: %q, %v", value, ok)
	}
	if err := setProcessVariable(name, "", false); err != nil {
		t.Fatal(err)
	}
	if value, ok := syscall.Getenv(name); ok {
		t.Fatalf("removed variable still reads %q", value)
	}
}
