package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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

// Signing and notarizing a macOS package is optional: without an identity nothing
// here runs and the package is exactly what it was before. Everything secret comes
// from the environment, never from viva.toml or the command line.
//
//	APPLE_SIGNING_IDENTITY      the identity to sign with, "Developer ID Application: …"
//	                            or its hash; "-" signs ad hoc, for this Mac only
//	APPLE_CERTIFICATE           a base64 .p12 to import into a temporary keychain
//	APPLE_CERTIFICATE_PASSWORD  its password (CI machines have no keychain to start with)
//
// and for notarization one of
//
//	APPLE_NOTARY_PROFILE        a profile made by `xcrun notarytool store-credentials`
//	APPLE_API_KEY, APPLE_API_KEY_ID, APPLE_API_ISSUER   an App Store Connect API key
//	APPLE_ID, APPLE_PASSWORD, APPLE_TEAM_ID             an Apple ID and app-specific password

// macSigning is what a package run was asked to sign and notarize.
type macSigning struct {
	identity     string
	keychain     string // keychain holding the identity, or empty for the search list
	entitlements string
	notarize     bool
	notary       []string // credential arguments for notarytool; may hold a password
}

func (signing macSigning) signs() bool { return signing.identity != "" }

// adHoc reports a signature that only this Mac accepts.
func (signing macSigning) adHoc() bool { return signing.identity == "-" }

// toolRunner runs one command and returns its combined output. It is the only
// way this file reaches codesign, notarytool and the rest, so a test can stand in.
type toolRunner func(name string, arguments ...string) (string, error)

func execTool(name string, arguments ...string) (string, error) {
	output, err := exec.Command(name, arguments...).CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return text, fmt.Errorf("%s was not found; %s", name, missingToolHint(name))
		}
		// Arguments can hold a password: only the command's name is repeated.
		return text, fmt.Errorf("%s failed: %w: %s", name, err, firstLines(text, 6))
	}
	return text, nil
}

// missingToolHint says where a signing tool comes from.
func missingToolHint(name string) string {
	switch name {
	case "osslsigncode":
		return "install it (brew install osslsigncode, apt install osslsigncode) or give a sign command"
	case "signtool":
		return "it ships with the Windows SDK; or install osslsigncode, which runs anywhere"
	}
	return "signing on macOS needs the Xcode command line tools"
}

func firstLines(text string, count int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > count {
		lines = append(lines[:count], "…")
	}
	return strings.Join(lines, "\n")
}

// signingOptions are the command line's say in signing.
type signingOptions struct {
	identity     string
	entitlements string
	notarize     bool
	skip         bool
	// signCommand is the Windows custom sign command (see package_winsign.go).
	signCommand string
}

// resolveSigning decides from the options, the environment and the manifest's
// entitlements whether and how to sign. It does not touch the system.
func resolveSigning(options signingOptions, environment func(string) string, manifestEntitlements string) (macSigning, error) {
	var signing macSigning
	if options.skip {
		if options.notarize || options.identity != "" {
			return signing, errors.New("--no-sign cannot be combined with --sign-identity or --notarize")
		}
		return signing, nil
	}
	signing.identity = options.identity
	if signing.identity == "" {
		signing.identity = environment("APPLE_SIGNING_IDENTITY")
	}
	if signing.identity == "" && environment("APPLE_CERTIFICATE") != "" {
		// A certificate to import names its own identity (see importCertificate).
		signing.identity = "certificate"
	}
	signing.entitlements = options.entitlements
	if signing.entitlements == "" {
		signing.entitlements = manifestEntitlements
	}
	signing.notary = notaryCredentials(environment)
	if options.notarize {
		if !signing.signs() || signing.adHoc() {
			return signing, errors.New("--notarize needs a Developer ID identity: set APPLE_SIGNING_IDENTITY or pass --sign-identity (an ad hoc signature cannot be notarized)")
		}
		if signing.notary == nil {
			return signing, errors.New("--notarize needs credentials in the environment: APPLE_NOTARY_PROFILE, or APPLE_API_KEY with APPLE_API_KEY_ID and APPLE_API_ISSUER, or APPLE_ID with APPLE_PASSWORD and APPLE_TEAM_ID")
		}
	}
	// Credentials in the environment notarize a signed package without being
	// asked, as long as the signature can be: that is what a release machine has.
	signing.notarize = signing.signs() && !signing.adHoc() && signing.notary != nil
	return signing, nil
}

