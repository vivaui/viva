//go:build darwin && !ios

package darwin

import (
	"strings"
	"testing"
)

// A storage partition's name is the same store in every run, so the identifier it is named by on the
// platform that wants one has to be the same too.
func TestAPartitionIdentifierIsTheSameForEveryNameEveryTime(t *testing.T) {
	first, second := partitionIdentifier("workspace-1"), partitionIdentifier("workspace-1")
	if first != second {
		t.Fatalf("%q and %q name the same partition", first, second)
	}
	if other := partitionIdentifier("workspace-2"); other == first {
		t.Fatalf("two partitions share the identifier %q", other)
	}
}

func TestAPartitionIdentifierIsAUUID(t *testing.T) {
	identifier := partitionIdentifier("workspace-1")
	if len(identifier) != 36 {
		t.Fatalf("identifier %q is %d characters, want 36", identifier, len(identifier))
	}
	for i, r := range identifier {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				t.Fatalf("identifier %q has %q at %d, want a hyphen", identifier, r, i)
			}
		default:
			if !strings.ContainsRune("0123456789ABCDEF", r) {
				t.Fatalf("identifier %q has %q at %d, want an upper-case hex digit", identifier, r, i)
			}
		}
	}
	// WKWebsiteDataStore takes the canonical form and nothing else, so the version and the variant
	// have to be the ones it validates.
	if identifier[14] != '5' || !strings.ContainsRune("89AB", rune(identifier[19])) {
		t.Fatalf("identifier %q is not a version 5 UUID with an RFC 4122 variant", identifier)
	}
}

func TestWebKitKeepsTheExistingPartitionIdentifier(t *testing.T) {
	const want = "26F5E273-A7C6-5A8C-B0C9-829B7CE44701"
	if got := partitionIdentifier("workspace-1"); got != want {
		t.Fatalf("identifier = %q, want %q so existing WebKit data remains accessible", got, want)
	}
}
