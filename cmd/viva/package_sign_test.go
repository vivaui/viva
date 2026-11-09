package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func signingFixture(t *testing.T) string {
	t.Helper()
	dir := fixture(t, packageTOML)
	writeIcon(t, filepath.Join(dir, "icon.png"), 512)
	return dir
}

func environment(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestSigningIsOffUnlessAnIdentityIsGiven(t *testing.T) {
	none, err := resolveSigning(signingOptions{}, environment(nil), "")
	if err != nil || none.signs() || none.notarize {
		t.Fatalf("nothing set: %+v, %v", none, err)
	}
	// Credentials alone sign nothing and notarize nothing.
	credentials := map[string]string{"APPLE_NOTARY_PROFILE": "release"}
	if got, err := resolveSigning(signingOptions{}, environment(credentials), ""); err != nil || got.signs() || got.notarize {
		t.Fatalf("credentials without an identity: %+v, %v", got, err)
	}
	if got, err := resolveSigning(signingOptions{skip: true}, environment(map[string]string{"APPLE_SIGNING_IDENTITY": "Developer ID Application: X"}), ""); err != nil || got.signs() {
		t.Fatalf("--no-sign still signs: %+v, %v", got, err)
	}
	if _, err := resolveSigning(signingOptions{skip: true, notarize: true}, environment(nil), ""); err == nil {
		t.Fatal("--no-sign with --notarize was accepted")
	}
}

func TestAnIdentityFromTheFlagBeatsTheEnvironmentAndNotarizationFollowsTheCredentials(t *testing.T) {
	env := environment(map[string]string{"APPLE_SIGNING_IDENTITY": "from env", "APPLE_API_KEY": "/k.p8", "APPLE_API_KEY_ID": "ID", "APPLE_API_ISSUER": "ISS"})
	got, err := resolveSigning(signingOptions{identity: "from flag"}, env, "manifest.plist")
	if err != nil || got.identity != "from flag" || !got.notarize || got.entitlements != "manifest.plist" {
		t.Fatalf("%+v, %v", got, err)
	}
	if strings.Join(got.notary, " ") != "--key /k.p8 --key-id ID --issuer ISS" {
		t.Fatalf("notary arguments %q", got.notary)
	}
	if got, _ := resolveSigning(signingOptions{entitlements: "flag.plist"}, env, "manifest.plist"); got.entitlements != "flag.plist" {
		t.Fatalf("entitlements = %q, want the flag's", got.entitlements)
	}
	// Each way of authenticating is recognized, a profile first.
	for want, values := range map[string]map[string]string{
		"--keychain-profile p":                  {"APPLE_NOTARY_PROFILE": "p", "APPLE_ID": "a", "APPLE_PASSWORD": "b", "APPLE_TEAM_ID": "c"},
		"--apple-id a --password b --team-id c": {"APPLE_ID": "a", "APPLE_PASSWORD": "b", "APPLE_TEAM_ID": "c"},
	} {
		if got := strings.Join(notaryCredentials(environment(values)), " "); got != want {
			t.Errorf("credentials = %q, want %q", got, want)
		}
	}
	if notaryCredentials(environment(map[string]string{"APPLE_ID": "a", "APPLE_PASSWORD": "b"})) != nil {
		t.Error("an Apple ID without a team counted as credentials")
	}
}

func TestAskingToNotarizeNeedsADeveloperIDAndCredentials(t *testing.T) {
	credentials := map[string]string{"APPLE_NOTARY_PROFILE": "p"}
	for name, test := range map[string]struct {
		options signingOptions
		env     map[string]string
		want    string
	}{
		"no identity":    {signingOptions{notarize: true}, credentials, "Developer ID identity"},
		"ad hoc":         {signingOptions{notarize: true, identity: "-"}, credentials, "ad hoc signature cannot be notarized"},
		"no credentials": {signingOptions{notarize: true, identity: "Developer ID Application: X"}, nil, "APPLE_NOTARY_PROFILE"},
	} {
		if _, err := resolveSigning(test.options, environment(test.env), ""); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: error = %v, want it to contain %q", name, err, test.want)
		}
	}
	if got, _ := resolveSigning(signingOptions{identity: "-"}, environment(credentials), ""); got.notarize {
		t.Error("an ad hoc signature was sent for notarization")
	}
}

