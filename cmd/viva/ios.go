package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/vivaui/viva/tools/appbundle"
	"github.com/vivaui/viva/tools/manifest"
)

// iosMinSystem is the oldest iOS an application is built for.
const iosMinSystem = "16.0"

// isIOS reports whether the plan builds an iOS application bundle, for the
// simulator or for devices.
func (plan buildPlan) isIOS() bool { return plan.target == "ios-sim" || plan.target == "ios" }

// iosToolchain is where Xcode keeps what cgo needs to build for an iOS SDK.
type iosToolchain struct {
	clang, sdk string
}

// xcrun runs Xcode's tool locator and returns its trimmed output; tests replace it.
var xcrun = func(arguments ...string) (string, error) {
	command := exec.Command("xcrun", arguments...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("xcrun %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(output)), nil
}

// iosSDK is the Xcode SDK a target is built against.
func iosSDK(target string) string {
	if target == "ios-sim" {
		return "iphonesimulator"
	}
	return "iphoneos"
}

// findIOSToolchain asks Xcode for the SDK and the C compiler of the target.
func findIOSToolchain(target string) (iosToolchain, error) {
	sdk := iosSDK(target)
	root, err := xcrun("--sdk", sdk, "--show-sdk-path")
	if err != nil {
		return iosToolchain{}, fmt.Errorf("the %s SDK was not found; install Xcode and run 'xcode-select -s /Applications/Xcode.app': %w", sdk, err)
	}
	clang, err := xcrun("--sdk", sdk, "-f", "clang")
	if err != nil {
		return iosToolchain{}, err
	}
	return iosToolchain{clang: clang, sdk: root}, nil
}

// triple is the clang target of an iOS build.
func (plan buildPlan) iosTriple() string {
	arch := plan.arch
	if arch == "amd64" {
		arch = "x86_64"
	}
	triple := arch + "-apple-ios" + iosMinSystem
	if plan.target == "ios-sim" {
		triple += "-simulator"
	}
	return triple
}

// iosEnvironment is what go build needs for iOS: cgo, with Xcode's compiler
// pointed at the SDK and the deployment target. The Metal and UIKit bridge is
// Objective-C, so there is no building it without them.
func (plan buildPlan) iosEnvironment(environment, base []string) []string {
	flags := "-isysroot " + plan.toolchain.sdk + " -target " + plan.iosTriple()
	environment = append(environment, "CGO_ENABLED=1", "CC="+plan.toolchain.clang+" "+flags)
	for _, name := range []string{"CGO_CFLAGS", "CGO_LDFLAGS"} {
		value := flags
		if existing := lastEnvironment(base, name); existing != "" {
			value = existing + " " + flags
		}
		environment = append(environment, name+"="+value)
	}
	return environment
}

// checkIOS reports what a machine or flag combination cannot build.
func checkIOS(shared buildFlags) error {
	if runtime.GOOS != "darwin" {
		return errors.New("building for iOS needs a macOS host with Xcode: the Metal and UIKit bridge is Objective-C")
	}
	if shared.target == "ios" && shared.arch != "arm64" {
		return fmt.Errorf("iOS devices run arm64; got --arch %s", shared.arch)
	}
	if shared.cgo {
		return errors.New("--cgo is for Linux and Windows; iOS builds always use cgo")
	}
	return nil
}

// bundleID is the application's identifier on a device.
func (plan buildPlan) bundleID() string {
	if id := plan.manifest.App.ID; id != "" {
		return id
	}
	return "dev.viva." + manifest.Slug(plan.executable)
}

// runIOS builds the program into the .app and writes the Info.plist beside it.
func (plan buildPlan) runIOS(errorOutput io.Writer) error {
	toolchain, err := findIOSToolchain(plan.target)
	if err != nil {
		return err
	}
	plan.toolchain = toolchain
	if err := os.RemoveAll(plan.out); err != nil {
		return err
	}
	if err := os.MkdirAll(plan.out, 0o755); err != nil {
		return err
	}
	if err := plan.build(plan.programPath(), errorOutput); err != nil {
		return err
	}
	name := plan.manifest.App.Name
	if name == "" {
		name = plan.executable
	}
	plist, err := appbundle.IOSInfo{
		Name: name, ID: plan.bundleID(), Version: plan.manifest.App.Version, Executable: plan.executable,
		MinSystem: iosMinSystem, Simulator: plan.target == "ios-sim",
	}.Plist()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(plan.out, "Info.plist"), plist, 0o644)
}

// simulator is one device of the iOS simulator.
type simulator struct {
	UDID    string `json:"udid"`
	Name    string `json:"name"`
	State   string `json:"state"`
	Runtime string `json:"-"`
}

// listSimulators reads the available devices out of 'simctl list devices -j'.
func listSimulators(data []byte) ([]simulator, error) {
	var listing struct {
		Devices map[string][]simulator `json:"devices"`
	}
	if err := json.Unmarshal(data, &listing); err != nil {
		return nil, fmt.Errorf("reading the simulator list: %w", err)
	}
	var devices []simulator
	for runtimeName, list := range listing.Devices {
		if !strings.Contains(runtimeName, ".iOS-") {
			continue
		}
		for _, device := range list {
			device.Runtime = runtimeName
			devices = append(devices, device)
		}
	}
	return devices, nil
}

// runtimeVersion orders runtime names such as ...SimRuntime.iOS-18-5 by version.
func runtimeVersion(name string) []int {
	_, version, _ := strings.Cut(name, ".iOS-")
	var numbers []int
	for _, part := range strings.Split(version, "-") {
		number, _ := strconv.Atoi(part)
		numbers = append(numbers, number)
	}
	return numbers
}

func versionLess(a, b []int) bool {
	for index := 0; index < len(a) && index < len(b); index++ {
		if a[index] != b[index] {
			return a[index] < b[index]
		}
	}
	return len(a) < len(b)
}

// chooseSimulator picks the device to run on: the one asked for by name or
// identifier; otherwise an iPhone already booted, or the first iPhone of the
// newest iOS the machine has.
func chooseSimulator(devices []simulator, requested string) (simulator, error) {
	if requested != "" {
		for _, device := range devices {
			if device.UDID == requested || device.Name == requested {
				return device, nil
			}
		}
		return simulator{}, fmt.Errorf("no simulator named %q; 'xcrun simctl list devices available' lists them", requested)
	}
	var phones []simulator
	for _, device := range devices {
		if strings.HasPrefix(device.Name, "iPhone") {
			phones = append(phones, device)
		}
	}
	if len(phones) == 0 {
		return simulator{}, errors.New("no iPhone simulator is installed; add one in Xcode's Devices and Simulators window")
	}
	sort.SliceStable(phones, func(i, j int) bool {
		a, b := phones[i], phones[j]
		if (a.State == "Booted") != (b.State == "Booted") {
			return a.State == "Booted"
		}
		if a.Runtime != b.Runtime {
			return versionLess(runtimeVersion(b.Runtime), runtimeVersion(a.Runtime))
		}
		return a.Name < b.Name
	})
	return phones[0], nil
}

// simctl runs the simulator's control tool; tests replace it.
var simctl = func(output io.Writer, arguments ...string) ([]byte, error) {
	command := exec.Command("xcrun", append([]string{"simctl"}, arguments...)...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("simctl %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// openSimulatorApp shows the Simulator window; tests replace it.
var openSimulatorApp = func(udid string) error {
	return exec.Command("open", "-a", "Simulator", "--args", "-CurrentDeviceUDID", udid).Run()
}

func runRun(arguments []string, output, errorOutput io.Writer) error {
	c, _ := findCommand("run")
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	var shared buildFlags
	shared.register(flags, "")
	var device string
	open := true
	flags.StringVar(&device, "device", "", "ios-sim: the simulator's name or identifier (default: a booted iPhone, else the newest one installed); android: the device's adb serial (default: the one that is ready)")
	flags.BoolVar(&open, "open", true, "show the Simulator window")
	positional, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if shared.target != "ios-sim" && shared.target != "android" {
		return fmt.Errorf("run starts an application on a phone: give --target ios-sim or --target android (got %q). Use viva dev for the desktop", shared.target)
	}
	if len(positional) > 1 {
		return errors.New("run takes at most one package; run 'viva help run'")
	}
	argument := ""
	if len(positional) == 1 {
		argument = positional[0]
	}
	plan, err := newPlan(shared, argument)
	if err != nil {
		return err
	}
	if err := plan.outputPath(""); err != nil {
		return err
	}
	if err := plan.run(plan.out, errorOutput); err != nil {
		return err
	}
	if plan.isAndroid() {
		return runOnAndroid(plan, device, output)
	}
	listing, err := simctl(errorOutput, "list", "devices", "available", "-j")
	if err != nil {
		return err
	}
	devices, err := listSimulators(listing)
	if err != nil {
		return err
	}
	chosen, err := chooseSimulator(devices, device)
	if err != nil {
		return err
	}
	if chosen.State != "Booted" {
		if _, err := simctl(errorOutput, "boot", chosen.UDID); err != nil {
			return err
		}
	}
	if open {
		if err := openSimulatorApp(chosen.UDID); err != nil {
			return fmt.Errorf("showing the Simulator window: %w", err)
		}
	}
	if _, err := simctl(errorOutput, "bootstatus", chosen.UDID, "-b"); err != nil {
		return err
	}
	if _, err := simctl(errorOutput, "install", chosen.UDID, plan.out); err != nil {
		return err
	}
	// Without --terminate-running-process a copy still running from the last
	// run would be brought forward instead of started again.
	if _, err := simctl(errorOutput, "launch", "--terminate-running-process", chosen.UDID, plan.bundleID()); err != nil {
		return err
	}
	fmt.Fprintf(output, "started %s on %s (%s)\n", plan.bundleID(), chosen.Name, chosen.UDID)
	return nil
}
