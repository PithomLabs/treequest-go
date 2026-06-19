package tree

import (
	"fmt"
	"sort"
	"sync"
)

// SearchTree manages the generic search tree structure.
// It is concurrent-safe via RWMutex.
type SearchTree[S any] struct {
	Root     *Node[S]
	AllNodes map[NodeID]*Node[S]
	mu       sync.RWMutex
}

// NewSearchTree creates and initializes a SearchTree.
func NewSearchTree[S any](rootState S, actions []ActionLabel) *SearchTree[S] {
	root := NewRootNode[S](rootState, actions)
	allNodes := make(map[NodeID]*Node[S])
	allNodes[root.ID] = root
	return &SearchTree[S]{
		Root:     root,
		AllNodes: allNodes,
	}
}

// GetNode looks up a node in the tree by its ID.
func (t *SearchTree[S]) GetNode(id NodeID) (*Node[S], bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.AllNodes[id]
	return n, ok
}

// AddNode creates and attaches a new child node to a designated parent.
func (t *SearchTree[S]) AddNode(parentID NodeID, nodeID NodeID, state S, score Score, action ActionLabel) (*Node[S], error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	parent, exists := t.AllNodes[parentID]
	if !exists {
		return nil, fmt.Errorf("parent node %q not found in tree", parentID)
	}

	parent.Lock()
	defer parent.Unlock()

	// New child's ExpandIdx will be the current size of tree minus 1 (excluding root, which has -1)
	expandIdx := len(t.AllNodes) - 1
	child := NewChildNode(parent, nodeID, state, score, action, expandIdx)

	parent.Children = append(parent.Children, child)
	parent.ChildrenIDs = append(parent.ChildrenIDs, child.ID)
	t.AllNodes[child.ID] = child

	return child, nil
}

// BestNode returns the non-root node with the highest score.
// If the tree only contains the root node, it returns the root.
// Ties are broken deterministically by earlier ExpandIdx.
func (t *SearchTree[S]) BestNode() *Node[S] {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.AllNodes) <= 1 {
		return t.Root
	}

	nodes := make([]*Node[S], 0, len(t.AllNodes))
	for _, n := range t.AllNodes {
		if n.ID == t.Root.ID {
			continue
		}
		nodes = append(nodes, n)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Score != nodes[j].Score {
			return nodes[i].Score > nodes[j].Score
		}
		return nodes[i].ExpandIdx < nodes[j].ExpandIdx
	})

	return nodes[0]
}

// TopK returns the k highest-scoring non-root nodes from the search tree.
// Ties are broken deterministically by earlier ExpandIdx.
func (t *SearchTree[S]) TopK(k int) []StateScore[S] {
	t.mu.RLock()
	defer t.mu.RUnlock()

	nodes := make([]*Node[S], 0, len(t.AllNodes))
	for _, n := range t.AllNodes {
		if n.ID == t.Root.ID {
			continue
		}
		nodes = append(nodes, n)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Score != nodes[j].Score {
			return nodes[i].Score > nodes[j].Score
		}
		return nodes[i].ExpandIdx < nodes[j].ExpandIdx
	})

	limit := k
	if len(nodes) < limit {
		limit = len(nodes)
	}

	res := make([]StateScore[S], 0, limit)
	for i := 0; i < limit; i++ {
		n := nodes[i]
		res = append(res, StateScore[S]{
			State: n.State,
			Reward: Reward{
				Score: n.Score,
			},
		})
	}
	return res
}

// GetNodes returns all nodes sorted in ascending order of their ExpandIdx.
// The root node (with ExpandIdx = -1) will be the first item.
func (t *SearchTree[S]) GetNodes() []*Node[S] {
	t.mu.RLock()
	defer t.mu.RUnlock()

	nodes := make([]*Node[S], 0, len(t.AllNodes))
	for _, n := range t.AllNodes {
		nodes = append(nodes, n)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ExpandIdx < nodes[j].ExpandIdx
	})

	return nodes
}

// Size returns the total number of nodes currently registered in the tree.
func (t *SearchTree[S]) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.AllNodes)
}
