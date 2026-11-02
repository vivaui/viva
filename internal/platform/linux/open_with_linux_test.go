//go:build linux

package linux

import "testing"

func TestOpenWithStartsProgramsDirectly(t *testing.T) {
	if err := OpenWith("/usr/bin/true", t.TempDir()); err != nil {
		t.Fatalf("direct launch: %v", err)
	}
}
