package dbusmenu

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
)

type node struct {
	id       int32
	props    map[string]any
	children []node
}

// readNode decodes one (ia{sv}av) the way a host does.
func readNode(t *testing.T, d *dbus.Decoder) node {
	t.Helper()
	d.Align(8)
	n := node{id: d.Int32(), props: map[string]any{}}
	end := d.ArrayLength(8)
	for d.Pos() < end && d.Err() == nil {
		d.Align(8)
		key := d.String()
		switch sig := d.Signature(); sig {
		case "s":
			n.props[key] = d.String()
		case "b":
			n.props[key] = d.Bool()
		case "i":
			n.props[key] = d.Int32()
		default:
			t.Fatalf("property %q has signature %q", key, sig)
		}
	}
	end = d.ArrayLength(1)
	for d.Pos() < end && d.Err() == nil {
		if sig := d.Signature(); sig != nodeType {
			t.Fatalf("child signature %q", sig)
		}
		n.children = append(n.children, readNode(t, d))
	}
	if d.Err() != nil {
		t.Fatal(d.Err())
	}
	return n
}

// propertyUpdate is one entry of an ItemsPropertiesUpdated signal.
type propertyUpdate struct {
	id    int32
	props map[string]any
}

// readItemsUpdated decodes the body of an ItemsPropertiesUpdated signal.
func readItemsUpdated(t *testing.T, body []byte) []propertyUpdate {
	t.Helper()
	if body == nil {
		t.Fatal("no ItemsPropertiesUpdated body")
	}
	d := dbus.NewDecoder(body)
	end := d.ArrayLength(8)
	var updates []propertyUpdate
	for d.Pos() < end && d.Err() == nil {
		d.Align(8)
		update := propertyUpdate{id: d.Int32(), props: map[string]any{}}
		propsEnd := d.ArrayLength(8)
		for d.Pos() < propsEnd && d.Err() == nil {
			d.Align(8)
			name := d.String()
			switch sig := d.Signature(); sig {
			case "s":
				update.props[name] = d.String()
			case "b":
				update.props[name] = d.Bool()
			case "i":
				update.props[name] = d.Int32()
			default:
				t.Fatalf("property %q has signature %q", name, sig)
			}
		}
		updates = append(updates, update)
	}
	removed := d.ArrayLength(8)
	if d.Pos() != removed {
		t.Fatal("a property removal was written")
	}
	if d.Err() != nil {
		t.Fatal(d.Err())
	}
	return updates
}

func call(member, signature string, args func(e *dbus.Encoder)) *dbus.Message {
	var e dbus.Encoder
	if args != nil {
		args(&e)
	}
	return &dbus.Message{Type: dbus.TypeCall, Path: Path, Interface: Interface, Member: member, Signature: signature, Body: e.Bytes()}
}

func aboutToShow(id int32) *dbus.Message {
	return call("AboutToShow", "i", func(e *dbus.Encoder) { e.Int32(id) })
}

func aboutToShowGroup(ids ...int32) *dbus.Message {
	return call("AboutToShowGroup", "ai", func(e *dbus.Encoder) {
		mark := e.BeginArray(4)
		for _, id := range ids {
			e.Int32(id)
		}
		e.EndArray(mark)
	})
}

func layoutCall(parent, depth int32, names ...string) *dbus.Message {
	return call("GetLayout", "iias", func(e *dbus.Encoder) {
		e.Int32(parent)
		e.Int32(depth)
		mark := e.BeginArray(4)
		for _, name := range names {
			e.String(name)
		}
		e.EndArray(mark)
	})
}

func newTestObject(chosen *[]string, builds *int) *Object {
	pick := func(name string) func() { return func() { *chosen = append(*chosen, name) } }
	return New(func() []platform.MenuItem {
		*builds++
		return []platform.MenuItem{
			{Title: "Header", Disabled: true},
			{Title: "Sessions", Submenu: []platform.MenuItem{
				{Title: "alpha", OnSelect: pick("alpha")},
				{Title: "beta", OnSelect: pick("beta")},
			}},
			{Separator: true},
			{Title: "New_terminal", Checked: true, OnSelect: pick("terminal")},
			{Title: "Quit", OnSelect: pick("quit")},
		}
	})
}

