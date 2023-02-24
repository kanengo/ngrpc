package registry

import (
	"context"
)

type Registrar interface {
	Register(ctx context.Context, ins *ServiceInstance) error

	DeRegister(ctx context.Context, ins *ServiceInstance) error
}

type Discovery interface {
	ListService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)

	Watch(ctx context.Context, serviceName string) (Watcher, error)
}

type Watcher interface {
	Next() ([]*ServiceInstance, error)

	Stop() error
}
