package mentions

import "testing"

func TestWordFindsTheMentionAtTheCaret(t *testing.T) {
	cases := []struct {
		value     string
		caret     int
		wantStart int
		wantQuery string
		wantOK    bool
	}{
		{"@", 1, 0, "", true},
		{"hi @gr", 6, 3, "gr", true},
		{"hi @gr there", 6, 3, "gr", true},
		{"hi @gr", 3, 0, "", false},
		{"x@gr", 4, 0, "", false},
		{"hi @gr ", 7, 0, "", false},
		{"你好 @张三", 6, 3, "张三", true},
		{"@a\n@b", 5, 3, "b", true},
		{"", 0, 0, "", false},
		{"@gr", 9, 0, "@gr"[1:], true}, // a caret past the end is the end
	}
	for _, c := range cases {
		start, query, ok := word(c.value, c.caret, '@')
		if ok != c.wantOK || (ok && (start != c.wantStart || query != c.wantQuery)) {
			t.Errorf("word(%q, %d) = %d, %q, %v; want %d, %q, %v", c.value, c.caret, start, query, ok, c.wantStart, c.wantQuery, c.wantOK)
		}
	}
}

func TestContainsFoldIgnoresCaseAndFindsInsideAWord(t *testing.T) {
	for _, c := range []struct {
		s, q string
		want bool
	}{
		{"Grace", "gr", true}, {"Grace", "ACE", true}, {"Grace", "x", false}, {"Grace", "", true},
		{"Ünal", "ün", true}, {"张三", "三", true}, {"Ada", "adam", false},
	} {
		if got := containsFold(c.s, c.q); got != c.want {
			t.Errorf("containsFold(%q, %q) = %v", c.s, c.q, got)
		}
	}
}

func TestWordDoesNotAllocate(t *testing.T) {
	if n := testing.AllocsPerRun(100, func() { word("hello @gra", 10, '@') }); n != 0 {
		t.Fatalf("word allocates %v times", n)
	}
}
