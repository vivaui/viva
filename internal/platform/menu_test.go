package platform

import (
	"reflect"
	"testing"
)

func TestNumberMenuIsDepthFirstFromOne(t *testing.T) {
	var chosen []string
	pick := func(name string) func() { return func() { chosen = append(chosen, name) } }
	entries := NumberMenu([]MenuItem{
		{Title: "a", OnSelect: pick("a")},
		{Title: "b", Submenu: []MenuItem{
			{Title: "b1", OnSelect: pick("b1")},
			{Title: "b2", Submenu: []MenuItem{{Title: "b2x", OnSelect: pick("b2x")}}},
		}},
		{Separator: true, Title: "ignored", Disabled: true, OnSelect: pick("sep")},
		{Title: "off", Disabled: true, OnSelect: pick("off")},
		{Title: "c", Checked: true, OnSelect: pick("c")},
	})
	want := []struct {
		title    string
		children int
	}{{"a", 0}, {"b", 2}, {"b1", 0}, {"b2", 1}, {"b2x", 0}, {"ignored", 0}, {"off", 0}, {"c", 0}}
	if len(entries) != len(want) {
		t.Fatalf("%d entries: %+v", len(entries), entries)
	}
	for index, entry := range entries {
		if entry.ID != index+1 || entry.Title != want[index].title || entry.Children != want[index].children {
			t.Fatalf("entry %d = %+v, want %+v", index, entry, want[index])
		}
	}
	if !entries[5].Separator || entries[5].Disabled || entries[5].OnSelect != nil {
		t.Fatalf("a separator kept fields it must not have: %+v", entries[5])
	}
	if !entries[7].Checked {
		t.Fatal("checked flag lost")
	}
	if TopLevel(entries) != 5 || SubtreeSize(entries, 1) != 3 || SubtreeSize(entries, 3) != 1 || SubtreeSize(entries, 0) != 0 {
		t.Fatalf("TopLevel %d, SubtreeSize(b) %d", TopLevel(entries), SubtreeSize(entries, 1))
	}
	for id := 0; id <= len(entries)+1; id++ {
		Select(entries, id)
	}
	if got := len(chosen); got != 4 || chosen[0] != "a" || chosen[1] != "b1" || chosen[2] != "b2x" || chosen[3] != "c" {
		t.Fatalf("chosen = %v: only reachable entries run", chosen)
	}
}

func TestNumberMenuOfNothingIsEmpty(t *testing.T) {
	if entries := NumberMenu(nil); len(entries) != 0 {
		t.Fatalf("entries = %+v", entries)
	}
}

// TestMenuKeysKeepAnEntryTheApplicationNamed is the platform's half of the
// status menu's stable ids: the dbusmenu package serves the id of a key, so a
// key that moves moves the id a host holds with it.
func TestMenuKeysKeepAnEntryTheApplicationNamed(t *testing.T) {
	keys := func(items ...MenuItem) []string {
		found := make([]string, 0, len(items))
		for _, entry := range NumberMenu(items) {
			found = append(found, entry.Key)
		}
		return found
	}
	named := func(title string) []string {
		return keys(
			MenuItem{ID: "status", Title: title},
			MenuItem{ID: "recent", Title: "Recent", Submenu: []MenuItem{{ID: "one", Title: title}}},
		)
	}
	if first, second := named("Idle"), named("Connected"); !reflect.DeepEqual(first, second) {
		t.Fatalf("an entry the application named changed its key with its title: %v then %v", first, second)
	}
	unnamed := func(title string) []string {
		return keys(MenuItem{Title: title}, MenuItem{Title: "Quit"})
	}
	if first, second := unnamed("Idle"), unnamed("Connected"); reflect.DeepEqual(first, second) {
		t.Fatalf("an entry the application did not name kept its key when its title changed: %v", first)
	}
	// A key says where in the menu the entry is, so an entry that moves to
	// another submenu is another entry.
	moved := keys(MenuItem{ID: "sub", Title: "Sub", Submenu: []MenuItem{{ID: "entry", Title: "Entry"}}})
	top := keys(MenuItem{ID: "sub", Title: "Sub"}, MenuItem{ID: "entry", Title: "Entry"})
	if moved[1] == top[1] {
		t.Fatalf("an entry kept its key after it moved to another submenu: %v then %v", moved, top)
	}
	// Two entries without an ID and of the same name are different entries,
	// and the first keeps its key when another joins it.
	twins := keys(MenuItem{Title: "Same"}, MenuItem{Title: "Same"})
	lone := keys(MenuItem{Title: "Same"})
	if twins[0] == twins[1] || twins[0] != lone[0] {
		t.Fatalf("entries of the same name: %v, alone %v", twins, lone)
	}
	// An ID names one entry of a menu, and names another entry in another
	// submenu; entries of one menu need distinct IDs to keep distinct keys.
	siblings := keys(MenuItem{ID: "one", Title: "One"}, MenuItem{ID: "two", Title: "Two"})
	nested := keys(MenuItem{ID: "sub", Title: "Sub", Submenu: []MenuItem{{ID: "one", Title: "One"}}})
	if siblings[0] == siblings[1] || nested[1] == siblings[0] {
		t.Fatalf("sibling IDs: %v, nested %v", siblings, nested)
	}
	// The length before each name keeps a title that reads like the marker of a
	// repeated entry from naming that entry.
	lookalike := keys(MenuItem{Title: "a#2"}, MenuItem{Title: "b"})
	repeated := keys(MenuItem{Title: "a"}, MenuItem{Title: "a"})
	if lookalike[0] == repeated[1] {
		t.Fatalf("a title that reads like a key named another entry: %v then %v", lookalike, repeated)
	}
	// A repeat's place among the repeats is part of its key: removing the last
	// one leaves the first as it was, and removing the first gives the one that
	// stays a new key, and so a new id.
	repeatedIDs := keys(MenuItem{ID: "same", Title: "Same"}, MenuItem{ID: "same", Title: "Same"})
	survivor := keys(MenuItem{ID: "same", Title: "Same"})
	if survivor[0] != repeatedIDs[0] {
		t.Fatalf("the first of two repeated IDs changed its key when the other went: %v then %v", repeatedIDs, survivor)
	}
	if survivor[0] == repeatedIDs[1] {
		t.Fatalf("the second of two repeated IDs kept its key after the first went: %v", repeatedIDs)
	}
}