func succeeded(t *testing.T, result Result) []byte {
	t.Helper()
	if result.ErrName != "" {
		t.Fatalf("error %s: %s", result.ErrName, result.ErrText)
	}
	return result.Body
}

func TestLayoutIsTheWholeMenuWithItsPropertiesAndIDsInEntryOrder(t *testing.T) {
	var chosen []string
	var builds int
	object := newTestObject(&chosen, &builds)
	result := object.Call(layoutCall(0, -1))
	if result.Signature != "u"+nodeType {
		t.Fatalf("signature %q", result.Signature)
	}
	d := dbus.NewDecoder(succeeded(t, result))
	if revision := d.Uint32(); revision != 1 {
		t.Fatalf("revision %d", revision)
	}
	root := readNode(t, d)
	if !d.Done() {
		t.Fatal("bytes left over after the layout")
	}
	if root.id != 0 || root.props["children-display"] != "submenu" || len(root.children) != 5 {
		t.Fatalf("root = %+v", root)
	}
	titles := []string{}
	for _, child := range root.children {
		label, _ := child.props["label"].(string)
		titles = append(titles, label)
	}
	if !reflect.DeepEqual(titles, []string{"Header", "Sessions", "", "New__terminal", "Quit"}) {
		t.Fatalf("labels %q", titles)
	}
	header, sessions, separator, terminal, quit := root.children[0], root.children[1], root.children[2], root.children[3], root.children[4]
	if header.id != 1 || header.props["enabled"] != false {
		t.Fatalf("header %+v", header)
	}
	if sessions.id != 2 || sessions.props["children-display"] != "submenu" || len(sessions.children) != 2 || sessions.children[0].id != 3 || sessions.children[1].id != 4 {
		t.Fatalf("sessions %+v", sessions)
	}
	if separator.id != 5 || separator.props["type"] != "separator" {
		t.Fatalf("separator %+v", separator)
	}
	if terminal.id != 6 || terminal.props["toggle-type"] != "checkmark" || terminal.props["toggle-state"] != int32(1) {
		t.Fatalf("terminal %+v", terminal)
	}
	// Every property is stated, defaults included, so a host that keeps what it
	// was told does not carry a separator's look over to the entry that takes
	// its id when the menu is built again.
	want := map[string]any{"type": "standard", "label": "Quit", "enabled": true, "visible": true, "toggle-type": "", "toggle-state": int32(0), "children-display": ""}
	if quit.id != 7 || !reflect.DeepEqual(quit.props, want) {
		t.Fatalf("quit = %+v, want %+v", quit, want)
	}
	if separator.props["label"] != "" || separator.props["enabled"] != true {
		t.Fatalf("a separator leaves a property unsaid: %+v", separator.props)
	}
}

func TestLayoutDepthAndParentSelectWhatIsWritten(t *testing.T) {
	var chosen []string
	var builds int
	object := newTestObject(&chosen, &builds)
	ids := layoutIDs(t, object)

	d := dbus.NewDecoder(succeeded(t, object.Call(layoutCall(0, 1))))
	d.Uint32()
	flat := readNode(t, d)
	if len(flat.children) != 5 || len(flat.children[1].children) != 0 {
		t.Fatalf("depth 1 wrote grandchildren: %+v", flat)
	}
	d = dbus.NewDecoder(succeeded(t, object.Call(layoutCall(0, 0))))
	d.Uint32()
	if bare := readNode(t, d); len(bare.children) != 0 {
		t.Fatalf("depth 0 wrote children: %+v", bare)
	}
	sessions := ids[1]
	d = dbus.NewDecoder(succeeded(t, object.Call(layoutCall(sessions, -1))))
	d.Uint32()
	sub := readNode(t, d)
	if sub.id != sessions || len(sub.children) != 2 || sub.children[1].props["label"] != "beta" {
		t.Fatalf("submenu %+v", sub)
	}
	if r := object.Call(layoutCall(99, -1)); r.ErrName == "" {
		t.Fatal("an unknown parent was answered")
	}
	d = dbus.NewDecoder(succeeded(t, object.Call(layoutCall(0, -1, "label"))))
	d.Uint32()
	only := readNode(t, d)
	if _, set := only.props["children-display"]; set || only.children[0].props["label"] != "Header" {
		t.Fatalf("property filter ignored: %+v", only)
	}
	if len(only.children[0].props) != 1 {
		t.Fatalf("properties that were not asked for were written: %+v", only.children[0].props)
	}
}

