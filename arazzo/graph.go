// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import "context"

// dependencyID keeps document and authored names separate: dots can be part of
// an identifier. An empty step identifies workflow completion.
type dependencyID struct {
	document      string
	workflow      string
	step          string
	prerequisites bool
}

type dependencyEdge struct {
	target dependencyID
	path   string
}

type dependencyGraph struct {
	order []dependencyID
	edges map[dependencyID][]dependencyEdge
	run   *validation
}

func newDependencyGraph() *dependencyGraph {
	return &dependencyGraph{edges: make(map[dependencyID][]dependencyEdge)}
}

func (g *dependencyGraph) addNode(id dependencyID) {
	if _, exists := g.edges[id]; !exists {
		if g.run != nil && !g.run.work(1, "") {
			return
		}
		g.order = append(g.order, id)
		g.edges[id] = nil
	}
}

func (g *dependencyGraph) addEdge(from, to dependencyID, path string) {
	if g.run != nil && !g.run.work(1, path) {
		return
	}
	g.addNode(from)
	g.addNode(to)
	if g.run != nil && g.run.err != nil {
		return
	}
	g.edges[from] = append(g.edges[from], dependencyEdge{target: to, path: path})
}

// check uses an iterative DFS so a document with a long dependency chain does
// not consume one stack frame per step. Control-flow goto/retry edges do not
// belong in this prerequisite graph.
func (g *dependencyGraph) check(ctx context.Context, emit func(string, string)) error {
	type frame struct {
		id   dependencyID
		next int
		via  string
	}
	state := make(map[dependencyID]uint8, len(g.edges))
	for _, root := range g.order {
		if state[root] != 0 {
			continue
		}
		stack := []frame{{id: root}}
		state[root] = 1
		for len(stack) != 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			top := &stack[len(stack)-1]
			out := g.edges[top.id]
			if top.next == len(out) {
				state[top.id] = 2
				stack = stack[:len(stack)-1]
				continue
			}
			edge := out[top.next]
			top.next++
			switch state[edge.target] {
			case 0:
				state[edge.target] = 1
				stack = append(stack, frame{id: edge.target, via: edge.path})
			case 1:
				path := edge.path
				if path == "" {
					for i := len(stack) - 1; i >= 0; i-- {
						if stack[i].via != "" {
							path = stack[i].via
							break
						}
						if stack[i].id == edge.target {
							break
						}
					}
				}
				if path != "" {
					emit(path, "prerequisite dependency contains a cycle")
				}
			}
		}
	}
	return nil
}
