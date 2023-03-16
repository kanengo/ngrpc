package etcd

import (
	"context"
	"testing"
	"time"

	"github.com/kanengo/ngrpc/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

func TestRegister(t *testing.T) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second, DialOptions: []grpc.DialOption{grpc.WithBlock()},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	s := &registry.ServiceInstance{
		ID:   "317",
		Name: "kanengo",
	}

	r := New(client)

	w, err := r.Watch(ctx, s.Name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = w.Stop()
	}()
	go func() {
		for {
			res, err1 := w.Next()
			if err1 != nil {
				return
			}
			t.Logf("watch: %d", len(res))
			for _, r := range res {
				t.Logf("next: %+v", r)
			}
		}
	}()
	time.Sleep(time.Second)

	if err1 := r.Register(ctx, s); err1 != nil {
		t.Fatal(err1)
	}
	time.Sleep(time.Second)

	res, err := r.ListService(ctx, s.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 && res[0].Name != s.Name {
		t.Errorf("not expected: %+v", res)
	}

	if err1 := r.Deregister(ctx, s); err1 != nil {
		t.Fatal(err1)
	}
	time.Sleep(time.Second)

	res, err = r.ListService(ctx, s.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("not expected empty")
	}
}
