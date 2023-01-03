package discovery

import (
	"context"
	"errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

var EtcdService = &etcdClient{}

type WatchInfo struct {
	Type  string
	Key   string
	Value string
}
type KeyEventFun func(ctx context.Context, keyInfo WatchInfo)

type etcdClient struct {
	cli *clientv3.Client //etcd客户端
}

// InitEtcdService 初始化etcd
func InitEtcdService(etcdAddrs []string, dialTimeout int) error {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdAddrs,
		DialTimeout: time.Duration(dialTimeout) * time.Second,
	})
	if err != nil {
		return err
	}
	EtcdService.cli = cli
	return nil
}

//// EtcdService 返回一个etcd客户端
//func EtcdService() (*etcdClient, error) {
//	if etcdService.cli == nil {
//		return nil, errors.New("etcd 客户端没有初始化")
//	}
//	return etcdService, nil
//}

// GetKey 获取key
func (e *etcdClient) GetKey(key string) (string, error) {
	var err error
	var res *clientv3.GetResponse
	res, err = e.cli.Get(context.TODO(), key)
	if err != nil {
		return "", errors.New("etcd 获取key失败" + err.Error())
	}
	for _, ev := range res.Kvs {
		return string(ev.Value), nil
	}
	return "", nil
}

// WatchKey 监听key 变动，返回一个上下文cancel，cancel()后取消监听
func (e *etcdClient) WatchKey(ctx context.Context, key string, updateFun, delFun KeyEventFun) context.CancelFunc {
	var watch clientv3.WatchChan
	ctx, cancel := context.WithCancel(ctx)
	watch = e.cli.Watch(ctx, key) //当key put或者delete时，watch管道会收到响应的事件

	go func() {
		for {
			res, ok := <-watch
			if ok {
				for _, event := range res.Events {
					w := WatchInfo{
						Type:  event.Type.String(),
						Key:   string(event.Kv.Key),
						Value: string(event.Kv.Value),
					}
					switch event.Type.String() {
					case "PUT":
						updateFun(ctx, w)
					case "DELETE":
						delFun(ctx, w)
					}
				}
			} else {
				break
			}
		}
	}()

	return cancel
}

// WatchKey 监听 key1 到 key2 的变动，返回一个上下文cancel，cancel()后取消监听
func (e *etcdClient) WatchPrefix(ctx context.Context, prefix string, updateFun, delFun KeyEventFun) context.CancelFunc {
	var watch clientv3.WatchChan
	ctx, cancel := context.WithCancel(ctx)
	// watch --prefix=true prefix
	watch = e.cli.Watch(ctx, prefix, clientv3.WithPrefix()) //当key put或者delete时，watch管道会收到响应的事件

	go func() {
		for {
			res, ok := <-watch
			if ok {
				for _, event := range res.Events {
					w := WatchInfo{
						Type:  event.Type.String(),
						Key:   string(event.Kv.Key),
						Value: string(event.Kv.Value),
					}
					switch event.Type.String() {
					case "PUT":
						updateFun(ctx, w)
					case "DELETE":
						delFun(ctx, w)
					}
				}
			} else {
				break
			}
		}
	}()

	return cancel
}