// recorder stands in for codesign, notarytool and the rest: it records each
// command and answers what the real ones would for a package Apple accepts.
type recorder struct {
	commands []string
	status   string
	failOn   string
}

func (r *recorder) run(name string, arguments ...string) (string, error) {
	line := name + " " + strings.Join(arguments, " ")
	r.commands = append(r.commands, line)
	if r.failOn != "" && strings.Contains(line, r.failOn) {
		return "", errors.New("failed")
	}
	switch {
	case strings.Contains(line, "notarytool submit"):
		status := r.status
		if status == "" {
			status = "Accepted"
		}
		return "Conducting pre-submission checks\n{\"id\":\"abc-123\",\"status\":\"" + status + "\"}", nil
	case strings.Contains(line, "notarytool log"):
		return `{"issues":[{"message":"The binary is not signed with a valid Developer ID certificate."}]}`, nil
	case strings.Contains(line, "find-identity"):
		return "  1) 0123456789ABCDEF0123456789ABCDEF01234567 \"Developer ID Application: Example (TEAM1234)\"\n     1 valid identities found", nil
	case strings.Contains(line, "list-keychains") && !strings.Contains(line, " -s "):
		return "    \"/Users/example/Library/Keychains/login.keychain-db\"\n", nil
	}
	return "", nil
}

func (r *recorder) index(prefix string) int {
	for i, command := range r.commands {
		if strings.HasPrefix(command, prefix) {
			return i
		}
	}
	return -1
}

func useRecorder(t *testing.T, r *recorder) {
	t.Helper()
	previous := runSigningTool
	runSigningTool = r.run
	t.Cleanup(func() { runSigningTool = previous })
}

// releaseEnvironment sets up a machine that signs and notarizes; it comes after
// the fixture, which clears the signing variables.
func releaseEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APPLE_SIGNING_IDENTITY", "Developer ID Application: Example (TEAM1234)")
	t.Setenv("APPLE_NOTARY_PROFILE", "release")
}

func TestADiskImageIsSignedNotarizedAndStapledOnce(t *testing.T) {
	requireMac(t)
	dir := signingFixture(t)
	releaseEnvironment(t)
	r := &recorder{}
	useRecorder(t, r)
	var output bytes.Buffer
	if err := runCLI([]string{"package", "-C", dir, "--format", "app,dmg"}, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	order := []string{
		"codesign --force --timestamp --options runtime --sign Developer ID Application: Example (TEAM1234) ",
		"codesign --verify --strict --deep ",
		"codesign --force --timestamp --sign Developer ID Application: Example (TEAM1234) ",
		"xcrun notarytool submit ",
		"xcrun stapler staple ",
		"xcrun stapler validate ",
	}
	last := -1
	for _, prefix := range order {
		found := -1
		for i, command := range r.commands {
			if i > last && strings.HasPrefix(command, prefix) {
				found = i
				break
			}
		}
		if found < 0 {
			t.Fatalf("no %q after command %d in:\n%s", prefix, last, strings.Join(r.commands, "\n"))
		}
		last = found
	}
	submits := 0
	for _, command := range r.commands {
		if strings.HasPrefix(command, "xcrun notarytool submit") {
			submits++
			if !strings.HasSuffix(command, "--keychain-profile release") {
				t.Errorf("submission = %q", command)
			}
		}
	}
	// Nothing is zipped here, so the image is the only thing Apple has to judge.
	if submits != 1 || !strings.Contains(r.commands[r.index("xcrun notarytool submit")], ".dmg ") {
		t.Fatalf("%d submissions, want the image alone:\n%s", submits, strings.Join(r.commands, "\n"))
	}
	for _, want := range []string{"  signing  signed with the hardened runtime", "  notarized accepted by Apple (submission abc-123), ticket stapled to ", ".dmg"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, output.String())
		}
	}
	if strings.Contains(output.String(), "signing  none") {
		t.Errorf("the report still says nothing is signed:\n%s", output.String())
	}
}

