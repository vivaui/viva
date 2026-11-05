package update

import (
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	// The precedence list from Semantic Versioning 2.0.0, oldest first.
	ordered := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
		"1.0.1",
		"1.1.0",
		"2.0.0",
	}
	for older := 0; older < len(ordered); older++ {
		for newer := older + 1; newer < len(ordered); newer++ {
			order, err := Compare(ordered[older], ordered[newer])
			if err != nil {
				t.Fatalf("Compare(%q, %q): %v", ordered[older], ordered[newer], err)
			}
			if order != -1 {
				t.Errorf("Compare(%q, %q) = %d, want -1", ordered[older], ordered[newer], order)
			}
			order, err = Compare(ordered[newer], ordered[older])
			if err != nil {
				t.Fatalf("Compare(%q, %q): %v", ordered[newer], ordered[older], err)
			}
			if order != 1 {
				t.Errorf("Compare(%q, %q) = %d, want 1", ordered[newer], ordered[older], order)
			}
		}
	}
}

func TestCompareIgnoresPrefixAndBuildMetadata(t *testing.T) {
	for _, pair := range [][2]string{
		{"v1.2.3", "1.2.3"},
		{"1.2.3+build.5", "1.2.3"},
		{"v0.2.4+linux", "0.2.4+darwin"},
	} {
		order, err := Compare(pair[0], pair[1])
		if err != nil {
			t.Fatalf("Compare(%q, %q): %v", pair[0], pair[1], err)
		}
		if order != 0 {
			t.Errorf("Compare(%q, %q) = %d, want 0", pair[0], pair[1], order)
		}
	}
}

func TestCompareRefusesWhatItCannotOrder(t *testing.T) {
	for _, text := range []string{
		"",
		"v",
		"1",
		"1.2",
		"1.2.3.4",
		"01.2.3",
		"1.02.3",
		"1.2.03",
		"1.2.x",
		"1.2.3-",
		"1.2.3-01",
		"1.2.3-alpha..1",
		"1.2.3-alpha_1",
		"1.2.3+",
		"1.2.3+a..b",
		"1.2.3+a_b",
	} {
		if _, err := Compare(text, "1.0.0"); err == nil {
			t.Errorf("Compare(%q, \"1.0.0\") accepted it", text)
		}
		if !strings.Contains(errText(t, text), "update:") {
			t.Errorf("Compare(%q) error %q does not name the package", text, errText(t, text))
		}
	}
}

func errText(t *testing.T, text string) string {
	t.Helper()
	_, err := parseVersion(text)
	if err == nil {
		return ""
	}
	return err.Error()
}
