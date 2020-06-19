package qkchash

const (
	Null_Value = uint64(0xFFFFFFFFFFFFFFFF)
)

// Tree is a Left-Leaning Red-Black (LLRB) implementation of 2-3 trees
type LLRB struct {
	count         int
	root          *Node
	rotationStats [4]uint64
}

type Node struct {
	Value       uint64
	Index       int
	Left, Right *Node // Pointers to left and right child nodes
	Black       bool  // If set, the color of the link (incoming from the parent) is black
	// In the LLRB, new nodes are always red, hence the zero-value for node
}

// New allocates a new tree
func NewLLRB() *LLRB {
	return &LLRB{0, nil, [4]uint64{0, 0, 0, 0}}
}

// Len returns the number of nodes in the tree.
func (t *LLRB) Len() int { return t.count }

// Has returns true if the tree contains an element whose order is the same as that of key.
func (t *LLRB) Has(key uint64) bool {
	return t.Get(key) != Null_Value
}

// Get retrieves an element value from the tree whose order is the same as that of key.
func (t *LLRB) Get(key uint64) uint64 {
	h := t.root
	for h != nil {
		switch {
		case key < h.Value:
			h = h.Left
		case key > h.Value:
			h = h.Right
		default:
			return h.Value
		}
	}

	return Null_Value
}

// Get retrieves an element from the tree whose order is the same as that of key.
func (t *LLRB) GetNode(key uint64) *Node {
	h := t.root
	for h != nil {
		switch {
		case key < h.Value:
			h = h.Left
		case key > h.Value:
			h = h.Right
		default:
			return h
		}
	}
	return nil
}

// Get retrieves an element from the tree whose index is the same as that of idx.
func (t *LLRB) GetNodeByIndex(idx int) *Node {
	if t.root == nil || idx >= t.count {
		return nil
	}

	h := t.root
	return t.getNodeByIndex(h, idx)
}

func (t *LLRB) getNodeByIndex(h *Node, idx int) *Node {
	if idx == h.Index {
		return h
	}
	if idx < h.Index {
		return t.getNodeByIndex(h.Left, idx)
	} else {
		return t.getNodeByIndex(h.Right, idx-h.Index-1)
	}
}

// Min returns the minimum element in the tree.
func (t *LLRB) Min() uint64 {
	h := t.root
	if h == nil {
		return Null_Value
	}
	for h.Left != nil {
		h = h.Left
	}
	return h.Value
}

// Max returns the maximum element in the tree.
func (t *LLRB) Max() uint64 {
	h := t.root
	if h == nil {
		return Null_Value
	}
	for h.Right != nil {
		h = h.Right
	}
	return h.Value
}

// ReplaceOrInsert inserts item into the tree. If an existing
// element has the same order, it is removed from the tree and returned.
func (t *LLRB) ReplaceOrInsert(item uint64) uint64 {
	var replaced uint64
	t.root, replaced = t.replaceOrInsert(t.root, item)
	t.root.Black = true
	if replaced == Null_Value {
		t.count++
	}
	return replaced
}

func (t *LLRB) replaceOrInsert(h *Node, item uint64) (*Node, uint64) {
	if h == nil {
		return newNode(item), Null_Value
	}

	h = t.walkDownRot23(h)

	var replaced uint64
	if item < h.Value { // BUG
		h.Left, replaced = t.replaceOrInsert(h.Left, item)
		if replaced == Null_Value {
			h.Index++
		}
	} else if h.Value < item {
		h.Right, replaced = t.replaceOrInsert(h.Right, item)
	} else {
		replaced, h.Value = h.Value, item
	}

	h = t.walkUpRot23(h)

	return h, replaced
}

// Rotation driver routines for 2-3 algorithm
func (t *LLRB) walkDownRot23(h *Node) *Node { return h }

func (t *LLRB) walkUpRot23(h *Node) *Node {
	if isRed(h.Right) && !isRed(h.Left) {
		h = t.rotateLeft(h)
	}

	if isRed(h.Left) && isRed(h.Left.Left) {
		h = t.rotateRight(h)
	}

	if isRed(h.Left) && isRed(h.Right) {
		flip(h)
	}

	return h
}

