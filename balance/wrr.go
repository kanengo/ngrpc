package balance

import (
	"google.golang.org/grpc"
	_ "google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
)

type wrrBuilder struct {
	cc balancer.ClientConn
}

const wrrBalanceName = "wrr"

func (w *wrrBuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	bl := &wrrBalance{}

	return bl
}

func (w *wrrBuilder) Name() string {
	return wrrBalanceName
}

type wrrBalance struct {
}

func (w wrrBalance) UpdateClientConnState(state balancer.ClientConnState) error {
	//TODO implement me】
	grpc.DialContext()
	panic("implement me")
}

func (w wrrBalance) ResolverError(err error) {
	//TODO implement me
	panic("implement me")
}

func (w wrrBalance) UpdateSubConnState(conn balancer.SubConn, state balancer.SubConnState) {
	//TODO implement me
	panic("implement me")
}

func (w wrrBalance) Close() {
	//TODO implement me
	panic("implement me")
}
