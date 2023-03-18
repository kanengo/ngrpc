package p2c

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/kanengo/ngrpc/selector"
	"github.com/kanengo/ngrpc/selector/node/ewma"
)

const (
	Name      = "p2c"
	pickTimes = 3

	forcePick = 5
)

type Builder struct {
}

func (b Builder) Build() selector.Balancer {
	return &Balancer{
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

type Balancer struct {
	r *rand.Rand

	picking int64
}

func (b *Balancer) choose(node1, node2 selector.WeightNode) selector.WeightNode {
	node1Weight, node2Weight := node1.Weight(), node2.Weight()
	var selected, unSelected selector.WeightNode
	if node1Weight > node2Weight {
		selected = node1
		unSelected = node2
	} else {
		selected = node2
		unSelected = node1
	}

	now := time.Now().Unix()
	if now-unSelected.PickLastTime() >= forcePick && atomic.CompareAndSwapInt64(&b.picking, 0, 1) {
		selected = unSelected
		atomic.StoreInt64(&b.picking, 0)
	}

	return selected
}

func (b *Balancer) Pick(ctx context.Context, nodes []selector.WeightNode) (selected selector.WeightNode, done selector.DoneFunc, err error) {
	switch len(nodes) {
	case 0:
		return nil, nil, selector.ErrNoAvailable
	case 1:
		return nodes[0], nodes[0].Pick(), nil
	case 2:
		selected = b.choose(nodes[0], nodes[1])
	default:
		var node1, node2 selector.WeightNode
		for i := 0; i < pickTimes; i++ {
			ra := b.r.Intn(len(nodes))
			rb := b.r.Intn(len(nodes) - 1)
			if rb >= ra {
				rb++
			}
			node1 = nodes[ra]
			node2 = nodes[rb]

			selected = b.choose(node1, node2)
		}
	}

	return selected, selected.Pick(), nil
}

func NewBuilder() selector.Builder {
	return &selector.DefaultBuilder{
		WeightNodeBuilder: ewma.NewBuilder(),
		BalancerBuilder:   &Builder{},
	}
}

func New() selector.Selector {
	return NewBuilder().Build()
}
