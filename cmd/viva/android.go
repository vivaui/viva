package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/internal/platform/android/hostsrc"
	"github.com/vivaui/viva/tools/manifest"
)

// An Android build is the program as a shared library (the Go toolchain with the
// NDK's C compiler, which purego and the JNI bridge need), the Java of the host
// (hostsrc) compiled with javac and d8, and an APK assembled from them with aapt2,
// zipalign and apksigner: the tools of the Android SDK and the JDK, with no Gradle.

const (
	// androidMinSDK is the oldest Android an application runs on: 8.0, which has the
	// Vulkan 1.1 and the text APIs the host uses.
	androidMinSDK = 26
	// androidTargetSDK is the Android the application is built for.
	androidTargetSDK = 35
	// androidActivity is the host's Activity, the launcher entry.
	androidActivity = "dev.viva.host.VivaActivity"
)

// isAndroid reports whether the plan builds an Android package.
func (plan buildPlan) isAndroid() bool { return plan.target == "android" }

// androidABI is the name Android gives an architecture.
func androidABI(arch string) string {
	if arch == "amd64" {
		return "x86_64"
	}
	return "arm64-v8a"
}

// androidSDK is where the Android tools are.
type androidSDK struct {
	home          string
	ndk           string
	buildTools    string
	androidJar    string
	javac, java   string
	clangDir      string
	adb, emulator string
}

// lookEnv is os.Getenv; tests replace it.
var lookEnv = os.Getenv

// findAndroidSDK finds the SDK, the NDK and the JDK, and says what to install when
// one is missing.
func findAndroidSDK() (androidSDK, error) {
	sdk := androidSDK{}
	var candidates []string
	for _, name := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if value := lookEnv(name); value != "" {
			candidates = append(candidates, value)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, "Library", "Android", "sdk"), // macOS, Android Studio
			filepath.Join(home, "Android", "Sdk"),            // Linux, Android Studio
			filepath.Join(home, "AppData", "Local", "Android", "Sdk"))
	}
	candidates = append(candidates, "/opt/homebrew/share/android-commandlinetools", "/usr/local/share/android-commandlinetools")
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "build-tools")); err == nil && info.IsDir() {
			sdk.home = candidate
			break
		}
	}
	if sdk.home == "" {
		return sdk, errors.New("the Android SDK was not found; install it with 'brew install --cask android-commandlinetools' " +
			"(or Android Studio) and set ANDROID_HOME, then 'sdkmanager \"platform-tools\" \"platforms;android-35\" \"build-tools;35.0.0\" \"ndk;28.2.13676358\"'")
	}
	var err error
	if sdk.buildTools, err = newestChild(filepath.Join(sdk.home, "build-tools"), func(name string) bool {
		return fileExists(filepath.Join(sdk.home, "build-tools", name, executable("aapt2")))
	}); err != nil {
		return sdk, fmt.Errorf("no Android build-tools in %s; run sdkmanager \"build-tools;35.0.0\"", sdk.home)
	}
	sdk.buildTools = filepath.Join(sdk.home, "build-tools", sdk.buildTools)
	platform, err := newestPlatform(filepath.Join(sdk.home, "platforms"))
	if err != nil {
		return sdk, fmt.Errorf("no Android platform (android-%d or newer) in %s; run sdkmanager \"platforms;android-%d\"", androidMinSDK, sdk.home, androidTargetSDK)
	}
	sdk.androidJar = filepath.Join(sdk.home, "platforms", platform, "android.jar")
	if ndk := lookEnv("ANDROID_NDK_HOME"); ndk != "" && fileExists(ndk) {
		sdk.ndk = ndk
	} else if name, err := newestChild(filepath.Join(sdk.home, "ndk"), func(string) bool { return true }); err == nil {
		sdk.ndk = filepath.Join(sdk.home, "ndk", name)
	} else {
		return sdk, fmt.Errorf("no Android NDK in %s; run sdkmanager \"ndk;28.2.13676358\"", sdk.home)
	}
	prebuilt, err := fs.Glob(os.DirFS(sdk.ndk), "toolchains/llvm/prebuilt/*")
	if err != nil || len(prebuilt) == 0 {
		return sdk, fmt.Errorf("the NDK in %s has no LLVM toolchain", sdk.ndk)
	}
	sdk.clangDir = filepath.Join(sdk.ndk, filepath.FromSlash(prebuilt[0]), "bin")
	if sdk.javac, err = findJava("javac"); err != nil {
		return sdk, err
	}
	if sdk.java, err = findJava("java"); err != nil {
		return sdk, err
	}
	sdk.adb = filepath.Join(sdk.home, "platform-tools", executable("adb"))
	if !fileExists(sdk.adb) {
		if found, err := exec.LookPath("adb"); err == nil {
			sdk.adb = found
		}
	}
	sdk.emulator = filepath.Join(sdk.home, "emulator", executable("emulator"))
	return sdk, nil
}