func TestOpeningTheMenuAsksTheApplicationAgain(t *testing.T) {
	var chosen []string
	var builds int
	object := newTestObject(&chosen, &builds)
	first := layoutIDs(t, object)
	if builds != 1 {
		t.Fatalf("builds %d", builds)
	}
	// A host that says it is about to show the menu asks the application
	// again. The menu did not change, so no id moved and no layout is
	// announced.
	result := object.Call(aboutToShow(0))
	d := dbus.NewDecoder(succeeded(t, result))
	if d.Bool() || builds != 2 {
		t.Fatalf("AboutToShow(0) said the layout changed: builds %d", builds)
	}
	if result.LayoutUpdated != 0 || result.ItemsUpdated != nil {
		t.Fatalf("AboutToShow(0) announced a change: revision %d, items %d", result.LayoutUpdated, len(result.ItemsUpdated))
	}
	second := layoutIDs(t, object)
	if !reflect.DeepEqual(first, second) || builds != 2 {
		t.Fatalf("ids %v then %v, builds %d", first, second, builds)
	}
	// A layout read with no announcement is a host that kept the old one.
	object.Call(layoutCall(0, -1))
	if builds != 3 {
		t.Fatalf("builds %d", builds)
	}
	result = object.Call(aboutToShow(2))
	d = dbus.NewDecoder(succeeded(t, result))
	if d.Bool() || builds != 3 {
		t.Fatal("a submenu's opening rebuilt the menu under its own ids")
	}
}

func event(id int32, kind string) *dbus.Message {
	return call("Event", "isvu", func(e *dbus.Encoder) {
		e.Int32(id)
		e.String(kind)
		e.Variant("s")
		e.String("payload")
		e.Uint32(1234)
	})
}

func TestClickingAnEntryRunsOnlyThatEntry(t *testing.T) {
	var chosen []string
	var builds int
	object := newTestObject(&chosen, &builds)
	object.Call(layoutCall(0, -1))
	for _, message := range []*dbus.Message{
		event(4, "hovered"), event(1, "clicked"), event(2, "clicked"), event(5, "clicked"),
		event(4, "clicked"), event(7, "clicked"), event(99, "clicked"), event(-1, "clicked"),
	} {
		if r := object.Call(message); r.ErrName != "" || r.Body != nil {
			t.Fatalf("Event answered %+v", r)
		}
	}
	if !reflect.DeepEqual(chosen, []string{"beta", "quit"}) {
		t.Fatalf("chosen %v", chosen)
	}
	group := call("EventGroup", "a(isvu)", func(e *dbus.Encoder) {
		mark := e.BeginArray(8)
		for _, item := range []struct {
			id   int32
			kind string
		}{{3, "clicked"}, {50, "clicked"}} {
			e.Align(8)
			e.Int32(item.id)
			e.String(item.kind)
			e.Variant("i")
			e.Int32(0)
			e.Uint32(0)
		}
		e.EndArray(mark)
	})
	d := dbus.NewDecoder(succeeded(t, object.Call(group)))
	end := d.ArrayLength(4)
	var failed []int32
	for d.Pos() < end {
		failed = append(failed, d.Int32())
	}
	if !reflect.DeepEqual(failed, []int32{50}) || !reflect.DeepEqual(chosen, []string{"beta", "quit", "alpha"}) {
		t.Fatalf("failed %v chosen %v", failed, chosen)
	}
}