// A zip is what the in-app updater downloads, so the bundle inside it has to
// carry its own ticket: without one the system asks Apple at every launch, and
// refuses the update wherever that lookup cannot be made.
func TestAZippedBundleIsNotarizedAndStapledBeforeTheImageIsMade(t *testing.T) {
	requireMac(t)
	dir := signingFixture(t)
	releaseEnvironment(t)
	r := &recorder{}
	useRecorder(t, r)
	var output bytes.Buffer
	if err := runCLI([]string{"package", "-C", dir, "--format", "app,zip,dmg"}, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	var submitted []string
	for _, command := range r.commands {
		if strings.HasPrefix(command, "xcrun notarytool submit") {
			submitted = append(submitted, command)
		}
	}
	if len(submitted) != 2 || !strings.Contains(submitted[0], ".app.zip ") || !strings.Contains(submitted[1], ".dmg ") {
		t.Fatalf("submissions %q\n%s", submitted, strings.Join(r.commands, "\n"))
	}
	// The bundle's ticket is stapled before the zip is written, so the zip holds
	// the stapled bundle; the image's own ticket follows the image.
	staple := r.index("xcrun stapler staple")
	if staple < 0 || !strings.Contains(r.commands[staple], ".app") {
		t.Fatalf("the bundle was not the first thing stapled:\n%s", strings.Join(r.commands, "\n"))
	}
	if !strings.Contains(output.String(), ".dmg") {
		t.Errorf("the report does not name the image:\n%s", output.String())
	}
}

func TestABundleWithoutADiskImageIsNotarizedByAZipAndStapledBeforeItIsZipped(t *testing.T) {
	requireMac(t)
	dir := signingFixture(t)
	releaseEnvironment(t)
	r := &recorder{}
	useRecorder(t, r)
	if err := runCLI([]string{"package", "-C", dir, "--format", "app,zip"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	submit, staple := r.index("xcrun notarytool submit"), r.index("xcrun stapler staple")
	if submit < 0 || staple < submit || !strings.Contains(r.commands[submit], ".app.zip ") || !strings.Contains(r.commands[staple], ".app") {
		t.Fatalf("commands:\n%s", strings.Join(r.commands, "\n"))
	}
	if r.index("codesign --force --timestamp --sign") >= 0 {
		t.Error("a disk image was signed though none was made")
	}
}

func TestARejectedNotarizationFailsThePackageWithApplesReasons(t *testing.T) {
	requireMac(t)
	dir := signingFixture(t)
	releaseEnvironment(t)
	r := &recorder{status: "Invalid"}
	useRecorder(t, r)
	err := runCLI([]string{"package", "-C", dir, "--format", "dmg"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "not signed with a valid Developer ID") || !strings.Contains(err.Error(), "abc-123") {
		t.Fatalf("error = %v", err)
	}
	if r.index("xcrun stapler staple") >= 0 {
		t.Error("a ticket was stapled to a rejected image")
	}
}

func TestNoSignLeavesAMachineThatIsSetUpToSignAlone(t *testing.T) {
	requireMac(t)
	dir := signingFixture(t)
	releaseEnvironment(t)
	r := &recorder{}
	useRecorder(t, r)
	var output bytes.Buffer
	if err := runCLI([]string{"package", "-C", dir, "--no-sign"}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(r.commands) != 0 || !strings.Contains(output.String(), "signing  none") {
		t.Fatalf("commands %v\n%s", r.commands, output.String())
	}
}

func TestAnAdHocSignatureIsRealAndCarriesTheEntitlements(t *testing.T) {
	requireMac(t)
	dir := signingFixture(t)
	entitlements := filepath.Join(dir, "app.entitlements")
	if err := os.WriteFile(entitlements, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>com.apple.security.network.client</key><true/></dict></plist>
`), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runCLI([]string{"package", "-C", dir, "--sign-identity", "-", "--entitlements", entitlements}, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	app := filepath.Join(dir, "build", "Fixture Notes.app")
	if out, err := exec.Command("codesign", "--verify", "--strict", "--deep", app).CombinedOutput(); err != nil {
		t.Fatalf("the signature does not verify: %v\n%s", err, out)
	}
	details, _ := exec.Command("codesign", "-d", "--verbose=2", app).CombinedOutput()
	if !strings.Contains(string(details), "Signature=adhoc") || !strings.Contains(string(details), "runtime") {
		t.Fatalf("details:\n%s", details)
	}
	granted, _ := exec.Command("codesign", "-d", "--entitlements", ":-", app).CombinedOutput()
	if !strings.Contains(string(granted), "com.apple.security.network.client") {
		t.Fatalf("entitlements:\n%s", granted)
	}
	if !strings.Contains(output.String(), "ad hoc: only this Mac accepts the signature") {
		t.Fatalf("the report does not say the signature is ad hoc:\n%s", output.String())
	}
}

func TestACertificateIsImportedIntoAKeychainOfItsOwnAndRemoved(t *testing.T) {
	r := &recorder{}
	keychain, identity, cleanup, err := importCertificate(r.run, "UEsDBA==", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if identity != "0123456789ABCDEF0123456789ABCDEF01234567" || !strings.HasSuffix(keychain, "viva-signing.keychain-db") {
		t.Fatalf("identity %q, keychain %q", identity, keychain)
	}
	if r.index("security list-keychains -d user -s "+keychain) < 0 {
		t.Errorf("the keychain was not added to the search list:\n%s", strings.Join(r.commands, "\n"))
	}
	if r.index("security import ") < 0 || r.index("security set-key-partition-list ") < 0 {
		t.Fatalf("commands:\n%s", strings.Join(r.commands, "\n"))
	}
	cleanup()
	if r.index("security delete-keychain ") < 0 {
		t.Error("the keychain was left behind")
	}
	if r.index("security list-keychains -d user -s /Users/example/Library/Keychains/login.keychain-db") < 0 {
		t.Errorf("the search list was not restored:\n%s", strings.Join(r.commands, "\n"))
	}
	if _, err := os.Stat(filepath.Dir(keychain)); !os.IsNotExist(err) {
		t.Error("the scratch directory was left behind")
	}

	if _, _, _, err := importCertificate(r.run, "not base64!", "pw"); err == nil {
		t.Error("garbage was imported")
	}
	failing := &recorder{failOn: "security import"}
	if _, _, _, err := importCertificate(failing.run, "UEsDBA==", "pw"); err == nil || failing.index("security delete-keychain ") < 0 {
		t.Errorf("a failed import: error %v, commands %v", err, failing.commands)
	}
}

func TestStaplerRetriesALostConnectionButNotARefusal(t *testing.T) {
	waits := 0
	previousWait, previousAttempts := stapleWait, stapleAttempts
	stapleWait = func(int) { waits++ }
	t.Cleanup(func() { stapleWait, stapleAttempts = previousWait, previousAttempts })

	lost := errors.New(`xcrun failed: exit status 68: error is Error Domain=NSURLErrorDomain Code=-1005 "The network connection was lost."`)
	calls := 0
	flaky := func(name string, arguments ...string) (string, error) {
		calls++
		if calls <= 2 {
			return "", lost
		}
		return "", nil
	}
	if err := staple(flaky, "App.dmg"); err != nil {
		t.Fatalf("a connection lost twice was not retried through: %v", err)
	}
	if waits != 2 {
		t.Fatalf("waited %d times, want 2", waits)
	}

	calls, waits = 0, 0
	refused := func(string, ...string) (string, error) {
		calls++
		return "", errors.New("xcrun failed: exit status 65: not notarized")
	}
	if err := staple(refused, "App.dmg"); err == nil || calls != 1 || waits != 0 {
		t.Fatalf("a refusal was retried: err=%v calls=%d waits=%d", err, calls, waits)
	}

	calls, stapleAttempts = 0, 3
	always := func(string, ...string) (string, error) { calls++; return "", lost }
	if err := staple(always, "App.dmg"); err == nil || calls != 3 {
		t.Fatalf("retries are not bounded: err=%v calls=%d", err, calls)
	}
}
