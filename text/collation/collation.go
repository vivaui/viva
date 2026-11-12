// Package collation orders user-visible text by the rules of a language. A
// byte comparison is right for ASCII and wrong for most other scripts: Chinese
// names sort by pinyin, with 福州 before 泉州, rather than by code point.
//
// It is a package of its own because the CLDR tables are not free to link: an
// application that never sorts text by a language's rules does not carry them.
package collation

import (
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// Collator compares strings in the order one language's readers expect.
type Collator struct {
	collator *collate.Collator
}

// New returns a Collator for tag. Chinese orders Han by pinyin, Traditional
// Chinese by stroke count, and other languages by their CLDR order. A
// polyphonic character takes the single reading in the bundled CLDR tables.
func New(tag language.Tag) *Collator {
	return &Collator{collator: collate.New(tag)}
}

// Compare returns -1, 0 or +1 as left sorts before, the same as, or after
// right. Two spellings that normalize to the same text compare the same, so a
// caller that needs one total order breaks that tie itself.
//
// A Collator keeps iteration state between calls and must not be used from
// several goroutines at once.
func (collator *Collator) Compare(left, right string) int {
	return collator.collator.CompareString(left, right)
}
