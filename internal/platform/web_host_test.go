package platform

import (
	"strings"
	"testing"
)

// A storage partition's name is the same store in every run, so the identifier it is named by on the
// platform that wants one has to be the same too.
func TestAPartitionIdentifierIsTheSameForEveryNameEveryTime(t *testing.T) {
	first, second := PartitionIdentifier("workspace-1"), PartitionIdentifier("workspace-1")
	if first != second {
		t.Fatalf("%q and %q name the same partition", first, second)
	}
	if other := PartitionIdentifier("workspace-2"); other == first {
		t.Fatalf("two partitions share the identifier %q", other)
	}
}

func TestAPartitionIdentifierIsAUUID(t *testing.T) {
	identifier := PartitionIdentifier("workspace-1")
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