func executable(name string) string {
	if runtime.GOOS == "windows" {
		switch name {
		case "aapt2", "adb", "emulator":
			return name + ".exe"
		case "zipalign":
			return name + ".exe"
		case "apksigner", "d8":
			return name + ".bat"
		}
	}
	return name
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

// newestChild is the child of dir with the highest version in its name that ok accepts.
func newestChild(dir string, ok func(string) bool) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && ok(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("nothing in %s", dir)
	}
	sort.Slice(names, func(i, j int) bool { return versionLess(versionNumbers(names[i]), versionNumbers(names[j])) })
	return names[len(names)-1], nil
}

var digits = regexp.MustCompile(`\d+`)

// versionNumbers are the numbers of a name such as 35.0.0 or android-35, in order.
func versionNumbers(name string) []int {
	var numbers []int
	for _, part := range digits.FindAllString(name, -1) {
		number, _ := strconv.Atoi(part)
		numbers = append(numbers, number)
	}
	return numbers
}

// newestPlatform is the newest platforms/android-N with N at least the minimum SDK and
// an android.jar in it.
func newestPlatform(dir string) (string, error) {
	return newestChild(dir, func(name string) bool {
		numbers := versionNumbers(name)
		return strings.HasPrefix(name, "android-") && len(numbers) > 0 && numbers[0] >= androidMinSDK &&
			fileExists(filepath.Join(dir, name, "android.jar"))
	})
}

// findJava finds a JDK tool: under JAVA_HOME, else on the path.
func findJava(tool string) (string, error) {
	if home := lookEnv("JAVA_HOME"); home != "" {
		if candidate := filepath.Join(home, "bin", tool); fileExists(candidate) || fileExists(candidate+".exe") {
			return candidate, nil
		}
	}
	found, err := exec.LookPath(tool)
	if err != nil {
		return "", fmt.Errorf("%s was not found: install a JDK (17 or newer) and put it on the path or set JAVA_HOME", tool)
	}
	return found, nil
}

// clang is the NDK's C compiler for an architecture and the minimum SDK.
func (sdk androidSDK) clang(arch string) string {
	triple := "aarch64-linux-android"
	if arch == "amd64" {
		triple = "x86_64-linux-android"
	}
	name := fmt.Sprintf("%s%d-clang", triple, androidMinSDK)
	if runtime.GOOS == "windows" {
		name += ".cmd"
	}
	return filepath.Join(sdk.clangDir, name)
}

// androidEnvironment is what go build needs for Android: cgo with the NDK's compiler.
func (plan buildPlan) androidEnvironment(environment []string) []string {
	return append(environment, "CGO_ENABLED=1", "CC="+plan.sdk.clang(plan.arch))
}

// checkAndroid reports what a flag combination cannot build.
func checkAndroid(shared buildFlags) error {
	if shared.cgo {
		return errors.New("--cgo is for Linux and Windows; Android builds always use cgo")
	}
	return nil
}

// androidPackageID is the application's identifier on a device.
func (plan buildPlan) androidPackageID() string {
	if id := plan.manifest.App.ID; id != "" {
		return id
	}
	return "dev.viva." + strings.ReplaceAll(manifest.Slug(plan.executable), "-", "_")
}

