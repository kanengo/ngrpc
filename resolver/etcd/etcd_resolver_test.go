package etcd

import (
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestEtcdResolver(t *testing.T) {
	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:        []string{"http://127.0.0.1:2379"},
		AutoSyncInterval: 0,
		DialTimeout:      time.Second * 3,
	})
	if err != nil {
		t.Error(err)
		return
	}
	builder := NewBuilder(etcdCli)
	builder.Scheme()
	t.Run("Build", func(t *testing.T) {

	})
}
