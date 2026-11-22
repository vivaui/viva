package main

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Releases are ordered by a number: a later version is never a smaller one, and a
// version the tool cannot read still installs.
func TestVersionCodeOrdersReleases(t *testing.T) {
	order := []string{"0.0.1", "0.1.0", "0.9.9", "1.0.0", "1.0.1", "1.2.0", "1.10.0", "2.0.0"}
	last := 0
	for _, version := range order {
		code := versionCode(version)
		if code <= last {
			t.Errorf("%s has code %d, not above %d", version, code, last)
		}
		last = code
	}
	if code := versionCode("1.2.3-beta.4"); code != versionCode("1.2.3") {
		t.Errorf("a pre-release of 1.2.3 has code %d, 1.2.3 has %d", code, versionCode("1.2.3"))
	}
	if versionCode("") < 1 || versionCode("preview") < 1 {
		t.Error("an unreadable version has no valid code")
	}
}

func TestAndroidManifestNamesTheApplicationAndTheHost(t *testing.T) {
	data := androidManifest("dev.example.notes", "Notes & <Co>", "2.0.1")
	var parsed struct {
		Package     string `xml:"package,attr"`
		VersionName string `xml:"versionName,attr"`
		Application struct {
			Label    string `xml:"label,attr"`
			Activity struct {
				Name string `xml:"name,attr"`
			} `xml:"activity"`
		} `xml:"application"`
	}
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("the manifest is not XML: %v\n%s", err, data)
	}
	if parsed.Package != "dev.example.notes" || parsed.VersionName != "2.0.1" {
		t.Errorf("package %q version %q", parsed.Package, parsed.VersionName)
	}
	if parsed.Application.Label != "Notes & <Co>" {
		t.Errorf("label %q lost its characters", parsed.Application.Label)
	}
	if parsed.Application.Activity.Name != androidActivity {
		t.Errorf("the launcher activity is %q, want the host's", parsed.Application.Activity.Name)
	}
	if !strings.Contains(string(data), "android.hardware.vulkan") {
		t.Error("the manifest does not ask for Vulkan, which the program draws with")
	}
}

