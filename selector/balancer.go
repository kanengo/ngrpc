package selector

import (
	"context"
)

type WeightNode interface {
	Node

	Weight() float64

	Raw() Node

	Pick() DoneFunc

	PickLastTime() int64
}

type Balancer interface {
	Pick(ctx context.Context, nodes []WeightNode) (selected WeightNode, done DoneFunc, err error)
}

type BalancerBuilder interface {
	Build() Balancer
}

type WeightNodeBuilder interface {
	Build(Node) WeightNode
}