func notaryCredentials(environment func(string) string) []string {
	switch {
	case environment("APPLE_NOTARY_PROFILE") != "":
		return []string{"--keychain-profile", environment("APPLE_NOTARY_PROFILE")}
	case environment("APPLE_API_KEY") != "" && environment("APPLE_API_KEY_ID") != "" && environment("APPLE_API_ISSUER") != "":
		return []string{"--key", environment("APPLE_API_KEY"), "--key-id", environment("APPLE_API_KEY_ID"), "--issuer", environment("APPLE_API_ISSUER")}
	case environment("APPLE_ID") != "" && environment("APPLE_PASSWORD") != "" && environment("APPLE_TEAM_ID") != "":
		return []string{"--apple-id", environment("APPLE_ID"), "--password", environment("APPLE_PASSWORD"), "--team-id", environment("APPLE_TEAM_ID")}
	}
	return nil
}

// importCertificate puts a base64 .p12 into a keychain of its own, in a scratch
// directory, and names the identity in it. The keychain is added to the user's
// search list while the build runs, because codesign resolves an identity there
// and only uses --keychain to reach its key; cleanup puts the old list back and
// deletes the keychain.
func importCertificate(run toolRunner, encoded, password string) (keychain, identity string, cleanup func(), err error) {
	cleanup = func() {}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", "", cleanup, errors.New("APPLE_CERTIFICATE is not base64")
	}
	dir, err := os.MkdirTemp("", "viva-sign-")
	if err != nil {
		return "", "", cleanup, err
	}
	keychain = filepath.Join(dir, "viva-signing.keychain-db")
	cleanup = func() {
		_, _ = run("security", "delete-keychain", keychain)
		_ = os.RemoveAll(dir)
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	pass := hex.EncodeToString(secret)
	file := filepath.Join(dir, "certificate.p12")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	steps := [][]string{
		{"create-keychain", "-p", pass, keychain},
		{"set-keychain-settings", "-lut", "3600", keychain},
		{"unlock-keychain", "-p", pass, keychain},
		{"import", file, "-k", keychain, "-P", password, "-T", "/usr/bin/codesign", "-T", "/usr/bin/security"},
		{"set-key-partition-list", "-S", "apple-tool:,apple:,codesign:", "-s", "-k", pass, keychain},
	}
	for _, step := range steps {
		if _, err := run("security", step...); err != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("importing APPLE_CERTIFICATE: %w", err)
		}
	}
	_ = os.Remove(file)
	// codesign looks the identity up in the keychain search list, so the scratch
	// keychain has to be in it for the build; cleanup restores the list before it
	// deletes the keychain.
	before, err := run("security", "list-keychains", "-d", "user")
	if err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("reading the keychain search list: %w", err)
	}
	listed := keychainPaths(before)
	if _, err := run("security", append([]string{"list-keychains", "-d", "user", "-s", keychain}, listed...)...); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("adding the keychain to the search list: %w", err)
	}
	cleanup = func() {
		if len(listed) > 0 {
			_, _ = run("security", append([]string{"list-keychains", "-d", "user", "-s"}, listed...)...)
		}
		_, _ = run("security", "delete-keychain", keychain)
		_ = os.RemoveAll(dir)
	}
	listing, err := run("security", "find-identity", "-v", "-p", "codesigning", keychain)
	if err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	identity = firstIdentity(listing)
	if identity == "" {
		reason := "APPLE_CERTIFICATE holds no code signing identity: the .p12 needs the certificate and its private key"
		if all, _ := run("security", "find-identity", "-p", "codesigning", keychain); firstIdentity(all) != "" {
			reason = "the certificate in APPLE_CERTIFICATE is not trusted: it is self-signed, expired, or its chain to Apple's root is missing (a Developer ID Application certificate with its private key is what is needed)"
		}
		cleanup()
		return "", "", func() {}, errors.New(reason)
	}
	return keychain, identity, cleanup, nil
}

