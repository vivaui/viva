package windows

import "testing"

// TestMonospaceFamiliesKeepsTheSystemVouchedAndMeasuresTheRest pins both
// signals a family needs to count as monospace (t8y2/motty#296): the system's
// own fixed-pitch vouch, and — for the families whose flag really says
// "proportional", which is wrong for some a terminal wants — a measured
// uniformity of the printable ASCII. A family that is proportional and
// measures proportional, such as Arial, stays out either way.
func TestMonospaceFamiliesKeepsTheSystemVouchedAndMeasuresTheRest(t *testing.T) {
	mono := map[string]bool{}
	for _, name := range MonospaceFamilies() {
		mono[name] = true
	}
	if !mono["Consolas"] && !mono["Cascadia Mono"] && !mono["Courier New"] {
		t.Fatal("none of the runners' fixed-pitch families made the list")
	}
	if mono["Arial"] || mono["Times New Roman"] {
		t.Fatal("a proportional family that measures proportional made the mono list")
	}
}
