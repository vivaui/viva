package gfx

import (
	"fmt"
	"testing"
)

func TestTreeInternsPathsAndNames(t *testing.T) {
	var tree Tree
	page := tree.Feature("proxies")
	if again := tree.Feature("proxies"); again != page {
		t.Fatal("the same feature is a different scope the second time")
	}
	rows := tree.Sub(page, tree.Name("rows"))
	if tree.Sub(page, tree.Name("rows")) != rows {
		t.Fatal("the same path is a different scope the second time")
	}
	if tree.Sub(page, tree.Name("dialog")) == rows || tree.Feature("keys") == page {
		t.Fatal("different paths are the same scope")
	}
	// The same word under another page is another place.
	other := tree.Sub(tree.Feature("keys"), tree.Name("rows"))
	if other == rows {
		t.Fatal("two lists named rows on different pages are one")
	}
	if tree.Name("add") != tree.Name("add") || tree.Name("add") == tree.Name("remove") {
		t.Fatal("names are not interned")
	}
}

func TestIdentitiesAreExactForManyNames(t *testing.T) {
	// A 32-bit hash meets itself among a few hundred thousand names; interning
	// never does.
	var tree Tree
	page := tree.Feature("page")
	seen := make(map[ID]string)
	for index := 0; index < 200000; index++ {
		word := fmt.Sprintf("control-%d", index)
		id := page.ID(tree.Name(word))
		if previous, dup := seen[id]; dup {
			t.Fatalf("%q and %q are the same identity", previous, word)
		}
		seen[id] = word
	}
	sub := make(map[Scope]int)
	for index := 0; index < 50000; index++ {
		scope := tree.Sub(page, tree.Name(fmt.Sprintf("list-%d", index)))
		if previous, dup := sub[scope]; dup {
			t.Fatalf("lists %d and %d are the same scope", previous, index)
		}
		sub[scope] = index
	}
}

func TestItemsAndWordsNeverMeet(t *testing.T) {
	var tree Tree
	for index := 0; index < 5000; index++ {
		word := tree.Name(fmt.Sprintf("w%d", index))
		if _, isItem := word.Index(); isItem {
			t.Fatalf("word %d reads as an item", index)
		}
		if Index(index) == word {
			t.Fatalf("item %d equals a word", index)
		}
	}
	for _, i := range []int{0, 1, 99, 1 << 20, maxIndex} {
		got, ok := Index(i).Index()
		if !ok || got != i {
			t.Fatalf("Index(%d) came back as %d, %v", i, got, ok)
		}
	}
	for _, i := range []int{-1, maxIndex + 1} {
		if Index(i) != 0 {
			t.Fatalf("Index(%d) must be the zero name", i)
		}
	}
}

func TestIDRoundTripsItsScopeAndName(t *testing.T) {
	var tree Tree
	rows := tree.Sub(tree.Feature("hosts"), tree.Name("rows"))
	id := rows.ID(Index(41))
	if id.Scope() != rows {
		t.Fatal("scope lost")
	}
	if row, ok := id.Name().Index(); !ok || row != 41 {
		t.Fatalf("row = %d, %v", row, ok)
	}
	if id.IsZero() || !(ID{}).IsZero() {
		t.Fatal("only the zero ID is no target")
	}
	// Row 0 of a real list is a target.
	if rows.ID(Index(0)).IsZero() {
		t.Fatal("item 0 must be a target")
	}
}

func TestWithinFollowsTheTreeAtAnyDepth(t *testing.T) {
	var tree Tree
	page := tree.Feature("sftp")
	dialog := tree.Sub(page, tree.Name("dialog"))
	field := tree.Sub(dialog, tree.Name("fields"))
	keys := tree.Feature("keys")
	for _, test := range []struct {
		scope, ancestor Scope
		want            bool
	}{
		{field, page, true}, {field, dialog, true}, {field, field, true}, {dialog, field, false},
		{field, keys, false}, {keys, page, false}, {page, page, true}, {0, page, false}, {page, 0, false},
		{Scope(9999), page, false},
	} {
		if got := tree.Within(test.scope, test.ancestor); got != test.want {
			t.Errorf("Within(%s, %s) = %v, want %v", tree.Path(test.scope), tree.Path(test.ancestor), got, test.want)
		}
	}
	if tree.FeatureOf(field) != page || tree.Parent(field) != dialog || tree.Parent(page) != 0 {
		t.Fatal("parent or feature wrong")
	}
}

func TestPathNamesWhereAScopeIs(t *testing.T) {
	var tree Tree
	scope := tree.Sub(tree.Sub(tree.Feature("proxies"), tree.Name("dialog")), Index(7))
	if got := tree.Path(scope); got != "proxies/dialog/#7" {
		t.Fatalf("path = %q", got)
	}
	if tree.Path(0) != "" || tree.NameText(Index(0)) != "#0" || tree.NameText(Index(1234567890)) != "#1234567890" {
		t.Fatal("name text wrong")
	}
	if tree.Len() != 3 {
		t.Fatalf("len = %d", tree.Len())
	}
}

