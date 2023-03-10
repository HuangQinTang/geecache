package geecache

import (
	"fmt"
	pb "geecache/geecachepb"
	"geecache/singleflight"
	"log"
	"sync"
)

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// Getter 获取key对应的数据，Get方法应封装key数据源的获取逻辑
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 类型，实现 Getter 接口
// 该类型是个方法，根据key获取value值时调用该方法，在方法内用户可自定义key的数据源
type GetterFunc func(key string) ([]byte, error)

// Get 实现 Getter 接口
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// Group 缓存命名空间，可以为不同数据创建不同的命名空间
type Group struct {
	name      string              //命名空间名
	getter    Getter              //缓存未命中时执行的回调，用户根据数据源编写回调逻辑
	mainCache cache               //管理缓存的实例
	peers     PeerPicker          //节点选择器，选择key在哈希环中应该映射的节点
	loader    *singleflight.Group //防止缓存穿透、击穿
}

// NewGroup 构建命名空间，每个命名空间管理一个缓存实例
// @param name 命名空间名
// @param cacheBytes 该命名空间缓存上限，单位字节，超过采用lru策略淘汰
// @param getter 获取数据的回调方法
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}

	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

// GetGroup 返回对应name的命名空间，没有返回nil
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// RegisterPeers registers a PeerPicker for choosing remote peer
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// Get 根据key获取缓存中对应的value
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok { //命中本地缓存
		log.Println("[GeeCache] hit")
		return v, nil
	}

	return g.load(key) //没有本地缓存则尝试载入缓存
}

// load 加载缓存
func (g *Group) load(key string) (value ByteView, err error) {
	if g.peers != nil {
		// PickPeer 会根据传入的key hash计算选择拿到对应远程节点http客户端
		if peer, ok := g.peers.PickPeer(key); ok {
			if value, err = g.getFromPeer(peer, key); err == nil {
				return value, nil
			}
			log.Println("[GeeCache] Failed to get from peer", err)
		}
	}
	return g.getLocally(key)
}

// getFromPeer 用传入的http客户端，获取key
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}

// getLocally 通过 Group.getter 回调加载缓存并放入缓存实例中管理
func (g *Group) getLocally(key string) (ByteView, error) {
	// 确保同个key同时只有1个请求，防止同时大量缓存穿透、击穿
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		bytes, err := g.getter.Get(key)
		if err != nil {
			return ByteView{}, err
		}
		value := ByteView{b: cloneBytes(bytes)}
		g.populateCache(key, value)
		return value, nil
	})
	if err != nil {
		return ByteView{}, err
	}
	return viewi.(ByteView), nil
}

// populateCache 将kv放入缓存实例
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