func TestPropertiesOfEntriesAreReadableOnTheirOwn(t *testing.T) {
	var chosen []string
	var builds int
	object := newTestObject(&chosen, &builds)
	object.Call(layoutCall(0, -1))

	d := dbus.NewDecoder(succeeded(t, object.Call(call("GetProperty", "is", func(e *dbus.Encoder) { e.Int32(3); e.String("label") }))))
	if sig := d.Signature(); sig != "s" || d.String() != "alpha" {
		t.Fatal("GetProperty label")
	}
	if r := object.Call(call("GetProperty", "is", func(e *dbus.Encoder) { e.Int32(3); e.String("icon-name") })); r.ErrName == "" {
		t.Fatal("a property the menu does not have was reported")
	}
	group := call("GetGroupProperties", "aias", func(e *dbus.Encoder) {
		ids := e.BeginArray(4)
		e.Int32(1)
		e.Int32(5)
		e.EndArray(ids)
		e.EndArray(e.BeginArray(4))
	})
	d = dbus.NewDecoder(succeeded(t, object.Call(group)))
	end := d.ArrayLength(8)
	got := map[int32]int{}
	for d.Pos() < end && d.Err() == nil {
		d.Align(8)
		id := d.Int32()
		propEnd := d.ArrayLength(8)
		for d.Pos() < propEnd {
			d.Align(8)
			_ = d.String()
			d.Skip(d.Signature())
			got[id]++
		}
	}
	if d.Err() != nil || got[1] != len(propNames) || got[5] != len(propNames) {
		t.Fatalf("group properties %v err %v", got, d.Err())
	}

	properties := func(member string, args func(*dbus.Encoder)) *dbus.Message {
		message := call(member, "", args)
		message.Interface = "org.freedesktop.DBus.Properties"
		return message
	}
	all := properties("GetAll", func(e *dbus.Encoder) { e.String(Interface) })
	d = dbus.NewDecoder(succeeded(t, object.Call(all)))
	end = d.ArrayLength(8)
	names := []string{}
	for d.Pos() < end && d.Err() == nil {
		d.Align(8)
		names = append(names, d.String())
		d.Skip(d.Signature())
	}
	if !reflect.DeepEqual(names, []string{"Version", "TextDirection", "Status", "IconThemePath"}) {
		t.Fatalf("properties %v", names)
	}
	d = dbus.NewDecoder(succeeded(t, object.Call(properties("Get", func(e *dbus.Encoder) { e.String(Interface); e.String("Version") }))))
	if sig := d.Signature(); sig != "u" || d.Uint32() != 3 {
		t.Fatal("Version is not 3")
	}
	if r := object.Call(properties("Set", nil)); r.ErrName == "" {
		t.Fatal("a property was set")
	}
}

func TestMalformedAndUnknownCallsAreRefused(t *testing.T) {
	var chosen []string
	var builds int
	object := newTestObject(&chosen, &builds)
	for _, message := range []*dbus.Message{
		call("Nonsense", "", nil),
		call("GetLayout", "i", func(e *dbus.Encoder) { e.Int32(0) }),
		call("Event", "isvu", func(e *dbus.Encoder) { e.Int32(1) }),
		{Interface: "org.example.Other", Member: "X", Path: Path},
	} {
		if r := object.Call(message); r.ErrName == "" {
			t.Fatalf("%s %s was answered", message.Interface, message.Member)
		}
	}
}

func TestAnEmptyMenuHasAnEmptyLayout(t *testing.T) {
	object := New(func() []platform.MenuItem { return nil })
	d := dbus.NewDecoder(succeeded(t, object.Call(layoutCall(0, -1))))
	d.Uint32()
	root := readNode(t, d)
	if root.id != 0 || len(root.children) != 0 || root.props["children-display"] != "" {
		t.Fatalf("root %+v", root)
	}
}