func TestUsingIdentitiesAllocatesNothing(t *testing.T) {
	var tree Tree
	page := tree.Feature("hosts")
	rows := tree.Sub(page, tree.Name("rows"))
	add := tree.Name("add")
	var sink int
	allocs := testing.AllocsPerRun(1000, func() {
		id := rows.ID(Index(sink & 1023))
		other := page.ID(add)
		if tree.Within(id.Scope(), page) && id != other {
			sink++
		}
		if scope := tree.Sub(page, tree.Name("rows")); scope == rows {
			sink++
		}
	})
	if allocs != 0 {
		t.Fatalf("%v allocations per use of identities", allocs)
	}
}

func BenchmarkRowIdentity(b *testing.B) {
	var tree Tree
	rows := tree.Sub(tree.Feature("hosts"), tree.Name("rows"))
	b.ReportAllocs()
	var sink int
	for i := 0; i < b.N; i++ {
		id := rows.ID(Index(i & 4095))
		if row, ok := id.Name().Index(); ok {
			sink += row
		}
	}
	_ = sink
}

func BenchmarkSubLookup(b *testing.B) {
	var tree Tree
	page := tree.Feature("hosts")
	name := tree.Name("rows")
	tree.Sub(page, name)
	b.ReportAllocs()
	var sink Scope
	for i := 0; i < b.N; i++ {
		sink = tree.Sub(page, name)
	}
	_ = sink
}

func BenchmarkWithinDeep(b *testing.B) {
	var tree Tree
	scope := tree.Feature("page")
	root := scope
	for index := 0; index < 6; index++ {
		scope = tree.Sub(scope, Index(index))
	}
	b.ReportAllocs()
	hits := 0
	for i := 0; i < b.N; i++ {
		if tree.Within(scope, root) {
			hits++
		}
	}
	_ = hits
}

func TestMountPlacesAViewInsideItsMountAndUndoes(t *testing.T) {
	var tree Tree
	page := tree.Feature("session")
	files := tree.Sub(page, tree.Name("files"))
	open := page.ID(tree.Name("open"))
	entry := files.ID(Index(3))

	paneA := tree.Sub(tree.Feature("workspace"), Index(1))
	paneB := tree.Sub(tree.Feature("workspace"), Index(2))
	a, b := tree.MountID(paneA, entry), tree.MountID(paneB, entry)
	if a == b || a == entry {
		t.Fatal("the same control in two panes must be two identities")
	}
	if !tree.Within(a.Scope(), paneA) || tree.Within(a.Scope(), paneB) || !tree.Within(b.Scope(), paneB) {
		t.Fatal("a mounted identity is inside its pane and only its pane")
	}
	if a.Name() != entry.Name() {
		t.Fatal("the item number must survive mounting")
	}
	if tree.MountID(paneA, entry) != a {
		t.Fatal("mounting twice must give the same identity")
	}
	local, ok := tree.Local(paneA, a)
	if !ok || local != entry {
		t.Fatalf("local = %v, %v", local, ok)
	}
	if _, ok := tree.Local(paneB, a); ok {
		t.Fatal("an identity of pane A is not one of pane B")
	}
	if _, ok := tree.Local(paneA, entry); ok {
		t.Fatal("an unmounted identity has no local form")
	}
	if got, _ := tree.Local(paneB, tree.MountID(paneB, open)); got != open {
		t.Fatal("a control directly on the page did not round trip")
	}
	if tree.FeatureOf(a.Scope()) != tree.FeatureOf(paneA) {
		t.Fatal("a mounted view belongs to the page that shows it")
	}
	if tree.MountID(0, entry) != entry || !tree.MountID(paneA, ID{}).IsZero() {
		t.Fatal("mounting at the root changes nothing and no target stays none")
	}
}

func TestNestedMountsUndoOneLevelAtATime(t *testing.T) {
	var tree Tree
	inner := tree.Sub(tree.Feature("inner"), tree.Name("list")).ID(Index(5))
	outerMount := tree.Sub(tree.Feature("window"), tree.Name("outer"))
	innerMount := tree.Sub(tree.Feature("window"), tree.Name("inner-mount"))
	atInner := tree.MountID(innerMount, inner)
	atOuter := tree.MountID(outerMount, atInner)
	if got, ok := tree.Local(outerMount, atOuter); !ok || got != atInner {
		t.Fatalf("outer undo = %v, %v", got, ok)
	}
	if got, ok := tree.Local(innerMount, atInner); !ok || got != inner {
		t.Fatalf("inner undo = %v, %v", got, ok)
	}
}

func BenchmarkMountID(b *testing.B) {
	var tree Tree
	id := tree.Sub(tree.Sub(tree.Feature("session"), tree.Name("files")), tree.Name("rows")).ID(Index(9))
	mount := tree.Sub(tree.Feature("workspace"), Index(1))
	tree.MountID(mount, id)
	b.ReportAllocs()
	var sink ID
	for i := 0; i < b.N; i++ {
		sink = tree.MountID(mount, id)
	}
	_ = sink
}

