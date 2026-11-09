package gfx

// Identity names the things a user can point at, focus and tab to. It is a
// path, not a number: a control is identified by where it sits (the page it is
// on, the list it belongs to) and by a name within that place. A feature names
// its own controls, two features cannot hand out the same one, and nothing is
// numbered by hand.
//
//	var tree gfx.Tree                                  // the application's, one
//	var proxies = tree.Feature("proxies")             // a page
//	var rows = tree.Sub(proxies, tree.Name("rows"))   // a list on it
//	var add = proxies.ID(tree.Name("add"))            // a control
//
//	canvas.Hit(add, bounds)
//	canvas.Hit(rows.ID(gfx.Index(i)), rowBounds)       // row i
//
//	if id.Scope() == rows { row, _ := id.Name().Index() } // a click on a row
//	if tree.Within(id.Scope(), proxies) { ... }           // anything on the page
//
// Paths and names are interned: the same path is always the same Scope and the
// same name the same Name, so identities are exact. Two different paths or names
// can never be confused, which a hash cannot promise. Using an item's identity
// allocates nothing; a scope or a name is made once, when the program starts or
// a list first appears, and is found again by a table lookup.

// Name names one control or item within a scope: a word (Tree.Name) for a
// control, or the number of an item (Index) for a row of a list.
type Name uint32

const (
	// wordBit marks a Name made from a word, so no word equals the number of an
	// item. Items are numbered below 1<<31.
	wordBit  Name = 1 << 31
	maxIndex      = 1<<31 - 1
)

// Index returns the name of item number i of a list: a row, a tab, the record
// with a given database number. A negative number, or one of 2^31 or more, is
// not an item and returns the zero name, which belongs to no list.
func Index(i int) Name {
	if i < 0 || i > maxIndex {
		return 0
	}
	return Name(i)
}

// Index returns the item number of an item name, and false for a word.
func (name Name) Index() (int, bool) {
	if name&wordBit != 0 {
		return 0, false
	}
	return int(name), true
}

// Scope is a place in the identity tree: a page, a dialog, a list. The zero
// Scope is no place.
type Scope uint32

// itemsScope is the anonymous scope of Item. Every Tree reserves it as its first
// place, so identities made without a tree can be mounted into one.
const itemsScope Scope = 1

// Item returns the identity of item n in the anonymous scope: for examples,
// previews and tests that need a few identities and no tree of their own. Item(0)
// is the zero ID, "no target", so a number kept in a plain field can say so. An
// application with more than a screenful of controls names them in a Tree.
func Item(n int) ID {
	if n == 0 {
		return ID{}
	}
	return ID{scope: itemsScope, name: Index(n)}
}

// In returns the item number of an identity that is item i of the list scope, and
// false for any other identity: the way a handler reads which row was clicked.
//
//	if row, ok := id.In(rows); ok { open(row) }
func (id ID) In(list Scope) (int, bool) {
	if id.scope != list || list == 0 {
		return 0, false
	}
	return id.name.Index()
}

// ID returns the identity of the control or item with the name in the scope.
func (scope Scope) ID(name Name) ID { return ID{scope: scope, name: name} }

// At returns the identity of item i of the list scope: ID(Index(i)).
func (scope Scope) At(i int) ID { return ID{scope: scope, name: Index(i)} }

// IsZero reports whether scope is no place.
func (scope Scope) IsZero() bool { return scope == 0 }

// ID identifies a pointer interaction region: a scope and a name. The zero ID
// means "no target". IDs compare with ==.
type ID struct {
	scope Scope
	name  Name
}

// IsZero reports whether the ID is "no target".
func (id ID) IsZero() bool { return id.scope == 0 }

// Scope returns the place the ID is in.
func (id ID) Scope() Scope { return id.scope }

// Name returns the ID's name within its scope: for an item of a list, its number.
func (id ID) Name() Name { return id.name }

// Bits packs the identity into 64 bits, for carrying it between processes (the
// development reload worker and its window). The number means something only to
// the Tree that made it; the process that receives it may compare it and send it
// back, and nothing else.
func (id ID) Bits() uint64 { return uint64(id.scope)<<32 | uint64(id.name) }

