package grpc

import (
	"github.com/kanengo/ngrpc/registry"
	"github.com/kanengo/ngrpc/selector"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/metadata"
)

var (
	_ base.PickerBuilder = (*balancerPickerBuilder)(nil)
	_ balancer.Picker    = (*balancerPicker)(nil)
)

const (
	balancerName = "selector"
)

func init() {
	b := base.NewBalancerBuilder(balancerName, &balancerPickerBuilder{
		selector.GlobalSelectorBuilder(),
	}, base.Config{HealthCheck: true})
	balancer.Register(b)
}

type balancerPickerBuilder struct {
	builder selector.Builder
}

func (b *balancerPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}
	nodes := make([]selector.Node, 0, len(info.ReadySCs))
	for conn, info := range info.ReadySCs {
		ins, _ := info.Address.Attributes.Value("__ServiceInstance__").(*registry.ServiceInstance)
		nodes = append(nodes, &grpcNode{
			Node:    selector.NewNode("grpc", info.Address.Addr, ins),
			subConn: conn,
		})
	}
	p := &balancerPicker{
		selector: b.builder.Build(),
	}
	p.selector.Apply(nodes)

	return p
}

type balancerPicker struct {
	selector selector.Selector
}

func (p balancerPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	node, done, err := p.selector.Select(info.Ctx)
	if err != nil {
		return balancer.PickResult{}, err
	}

	return balancer.PickResult{
		SubConn: node.(*grpcNode).subConn,
		Done: func(di balancer.DoneInfo) {
			done(info.Ctx, selector.DoneInfo{
				Err:           di.Err,
				BytesSent:     di.BytesSent,
				BytesReceived: di.BytesReceived,
				ReplyMD:       Trailer(di.Trailer),
			})
		},
	}, nil
}

type Trailer metadata.MD

func (t Trailer) Get(key string) string {
	v := metadata.MD(t).Get(key)
	if len(v) > 0 {
		return v[0]
	}
	return ""
}

type grpcNode struct {
	selector.Node
	subConn balancer.SubConn
}
