package geecache

import (
	"fmt"
	"log"
	"sync"
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
	name      string //命名空间名
	getter    Getter //缓存未命中时执行的回调，用户根据数据源编写回调逻辑
	mainCache cache  //管理缓存的实例
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// NewGroup 构造 Group
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

// Get 根据key获取缓存中对应的value
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok { //命中缓存
		log.Println("[GeeCache] hit")
		return v, nil
	}

	return g.load(key) //没有命中缓存则尝试载入缓存
}

// load 加载缓存
func (g *Group) load(key string) (value ByteView, err error) {
	return g.getLocally(key)
}

// getLocally 通过 Group.getter 加载缓存并放入缓存实例中管理
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err

	}
	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

// populateCache 将kv放入缓存实例
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