// IDFromBits is the inverse of Bits.
func IDFromBits(bits uint64) ID { return ID{scope: Scope(bits >> 32), name: Name(bits)} }

// Tree is the set of places an application has named. The application owns one,
// and every canvas that paints its identities shares it; an embedded view is
// given a scope of the tree to build under, so it needs no translation between
// its identities and the page's.
//
// A Tree is not safe for concurrent use: the window thread paints and handles
// events, and so uses it.
type Tree struct {
	nodes  []treeNode
	index  map[treeStep]Scope
	words  map[string]Name
	text   []string
	mounts map[treeStep]Scope
}

type treeNode struct {
	parent  Scope
	name    Name
	feature Scope
	depth   uint16
	// mount and local are set on a place made by Mount: the scope it was
	// mounted at and the scope of the embedded view it stands for.
	mount Scope
	local Scope
}

type treeStep struct {
	parent Scope
	name   Name
}

// Name returns the name for a word, making it the first time the word is asked
// for. Call it when the program starts (a package-level variable), not while
// painting. A name from a word is never equal to an item's, or to another word's.
func (tree *Tree) Name(word string) Name {
	if name, ok := tree.words[word]; ok {
		return name
	}
	if tree.words == nil {
		tree.words = make(map[string]Name)
	}
	name := wordBit | Name(len(tree.text))
	tree.text = append(tree.text, word)
	tree.words[word] = name
	return name
}

// ID returns the identity of the control a word names in scope: the common way to
// declare one.
//
//	var hitAdd = tree.ID(proxies, "add")
func (tree *Tree) ID(scope Scope, word string) ID {
	return ID{scope: scope, name: tree.Name(word)}
}

// Feature returns the top-level scope named by a page or overlay.
func (tree *Tree) Feature(word string) Scope {
	return tree.Sub(0, tree.Name(word))
}

// Sub returns the scope of a list, a group or a dialog inside parent, making it
// the first time it is asked for. The zero parent is the root.
func (tree *Tree) Sub(parent Scope, name Name) Scope {
	step := treeStep{parent: parent, name: name}
	if scope, ok := tree.index[step]; ok {
		return scope
	}
	tree.init()
	scope := Scope(len(tree.nodes))
	node := treeNode{parent: parent, name: name, feature: scope}
	if parent != 0 {
		node.feature = tree.nodes[parent].feature
		node.depth = tree.nodes[parent].depth + 1
	}
	tree.nodes = append(tree.nodes, node)
	tree.index[step] = scope
	return scope
}

// Mount returns where the place local of an embedded view sits when the view is
// mounted at mount. The view paints with identities of its own and does not know
// where it is placed: a session page is painted in every pane with the same
// controls. Mounting replays the view's path below mount, so the result is a
// place inside mount (Within says so) and the embedding canvas needs no
// translation tables or numeric windows. The first call for a place makes it; a
// later one is a lookup.
func (tree *Tree) Mount(mount, local Scope) Scope {
	if !tree.has(local) {
		return 0
	}
	if mount == 0 {
		return local
	}
	step := treeStep{parent: mount, name: Name(local)}
	if scope, ok := tree.mounts[step]; ok {
		return scope
	}
	node := tree.nodes[local]
	parent := mount
	if node.parent != 0 {
		parent = tree.Mount(mount, node.parent)
	}
	scope := tree.Sub(parent, node.name)
	tree.nodes[scope].mount, tree.nodes[scope].local = mount, local
	if tree.mounts == nil {
		tree.mounts = make(map[treeStep]Scope)
	}
	tree.mounts[step] = scope
	return scope
}

// MountID is Mount for an identity.
func (tree *Tree) MountID(mount Scope, id ID) ID {
	if id.IsZero() || mount == 0 {
		return id
	}
	return ID{scope: tree.Mount(mount, id.scope), name: id.name}
}