func FuzzCallNeverPanicsOnAnyBody(f *testing.F) {
	members := []string{"GetLayout", "GetGroupProperties", "GetProperty", "Event", "EventGroup", "AboutToShow", "AboutToShowGroup"}
	signatures := []string{"iias", "aias", "is", "isvu", "a(isvu)", "i", "ai", ""}
	f.Add(uint8(0), uint8(0), []byte{0, 0, 0, 0, 255, 255, 255, 255, 0, 0, 0, 0})
	f.Add(uint8(4), uint8(4), []byte{4, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3})
	f.Fuzz(func(t *testing.T, member, signature uint8, body []byte) {
		object := New(func() []platform.MenuItem {
			return []platform.MenuItem{{Title: "a", OnSelect: func() {}}, {Title: "b", Submenu: []platform.MenuItem{{Title: "c", OnSelect: func() {}}}}}
		})
		object.Call(&dbus.Message{
			Path: Path, Interface: Interface, Member: members[int(member)%len(members)],
			Signature: signatures[int(signature)%len(signatures)], Body: body,
		})
	})
}

// layoutIDs reads the whole menu and returns the ids it wrote, in order.
func layoutIDs(t *testing.T, object *Object) []int32 {
	t.Helper()
	d := dbus.NewDecoder(succeeded(t, object.Call(layoutCall(0, -1))))
	d.Uint32()
	var found []int32
	var walk func(n node)
	walk = func(n node) {
		if n.id != 0 {
			found = append(found, n.id)
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(readNode(t, d))
	return found
}

// newSessionMenu is a menu whose sessions a test can add to between builds.
func newSessionMenu(chosen *[]string, sessions *[]string) *Object {
	return New(func() []platform.MenuItem {
		items := []platform.MenuItem{{Title: "Sessions", Disabled: true}}
		for _, name := range *sessions {
			name := name
			items = append(items, platform.MenuItem{Title: name, OnSelect: func() { *chosen = append(*chosen, name) }})
		}
		return append(items, platform.MenuItem{Title: "Quit", OnSelect: func() { *chosen = append(*chosen, "quit") }})
	})
}

func TestAnUnchangedMenuKeepsTheIDsAHostAlreadyRead(t *testing.T) {
	var chosen []string
	var builds int
	object := newTestObject(&chosen, &builds)
	first := layoutIDs(t, object)

	result := object.Call(aboutToShowGroup(0))
	d := dbus.NewDecoder(succeeded(t, result))
	if end := d.ArrayLength(4); d.Pos() != end {
		t.Fatal("AboutToShowGroup said an unchanged menu had updates")
	}
	if result.LayoutUpdated != 0 || result.ItemsUpdated != nil {
		t.Fatalf("an unchanged menu announced revision %d with %d items", result.LayoutUpdated, len(result.ItemsUpdated))
	}
	second := layoutIDs(t, object)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("an unchanged menu changed its ids: %v then %v", first, second)
	}
	object.Call(event(first[2], "clicked"))
	if !reflect.DeepEqual(chosen, []string{"alpha"}) {
		t.Fatalf("chosen %v", chosen)
	}
}

func TestAnAddedEntryTakesANewIDAndTheOthersKeepTheirs(t *testing.T) {
	var chosen []string
	sessions := []string{"alpha"}
	object := newSessionMenu(&chosen, &sessions)
	first := layoutIDs(t, object)
	if len(first) != 3 {
		t.Fatalf("ids %v", first)
	}

	sessions = append(sessions, "beta")
	result := object.Call(aboutToShow(0))
	d := dbus.NewDecoder(succeeded(t, result))
	if !d.Bool() {
		t.Fatal("AboutToShow did not report the changed layout")
	}
	if result.LayoutUpdated == 0 {
		t.Fatal("a changed layout was not announced")
	}
	second := layoutIDs(t, object)
	if len(second) != 4 {
		t.Fatalf("ids %v then %v", first, second)
	}
	// The entries the menu still lists keep the ids hosts already hold,
	// whatever position the new entry took between them.
	if second[0] != first[0] || second[1] != first[1] || second[3] != first[2] {
		t.Fatalf("an entry that is still listed changed its id: %v then %v", first, second)
	}
	for _, id := range first {
		if second[2] == id {
			t.Fatalf("the new entry reused the id %d: %v then %v", id, first, second)
		}
	}
	// The ids a host holds go on choosing the entries they named, and the new
	// id chooses the new entry.
	object.Call(event(first[1], "clicked"))
	if !reflect.DeepEqual(chosen, []string{"alpha"}) {
		t.Fatalf("chosen %v", chosen)
	}
	object.Call(event(second[2], "clicked"))
	if !reflect.DeepEqual(chosen, []string{"alpha", "beta"}) {
		t.Fatalf("chosen %v", chosen)
	}
	object.Call(event(first[2], "clicked"))
	if !reflect.DeepEqual(chosen, []string{"alpha", "beta", "quit"}) {
		t.Fatalf("chosen %v", chosen)
	}
}

func TestAPropertyChangeKeepsTheIDsAndIsAnnouncedInPlace(t *testing.T) {
	var chosen []string
	title, disabled := "Connecting", false
	object := New(func() []platform.MenuItem {
		return []platform.MenuItem{
			{ID: "status", Title: title, Disabled: disabled, OnSelect: func() {}},
			{ID: "quit", Title: "Quit", OnSelect: func() { chosen = append(chosen, "quit") }},
		}
	})
	first := layoutIDs(t, object)

	title, disabled = "Connected", true
	result := object.Call(aboutToShow(0))
	d := dbus.NewDecoder(succeeded(t, result))
	if d.Bool() {
		t.Fatal("AboutToShow asked for a re-read for a property change")
	}
	if result.LayoutUpdated != 0 {
		t.Fatalf("a property change announced revision %d", result.LayoutUpdated)
	}
	updates := readItemsUpdated(t, result.ItemsUpdated)
	if len(updates) != 1 || updates[0].id != first[0] {
		t.Fatalf("updates %+v, ids %v", updates, first)
	}
	if updates[0].props["label"] != "Connected" || updates[0].props["enabled"] != false {
		t.Fatalf("properties %+v", updates[0].props)
	}
	second := layoutIDs(t, object)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("a property change moved the ids: %v then %v", first, second)
	}
	object.Call(event(first[1], "clicked"))
	if !reflect.DeepEqual(chosen, []string{"quit"}) {
		t.Fatalf("chosen %v", chosen)
	}
}

