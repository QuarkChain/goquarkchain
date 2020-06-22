package qkchash

const (
	Null_Value = uint64(0xFFFFFFFFFFFFFFFF)
)

type llrb struct {
	size          int
	root          *Node
	rotationStats [4]uint64
}

type Node struct {
	Value       uint64
	Index       int
	Left, Right *Node
	Black       bool
}

// Size returns the number of nodes in the tree.
func (t *llrb) Size() int { return t.size }

// Get retrieves an element from the tree whose value is the same as key.
func (t *llrb) Get(key uint64) *Node {
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

// GetByIndex retrieves an element from the tree whose index is the same as that of idx.
func (t *llrb) GetByIndex(idx int) *Node {
	if t.root == nil || idx >= t.size {
		return nil
	}

	h := t.root
	return t.getByIndex(h, idx)
}

func (t *llrb) getByIndex(h *Node, idx int) *Node {
	if idx == h.Index {
		return h
	}
	if idx < h.Index {
		return t.getByIndex(h.Left, idx)
	} else {
		return t.getByIndex(h.Right, idx-h.Index-1)
	}
}

// Insert inserts value into the tree.
func (t *llrb) Insert(value uint64) uint64 {
	var replaced uint64
	t.root, replaced = t.insert(t.root, value)
	t.root.Black = true
	if replaced == Null_Value {
		t.size++
	}
	return replaced
}

func (t *llrb) insert(h *Node, value uint64) (*Node, uint64) {
	if h == nil {
		return &Node{Value: value}, Null_Value
	}

	var replaced uint64
	if value < h.Value {
		h.Left, replaced = t.insert(h.Left, value)
		if replaced == Null_Value {
			h.Index++
		}
	} else if h.Value < value {
		h.Right, replaced = t.insert(h.Right, value)
	} else {
		replaced, h.Value = h.Value, value
	}

	if isRed(h.Right) && !isRed(h.Left) {
		h = t.rotateLeft(h)
	}

	if isRed(h.Left) && isRed(h.Left.Left) {
		h = t.rotateRight(h)
	}

	if isRed(h.Left) && isRed(h.Right) {
		flip(h)
	}

	return h, replaced
}

func (t *llrb) deleteMin(h *Node) (*Node, uint64) {
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

// Delete deletes a value from the tree whose value equals key.
// The deleted value is return, otherwise Null_Value is returned.
func (t *llrb) Delete(key uint64) uint64 {
	var deleted uint64
	t.root, deleted = t.delete(t.root, key)
	if t.root != nil {
		t.root.Black = true
	}
	if deleted != Null_Value {
		t.size--
	}

	return deleted
}

func (t *llrb) delete(h *Node, value uint64) (*Node, uint64) {
	var deleted uint64
	if h == nil {
		return nil, Null_Value
	}
	if value < h.Value {
		if h.Left == nil {
			return h, Null_Value
		}
		if !isRed(h.Left) && !isRed(h.Left.Left) {
			h = t.moveRedLeft(h)
		}
		h.Left, deleted = t.delete(h.Left, value)
		if deleted != Null_Value {
			h.Index--
		}
	} else {
		if isRed(h.Left) {
			h = t.rotateRight(h)
		}
		if h.Value == value && h.Right == nil {
			return nil, h.Value
		}
		if h.Right != nil && !isRed(h.Right) && !isRed(h.Right.Left) {
			h = t.moveRedRight(h)
		}
		if h.Value == value {
			deleted = h.Value
			h.Right, h.Value = t.deleteMin(h.Right)
		} else {
			h.Right, deleted = t.delete(h.Right, value)
		}
	}

	return t.fixUp(h), deleted
}

// DeleteAt deletes a value from the tree whose index equals idx.
// The deleted value is return, otherwise Null_Value is returned.
func (t *llrb) DeleteAt(idx int) uint64 {
	var deleted uint64
	t.root, deleted = t.deleteAt(t.root, idx)
	if t.root != nil {
		t.root.Black = true
	}
	if deleted != Null_Value {
		t.size--
	}

	return deleted
}

func (t *llrb) deleteAt(h *Node, idx int) (*Node, uint64) {
	var deleted uint64
	if h == nil {
		return nil, Null_Value
	}
	if idx < h.Index {
		if h.Left == nil {
			panic("Left should exist")
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
		if h.Index == idx && h.Right == nil {
			return nil, h.Value
		}
		if h.Right != nil && !isRed(h.Right) && !isRed(h.Right.Left) {
			h = t.moveRedRight(h)
		}
		if h.Index == idx {
			deleted = h.Value
			h.Right, h.Value = t.deleteMin(h.Right)
		} else {
			h.Right, deleted = t.deleteAt(h.Right, idx-h.Index-1)
		}
	}

	return t.fixUp(h), deleted
}

func (t *llrb) GetRotationStats() [4]uint64 {
	return t.rotationStats
}

func (t *llrb) ResetRotationStats() {
	for i := 0; i < len(t.rotationStats); i++ {
		t.rotationStats[i] = uint64(0)
	}
}

func (t *llrb) rotateLeft(h *Node) *Node {
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

func (t *llrb) rotateRight(h *Node) *Node {
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

func (t *llrb) moveRedLeft(h *Node) *Node {
	flip(h)
	if isRed(h.Right.Left) {
		h.Right = t.rotateRight(h.Right)
		h = t.rotateLeft(h)
		flip(h)
	}
	return h
}

func (t *llrb) moveRedRight(h *Node) *Node {
	flip(h)
	if isRed(h.Left.Left) {
		h = t.rotateRight(h)
		flip(h)
	}
	return h
}

func (t *llrb) fixUp(h *Node) *Node {
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

func isRed(h *Node) bool {
	if h == nil {
		return false
	}
	return !h.Black
}

func flip(h *Node) {
	h.Black = !h.Black
	h.Left.Black = !h.Left.Black
	h.Right.Black = !h.Right.Black
}
