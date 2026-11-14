//go:build windows

package instance

import "testing"

// isolate has nothing to hand out on Windows: a claim is a named pipe, not a
// file in a directory, so the tests share the machine's pipe namespace. Each
// test names an application of its own, which is what keeps them apart.
func isolate(t testing.TB) { t.Helper() }

// The address must be a pipe and must not move between calls: every copy of the
// application computes it on its own and has to arrive at the same one.
func TestTheClaimAddressIsAStablePipe(t *testing.T) {
	address, err := claimAddress("dev.test.pipe")
	if err != nil {
		t.Fatal(err)
	}
	if len(address) <= len(pipePrefix) || address[:len(pipePrefix)] != pipePrefix {
		t.Fatalf("the claim address %q is not a named pipe", address)
	}
	if again, err := claimAddress("dev.test.pipe"); err != nil || again != address {
		t.Fatalf("the claim address is not stable: %q then %q (%v)", address, again, err)
	}
	if other, _ := claimAddress("dev.test.other"); other == address {
		t.Fatalf("two applications share the claim address %q", address)
	}
}
