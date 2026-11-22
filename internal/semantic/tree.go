package semantic

// ID is a stable generational semantic-node identity.
type ID uint64

// Role describes the accessible purpose of a node.
type Role uint8

const (
	RoleApplication Role = iota + 1
	RoleGroup
	RoleButton
	RoleText
	RoleTextField
	RoleCheckbox
	RoleSlider
	RoleImage
	RoleLink
	RoleRadio
	RoleTab
	RoleMenuItem
	RoleCombobox
	RoleSwitch
	RoleMenu
	RoleMenubar
	RoleListBox
	RoleOption
	RoleGrid
	RoleGridCell
	RoleHeading
	RoleStatus
	RoleSeparator
	RoleProgress
)

// Operation describes an incremental accessibility-tree update.
type Operation uint8

const (
	OperationUpsert Operation = iota + 1
	OperationDelete
)

// Rect is an accessibility bounds rectangle in logical window pixels.
type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Node describes platform-neutral accessible state.
type Node struct {
	Role      Role
	Label     string
	Value     string
	Hint      string
	Bounds    Rect
	Focusable bool
	Focused   bool
	Disabled  bool
	Checked   bool
	Mixed     bool
	Selected  bool
	Expanded  bool
	Multiline bool
	ReadOnly  bool
	// Invalid marks a control whose value failed validation.
	Invalid bool
	Secure  bool
	Hidden  bool
}

// Change is one coalesced platform accessibility update.
type Change struct {
	ID              ID
	Operation       Operation
	Parent          ID
	PreviousSibling ID
	Node            Node
}

type entry struct {
	parent      ID
	firstChild  ID
	lastChild   ID
	previous    ID
	next        ID
	data        Node
	generation  uint32
	alive       bool
	queued      bool
	published   bool
	pendingFree bool
}

// Tree stores semantic nodes independently from visual scene nodes.
type Tree struct {
	nodes   []entry
	free    []uint32
	dirty   []ID
	scratch []ID
	root    ID
}

// New creates a semantic tree with an application root.
func New(label string) *Tree {
	tree := &Tree{nodes: make([]entry, 1, 8), dirty: make([]ID, 0, 8)}
	tree.nodes[0] = entry{
		data:       Node{Role: RoleApplication, Label: label},
		generation: 1,
		alive:      true,
	}
	tree.root = makeID(0, 1)
	tree.markDirty(tree.root, &tree.nodes[0])
	return tree
}

// NewWithCapacity creates a tree sized for an expected total node count.
func NewWithCapacity(label string, capacity int) *Tree {
	if capacity < 1 {
		capacity = 1
	}
	tree := New(label)
	nodes := make([]entry, 1, capacity)
	nodes[0] = tree.nodes[0]
	tree.nodes = nodes
	tree.dirty = make([]ID, 0, capacity)
	tree.nodes[0].queued = false
	tree.markDirty(tree.root, &tree.nodes[0])
	return tree
}

// Root returns the permanent application root.
func (tree *Tree) Root() ID {
	return tree.root
}

// Add appends a semantic node below parent.
func (tree *Tree) Add(parent ID, data Node) (ID, bool) {
	parentNode, ok := tree.live(parent)
	if !ok || data.Role == 0 || data.Role == RoleApplication {
		return 0, false
	}
	index := tree.allocate()
	parentNode, _ = tree.live(parent)
	current := &tree.nodes[index]
	current.parent = parent
	current.data = data
	current.alive = true
	id := makeID(index, current.generation)
	if parentNode.lastChild == 0 {
		parentNode.firstChild = id
		parentNode.lastChild = id
	} else {
		last, _ := tree.live(parentNode.lastChild)
		last.next = id
		current.previous = parentNode.lastChild
		parentNode.lastChild = id
	}
	tree.markDirty(id, current)
	return id, true
}

// Set updates one node and coalesces repeated changes.
func (tree *Tree) Set(id ID, data Node) bool {
	current, ok := tree.live(id)
	if !ok || data.Role == 0 || (id != tree.root && data.Role == RoleApplication) {
		return false
	}
	if current.data == data {
		return false
	}
	current.data = data
	tree.markDirty(id, current)
	return true
}

// Get returns one live semantic node.
func (tree *Tree) Get(id ID) (Node, bool) {
	current, ok := tree.live(id)
	if !ok {
		return Node{}, false
	}
	return current.data, true
}

