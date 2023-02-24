package selector

import (
	"context"
	"sync/atomic"

	"github.com/kanengo/ngrpc/errors"
)

var ErrNoAvailable = errors.ServiceUnavailable("no available node")

type DefaultSelector struct {
	WeightNodeBuilder WeightNodeBuilder
	Balancer          Balancer

	nodes atomic.Value
}

var (
	_ Selector = (*DefaultSelector)(nil)
)

func (d *DefaultSelector) Select(ctx context.Context, opts ...SelectOption) (selected Node, done DoneFunc, err error) {
	var (
		options    SelectOptions
		candidates []WeightNode
	)

	nodes, ok := d.nodes.Load().([]WeightNode)
	if !ok {
		return nil, nil, ErrNoAvailable
	}

	for _, opt := range opts {
		opt(&options)
	}

	if len(options.NodeFilters) > 0 {
		newNodes := make([]Node, len(nodes))
		for i, n := range nodes {
			newNodes[i] = n
		}
		for _, filter := range options.NodeFilters {
			newNodes = filter(ctx, newNodes)
		}
		candidates = make([]WeightNode, len(newNodes))
		for i, n := range newNodes {
			candidates[i] = n.(WeightNode)
		}
	} else {
		candidates = nodes
	}

	selectedWeightNode, done, err := d.Balancer.Pick(ctx, candidates)
	if err != nil {
		return nil, nil, err
	}

	selected = selectedWeightNode.Raw()

	return

}

func (d *DefaultSelector) Apply(nodes []Node) {
	weightNodes := make([]WeightNode, 0, len(nodes))
	for _, n := range nodes {
		weightNodes = append(weightNodes, d.WeightNodeBuilder.Build(n))
	}

	d.nodes.Store(weightNodes)
}

type DefaultBuilder struct {
	WeightNodeBuilder WeightNodeBuilder
	BalancerBuilder   BalancerBuilder
}

func (db *DefaultBuilder) Build() Selector {
	return &DefaultSelector{
		WeightNodeBuilder: db.WeightNodeBuilder,
		Balancer:          db.BalancerBuilder.Build(),
	}
}