// versionCode is the integer Android orders releases by: major, minor and patch in
// three digits each, so 1.2.3 is 1002003. A version it cannot read is 1.
func versionCode(version string) int {
	numbers := versionNumbers(strings.SplitN(version, "-", 2)[0])
	if len(numbers) == 0 {
		return 1
	}
	code := 0
	for index := 0; index < 3; index++ {
		part := 0
		if index < len(numbers) {
			part = min(numbers[index], 999)
		}
		code = code*1000 + part
	}
	return max(code, 1)
}

// androidManifest is AndroidManifest.xml: one activity, the host's, which the
// application's window is.
func androidManifest(id, name, version string) []byte {
	var escaped bytes.Buffer
	xml.EscapeText(&escaped, []byte(name))
	return []byte(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="` + id + `" android:versionCode="` + strconv.Itoa(versionCode(version)) + `" android:versionName="` + version + `">
    <uses-sdk android:minSdkVersion="` + strconv.Itoa(androidMinSDK) + `" android:targetSdkVersion="` + strconv.Itoa(androidTargetSDK) + `"/>
    <uses-permission android:name="android.permission.INTERNET"/>
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE"/>
    <uses-feature android:glEsVersion="0x00030000" android:required="false"/>
    <uses-feature android:name="android.hardware.vulkan.version" android:version="0x400003" android:required="true"/>
    <application android:label="` + escaped.String() + `" android:hasCode="true" android:extractNativeLibs="false" android:supportsRtl="true">
        <activity android:name="` + androidActivity + `" android:exported="true"
            android:configChanges="orientation|screenSize|screenLayout|smallestScreenSize|keyboard|keyboardHidden|navigation|density|uiMode|fontScale|layoutDirection|locale|colorMode"
            android:windowSoftInputMode="adjustResize">
            <intent-filter>
                <action android:name="android.intent.action.MAIN"/>
                <category android:name="android.intent.category.LAUNCHER"/>
            </intent-filter>
        </activity>
    </application>
</manifest>
`)
}

// addToAPK writes base (aapt2's package) to out with the dex file and the program's
// library added. The library is stored, not compressed, so the system maps it from the
// package, as zipalign -p then aligns it to a page.
func addToAPK(base, out string, dex []byte, library, abi string) error {
	reader, err := zip.OpenReader(base)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.Create(out)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	for _, entry := range reader.File {
		if err := writer.Copy(entry); err != nil {
			return err
		}
	}
	dexEntry, err := writer.Create("classes.dex")
	if err != nil {
		return err
	}
	if _, err := dexEntry.Write(dex); err != nil {
		return err
	}
	libraryEntry, err := writer.CreateHeader(&zip.FileHeader{Name: path.Join("lib", abi, "libviva.so"), Method: zip.Store, Modified: time.Unix(0, 0)})
	if err != nil {
		return err
	}
	source, err := os.Open(library)
	if err != nil {
		return err
	}
	defer source.Close()
	if _, err := io.Copy(libraryEntry, source); err != nil {
		return err
	}
	return writer.Close()
}

// runCommand runs a tool and puts its output in the error when it fails.
func runCommand(dir string, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Dir = dir
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s: %w\n%s", filepath.Base(name), err, strings.TrimSpace(output.String()))
	}
	return nil
}

// debugKeystore is the keystore a debug build is signed with: the one Android Studio
// makes, created when there is none.
func (sdk androidSDK) debugKeystore() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	keystore := filepath.Join(home, ".android", "debug.keystore")
	if fileExists(keystore) {
		return keystore, nil
	}
	if err := os.MkdirAll(filepath.Dir(keystore), 0o700); err != nil {
		return "", err
	}
	keytool, err := findJava("keytool")
	if err != nil {
		return "", err
	}
	if err := runCommand("", keytool, "-genkeypair", "-keystore", keystore, "-storepass", "android", "-keypass", "android",
		"-alias", "androiddebugkey", "-keyalg", "RSA", "-keysize", "2048", "-validity", "10000",
		"-dname", "CN=Android Debug,O=Android,C=US"); err != nil {
		return "", err
	}
	return keystore, nil
}

// runAndroid builds the program into a library and the package around it.
func (plan buildPlan) runAndroid(errorOutput io.Writer) error {
	sdk, err := findAndroidSDK()
	if err != nil {
		return err
	}
	plan.sdk = sdk
	work, err := os.MkdirTemp("", "viva-android-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	abi := androidABI(plan.arch)

	library := filepath.Join(work, "lib", abi, "libviva.so")
	if err := plan.buildLibrary(library, errorOutput); err != nil {
		return err
	}

	classes := filepath.Join(work, "classes")
	sources, err := extractHost(work)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(classes, 0o755); err != nil {
		return err
	}
	arguments := append([]string{"-source", "8", "-target", "8", "-Xlint:-options", "-nowarn", "-classpath", sdk.androidJar, "-d", classes}, sources...)
	if err := runCommand("", sdk.javac, arguments...); err != nil {
		return err
	}
	var classFiles []string
	filepath.WalkDir(classes, func(name string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(name, ".class") {
			classFiles = append(classFiles, name)
		}
		return nil
	})
	dexDir := filepath.Join(work, "dex")
	if err := os.MkdirAll(dexDir, 0o755); err != nil {
		return err
	}
	if err := runCommand("", filepath.Join(sdk.buildTools, executable("d8")),
		append([]string{"--min-api", strconv.Itoa(androidMinSDK), "--lib", sdk.androidJar, "--output", dexDir}, classFiles...)...); err != nil {
		return err
	}
	dex, err := os.ReadFile(filepath.Join(dexDir, "classes.dex"))
	if err != nil {
		return err
	}

	name := plan.manifest.App.Name
	if name == "" {
		name = plan.executable
	}
	manifestFile := filepath.Join(work, "AndroidManifest.xml")
	if err := os.WriteFile(manifestFile, androidManifest(plan.androidPackageID(), name, plan.manifest.App.Version), 0o644); err != nil {
		return err
	}
	base := filepath.Join(work, "base.apk")
	if err := runCommand("", filepath.Join(sdk.buildTools, executable("aapt2")), "link", "-o", base, "-I", sdk.androidJar,
		"--manifest", manifestFile, "--min-sdk-version", strconv.Itoa(androidMinSDK), "--target-sdk-version", strconv.Itoa(androidTargetSDK)); err != nil {
		return err
	}
	assembled := filepath.Join(work, "assembled.apk")
	if err := addToAPK(base, assembled, dex, library, abi); err != nil {
		return err
	}
	aligned := filepath.Join(work, "aligned.apk")
	if err := runCommand("", filepath.Join(sdk.buildTools, executable("zipalign")), "-f", "-p", "4", assembled, aligned); err != nil {
		return err
	}
	keystore, err := sdk.debugKeystore()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plan.out), 0o755); err != nil {
		return err
	}
	os.Remove(plan.out)
	return runCommand("", filepath.Join(sdk.buildTools, executable("apksigner")), "sign", "--ks", keystore, "--ks-pass", "pass:android",
		"--key-pass", "pass:android", "--out", plan.out, aligned)
}

// buildLibrary compiles the program as the library the host loads.
func (plan buildPlan) buildLibrary(output string, errorOutput io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := requireGo(errorOutput); err != nil {
		return err
	}
	arguments := []string{"build", "-trimpath", "-buildmode=c-shared", "-ldflags", plan.ldflags()}
	if len(plan.tags) > 0 {
		arguments = append(arguments, "-tags", strings.Join(plan.tags, ","))
	}
	arguments = append(arguments, "-o", output, plan.pkg)
	command := exec.Command("go", arguments...)
	command.Dir = plan.manifest.Dir
	command.Env = plan.environment(os.Environ())
	command.Stdout, command.Stderr = errorOutput, errorOutput
	if err := command.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("the go tool was not found in PATH")
		}
		return fmt.Errorf("go build failed: %w", err)
	}
	return nil
}

// extractHost writes the host's Java sources under dir and returns their paths.
func extractHost(dir string) ([]string, error) {
	var sources []string
	err := fs.WalkDir(hostsrc.Java, "java", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := hostsrc.Java.ReadFile(name)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, "host", filepath.FromSlash(strings.TrimPrefix(name, "java/")))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		sources = append(sources, target)
		return os.WriteFile(target, data, 0o644)
	})
	return sources, err
}

// adbDevice is one entry of 'adb devices'.
type adbDevice struct {
	Serial string
	State  string
}

// parseADBDevices reads the output of 'adb devices'.
func parseADBDevices(output string) []adbDevice {
	var devices []adbDevice
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] != "List" && !strings.HasPrefix(fields[0], "*") {
			devices = append(devices, adbDevice{Serial: fields[0], State: fields[1]})
		}
	}
	return devices
}

// chooseDevice picks the device to run on: the one asked for by serial; otherwise the
// only one that is ready, or a real one before an emulator.
func chooseDevice(devices []adbDevice, requested string) (adbDevice, error) {
	var ready []adbDevice
	for _, device := range devices {
		if device.State != "device" {
			continue
		}
		if requested != "" {
			if device.Serial == requested {
				return device, nil
			}
			continue
		}
		ready = append(ready, device)
	}
	if requested != "" {
		return adbDevice{}, fmt.Errorf("no device %q is ready; 'adb devices' lists them", requested)
	}
	if len(ready) == 0 {
		return adbDevice{}, errors.New("no Android device is ready: connect a phone with USB debugging on, or start an emulator")
	}
	sort.SliceStable(ready, func(i, j int) bool {
		return !strings.HasPrefix(ready[i].Serial, "emulator-") && strings.HasPrefix(ready[j].Serial, "emulator-")
	})
	return ready[0], nil
}

// adb runs the Android debug bridge; tests replace it.
var adb = func(sdk androidSDK, serial string, arguments ...string) (string, error) {
	if serial != "" {
		arguments = append([]string{"-s", serial}, arguments...)
	}
	command := exec.Command(sdk.adb, arguments...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return stdout.String(), fmt.Errorf("adb %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// runOnAndroid installs the built package on a device and starts it.
func runOnAndroid(plan buildPlan, requested string, output io.Writer) error {
	sdk, err := findAndroidSDK()
	if err != nil {
		return err
	}
	if !fileExists(sdk.adb) && filepath.Base(sdk.adb) == executable("adb") {
		return fmt.Errorf("adb was not found in %s; run sdkmanager \"platform-tools\"", sdk.home)
	}
	listing, err := adb(sdk, "", "devices")
	if err != nil {
		return err
	}
	device, err := chooseDevice(parseADBDevices(listing), requested)
	if err != nil {
		return err
	}
	if err := waitForPackageService(sdk, device.Serial, output); err != nil {
		return err
	}
	if _, err := adb(sdk, device.Serial, "install", "-r", plan.out); err != nil {
		return err
	}
	id := plan.androidPackageID()
	if _, err := adb(sdk, device.Serial, "shell", "am", "start", "-n", id+"/"+androidActivity); err != nil {
		return err
	}
	fmt.Fprintf(output, "started %s on %s\n", id, device.Serial)
	return nil
}

// bootWait bounds how long run waits for a device that has just started.
const (
	bootAttempts = 90
	bootPause    = 2 * time.Second
)

// pause waits between boot checks; tests replace it.
var pause = time.Sleep

// waitForPackageService waits for the device to have its package service: an emulator
// that is still starting is a device adb already lists, and answers an install with
// "Can't find service: package".
func waitForPackageService(sdk androidSDK, serial string, output io.Writer) error {
	for attempt := 0; attempt < bootAttempts; attempt++ {
		if _, err := adb(sdk, serial, "shell", "pm", "path", "android"); err == nil {
			return nil
		}
		if attempt == 0 {
			fmt.Fprintf(output, "waiting for %s to finish starting\n", serial)
		}
		pause(bootPause)
	}
	return fmt.Errorf("%s did not finish starting in %v", serial, bootAttempts*bootPause)
}