// DeleteMin deletes the minimum element in the tree and returns the
// deleted item or nil otherwise.
func (t *LLRB) DeleteMin() uint64 {
	var deleted uint64
	t.root, deleted = t.deleteMin(t.root)
	if t.root != nil {
		t.root.Black = true
	}
	if deleted != Null_Value {
		t.count--
	}
	return deleted
}

// deleteMin code for LLRB 2-3 trees
func (t *LLRB) deleteMin(h *Node) (*Node, uint64) {
	if h == nil {
		return nil, Null_Value
	}
	if h.Left == nil {
		return nil, h.Value
	}

	if !isRed(h.Left) && !isRed(h.Left.Left) {
		h = t.moveRedLeft(h)
	}

	var deleted uint64
	h.Left, deleted = t.deleteMin(h.Left)
	h.Index--

	return t.fixUp(h), deleted
}

// DeleteMax deletes the maximum element in the tree and returns
// the deleted item or nil otherwise
func (t *LLRB) DeleteMax() uint64 {
	var deleted uint64
	t.root, deleted = t.deleteMax(t.root)
	if t.root != nil {
		t.root.Black = true
	}
	if deleted != Null_Value {
		t.count--
	}
	return deleted
}

func (t *LLRB) deleteMax(h *Node) (*Node, uint64) {
	if h == nil {
		return nil, Null_Value
	}
	if isRed(h.Left) {
		h = t.rotateRight(h)
	}
	if h.Right == nil {
		return nil, h.Value
	}
	if !isRed(h.Right) && !isRed(h.Right.Left) {
		h = t.moveRedRight(h)
	}
	var deleted uint64
	h.Right, deleted = t.deleteMax(h.Right)

	return t.fixUp(h), deleted
}

// Delete deletes an item from the tree whose key equals key.
// The deleted item is return, otherwise nil is returned.
func (t *LLRB) Delete(key uint64) uint64 {
	var deleted uint64
	t.root, deleted = t.delete(t.root, key)
	if t.root != nil {
		t.root.Black = true
	}
	if deleted != Null_Value {
		t.count--
	}
	return deleted
}

func (t *LLRB) delete(h *Node, item uint64) (*Node, uint64) {
	var deleted uint64
	if h == nil {
		return nil, Null_Value
	}
	if item < h.Value {
		if h.Left == nil { // item not present. Nothing to delete
			return h, Null_Value
		}
		if !isRed(h.Left) && !isRed(h.Left.Left) {
			h = t.moveRedLeft(h)
		}
		h.Left, deleted = t.delete(h.Left, item)
		if deleted != Null_Value {
			h.Index--
		}
	} else {
		if isRed(h.Left) {
			h = t.rotateRight(h)
		}
		// If @item equals @h.uint64 and no right children at @h
		if !(h.Value < item) && h.Right == nil {
			return nil, h.Value
		}
		// PETAR: Added 'h.Right != nil' below
		if h.Right != nil && !isRed(h.Right) && !isRed(h.Right.Left) {
			h = t.moveRedRight(h)
		}
		// If @item equals @h.uint64, and (from above) 'h.Right != nil'
		if !(h.Value < item) {
			var subDeleted uint64
			h.Right, subDeleted = t.deleteMin(h.Right)
			if subDeleted == Null_Value {
				panic("logic")
			}
			deleted, h.Value = h.Value, subDeleted
		} else { // Else, @item is bigger than @h.uint64
			h.Right, deleted = t.delete(h.Right, item)
		}
	}

	return t.fixUp(h), deleted
}

// Delete deletes an item from the tree whose key equals key.
// The deleted item is return, otherwise nil is returned.
func (t *LLRB) DeleteAt(idx int) uint64 {
	var deleted uint64
	t.root, deleted = t.deleteAt(t.root, idx)
	if t.root != nil {
		t.root.Black = true
	}
	if deleted != Null_Value {
		t.count--
	}
	return deleted
}

