package main

import (
	"strings"
	"testing"
)

// The lines below are copied from a run of the headless build, so the parser
// answers for the trace as the runtime actually writes it.
func TestHeapTraceReadsLiveFromGCLine(t *testing.T) {
	trace := newHeapTrace(strings.NewReader(strings.Join([]string{
		`gc 5 @1.711s 2%: 0.042+13+0.011 ms clock, 0.42+6.9/7.6/0.66+0.11 ms cpu, 12->15->6 MB, 13 MB goal, 0 MB stacks, 0 MB globals, 10 P`,
		`gc 6 @2.112s 1%: 0.97+22+0.007 ms clock, 9.7+1.5/0.041/0+0.078 ms cpu, 12->14->7 MB, 14 MB goal, 0 MB stacks, 0 MB globals, 10 P`,
		`scavenger gap: 4.7s`,
		`gc 8 @2.163s 2%: 10+3.3+0.44 ms clock, 103+0.70/0.003/0+4.4 ms cpu, 17->20->12 MB, 20 MB goal, 0 MB stacks, 0 MB globals, 10 P`,
		"",
	}, "\n")))
	trace.read()
	if got := trace.live(); got != 12 {
		t.Fatalf("heap live = %v, want 12 (the third figure of the last gc line)", got)
	}
}

func TestHeapTraceIgnoresOtherLines(t *testing.T) {
	trace := newHeapTrace(strings.NewReader("hello world\nno gc here\n"))
	trace.read()
	if got := trace.live(); got != 0 {
		t.Fatalf("heap live = %v without a gc line, want 0", got)
	}
}