// fakeSDK lays out the parts of an SDK the finder looks at.
func fakeSDK(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	touch := func(parts ...string) {
		path := filepath.Join(append([]string{home}, parts...)...)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, version := range []string{"9.0.0", "34.0.0", "35.0.0"} {
		touch("build-tools", version, executable("aapt2"))
	}
	for _, platform := range []string{"android-9", "android-34", "android-35"} {
		touch("platforms", platform, "android.jar")
	}
	os.MkdirAll(filepath.Join(home, "platforms", "android-36"), 0o755) // no jar: not a platform
	for _, ndk := range []string{"27.1.0", "28.2.0"} {
		os.MkdirAll(filepath.Join(home, "ndk", ndk, "toolchains", "llvm", "prebuilt", "host", "bin"), 0o755)
	}
	touch("platform-tools", executable("adb"))
	jdk := t.TempDir()
	touch2 := filepath.Join(jdk, "bin")
	os.MkdirAll(touch2, 0o755)
	for _, tool := range []string{"javac", "java"} {
		os.WriteFile(filepath.Join(touch2, tool), nil, 0o755)
	}
	t.Setenv("JAVA_HOME", jdk)
	t.Setenv("ANDROID_HOME", home)
	t.Setenv("ANDROID_SDK_ROOT", "")
	t.Setenv("ANDROID_NDK_HOME", "")
	return home
}

// The newest of what is installed is used, by version and not by name ("9" sorts
// after "35"), and a platform too old to run on, or without its jar, is not one.
func TestFindAndroidSDKTakesTheNewestOfEachTool(t *testing.T) {
	home := fakeSDK(t)
	sdk, err := findAndroidSDK()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(sdk.buildTools) != "35.0.0" {
		t.Errorf("build-tools %s, want 35.0.0", sdk.buildTools)
	}
	if filepath.Base(filepath.Dir(sdk.androidJar)) != "android-35" {
		t.Errorf("platform %s, want android-35", sdk.androidJar)
	}
	if filepath.Base(sdk.ndk) != "28.2.0" {
		t.Errorf("ndk %s, want 28.2.0", sdk.ndk)
	}
	if !strings.HasPrefix(sdk.clangDir, home) || filepath.Base(sdk.clangDir) != "bin" {
		t.Errorf("the NDK's compiler directory is %s", sdk.clangDir)
	}
}

func TestFindAndroidSDKSaysWhatToInstall(t *testing.T) {
	t.Setenv("ANDROID_HOME", t.TempDir())
	t.Setenv("ANDROID_SDK_ROOT", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	if _, err := findAndroidSDK(); err == nil {
		t.Skip("this machine has an Android SDK in a default place")
	} else if !strings.Contains(err.Error(), "sdkmanager") {
		t.Errorf("the error does not say how to install the SDK: %v", err)
	}
}

func TestAndroidEnvironmentUsesTheNDKCompilerOfTheArchitecture(t *testing.T) {
	sdk := androidSDK{clangDir: filepath.Join("ndk", "bin")}
	for _, test := range []struct{ arch, triple string }{{"arm64", "aarch64-linux-android"}, {"amd64", "x86_64-linux-android"}} {
		plan := buildPlan{target: "android", arch: test.arch, sdk: sdk}
		goos, goarch := plan.goTarget()
		if goos != "android" || goarch != test.arch {
			t.Errorf("%s builds for %s/%s", test.arch, goos, goarch)
		}
		environment := plan.environment(nil)
		if lastEnvironment(environment, "CGO_ENABLED") != "1" {
			t.Errorf("%s: cgo is off", test.arch)
		}
		compiler := filepath.Base(lastEnvironment(environment, "CC"))
		if !strings.HasPrefix(compiler, test.triple) || !strings.Contains(compiler, "26-clang") {
			t.Errorf("%s: compiler %q", test.arch, compiler)
		}
	}
	if checkAndroid(buildFlags{target: "android", cgo: true}) == nil {
		t.Error("--cgo was accepted for Android, which always uses cgo")
	}
}

func TestAndroidOutputIsAnAPKOfItsOwn(t *testing.T) {
	dir := fixture(t, notesTOML)
	plan := buildPlan{manifest: loadPlan(t, dir, buildFlags{target: "linux"}, "").manifest, target: "android", arch: "arm64", executable: "notes"}
	if err := plan.outputPath(""); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(plan.out, "notes-android.apk") {
		t.Errorf("output %s", plan.out)
	}
}

func TestChooseDevicePrefersAPhoneToAnEmulator(t *testing.T) {
	listing := "List of devices attached\nemulator-5554\tdevice\nR58M123ABC\tdevice\nR58M999XYZ\tunauthorized\n\n"
	devices := parseADBDevices(listing)
	if len(devices) != 3 {
		t.Fatalf("parsed %v", devices)
	}
	chosen, err := chooseDevice(devices, "")
	if err != nil || chosen.Serial != "R58M123ABC" {
		t.Fatalf("chose %+v, %v; want the phone that is ready", chosen, err)
	}
	if chosen, err := chooseDevice(devices, "emulator-5554"); err != nil || chosen.Serial != "emulator-5554" {
		t.Fatalf("--device chose %+v, %v", chosen, err)
	}
	if _, err := chooseDevice(devices, "R58M999XYZ"); err == nil {
		t.Error("a device that has not been authorized was chosen")
	}
	if _, err := chooseDevice(parseADBDevices("List of devices attached\n"), ""); err == nil {
		t.Error("no devices, and one was chosen")
	}
}

// The package keeps what aapt2 made, adds the program's library stored and not
// compressed (the system maps it from the package), and the dex.
func TestAddToAPKStoresTheLibraryAndKeepsTheBase(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.apk")
	file, err := os.Create(base)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, _ := writer.Create("AndroidManifest.xml")
	entry.Write([]byte("manifest"))
	writer.Close()
	file.Close()
	library := filepath.Join(dir, "libviva.so")
	if err := os.WriteFile(library, []byte("library bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.apk")
	if err := addToAPK(base, out, []byte("dex bytes"), library, "arm64-v8a"); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	contents := map[string]string{}
	methods := map[string]uint16{}
	for _, entry := range reader.File {
		opened, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(opened)
		opened.Close()
		contents[entry.Name], methods[entry.Name] = string(data), entry.Method
	}
	if contents["AndroidManifest.xml"] != "manifest" || contents["classes.dex"] != "dex bytes" {
		t.Errorf("package holds %v", contents)
	}
	const name = "lib/arm64-v8a/libviva.so"
	if contents[name] != "library bytes" || methods[name] != zip.Store {
		t.Errorf("library entry %q method %d, want it stored", contents[name], methods[name])
	}
}