// Remove deletes a semantic subtree. The application root cannot be removed.
func (tree *Tree) Remove(id ID) bool {
	current, ok := tree.live(id)
	if !ok || id == tree.root {
		return false
	}
	tree.detach(current)
	tree.scratch = append(tree.scratch[:0], id)
	for cursor := 0; cursor < len(tree.scratch); cursor++ {
		currentID := tree.scratch[cursor]
		currentNode, exists := tree.live(currentID)
		if !exists {
			continue
		}
		for child := currentNode.firstChild; child != 0; {
			childNode, childExists := tree.live(child)
			if !childExists {
				break
			}
			next := childNode.next
			tree.scratch = append(tree.scratch, child)
			child = next
		}
	}
	for index := len(tree.scratch) - 1; index >= 0; index-- {
		currentID := tree.scratch[index]
		currentNode, exists := tree.live(currentID)
		if exists {
			currentNode.alive = false
			currentNode.pendingFree = true
			tree.markDirty(currentID, currentNode)
		}
	}
	return true
}

// DrainChanges appends coalesced semantic changes to destination.
func (tree *Tree) DrainChanges(destination []Change) []Change {
	for _, id := range tree.dirty {
		index, current, ok := tree.entry(id)
		if !ok || !current.queued {
			continue
		}
		current.queued = false
		if current.pendingFree {
			if current.published {
				destination = append(destination, Change{ID: id, Operation: OperationDelete})
			}
			tree.reclaim(index, current)
			continue
		}
		destination = append(destination, Change{
			ID:              id,
			Operation:       OperationUpsert,
			Parent:          current.parent,
			PreviousSibling: current.previous,
			Node:            current.data,
		})
		current.published = true
	}
	tree.dirty = tree.dirty[:0]
	return destination
}

// PendingChanges returns the number of distinct dirty semantic nodes.
func (tree *Tree) PendingChanges() int {
	return len(tree.dirty)
}

func (tree *Tree) detach(current *entry) {
	parent, _ := tree.live(current.parent)
	if current.previous == 0 {
		parent.firstChild = current.next
	} else {
		previous, _ := tree.live(current.previous)
		previous.next = current.next
	}
	if current.next == 0 {
		parent.lastChild = current.previous
	} else {
		next, _ := tree.live(current.next)
		next.previous = current.previous
	}
}

func (tree *Tree) allocate() uint32 {
	if len(tree.free) == 0 {
		index := uint32(len(tree.nodes))
		tree.nodes = append(tree.nodes, entry{generation: 1})
		return index
	}
	last := len(tree.free) - 1
	index := tree.free[last]
	tree.free = tree.free[:last]
	generation := tree.nodes[index].generation
	tree.nodes[index] = entry{generation: generation}
	return index
}

func (tree *Tree) reclaim(index uint32, current *entry) {
	nextGeneration := current.generation + 1
	if nextGeneration == 0 {
		nextGeneration = 1
	}
	*current = entry{generation: nextGeneration}
	tree.free = append(tree.free, index)
}

func (tree *Tree) markDirty(id ID, current *entry) {
	if current.queued {
		return
	}
	current.queued = true
	tree.dirty = append(tree.dirty, id)
}

func (tree *Tree) live(id ID) (*entry, bool) {
	_, current, ok := tree.entry(id)
	return current, ok && current.alive
}

func (tree *Tree) entry(id ID) (uint32, *entry, bool) {
	index, generation, ok := splitID(id)
	if !ok || int(index) >= len(tree.nodes) {
		return 0, nil, false
	}
	current := &tree.nodes[index]
	if current.generation != generation {
		return 0, nil, false
	}
	return index, current, true
}

func makeID(index uint32, generation uint32) ID {
	return ID(uint64(generation)<<32 | uint64(index+1))
}

func splitID(id ID) (uint32, uint32, bool) {
	index := uint32(id)
	generation := uint32(uint64(id) >> 32)
	if index == 0 || generation == 0 {
		return 0, 0, false
	}
	return index - 1, generation, true
}

// Len is how many nodes are alive, the root included.
func (tree *Tree) Len() int { return len(tree.nodes) - len(tree.free) }

// Slots is how many node slots the tree has ever allocated; see Scene.Slots.
func (tree *Tree) Slots() int { return len(tree.nodes) }
