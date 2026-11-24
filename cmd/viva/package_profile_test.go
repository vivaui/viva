package main

import (
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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
	decoded := `<plist><dict>
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

func profileTools(t *testing.T, payload string) toolRunner {
	t.Helper()
	signer := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("profile signer")})
	issuer := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("profile issuer")})
	return func(name string, args ...string) (string, error) {
		switch args[0] {
		case "smime":
			if name != "/usr/bin/openssl" || !slices.Contains(args, "-verify") {
				t.Fatalf("unexpected verifier: %q %q", name, args)
			}
			if err := os.WriteFile(args[slices.Index(args, "-signer")+1], signer, 0o600); err != nil {
				return "", err
			}
			return "", os.WriteFile(args[slices.Index(args, "-out")+1], []byte(payload), 0o600)
		case "pkcs7":
			return string(signer) + string(issuer), nil
		case "verify-cert":
			if name != "/usr/bin/security" || len(args) != 8 ||
				(args[3] != "1.2.840.113635.100.1.40" && args[3] != "1.2.840.113635.100.1.56") {
				t.Fatalf("unexpected trust policy or certificates: %q %q", name, args)
			}
			actualSigner, err := os.ReadFile(args[5])
			if err != nil || string(actualSigner) != string(signer) {
				t.Fatal("signature's actual signer must be verified")
			}
			actualIssuer, err := os.ReadFile(args[7])
			if err != nil || string(actualIssuer) != "profile issuer" {
				t.Fatal("profile issuer was not passed to Security")
			}
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %q", args)
		}
	}
}

func TestProvisioningPreservesPermissionsAndUsesExactBundleID(t *testing.T) {
	signing, decoded := testProvisioningProfile(t)
	// An app's own application ID is an implicit Keychain access group.
	decoded = strings.ReplaceAll(decoded, "<key>keychain-access-groups</key><array><string>TEAM1234.*</string></array>", "")
	prepared, cleanup, err := signing.prepareProvisioning(profileTools(t, decoded), "dev.example.app")
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
	if _, ok := grants["keychain-access-groups"]; ok {
		t.Fatal("unexpected shared keychain groups")
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
		"expired":     {"2099-01-01T00:00:00Z", "2000-01-01T00:00:00Z"},
		"another app": {"TEAM1234.dev.example.*", "TEAM1234.other.app"},
		"non Mac":     {"<string>OSX</string>", "<string>iOS</string>"},
	} {
		t.Run(name, func(t *testing.T) {
			signing, decoded := testProvisioningProfile(t)
			decoded = strings.ReplaceAll(decoded, replacements[0], replacements[1])
			_, cleanup, err := signing.prepareProvisioning(profileTools(t, decoded), "dev.example.app")
			defer cleanup()
			if err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
}

func TestProvisioningRequiresVerifiedCMSSignatureAndAppleSigner(t *testing.T) {
	for _, stage := range []string{"smime", "verify-cert"} {
		t.Run(stage, func(t *testing.T) {
			signing, decoded := testProvisioningProfile(t)
			run := profileTools(t, decoded)
			_, cleanup, err := signing.prepareProvisioning(func(name string, args ...string) (string, error) {
				if args[0] == stage {
					return "", errors.New("verification failed")
				}
				return run(name, args...)
			}, "dev.example.app")
			defer cleanup()
			if err == nil {
				t.Fatalf("profile accepted after %s failure", stage)
			}
		})
	}
}

func TestProvisioningVerifiesSignedContentWithOpenSSL(t *testing.T) {
	tool := "openssl"
	if runtime.GOOS == "darwin" {
		tool = "/usr/bin/openssl"
	}
	openssl, err := exec.LookPath(tool)
	if err != nil {
		t.Skip("openssl unavailable")
	}
	dir := t.TempDir()
	config := filepath.Join(dir, "openssl.cnf")
	if err := os.WriteFile(config, []byte("[req]\ndistinguished_name=dn\n[dn]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	input, cert, key := filepath.Join(dir, "input"), filepath.Join(dir, "test.pem"), filepath.Join(dir, "test.key")
	profile := filepath.Join(dir, "signed.provisionprofile")
	const content = "<plist><dict/></plist>\n"
	if err := os.WriteFile(input, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"req", "-config", config, "-x509", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=Viva Test", "-days", "1", "-keyout", key, "-out", cert},
		{"smime", "-sign", "-binary", "-nodetach", "-signer", cert, "-inkey", key, "-in", input, "-outform", "DER", "-out", profile},
	} {
		if _, err := execTool(openssl, args...); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) (string, error) {
		if name == "/usr/bin/openssl" {
			return execTool(openssl, args...)
		}
		// macOS also checks that Security rejects this temporary test signer.
		if runtime.GOOS == "darwin" {
			return execTool(name, args...)
		}
		return "", nil
	}
	data, err := verifiedProfilePlist(run, profile)
	if runtime.GOOS == "darwin" {
		if err == nil || !strings.Contains(err.Error(), "verify Apple provisioning profile signer") {
			t.Fatalf("expected rejection by Apple's provisioning policy: %v", err)
		}
	} else if err != nil || string(data) != content {
		t.Fatalf("verified content %q, error %v", data, err)
	}
}

func TestProvisioningHonorsExplicitKeychainGroups(t *testing.T) {
	for _, tc := range []struct {
		group string
		valid bool
	}{
		{"<string>TEAM1234.shared</string>", true},
		{"<string>OTHER.shared</string>", false},
		{"<integer>1</integer>", false},
	} {
		signing, decoded := testProvisioningProfile(t)
		if err := os.WriteFile(signing.entitlements, []byte("<plist><dict><key>keychain-access-groups</key><array>"+tc.group+"</array></dict></plist>"), 0o600); err != nil {
			t.Fatal(err)
		}
		prepared, cleanup, err := signing.prepareProvisioning(profileTools(t, decoded), "dev.example.app")
		defer cleanup()
		if (err == nil) != tc.valid {
			t.Fatalf("entitlement %q: %v", tc.group, err)
		}
		if tc.valid {
			merged, err := os.ReadFile(prepared.entitlements)
			if err != nil || !strings.Contains(string(merged), tc.group) {
				t.Fatal("explicit shared group was lost")
			}
		}
	}
}

func TestProvisioningNeedsARealSigningIdentity(t *testing.T) {
	for _, options := range []signingOptions{{provisionProfile: "test.provisionprofile"}, {identity: "-", provisionProfile: "test.provisionprofile"}, {skip: true, provisionProfile: "test.provisionprofile"}} {
		if _, err := resolveSigning(options, environment(nil), ""); err == nil {
			t.Fatal("profile without a real signature accepted")
		}
	}
}
