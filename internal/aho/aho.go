// Package aho implements a minimal Aho-Corasick automaton for multi-literal
// pre-filtering. It exposes only MatchesAny, which short-circuits on the first
// match — exactly the semantics needed to skip non-matching files in the
// search hot path.
package aho

type node struct {
	children [256]*node
	fail     *node
	output   bool
}

// Machine is a compiled Aho-Corasick automaton. Safe for concurrent use after
// construction; the match method is allocation-free.
type Machine struct {
	root  *node
	count int
}

// New compiles an automaton from the given literal patterns. Empty patterns
// are silently skipped. If no patterns survive, the returned Machine is nil
// and MatchesAny always returns false.
func New(patterns [][]byte) *Machine {
	root := &node{}

	count := 0
	for _, p := range patterns {
		if len(p) == 0 {
			continue
		}
		n := root
		for _, c := range p {
			if n.children[c] == nil {
				n.children[c] = &node{}
			}
			n = n.children[c]
		}
		if !n.output {
			n.output = true
			count++
		}
	}
	if count == 0 {
		return nil
	}

	// BFS to build failure links.
	queue := make([]*node, 0, 256)
	for c := 0; c < 256; c++ {
		if root.children[c] != nil {
			root.children[c].fail = root
			queue = append(queue, root.children[c])
		} else {
			root.children[c] = root
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for c := 0; c < 256; c++ {
			if cur.children[c] != nil {
				cur.children[c].fail = cur.fail.children[c]
				cur.children[c].output = cur.children[c].output || cur.fail.children[c].output
				queue = append(queue, cur.children[c])
			} else {
				cur.children[c] = cur.fail.children[c]
			}
		}
	}

	return &Machine{root: root, count: count}
}

// MatchesAny returns true if any compiled pattern appears in data.
// Stops at the first match.
func (m *Machine) MatchesAny(data []byte) bool {
	if m == nil || len(data) == 0 {
		return false
	}

	n := m.root
	for _, c := range data {
		n = n.children[c]
		if n.output {
			return true
		}
	}
	return false
}
