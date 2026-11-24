package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testProvisioningProfile(t *testing.T) (macSigning, string) {
	t.Helper()
	dir := t.TempDir()
	profile := filepath.Join(dir, "test.provisionprofile")
	if err := os.WriteFile(profile, []byte("frozen CMS profile bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	entitlements := filepath.Join(dir, "original.plist")
	if err := os.WriteFile(entitlements, []byte(`<plist><dict><key>com.apple.security.network.client</key><true/></dict></plist>`), 0o600); err != nil {
		t.Fatal(err)
	}
	decoded := `SMIME: level=0.1; type=signedData; nsigners=1;
signer0.id="Apple Mac OS X Provisioning Profile Signing"; signer0.status=GoodSignature;
<plist><dict>
<key>ExpirationDate</key><date>2099-01-01T00:00:00Z</date>
<key>Platform</key><array><string>OSX</string></array>
<key>TeamIdentifier</key><array><string>TEAM1234</string></array>
<key>ApplicationIdentifierPrefix</key><array><string>TEAM1234</string></array>
<key>DeveloperCertificates</key><array><data>` + base64.StdEncoding.EncodeToString([]byte("authorized certificate")) + `</data></array>
<key>Entitlements</key><dict>
<key>com.apple.application-identifier</key><string>TEAM1234.dev.example.*</string>
<key>keychain-access-groups</key><array><string>TEAM1234.*</string></array>
</dict></dict></plist>`
	return macSigning{identity: "Developer ID Application: Tests", provisionProfile: profile, entitlements: entitlements}, decoded
}

func TestProvisioningPreservesPermissionsAndUsesExactBundleID(t *testing.T) {
	signing, decoded := testProvisioningProfile(t)
	prepared, cleanup, err := signing.prepareProvisioning(func(name string, args ...string) (string, error) { return decoded, nil }, "dev.example.app")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(prepared.entitlements)
	if err != nil {
		t.Fatal(err)
	}
	grants, err := plistDictionary(data)
	if err != nil {
		t.Fatal(err)
	}
	if grants["com.apple.application-identifier"].Text != "TEAM1234.dev.example.app" {
		t.Fatal("bundle identity was not expanded exactly")
	}
	if grants["com.apple.security.network.client"].XMLName.Local != "true" {
		t.Fatal("existing entitlement lost")
	}
	if got := plistStrings(grants["keychain-access-groups"]); len(got) != 1 || got[0] != "TEAM1234.dev.example.app" {
		t.Fatalf("unexpected keychain groups: %v", got)
	}
	if string(prepared.profileData) != "frozen CMS profile bytes" {
		t.Fatal("profile bytes changed")
	}
	// Verify both the actual signed team/certificate and signed entitlement
	// values, rather than assuming codesign used the requested input file.
	runner := func(name string, args ...string) (string, error) {
		if len(args) > 1 && args[1] == "--verbose=4" {
			return "TeamIdentifier=TEAM1234", nil
		}
		if len(args) > 1 && args[1] == "--extract-certificates" {
			return "", os.WriteFile(args[2]+"0", []byte("authorized certificate"), 0o600)
		}
		if len(args) > 1 && args[1] == "--entitlements" {
			return string(data), nil
		}
		return "", fmt.Errorf("unexpected tool invocation")
	}
	if err := prepared.verifyProfileSignature(runner, "Test.app"); err != nil {
		t.Fatal(err)
	}
	bad := runner
	if err := prepared.verifyProfileSignature(func(name string, args ...string) (string, error) {
		if len(args) > 1 && args[1] == "--extract-certificates" {
			return "", os.WriteFile(args[2]+"0", []byte("foreign certificate"), 0o600)
		}
		return bad(name, args...)
	}, "Test.app"); err == nil {
		t.Fatal("foreign signing certificate was accepted")
	}
}

func TestProvisioningRejectsIncompatibleOrInvalidProfiles(t *testing.T) {
	for name, replacements := range map[string][2]string{
		"expired":            {"2099-01-01T00:00:00Z", "2000-01-01T00:00:00Z"},
		"another app":        {"TEAM1234.dev.example.*", "TEAM1234.other.app"},
		"another group":      {"<string>TEAM1234.*</string>", "<string>TEAM1234.other.*</string>"},
		"bad CMS signature":  {"signer0.status=GoodSignature;", "signer0.status=BadSignature;"},
		"foreign CMS signer": {"Apple Mac OS X Provisioning Profile Signing", "Unrelated Signer"},
		"non Mac":            {"<string>OSX</string>", "<string>iOS</string>"},
	} {
		t.Run(name, func(t *testing.T) {
			signing, decoded := testProvisioningProfile(t)
			decoded = strings.ReplaceAll(decoded, replacements[0], replacements[1])
			_, cleanup, err := signing.prepareProvisioning(func(string, ...string) (string, error) { return decoded, nil }, "dev.example.app")
			defer cleanup()
			if err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
}

func TestProvisioningNeedsARealSigningIdentity(t *testing.T) {
	for _, options := range []signingOptions{{provisionProfile: "test.provisionprofile"}, {identity: "-", provisionProfile: "test.provisionprofile"}, {skip: true, provisionProfile: "test.provisionprofile"}} {
		if _, err := resolveSigning(options, environment(nil), ""); err == nil {
			t.Fatal("profile without a real signature accepted")
		}
	}
}
