package collation

import (
	"fmt"
	"sort"
	"testing"

	"golang.org/x/text/language"
)

func requireOrder(t *testing.T, collator *Collator, ordered ...string) {
	t.Helper()
	for index := 1; index < len(ordered); index++ {
		if order := collator.Compare(ordered[index-1], ordered[index]); order >= 0 {
			t.Fatalf("%q does not sort before %q: %d", ordered[index-1], ordered[index], order)
		}
	}
}

func TestCollatorOrdersChineseByPinyin(t *testing.T) {
	collator := New(language.SimplifiedChinese)
	requireOrder(t, collator,
		"北京理工大学", "北京农学院", "福州大学", "泉州职业技术大学",
		"山西财政税务专科学校", "中国科学院大学")
	requireOrder(t, collator, "高校", "个人", "公司", "合作", "临时", "托管")
}

func TestCollatorFollowsTheLocale(t *testing.T) {
	simplified := New(language.SimplifiedChinese)
	traditional := New(language.TraditionalChinese)
	if simplified.Compare("福州", "泉州") >= 0 {
		t.Fatal("pinyin order puts fu after quan")
	}
	// Stroke order reads 泉 (nine strokes) before 福 (thirteen).
	if traditional.Compare("泉州", "福州") >= 0 {
		t.Fatal("stroke order does not put quan before fu")
	}
	// A regional tag reaches the same order as its language.
	if order := New(language.MustParse("zh-CN")).Compare("福州", "泉州"); order >= 0 {
		t.Fatalf("zh-CN does not follow pinyin: %d", order)
	}
}

func TestCollatorOrdersLatinText(t *testing.T) {
	collator := New(language.English)
	requireOrder(t, collator, "Alpha", "Beta", "Mapped", "zulu")
	if order := collator.Compare("Alpha", "alpha"); order == 0 {
		t.Fatal("case did not take part in the order")
	}
}

func TestCollatorTreatsNormalizedSpellingsAsEqual(t *testing.T) {
	collator := New(language.SimplifiedChinese)
	if order := collator.Compare("é", "é"); order != 0 {
		t.Fatalf("composed and decomposed forms differ: %d", order)
	}
}

var benchmarkNames = func() []string {
	syllables := []rune("福州大学泉州职业技术中国科学院北京农学院理工山西财政税务专科学校")
	names := make([]string, 200)
	for index := range names {
		runes := make([]rune, 6)
		for at := range runes {
			runes[at] = syllables[(index*7+at*3)%len(syllables)]
		}
		names[index] = fmt.Sprintf("%02d-%s", index, string(runes))
	}
	return names
}()

func BenchmarkCompare(b *testing.B) {
	collator := New(language.SimplifiedChinese)
	b.ReportAllocs()
	for b.Loop() {
		for index := 1; index < len(benchmarkNames); index++ {
			collator.Compare(benchmarkNames[index-1], benchmarkNames[index])
		}
	}
}

func BenchmarkSort(b *testing.B) {
	collator := New(language.SimplifiedChinese)
	names := make([]string, len(benchmarkNames))
	b.ReportAllocs()
	for b.Loop() {
		copy(names, benchmarkNames)
		sort.Slice(names, func(left, right int) bool {
			return collator.Compare(names[left], names[right]) < 0
		})
	}
}
