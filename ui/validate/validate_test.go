package validate_test

import (
	"regexp"
	"testing"

	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/validate"
)

func TestRules(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+$`)
	cases := []struct {
		name  string
		rule  validate.Rule
		pass  []string
		fails []string
	}{
		{"required", validate.Required("m"), []string{"a"}, []string{"", "  \t"}},
		{"minlength", validate.MinLength(3, "m"), []string{"", "abc", "你好吗"}, []string{"ab", "你好"}},
		{"maxlength", validate.MaxLength(2, "m"), []string{"", "ab", "你好"}, []string{"abc", "你好吗"}},
		{"min", validate.Min(1, "m"), []string{"", "1", " 2.5 "}, []string{"0", "x"}},
		{"max", validate.Max(10, "m"), []string{"", "10", "-3"}, []string{"11", "x"}},
		{"pattern", validate.Pattern(re, "m"), []string{"", "abc"}, []string{"ab1"}},
		{"email", validate.Email("m"), []string{"", "a@b.co"}, []string{"a", "A <a@b.co>", "a@"}},
		{"url", validate.URL("m"), []string{"", "https://x.dev/p?q=1"}, []string{"x.dev", "https://", "/p"}},
	}
	for _, c := range cases {
		for _, v := range c.pass {
			if got := c.rule(v); got != "" {
				t.Errorf("%s(%q) = %q, want pass", c.name, v, got)
			}
		}
		for _, v := range c.fails {
			if got := c.rule(v); got != "m" {
				t.Errorf("%s(%q) = %q, want failure", c.name, v, got)
			}
		}
	}
}

func TestCheckReportsFirstFailureInOrder(t *testing.T) {
	rules := []validate.Rule{validate.Required("req"), validate.MinLength(3, "short")}
	if got := validate.Check("", rules); got != "req" {
		t.Errorf("empty = %q, want req", got)
	}
	if got := validate.Check("ab", rules); got != "short" {
		t.Errorf("ab = %q, want short", got)
	}
	if got := validate.Check("abc", rules); got != "" {
		t.Errorf("abc = %q, want pass", got)
	}
}

func TestApplySetsAndClearsTheField(t *testing.T) {
	var f ui.FormFieldState
	rules := []validate.Rule{validate.Required("req")}
	if validate.Apply(&f, "", rules) || f.Error != "req" || !f.Invalid {
		t.Fatalf("failing value left the field %+v", f)
	}
	if !validate.Apply(&f, "x", rules) || f.Error != "" || f.Invalid {
		t.Fatalf("passing value left the field %+v", f)
	}
}

func TestCheckDoesNotAllocate(t *testing.T) {
	rules := []validate.Rule{validate.Required("r"), validate.MaxLength(8, "l"), validate.Email("e")}
	if n := testing.AllocsPerRun(100, func() { validate.Check("a@b.co", rules) }); n != 0 {
		t.Fatalf("Check allocates %v times", n)
	}
}

func BenchmarkCheck(b *testing.B) {
	rules := []validate.Rule{validate.Required("r"), validate.MaxLength(32, "l"), validate.Email("e")}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		validate.Check("user@example.com", rules)
	}
}