func TestAnEntryWithoutIDThatChangesItsTitleTakesANewID(t *testing.T) {
	title := "one"
	object := New(func() []platform.MenuItem {
		return []platform.MenuItem{
			{Title: title, OnSelect: func() {}},
			{Title: "Quit", OnSelect: func() {}},
		}
	})
	first := layoutIDs(t, object)
	title = "two"
	result := object.Call(aboutToShow(0))
	d := dbus.NewDecoder(succeeded(t, result))
	if !d.Bool() || result.LayoutUpdated == 0 {
		t.Fatal("an entry the application did not name changed under its id")
	}
	second := layoutIDs(t, object)
	if second[0] == first[0] || second[1] != first[1] {
		t.Fatalf("ids %v then %v", first, second)
	}
}

func TestAnEntryThatChangesKindTakesANewID(t *testing.T) {
	standard := false
	object := New(func() []platform.MenuItem {
		items := []platform.MenuItem{{ID: "first", Title: "First", OnSelect: func() {}}}
		if standard {
			items = append(items, platform.MenuItem{ID: "thing", Title: "Thing", OnSelect: func() {}})
		} else {
			items = append(items, platform.MenuItem{ID: "thing", Separator: true})
		}
		return append(items, platform.MenuItem{ID: "last", Title: "Last", OnSelect: func() {}})
	})
	first := layoutIDs(t, object)
	standard = true
	result := object.Call(aboutToShow(0))
	d := dbus.NewDecoder(succeeded(t, result))
	if !d.Bool() || result.LayoutUpdated == 0 {
		t.Fatal("an entry that became selectable was not a layout change")
	}
	second := layoutIDs(t, object)
	if second[0] != first[0] || second[2] != first[2] {
		t.Fatalf("an entry that kept its kind changed its id: %v then %v", first, second)
	}
	if second[1] == first[1] {
		t.Fatalf("the id of an entry that changed kind was reused: %v then %v", first, second)
	}
}