func TestIDInReadsTheItemOfOneList(t *testing.T) {
	var tree Tree
	rows := tree.Sub(tree.Feature("page"), tree.Name("rows"))
	other := tree.Sub(tree.Feature("page"), tree.Name("other"))
	if row, ok := rows.ID(Index(7)).In(rows); !ok || row != 7 {
		t.Fatalf("In = %d, %v; want 7, true", row, ok)
	}
	if _, ok := rows.ID(Index(7)).In(other); ok {
		t.Fatal("an item of one list answered for another")
	}
	if _, ok := rows.ID(tree.Name("header")).In(rows); ok {
		t.Fatal("a word was read as an item")
	}
	if _, ok := (ID{}).In(0); ok {
		t.Fatal("no target is not an item of no list")
	}
}

func TestDeclaringByWord(t *testing.T) {
	var tree Tree
	page := tree.Feature("page")
	rows := tree.Sub(page, tree.Name("rows"))
	if tree.ID(page, "add") != page.ID(tree.Name("add")) || tree.ID(page, "add") == tree.ID(page, "remove") {
		t.Fatal("a word does not name one control")
	}
	if rows.At(4) != rows.ID(Index(4)) {
		t.Fatal("At is not ID(Index)")
	}
	if got := tree.ScopeName(rows); got != tree.Name("rows") {
		t.Fatalf("ScopeName = %v", got)
	}
	mount := tree.Sub(page, Index(3))
	if n, ok := tree.ScopeName(mount).Index(); !ok || n != 3 {
		t.Fatalf("mount name = %d, %v", n, ok)
	}
}

func TestLocalOfTheZeroMountIsTheIdentityItself(t *testing.T) {
	var tree Tree
	id := tree.Feature("page").ID(tree.Name("go"))
	if got, ok := tree.Local(0, id); !ok || got != id {
		t.Fatalf("Local(0, id) = %v, %v", got, ok)
	}
	if _, ok := tree.Local(0, ID{}); ok {
		t.Fatal("no target is not an identity of the page")
	}
}

// Naming a control by its word at the call site costs one map lookup; a program
// that cares pre-interns the name once.
func BenchmarkIDByWord(b *testing.B) {
	var tree Tree
	page := tree.Feature("page")
	tree.ID(page, "save")
	b.ReportAllocs()
	b.ResetTimer()
	var id ID
	for i := 0; i < b.N; i++ {
		id = tree.ID(page, "save")
	}
	_ = id
}

func BenchmarkIDByInternedName(b *testing.B) {
	var tree Tree
	page := tree.Feature("page")
	save := tree.Name("save")
	b.ReportAllocs()
	b.ResetTimer()
	var id ID
	for i := 0; i < b.N; i++ {
		id = page.ID(save)
	}
	_ = id
}

// Whether a paint listens for clicks is a property of that paint: a page that
// stops asking Clicked must stop costing an event pass.
func TestListeningIsPerPaint(t *testing.T) {
	var canvas Canvas
	var tree Tree
	id := tree.ID(tree.Feature("page"), "go")
	canvas.Reset()
	canvas.Clicked(id)
	if !canvas.ListensForClicks() {
		t.Fatal("asking Clicked did not mark the paint as listening")
	}
	canvas.Reset()
	if canvas.ListensForClicks() {
		t.Fatal("a paint that never asked still listens because an earlier one did")
	}
}

// A central dispatcher reads ClickTarget alone; that must also make the runtime
// schedule the event pass, or its clicks are skipped.
func TestClickTargetListensAndNamesTheClickedControl(t *testing.T) {
	var canvas Canvas
	var tree Tree
	id := tree.ID(tree.Feature("page"), "go")
	canvas.Reset()
	if _, ok := canvas.ClickTarget(); ok || !canvas.ListensForClicks() {
		t.Fatal("an ordinary paint must report no target and still listen")
	}
	canvas.BeginEventPass(id)
	got, ok := canvas.ClickTarget()
	canvas.EndEventPass()
	if !ok || got != id {
		t.Fatal("the event pass does not name the clicked control")
	}
}

// AfterPaint only defers in the event pass, runs in order, and runs once.
func TestAfterPaintDefersOnlyInTheEventPass(t *testing.T) {
	var canvas Canvas
	var tree Tree
	id := tree.ID(tree.Feature("page"), "go")
	var order []int
	canvas.Reset()
	canvas.AfterPaint(func() { order = append(order, 0) })
	canvas.RunAfterPaint()
	if len(order) != 0 {
		t.Fatal("an ordinary paint deferred a function")
	}
	canvas.BeginEventPass(id)
	canvas.AfterPaint(func() { order = append(order, 1) })
	canvas.AfterPaint(func() { order = append(order, 2) })
	canvas.EndEventPass()
	canvas.RunAfterPaint()
	canvas.RunAfterPaint()
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("deferred functions ran as %v, want [1 2] once", order)
	}
}