// Local is the inverse of MountID: for an identity made by mounting a view at
// mount it returns the identity the view painted, which is how an event on an
// embedded view's control reaches that view's own handler. It reports false for
// an identity that was not made by mounting at mount. The zero mount is no
// mounting: the view is the page, and every identity is its own.
func (tree *Tree) Local(mount Scope, id ID) (ID, bool) {
	if mount == 0 {
		return id, !id.IsZero()
	}
	if !tree.has(id.scope) {
		return ID{}, false
	}
	node := tree.nodes[id.scope]
	if node.local == 0 || node.mount != mount {
		return ID{}, false
	}
	return ID{scope: node.local, name: id.name}, true
}

// MountOf returns the mount an identity's scope was made by, and false when it
// was not made by mounting. Nested mounts return the innermost.
func (tree *Tree) MountOf(scope Scope) (Scope, bool) {
	if !tree.has(scope) || tree.nodes[scope].mount == 0 {
		return 0, false
	}
	return tree.nodes[scope].mount, true
}

// init makes the root and the anonymous scope of Item, the first two places.
func (tree *Tree) init() {
	if tree.index == nil {
		tree.index = make(map[treeStep]Scope)
		tree.nodes = append(tree.nodes, treeNode{}, treeNode{feature: itemsScope}) // 0: "no place", 1: Item
	}
}

// Within reports whether scope is ancestor or a place below it, however deep.
// It is how a window-level handler hands an event to the page it belongs to.
func (tree *Tree) Within(scope, ancestor Scope) bool {
	if !tree.has(scope) || !tree.has(ancestor) {
		return false
	}
	depth := tree.nodes[ancestor].depth
	for tree.nodes[scope].depth > depth {
		scope = tree.nodes[scope].parent
	}
	return scope == ancestor
}

// ScopeName returns the name a scope was made with: the number of a mount made for
// item i of a list, or the word of a page or a list.
func (tree *Tree) ScopeName(scope Scope) Name {
	if !tree.has(scope) {
		return 0
	}
	return tree.nodes[scope].name
}

// Parent returns the place scope is inside, and the zero Scope for a feature.
func (tree *Tree) Parent(scope Scope) Scope {
	if !tree.has(scope) {
		return 0
	}
	return tree.nodes[scope].parent
}

// FeatureOf returns the page or overlay a scope is in.
func (tree *Tree) FeatureOf(scope Scope) Scope {
	if !tree.has(scope) {
		return 0
	}
	return tree.nodes[scope].feature
}

// Len returns how many places the tree has named.
func (tree *Tree) Len() int {
	if len(tree.nodes) < 2 {
		return 0
	}
	return len(tree.nodes) - 2
}

func (tree *Tree) has(scope Scope) bool {
	if scope == itemsScope {
		tree.init()
	}
	return scope != 0 && int(scope) < len(tree.nodes)
}

// Path writes a scope as the names that lead to it, such as "proxies/rows", for
// logs and test failures. It is not for painting: it allocates.
func (tree *Tree) Path(scope Scope) string {
	if !tree.has(scope) {
		return ""
	}
	path := ""
	for ; scope > itemsScope; scope = tree.nodes[scope].parent {
		step := tree.NameText(tree.nodes[scope].name)
		if path != "" {
			step += "/"
		}
		path = step + path
	}
	return path
}

// Text writes an identity as its path and name, such as "proxies/rows/#3" or
// "proxies/add", and an Item as "#n": for the headless driver's replies, logs and
// test failures. It allocates.
func (tree *Tree) Text(id ID) string {
	if id.IsZero() {
		return "none"
	}
	if path := tree.Path(id.scope); path != "" {
		return path + "/" + tree.NameText(id.name)
	}
	return tree.NameText(id.name)
}

// NameText returns the word of a name made by Tree.Name, or "#n" for item n.
func (tree *Tree) NameText(name Name) string {
	if name&wordBit != 0 {
		if at := int(name &^ wordBit); at < len(tree.text) {
			return tree.text[at]
		}
		return "?"
	}
	digits := [11]byte{}
	at := len(digits)
	for value := int(name); ; value /= 10 {
		at--
		digits[at] = byte('0' + value%10)
		if value < 10 {
			break
		}
	}
	return "#" + string(digits[at:])
}