func (t *LLRB) deleteAt(h *Node, idx int) (*Node, uint64) {
	var deleted uint64
	if h == nil {
		return nil, Null_Value
	}
	if idx < h.Index {
		if h.Left == nil { // item not present. Nothing to delete
			panic("logic")
		}
		if !isRed(h.Left) && !isRed(h.Left.Left) {
			h = t.moveRedLeft(h)
		}
		h.Left, deleted = t.deleteAt(h.Left, idx)
		h.Index--
	} else {
		if isRed(h.Left) {
			h = t.rotateRight(h)
		}
		// If @item equals @h.uint64 and no right children at @h
		if h.Index == idx && h.Right == nil {
			return nil, h.Value
		}
		// PETAR: Added 'h.Right != nil' below
		if h.Right != nil && !isRed(h.Right) && !isRed(h.Right.Left) {
			h = t.moveRedRight(h)
		}
		// If @item equals @h.uint64, and (from above) 'h.Right != nil'
		if h.Index == idx {
			var subDeleted uint64
			h.Right, subDeleted = t.deleteMin(h.Right)
			if subDeleted == Null_Value {
				panic("logic")
			}
			deleted, h.Value = h.Value, subDeleted
		} else { // Else, @item is bigger than @h.uint64
			h.Right, deleted = t.deleteAt(h.Right, idx-h.Index-1)
		}
	}

	return t.fixUp(h), deleted
}

func (t *LLRB) GetRotationStats() [4]uint64 {
	return t.rotationStats
}

func (t *LLRB) ResetRotationStats() {
	for i := 0; i < len(t.rotationStats); i++ {
		t.rotationStats[i] = uint64(0)
	}
}

func (t *LLRB) rotateLeft(h *Node) *Node {
	x := h.Right
	if x.Black {
		panic("rotating a black link")
	}
	h.Right = x.Left
	x.Left = h
	x.Black = h.Black
	h.Black = false
	x.Index += (h.Index + 1)
	c := uint64(0)
	for i := 0; i < len(t.rotationStats); i++ {
		nc := uint64(1)
		if t.rotationStats[i]&(uint64(1)<<63) == 0 {
			nc = 0
		}
		t.rotationStats[i] = (t.rotationStats[i] << 1) ^ c
		c = nc
	}
	t.rotationStats[0] = t.rotationStats[0] ^ c
	return x
}

func (t *LLRB) rotateRight(h *Node) *Node {
	x := h.Left
	if x.Black {
		panic("rotating a black link")
	}
	h.Left = x.Right
	x.Right = h
	x.Black = h.Black
	h.Black = false
	h.Index -= (x.Index + 1)
	c := uint64(1)
	for i := 0; i < len(t.rotationStats); i++ {
		nc := uint64(1)
		if t.rotationStats[i]&(uint64(1)<<63) == 0 {
			nc = 0
		}
		t.rotationStats[i] = (t.rotationStats[i] << 1) ^ c
		c = nc
	}
	t.rotationStats[0] = t.rotationStats[0] ^ c
	return x
}

// REQUIRE: Left and Right children must be present
func (t *LLRB) moveRedLeft(h *Node) *Node {
	flip(h)
	if isRed(h.Right.Left) {
		h.Right = t.rotateRight(h.Right)
		h = t.rotateLeft(h)
		flip(h)
	}
	return h
}

// REQUIRE: Left and Right children must be present
func (t *LLRB) moveRedRight(h *Node) *Node {
	flip(h)
	if isRed(h.Left.Left) {
		h = t.rotateRight(h)
		flip(h)
	}
	return h
}

func (t *LLRB) fixUp(h *Node) *Node {
	if isRed(h.Right) {
		h = t.rotateLeft(h)
	}

	if isRed(h.Left) && isRed(h.Left.Left) {
		h = t.rotateRight(h)
	}

	if isRed(h.Left) && isRed(h.Right) {
		flip(h)
	}

	return h
}

// Internal node manipulation routines
func newNode(item uint64) *Node { return &Node{Value: item} }

func isRed(h *Node) bool {
	if h == nil {
		return false
	}
	return !h.Black
}

// REQUIRE: Left and Right children must be present
func flip(h *Node) {
	h.Black = !h.Black
	h.Left.Black = !h.Left.Black
	h.Right.Black = !h.Right.Black
}