func TestReorderingKeepsEachEntrysID(t *testing.T) {
	order := []string{"a", "b", "c"}
	object := New(func() []platform.MenuItem {
		items := make([]platform.MenuItem, 0, len(order))
		for _, name := range order {
			items = append(items, platform.MenuItem{ID: name, Title: name, OnSelect: func() {}})
		}
		return items
	})
	first := layoutIDs(t, object)
	order = []string{"c", "a", "b"}
	result := object.Call(aboutToShow(0))
	if result.LayoutUpdated == 0 {
		t.Fatal("a reordered menu was not announced")
	}
	second := layoutIDs(t, object)
	if !reflect.DeepEqual(second, []int32{first[2], first[0], first[1]}) {
		t.Fatalf("ids %v then %v", first, second)
	}
}

func TestIdsOfRemovedEntriesAreNotReused(t *testing.T) {
	items := []string{"a", "b"}
	object := New(func() []platform.MenuItem {
		out := make([]platform.MenuItem, 0, len(items))
		for _, name := range items {
			out = append(out, platform.MenuItem{Title: name, OnSelect: func() {}})
		}
		return out
	})
	first := layoutIDs(t, object)
	items = []string{"a"}
	object.Call(aboutToShow(0))
	items = []string{"a", "c"}
	object.Call(aboutToShow(0))
	third := layoutIDs(t, object)
	if len(third) != 2 || third[1] == first[1] {
		t.Fatalf("the id of a removed entry was reused: %v then %v", first, third)
	}
}

func TestAReadAnnouncesPropertiesOnce(t *testing.T) {
	title := "one"
	object := New(func() []platform.MenuItem {
		return []platform.MenuItem{{ID: "a", Title: title, OnSelect: func() {}}}
	})
	layoutIDs(t, object)
	title = "two"
	result := object.Call(layoutCall(0, -1))
	if result.LayoutUpdated != 0 {
		t.Fatalf("a property change announced revision %d", result.LayoutUpdated)
	}
	updates := readItemsUpdated(t, result.ItemsUpdated)
	if len(updates) != 1 || updates[0].props["label"] != "two" {
		t.Fatalf("updates %+v", updates)
	}
	again := object.Call(layoutCall(0, -1))
	if again.ItemsUpdated != nil {
		t.Fatal("the read that answered the update announced it again")
	}
}

func TestAboutToShowGroupAnnouncesAChangedLayout(t *testing.T) {
	var chosen []string
	sessions := []string{"alpha"}
	object := newSessionMenu(&chosen, &sessions)
	layoutIDs(t, object)

	sessions = append(sessions, "beta")
	result := object.Call(aboutToShowGroup(0))
	d := dbus.NewDecoder(succeeded(t, result))
	end := d.ArrayLength(4)
	if d.Pos() == end {
		t.Fatal("AboutToShowGroup did not report the updated root")
	}
	if id := d.Int32(); id != 0 {
		t.Fatalf("updated id %d, want the root", id)
	}
	if result.LayoutUpdated == 0 {
		t.Fatal("a changed layout was not announced")
	}
}

func TestAReadThatRebuildsAChangedMenuAnnouncesIt(t *testing.T) {
	var chosen []string
	sessions := []string{"alpha"}
	object := newSessionMenu(&chosen, &sessions)
	layoutIDs(t, object)

	sessions = append(sessions, "beta")
	// No AboutToShow: the read itself notices the change, and hosts that kept
	// the earlier layout are told to read this one.
	result := object.Call(layoutCall(0, -1))
	if result.LayoutUpdated == 0 {
		t.Fatal("a layout read that rebuilt the menu announced nothing")
	}
	d := dbus.NewDecoder(succeeded(t, result))
	if revision := d.Uint32(); revision != result.LayoutUpdated {
		t.Fatalf("reply revision %d, announced %d", revision, result.LayoutUpdated)
	}
	// The read that answers the announcement finds the same menu and does not
	// announce it again.
	again := object.Call(layoutCall(0, -1))
	if again.LayoutUpdated != 0 {
		t.Fatalf("the answering read announced revision %d", again.LayoutUpdated)
	}
}
