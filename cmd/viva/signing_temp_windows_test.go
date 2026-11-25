package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func signingACL(t *testing.T, path string) string {
	t.Helper()
	wide, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	var descriptor uintptr
	code, _, _ := signingAdvapi.NewProc("GetNamedSecurityInfoW").Call(uintptr(unsafe.Pointer(wide)), 1, 4, 0, 0, 0, 0, uintptr(unsafe.Pointer(&descriptor)))
	if code != 0 {
		t.Fatal(syscall.Errno(code))
	}
	defer syscall.LocalFree(syscall.Handle(descriptor))
	var value *uint16
	var size uint32
	if ok, _, err := signingAdvapi.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW").Call(descriptor, 1, 4, uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&size))); ok == 0 {
		t.Fatal(err)
	}
	defer syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(value))))
	return syscall.UTF16ToString(unsafe.Slice(value, size))
}

// sddlTrustee returns how a security descriptor string names the account with
// sid. Windows writes the well-known accounts by a two-letter alias rather than
// by SID (the built-in Administrator, RID 500, is "LA"), so comparing the
// trustee of a descriptor read back from disk with the SID string fails for
// exactly the accounts that have an alias. Round-tripping a one-entry
// descriptor through the same conversion gives the form the system will use.
func sddlTrustee(t *testing.T, sid string) string {
	t.Helper()
	sddl, err := syscall.UTF16PtrFromString("D:(A;;FA;;;" + sid + ")")
	if err != nil {
		t.Fatal(err)
	}
	var descriptor uintptr
	if ok, _, err := signingSDFromString.Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0); ok == 0 {
		t.Fatal(err)
	}
	defer syscall.LocalFree(syscall.Handle(descriptor))
	var value *uint16
	var size uint32
	if ok, _, err := signingAdvapi.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW").Call(descriptor, 1, 4, uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&size))); ok == 0 {
		t.Fatal(err)
	}
	defer syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(value))))
	entry := syscall.UTF16ToString(unsafe.Slice(value, size))
	return entry[strings.LastIndex(entry, ";")+1 : len(entry)-1]
}

func assertPrivateSigningFile(t *testing.T, path string) {
	t.Helper()
	sid, err := signingUserSID()
	if err != nil {
		t.Fatal(err)
	}
	trustee := sddlTrustee(t, sid)
	for _, name := range []string{filepath.Dir(path), path} {
		acl := signingACL(t, name)
		if name == filepath.Dir(path) && !strings.HasPrefix(acl, "D:P") {
			t.Fatalf("directory inherits parent ACL: %s", acl)
		}
		entries := strings.Split(acl, "(")
		if len(entries) != 3 {
			t.Fatalf("unexpected ACL: %s", acl)
		}
		for _, entry := range entries[1:] {
			parts := strings.Split(strings.TrimSuffix(entry, ")"), ";")
			if len(parts) != 6 || parts[0] != "A" || parts[2] != "FA" || (parts[5] != trustee && parts[5] != "SY") {
				t.Fatalf("unexpected access grant: %s", acl)
			}
		}
	}
}

func TestSigningSecretsDoNotInheritBroadTempPermissions(t *testing.T) {
	parent := t.TempDir()
	sid, err := signingUserSID()
	if err != nil {
		t.Fatal(err)
	}
	// Explicitly make the test parent inheritable/readable by Everyone, even on
	// machines whose normal temp directory is already private.
	sddl, _ := syscall.UTF16PtrFromString("D:P(A;OICI;FA;;;" + sid + ")(A;OICI;GR;;;WD)")
	var descriptor uintptr
	if ok, _, err := signingSDFromString.Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0); ok == 0 {
		t.Fatal(err)
	}
	defer syscall.LocalFree(syscall.Handle(descriptor))
	wide, _ := syscall.UTF16PtrFromString(parent)
	if ok, _, err := signingAdvapi.NewProc("SetFileSecurityW").Call(uintptr(unsafe.Pointer(wide)), 4|0x80000000, descriptor); ok == 0 {
		t.Fatal(err)
	}
	t.Setenv("TMP", parent)
	t.Setenv("TEMP", parent)
	signing, err := resolveWindowsSigning(signingOptions{}, environment(map[string]string{"WINDOWS_CERTIFICATE": base64.StdEncoding.EncodeToString([]byte("dummy certificate"))}))
	if err != nil {
		t.Fatal(err)
	}
	defer signing.cleanup()
	assertPrivateSigningFile(t, signing.certificate)
	signing.cleanup()
	if _, err := os.Stat(filepath.Dir(signing.certificate)); !os.IsNotExist(err) {
		t.Fatalf("certificate directory not cleaned: %v", err)
	}

	program := filepath.Join(parent, "program.exe")
	if err := os.WriteFile(program, []byte("dummy program"), 0o600); err != nil {
		t.Fatal(err)
	}
	signing.password = "dummy password"
	var passwordFile string
	err = signing.signWithOpenSSL(func(_ string, args ...string) (string, error) {
		var output string
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-readpass" {
				passwordFile = args[i+1]
			}
			if args[i] == "-out" {
				output = args[i+1]
			}
		}
		if passwordFile == "" {
			t.Fatal("password was not passed through a file")
		}
		assertPrivateSigningFile(t, passwordFile)
		return "", os.WriteFile(output, []byte("signed fixture"), 0o600)
	}, program)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(passwordFile)); !os.IsNotExist(err) {
		t.Fatalf("password directory not cleaned: %v", err)
	}
}

func TestSigningTempFailureDoesNotFallBackToAnUnprotectedDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing", "temp")
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	if path, err := signingTempDir(); err == nil || path != "" {
		t.Fatalf("unexpected success: %q, %v", path, err)
	}
}
