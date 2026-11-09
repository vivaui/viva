package platform

import "strconv"

// MenuEntry is one MenuItem in the order a native menu is built: the entries of
// a menu, each followed by its submenu's entries, depth first. Its ID is its
// position in that order, counting from 1, so the number a platform hands back
// when the user chooses an entry leads straight to it. Key names the entry
// across builds, so a menu served to another process can keep the id it gave an
// entry while the entry's title, state or position changes. An entry the
// application gave an ID keeps its key; one it did not is known by its title, so
// a new title is a new entry. An application must keep an ID unique among the
// entries of one menu: repeats are told apart by their order, and removing one
// of them gives the repeats after it a new key, and a new id.
type MenuEntry struct {
	ID        int
	Key       string
	Title     string
	Separator bool
	Disabled  bool
	Checked   bool
	// Children is how many entries the submenu has directly. They follow this
	// entry, each with its own submenu's entries after it.
	Children int
	OnSelect func()
}

// NumberMenu lays a menu out as entries, ID 1 first.
func NumberMenu(items []MenuItem) []MenuEntry {
	entries := make([]MenuEntry, 0, countMenu(items))
	return appendMenu(entries, items, "")
}

func countMenu(items []MenuItem) int {
	count := len(items)
	for index := range items {
		count += countMenu(items[index].Submenu)
	}
	return count
}

func appendMenu(entries []MenuEntry, items []MenuItem, parent string) []MenuEntry {
	seen := make(map[string]int, len(items))
	for index := range items {
		item := &items[index]
		base := menuKeyBase(item)
		seen[base]++
		// The length keeps two different bases from reading as one key when a
		// title contains the slash between them. The parent's key comes first,
		// so an entry that moves to another submenu is another entry, and one of
		// several entries of the same name is told apart by its place among them.
		key := parent + "/" + strconv.Itoa(len(base)) + ":" + base
		if seen[base] > 1 {
			key += "#" + strconv.Itoa(seen[base])
		}
		entry := MenuEntry{
			ID: len(entries) + 1, Key: key, Title: item.Title, Separator: item.Separator,
			Disabled: item.Disabled, Checked: item.Checked, Children: len(item.Submenu), OnSelect: item.OnSelect,
		}
		if item.Separator {
			entry.Disabled, entry.Checked, entry.Children, entry.OnSelect = false, false, 0, nil
			entries = append(entries, entry)
			continue
		}
		entries = append(entries, entry)
		entries = appendMenu(entries, item.Submenu, key)
	}
	return entries
}

// menuKeyBase is what an entry is known by across builds: the ID the
// application gave it, or else what it looks like. An entry without an ID that
// changes its title therefore takes a new key, and a new id: the application
// did not say it was the same entry, and handing its id to whatever takes its
// place could run that entry's handler for it. An ID repeated among the entries
// of one menu gives both the same base; they are told apart by their order, so
// removing one of them gives the repeats after it a new key, which is why the
// application must keep an ID unique among the entries of one menu.
func menuKeyBase(item *MenuItem) string {
	switch {
	case item.ID != "":
		return "i" + item.ID
	case item.Separator:
		return "s"
	default:
		return "t" + item.Title
	}
}

// Select runs the handler of the entry with this ID, if it has one that the
// user can reach: not a separator, not disabled and not a submenu's parent.
func Select(entries []MenuEntry, id int) {
	if id < 1 || id > len(entries) {
		return
	}
	entry := entries[id-1]
	if entry.OnSelect != nil && !entry.Separator && !entry.Disabled && entry.Children == 0 {
		entry.OnSelect()
	}
}

// SubtreeSize is how many entries follow entries[index] as its submenu, at
// every depth.
func SubtreeSize(entries []MenuEntry, index int) int {
	size := 0
	for child, remaining := index+1, entries[index].Children; remaining > 0; remaining-- {
		below := 1 + SubtreeSize(entries, child)
		size += below
		child += below
	}
	return size
}

// TopLevel is how many entries the menu itself has, not counting its submenus'.
func TopLevel(entries []MenuEntry) int {
	count := 0
	for index := 0; index < len(entries); index += 1 + SubtreeSize(entries, index) {
		count++
	}
	return count
}
