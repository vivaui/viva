package main

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

const simulatorListing = `{"devices": {
  "com.apple.CoreSimulator.SimRuntime.iOS-9-0": [
    {"udid": "OLD", "name": "iPhone 6", "state": "Shutdown"}],
  "com.apple.CoreSimulator.SimRuntime.iOS-18-5": [
    {"udid": "PRO", "name": "iPhone 16 Pro", "state": "Shutdown"},
    {"udid": "PAD", "name": "iPad Pro", "state": "Booted"},
    {"udid": "SIX", "name": "iPhone 16", "state": "Shutdown"}],
  "com.apple.CoreSimulator.SimRuntime.watchOS-11-0": [
    {"udid": "WATCH", "name": "Apple Watch", "state": "Booted"}]
}}`

func simulators(t *testing.T) []simulator {
	t.Helper()
	devices, err := listSimulators([]byte(simulatorListing))
	if err != nil {
		t.Fatal(err)
	}
	return devices
}

// The newest iOS wins by version, not by the order of the names: 18-5 is newer
// than 9-0 though "9" sorts after "1".
func TestRunPicksAnIPhoneOfTheNewestRuntime(t *testing.T) {
	devices := simulators(t)
	chosen, err := chooseSimulator(devices, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chosen.Runtime, "iOS-18-5") || !strings.HasPrefix(chosen.Name, "iPhone") {
		t.Fatalf("chose %+v, want an iPhone of iOS 18.5", chosen)
	}
}

// What is already running is used rather than booting a second device, and a
// watch or an iPad is not an iPhone to start on.
func TestRunPrefersABootedIPhone(t *testing.T) {
	devices := simulators(t)
	for index := range devices {
		if devices[index].UDID == "OLD" {
			devices[index].State = "Booted"
		}
	}
	chosen, err := chooseSimulator(devices, "")
	if err != nil {
		t.Fatal(err)
	}
	if chosen.UDID != "OLD" {
		t.Fatalf("chose %+v, want the booted iPhone", chosen)
	}
}

func TestRunTakesADeviceByNameOrIdentifier(t *testing.T) {
	devices := simulators(t)
	for _, requested := range []string{"SIX", "iPhone 16"} {
		chosen, err := chooseSimulator(devices, requested)
		if err != nil || chosen.UDID != "SIX" {
			t.Fatalf("--device %s chose %+v, %v", requested, chosen, err)
		}
	}
	if _, err := chooseSimulator(devices, "iPhone 99"); err == nil {
		t.Fatal("an unknown device was accepted")
	}
}

func TestRunNeedsAnIPhoneSimulator(t *testing.T) {
	devices, err := listSimulators([]byte(`{"devices": {"com.apple.CoreSimulator.SimRuntime.iOS-18-5": [{"udid": "PAD", "name": "iPad Pro", "state": "Shutdown"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chooseSimulator(devices, ""); err == nil {
		t.Fatal("no iPhone, and still a device was chosen")
	}
}

func TestIOSBuildChecks(t *testing.T) {
	err := checkIOS(buildFlags{target: "ios-sim", arch: "arm64"})
	if runtime.GOOS != "darwin" {
		// Only a Mac has Xcode: the check says so before anything else.
		if err == nil {
			t.Fatal("an iOS build was accepted on a host without Xcode")
		}
		return
	}
	if err != nil {
		t.Fatalf("a simulator build was refused on a Mac: %v", err)
	}
	if checkIOS(buildFlags{target: "ios", arch: "amd64"}) == nil {
		t.Error("an amd64 device build was accepted; devices run arm64")
	}
	if checkIOS(buildFlags{target: "ios-sim", arch: "arm64", cgo: true}) == nil {
		t.Error("--cgo was accepted for iOS, which always uses cgo")
	}
}

// An iOS build compiles the Objective-C bridge against the SDK of its target:
// the environment must name the SDK, the architecture and, for the simulator
// only, the simulator.
func TestIOSEnvironmentPointsCgoAtTheSDK(t *testing.T) {
	for _, test := range []struct {
		target, arch, triple string
		simulator            bool
	}{
		{"ios-sim", "arm64", "arm64-apple-ios" + iosMinSystem + "-simulator", true},
		{"ios-sim", "amd64", "x86_64-apple-ios" + iosMinSystem + "-simulator", true},
		{"ios", "arm64", "arm64-apple-ios" + iosMinSystem, false},
	} {
		plan := buildPlan{target: test.target, arch: test.arch, toolchain: iosToolchain{clang: "/x/clang", sdk: "/x/SDK.sdk"}}
		environment := plan.environment([]string{"CGO_CFLAGS=-O2"})
		for _, want := range []string{"GOOS=ios", "GOARCH=" + test.arch, "CGO_ENABLED=1"} {
			if !slices.Contains(environment, want) {
				t.Errorf("%s/%s: environment lacks %s", test.target, test.arch, want)
			}
		}
		for _, name := range []string{"CC", "CGO_CFLAGS", "CGO_LDFLAGS"} {
			value := lastEnvironment(environment, name)
			if !strings.Contains(value, "-isysroot /x/SDK.sdk") || !strings.Contains(value, "-target "+test.triple) {
				t.Errorf("%s/%s: %s = %q, want the SDK and target %s", test.target, test.arch, name, value, test.triple)
			}
			if strings.HasSuffix(test.triple, "-simulator") != test.simulator {
				t.Errorf("test table disagrees with itself for %s", test.target)
			}
		}
		if value := lastEnvironment(environment, "CGO_CFLAGS"); !strings.Contains(value, "-O2") {
			t.Errorf("%s: the developer's CGO_CFLAGS were dropped: %q", test.target, value)
		}
	}
}

// The simulator's app and the device's do not overwrite each other, and both
// are bundles, not files.
func TestIOSOutputsAreSeparateBundles(t *testing.T) {
	dir := fixture(t, notesTOML)
	var outputs []string
	for _, target := range []string{"ios-sim", "ios"} {
		plan := buildPlan{manifest: loadPlan(t, dir, buildFlags{target: runtime.GOOS}, "").manifest, target: target, arch: "arm64", executable: "notes"}
		if err := plan.outputPath(""); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(plan.out, ".app") || !strings.HasSuffix(plan.programPath(), "/notes") {
			t.Errorf("%s: output %q, program %q", target, plan.out, plan.programPath())
		}
		outputs = append(outputs, plan.out)
	}
	if outputs[0] == outputs[1] {
		t.Fatalf("both targets write to %s", outputs[0])
	}
}

func TestPackagingRefusesIOS(t *testing.T) {
	dir := fixture(t, notesTOML)
	if err := run([]string{"package", "-C", dir, "--target", "ios-sim"}, new(strings.Builder)); err == nil {
		t.Fatal("package accepted an iOS target")
	}
}