// keychainPaths pulls the quoted paths out of a `security list-keychains` listing.
func keychainPaths(listing string) []string {
	var paths []string
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimSpace(line)
		if len(line) >= 2 && line[0] == '"' && line[len(line)-1] == '"' {
			paths = append(paths, line[1:len(line)-1])
		}
	}
	return paths
}

var identityLine = regexp.MustCompile(`^\s*\d+\)\s+([0-9A-Fa-f]{40})\s+"`)

// firstIdentity is the hash of the first identity `security find-identity` lists.
func firstIdentity(listing string) string {
	for _, line := range strings.Split(listing, "\n") {
		if match := identityLine.FindStringSubmatch(line); match != nil {
			return match[1]
		}
	}
	return ""
}

// signPath signs a bundle (with the hardened runtime and entitlements) or a disk
// image (plainly) in place.
func (signing macSigning) signPath(run toolRunner, path string, bundle bool) error {
	arguments := []string{"--force"}
	if signing.adHoc() {
		arguments = append(arguments, "--timestamp=none")
	} else {
		arguments = append(arguments, "--timestamp")
	}
	if bundle {
		arguments = append(arguments, "--options", "runtime")
		if signing.entitlements != "" {
			arguments = append(arguments, "--entitlements", signing.entitlements)
		}
	}
	if signing.keychain != "" {
		arguments = append(arguments, "--keychain", signing.keychain)
	}
	arguments = append(arguments, "--sign", signing.identity, path)
	if _, err := run("codesign", arguments...); err != nil {
		return err
	}
	if bundle {
		_, err := run("codesign", "--verify", "--strict", "--deep", path)
		return err
	}
	_, err := run("codesign", "--verify", "--strict", path)
	return err
}

// notarization is the answer of `notarytool submit --wait`.
type notarization struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// submit sends a file (a disk image or a zip) to Apple and waits. A rejection
// is returned with the reasons Apple gives.
func (signing macSigning) submit(run toolRunner, path string) (string, error) {
	arguments := append([]string{"notarytool", "submit", path, "--wait", "--output-format", "json"}, signing.notary...)
	output, err := run("xcrun", arguments...)
	result := parseNotarization(output)
	if result.Status == "Accepted" {
		return result.ID, nil
	}
	if result.ID != "" {
		log, _ := run("xcrun", append([]string{"notarytool", "log", result.ID}, signing.notary...)...)
		return result.ID, fmt.Errorf("Apple did not accept %s (status %q, submission %s):\n%s", filepath.Base(path), result.Status, result.ID, firstLines(log, 20))
	}
	if err != nil {
		return "", fmt.Errorf("submitting %s for notarization: %w", filepath.Base(path), err)
	}
	return "", fmt.Errorf("notarytool answered something unexpected: %s", firstLines(output, 4))
}

// parseNotarization finds the JSON object in notarytool's output, which may be
// preceded by progress text.
func parseNotarization(output string) notarization {
	var result notarization
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start < 0 || end < start {
		return result
	}
	_ = json.Unmarshal([]byte(output[start:end+1]), &result)
	return result
}

// stapleAttempts and stapleWait bound the retries of a stapler call that lost its
// connection to Apple. The wait grows with each attempt, and a test replaces it.
var (
	stapleAttempts = 5
	stapleWait     = func(attempt int) { time.Sleep(time.Duration(attempt) * 10 * time.Second) }
)

// staple attaches the notarization ticket to a bundle or disk image, so it opens
// on a Mac that is offline, and checks it took.
func staple(run toolRunner, path string) error {
	if err := retryStapler(run, "staple", path); err != nil {
		return err
	}
	return retryStapler(run, "validate", path)
}

// retryStapler runs one stapler action. Both actions fetch the ticket from Apple
// over the network, and a CI runner's connection to it drops now and then; that
// failure is retried a few times. Any other failure, such as a bundle Apple has
// not notarized, is final.
func retryStapler(run toolRunner, action, path string) error {
	var err error
	for attempt := 1; attempt <= stapleAttempts; attempt++ {
		if _, err = run("xcrun", "stapler", action, path); err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "NSURLErrorDomain") || attempt == stapleAttempts {
			return err
		}
		stapleWait(attempt)
	}
	return err
}

// report writes a line of the package summary.
func reportSigning(output io.Writer, label, text string) {
	fmt.Fprintf(output, "  %-8s %s\n", label, text)
}
