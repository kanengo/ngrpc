package selector

import (
	"strconv"

	"github.com/kanengo/ngrpc/registry"
)

var _ Node = (*DefaultNode)(nil)

type DefaultNode struct {
	scheme   string
	address  string
	weight   int64
	version  string
	name     string
	metadata map[string]string
}

func (dn *DefaultNode) Scheme() string {
	return dn.scheme
}

func (dn *DefaultNode) ServiceName() string {
	return dn.name
}

func (dn *DefaultNode) Address() string {
	return dn.address
}

func (dn *DefaultNode) Version() string {
	return dn.version
}

func (dn *DefaultNode) Metadata() map[string]string {
	return dn.metadata
}

func NewNode(scheme, addr string, ins *registry.ServiceInstance) Node {
	n := &DefaultNode{
		scheme:  scheme,
		address: addr,
	}

	if ins != nil {
		n.version = ins.Version
		n.name = ins.Name
		n.metadata = ins.Metadata
		if s, ok := ins.Metadata["weight"]; ok {
			if weight, err := strconv.ParseInt(s, 10, 64); err == nil {
				n.weight = weight
			}
		}
	}

	return n
}
