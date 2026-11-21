package windows

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// A page that names no partition and keeps nothing asks for the default profile through the plain
// controller: that is the way every page was made before partitions, and it must not change.
func TestAPageWithoutAPartitionAsksForNoProfileOfItsOwn(t *testing.T) {
	name, inPrivate, options := controllerProfile(platform.WebViewConfig{})
	if options || inPrivate || name != "" {
		t.Fatalf("a page with nothing to ask for got profile %q, in private %v, options %v", name, inPrivate, options)
	}
}

func TestAPartitionNamesTheWebView2Profile(t *testing.T) {
	name, inPrivate, options := controllerProfile(platform.WebViewConfig{Partition: "workspace-1"})
	if !options || inPrivate || name != "workspace-1" {
		t.Fatalf("partition workspace-1 got profile %q, in private %v, options %v", name, inPrivate, options)
	}
}

// A partition that must keep nothing is an off-the-record profile of the partition's name, so its
// pages share it and the pages of another partition do not.
func TestAnEphemeralPartitionIsAnOffTheRecordProfile(t *testing.T) {
	name, inPrivate, options := controllerProfile(platform.WebViewConfig{Partition: "workspace-1", Ephemeral: true})
	if !options || !inPrivate || name != "workspace-1" {
		t.Fatalf("ephemeral partition workspace-1 got profile %q, in private %v, options %v", name, inPrivate, options)
	}
}

// An ephemeral page with no partition keeps its own store: WebView2 derives one off-the-record
// profile from the default profile's name, so two of them without a name of their own would share it.
func TestEphemeralPagesWithoutAPartitionEachGetAProfileOfTheirOwn(t *testing.T) {
	first, inPrivate, options := controllerProfile(platform.WebViewConfig{Ephemeral: true})
	second, _, _ := controllerProfile(platform.WebViewConfig{Ephemeral: true})
	if !options || !inPrivate {
		t.Fatalf("an ephemeral page got in private %v, options %v", inPrivate, options)
	}
	if first == second {
		t.Fatalf("two ephemeral pages share the profile %q", first)
	}
	for _, name := range []string{first, second} {
		if !strings.HasPrefix(name, "viva-ephemeral-"+strconv.Itoa(os.Getpid())+"-") {
			t.Fatalf("profile %q is not this process's own", name)
		}
		if len(name) > 64 {
			t.Fatalf("profile %q is longer than the 64 characters WebView2 accepts", name)
		}
	}
}
