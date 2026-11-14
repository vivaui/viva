package instance

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSecondLaunchHandsItsArgumentsToTheFirst(t *testing.T) {
	isolate(t)
	got := make(chan []string, 4)
	first, primary, err := Claim("dev.test.one", []string{"first"}, func(args []string) { got <- args })
	if err != nil || !primary {
		t.Fatalf("first claim: primary %v, err %v", primary, err)
	}
	defer first.Close()
	want := []string{"ssh://user@host:2222/", "", "ünï cödé", "with space"}
	second, primary, err := Claim("dev.test.one", want, func([]string) { t.Error("the second process must not serve") })
	if err != nil || primary || second != nil {
		t.Fatalf("second claim: primary %v, err %v", primary, err)
	}
	select {
	case args := <-got:
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("handler got %q, want %q", args, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the arguments never arrived")
	}
}

func TestDifferentIDsDoNotMeetAndEmptyArgumentsWork(t *testing.T) {
	isolate(t)
	a, primaryA, _ := Claim("dev.test.a", nil, func([]string) {})
	b, primaryB, _ := Claim("dev.test.b", nil, func([]string) {})
	if !primaryA || !primaryB {
		t.Fatal("two applications should both be primary")
	}
	a.Close()
	b.Close()
	got := make(chan []string, 1)
	c, primary, _ := Claim("dev.test.a", nil, func(args []string) { got <- args })
	if !primary {
		t.Fatal("a closed instance left its claim behind")
	}
	defer c.Close()
	Claim("dev.test.a", nil, nil)
	if args := <-got; len(args) != 0 {
		t.Fatalf("args = %q", args)
	}
}

func TestAClosedClaimIsReclaimed(t *testing.T) {
	isolate(t)
	first, primary, _ := Claim("dev.test.stale", nil, func([]string) {})
	if !primary {
		t.Fatal("not primary")
	}
	// A process that died leaves its listener behind: close that only. On Windows
	// the pipe goes with it and nothing is left; on Unix the socket file stays
	// until the next claim replaces it.
	first.listener.Close()
	<-first.done
	second, primary, err := Claim("dev.test.stale", nil, func([]string) {})
	if err != nil || !primary {
		t.Fatalf("after a crash: primary %v, err %v", primary, err)
	}
	second.Close()
}

func TestOversizeAndMalformedMessagesAreRefused(t *testing.T) {
	if err := writeArgs(&strings.Builder{}, []string{strings.Repeat("x", maxMessage)}); err == nil {
		t.Fatal("an oversize message was written")
	}
	if err := writeArgs(&strings.Builder{}, []string{"a\x00b"}); err == nil {
		t.Fatal("NUL in an argument was written")
	}
	if _, primary, err := Claim("", nil, nil); err == nil || primary {
		t.Fatal("an empty id was accepted")
	}
}

// The claim failed twice over, and the platform's reason for the half this
// process tried must be in the error: without it an address the system does not
// have looked exactly like an application that was not running.
func TestClaimFailureNamesTheListenerError(t *testing.T) {
	cause := errors.New("socket: address family not supported")
	err := claimFailure(`\\.\pipe\viva-test`, cause)
	if !errors.Is(err, errNobody) {
		t.Fatalf("the failure does not report that nobody listens: %v", err)
	}
	for _, want := range []string{"cannot claim", `\\.\pipe\viva-test`, cause.Error()} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the failure %q does not name %q", err, want)
		}
	}
}

func BenchmarkClaimAndClose(b *testing.B) {
	isolate(b)
	b.ReportAllocs()
	for b.Loop() {
		claim, primary, err := Claim("dev.test.bench", nil, func([]string) {})
		if err != nil || !primary {
			b.Fatalf("claim: primary %v, err %v", primary, err)
		}
		claim.Close()
	}
}
