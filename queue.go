package di

import (
	"fmt"
	"sort"
	"sync"
)

type providerDones struct {
	dones map[*provider]struct{}
	mu    sync.RWMutex
}

func (p *providerDones) isDone(prov *provider) bool {
	p.mu.RLock()
	_, has := p.dones[prov]
	p.mu.RUnlock()
	return has
}

func (p *providerDones) markDone(prov *provider) {
	p.mu.Lock()
	if p.dones == nil {
		p.dones = make(map[*provider]struct{})
	}
	p.dones[prov] = struct{}{}
	p.mu.Unlock()
}

type queueNode struct {
	provider *provider
	weight   int

	parentDone bool
}

type queue struct {
	deps  dependencies
	nodes []*queueNode
}

func (q *queue) search(p *provider) *queueNode {
	for _, n := range q.nodes {
		if n.provider == p {
			return n
		}
	}
	return nil
}

func (q *queue) append(p *provider) *queueNode {
	n := &queueNode{
		provider: p,
		weight:   1,
	}
	q.nodes = append(q.nodes, n)
	return n
}

func (q *queue) add(p *provider, context []string, dones *providerDones) (*queueNode, error) {
	node := q.search(p)
	if dones.isDone(p) {
		return node, nil
	}

	if node != nil {
		if node.parentDone {
			return node, nil
		}
		context = append(context, p.name)
		return nil, fmt.Errorf("cycle dependencies: %v", context)
	}

	context = append(context, p.name)
	node = q.append(p)
	var (
		parent *queueNode
		err    error
	)
	for _, dep := range p.deps {
		mod := q.deps.match(dep)
		if mod == nil {
			return nil, dep.notExistError(p.name)
		}
		parent, err = q.add(mod.Provider, context, dones)
		if err != nil {
			return nil, err
		}
		if parent != nil {
			node.weight += parent.weight
		}
	}
	node.parentDone = true
	return node, nil
}

func (q *queue) Len() int {
	return len(q.nodes)
}

func (q *queue) Less(i, j int) bool {
	return q.nodes[i].weight < q.nodes[j].weight
}

func (q *queue) Swap(i, j int) {
	q.nodes[i], q.nodes[j] = q.nodes[j], q.nodes[i]
}

func newQueue(providers []*provider, mods dependencies, dones *providerDones) ([]*queueNode, error) {
	var (
		queue = queue{
			deps: mods,
		}
		err error
	)
	for _, p := range providers {
		_, err = queue.add(p, nil, dones)
		if err != nil {
			return nil, err
		}
	}
	sort.Sort(&queue)
	return queue.nodes, nil
}
